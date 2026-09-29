package work

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"time"
)

type Change struct {
	Operation       string         `json:"operation"`
	TargetType      string         `json:"targetType"`
	TargetID        string         `json:"targetId,omitempty"`
	ClientRef       string         `json:"clientRef,omitempty"`
	ExpectedVersion int64          `json:"expectedVersion,omitempty"`
	Fields          map[string]any `json:"fields,omitempty"`
	DependsOn       []string       `json:"dependsOn,omitempty"`
}

// ApplyChanges is the only formal work mutation entry used by approval.
// The caller owns the project lock and transaction.
func ApplyChanges(ctx context.Context, tx pgx.Tx, projectID, actor uuid.UUID, changes []Change) (map[string]string, error) {
	created := map[string]string{}
	now := time.Now().UTC()
	ordered, err := orderChanges(changes)
	if err != nil {
		return nil, err
	}
	for _, change := range ordered {
		if change.Operation == "create_plan" || change.Operation == "create_task" {
			if err := validateNewWork(ctx, tx, projectID, change, created); err != nil {
				return nil, err
			}
		}
		switch change.Operation {
		case "create_plan":
			id := uuid.New()
			title, _ := change.Fields["title"].(string)
			if _, err := tx.Exec(ctx, `
				INSERT INTO plans (project_id, id, title, goal, acceptance_criteria, owner_identity_id, workflow_id, status, created_at, updated_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,'active',$8,$8)`,
				projectID, id, title,
				strOrDefault(change.Fields, "goal"), strOrDefault(change.Fields, "acceptanceCriteria"),
				optionalUUIDField(change.Fields, "ownerIdentityId"), optionalUUIDField(change.Fields, "workflowId"), now); err != nil {
				return nil, apierrors.New(apierrors.Internal, "plan apply failed").Wrap(err)
			}
			if change.ClientRef != "" {
				created[change.ClientRef] = id.String()
			}
		case "create_task":
			id := uuid.New()
			planRaw, _ := change.Fields["planId"].(string)
			planID := optionalUUIDField(change.Fields, "planId")
			if ref, ok := planRefFromCreated(created, planRaw); ok {
				if parsed, err := uuid.Parse(ref); err == nil {
					planID = &parsed
				}
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO tasks (project_id, id, plan_id, title, expected_output, acceptance_criteria,
					kind, reviewer_identity_id, workflow_id, node_id, status, created_at, updated_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'ready',$11,$11)`,
				projectID, id, planID,
				strOrDefault(change.Fields, "title"), strOrDefault(change.Fields, "expectedOutput"),
				strOrDefault(change.Fields, "acceptanceCriteria"), strOrDefault2(change.Fields, "kind", "task"),
				optionalUUIDField(change.Fields, "reviewerIdentityId"), optionalUUIDField(change.Fields, "workflowId"),
				nullableStringField(change.Fields, "nodeId"), now); err != nil {
				return nil, apierrors.New(apierrors.Internal, "task apply failed").Wrap(err)
			}
			if strOrDefault(change.Fields, "kind") == "bug" {
				var details BugDetails
				raw, _ := json.Marshal(change.Fields["bugDetails"])
				if err := json.Unmarshal(raw, &details); err != nil {
					return nil, apierrors.Fields("bugDetails", "invalid")
				}
				if err := insertBugDetails(ctx, tx, projectID, id, &details); err != nil {
					return nil, err
				}
			}
			if ids, ok := change.Fields["participantIdentityIds"].([]any); ok {
				for _, raw := range ids {
					if str, ok := raw.(string); ok {
						if pid, err := uuid.Parse(str); err == nil {
							if _, err := tx.Exec(ctx, `
								INSERT INTO task_participants (project_id, task_id, identity_id) VALUES ($1,$2,$3)`,
								projectID, id, pid); err != nil {
								return nil, apierrors.New(apierrors.Internal, "participant apply failed").Wrap(err)
							}
						}
					}
				}
			}
			if raw := strOrDefault(change.Fields, "parentTaskId"); raw != "" {
				parent := resolveTarget(raw, created)
				var valid bool
				if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tasks p JOIN tasks t ON t.project_id=p.project_id AND t.plan_id IS NOT DISTINCT FROM p.plan_id WHERE p.id=$1 AND t.id=$2 AND p.project_id=$3)`, parent, id, projectID).Scan(&valid); err != nil {
					return nil, err
				}
				if !valid {
					return nil, apierrors.New(apierrors.InvalidReference, "parent must share project and owning plan")
				}
				if _, err := tx.Exec(ctx, `UPDATE tasks SET parent_task_id=$2 WHERE id=$1`, id, parent); err != nil {
					return nil, err
				}
			}
			if raw, ok := change.Fields["requirements"]; ok {
				data, _ := json.Marshal(raw)
				var requirements []Requirement
				if err := json.Unmarshal(data, &requirements); err != nil {
					return nil, apierrors.Fields("requirements", "invalid")
				}
				if err := replaceRequirements(ctx, tx, projectID, id, requirements); err != nil {
					return nil, err
				}
			}
			if change.ClientRef != "" {
				created[change.ClientRef] = id.String()
			}
		case "reference_task", "activate_object", "cancel_task", "cancel_plan", "update_scope", "set_assignment", "set_requirements", "link_material", "link_topic":
			target := resolveTarget(change.TargetID, created)
			if err := applyExisting(ctx, tx, projectID, actor, target, change, created); err != nil {
				return nil, err
			}
		default:
			return nil, apierrors.Newf(apierrors.Validation, "unknown operation %s", change.Operation)
		}
	}
	return created, nil
}
func resolveTarget(targetID string, created map[string]string) uuid.UUID {
	if ref, ok := created[targetID]; ok {
		targetID = ref
	}
	id, _ := uuid.Parse(targetID)
	return id
}

func planRefFromCreated(created map[string]string, raw string) (string, bool) {
	if raw == "" {
		return "", false
	}
	ref, ok := created[raw]
	return ref, ok
}

func strOrDefault(fields map[string]any, key string) string {
	v, _ := fields[key].(string)
	return v
}

func strOrDefault2(fields map[string]any, key, fallback string) string {
	if v, ok := fields[key].(string); ok && v != "" {
		return v
	}
	return fallback
}

func optionalUUIDField(fields map[string]any, key string) any {
	raw, _ := fields[key].(string)
	if raw == "" {
		return nil
	}
	if id, err := uuid.Parse(raw); err == nil {
		return id
	}
	return nil
}

func nullableStringField(fields map[string]any, key string) any {
	raw, _ := fields[key].(string)
	if raw == "" {
		return nil
	}
	return raw
}
