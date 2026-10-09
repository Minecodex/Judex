package work

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/events"
	"strings"
)

type DraftUpdate struct {
	ExpectedVersion int64          `json:"expectedVersion"`
	Fields          map[string]any `json:"fields"`
}

func (s *Service) UpdateDraft(ctx context.Context, user, project, target uuid.UUID, kind string, in DraftUpdate) error {
	return s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, project.String()); err != nil {
			return err
		}
		table := map[string]string{"task": "tasks", "plan": "plans"}[kind]
		if table == "" {
			return apierrors.Fields("kind", "enum")
		}
		var version int64
		var status string
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT version,status FROM %s WHERE project_id=$1 AND id=$2 FOR UPDATE`, table), project, target).Scan(&version, &status); err != nil {
			return apierrors.New(apierrors.NotFound, "draft not found")
		}
		access, err := workAccess(ctx, tx, project, user, target, kind)
		if err != nil {
			return err
		}
		if !access.Capabilities.EditDraft {
			return apierrors.New(apierrors.Forbidden, "draft edit requires author, plan owner or manager")
		}
		if version != in.ExpectedVersion {
			return apierrors.New(apierrors.VersionConflict, "draft changed")
		}
		scope, assignment := map[string]any{}, map[string]any{}
		var requirements any
		hasReq := false
		for key, value := range in.Fields {
			switch key {
			case "title", "acceptanceCriteria":
				scope[key] = value
			case "goal":
				if kind != "plan" {
					return apierrors.Fields("fields.goal", "plan only")
				}
				scope[key] = value
			case "expectedOutput":
				if kind != "task" {
					return apierrors.Fields("fields.expectedOutput", "task only")
				}
				scope[key] = value
			case "ownerIdentityId":
				if kind != "plan" {
					return apierrors.Fields("fields.ownerIdentityId", "plan only")
				}
				assignment[key] = value
			case "reviewerIdentityId", "participantIdentityIds":
				if kind != "task" {
					return apierrors.Fields("fields."+key, "task only")
				}
				assignment[key] = value
			case "requirements":
				if kind != "task" {
					return apierrors.Fields("fields.requirements", "task only")
				}
				requirements = value
				hasReq = true
			default:
				return apierrors.Fields("fields."+key, "unsupported")
			}
		}
		if len(scope)+len(assignment) == 0 && !hasReq {
			return apierrors.Fields("fields", "required")
		}
		next := version
		for _, op := range []struct {
			operation string
			fields    map[string]any
		}{{"update_scope", scope}, {"set_assignment", assignment}} {
			if len(op.fields) == 0 {
				continue
			}
			if err = applyExisting(ctx, tx, project, user, target, Change{Operation: op.operation, TargetType: kind, ExpectedVersion: next, Fields: op.fields}, nil); err != nil {
				return err
			}
			next++
		}
		if hasReq {
			if err = applyExisting(ctx, tx, project, user, target, Change{Operation: "set_requirements", TargetType: kind, ExpectedVersion: next, Fields: map[string]any{"requirements": requirements}}, nil); err != nil {
				return err
			}
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s SET version=$3,updated_at=$4 WHERE project_id=$1 AND id=$2`, table), project, target, version+1, s.now()); err != nil {
			return err
		}
		if err = s.withdrawDraftReviews(ctx, tx, project, []uuid.UUID{target}); err != nil {
			return err
		}
		return s.recordWorkChange(ctx, tx, project, user, target, kind, "draft.updated", "草稿安排已修订，旧审阅已撤回", map[string]any{"previousVersion": version, "fields": in.Fields})
	})
}
func (s *Service) withdrawDraftReviews(ctx context.Context, tx pgx.Tx, project uuid.UUID, ids []uuid.UUID) error {
	return withdrawDraftReviews(ctx, tx, project, ids, uuid.Nil, s.now())
}
func (s *Service) recordWorkChange(ctx context.Context, tx pgx.Tx, project, user, target uuid.UUID, kind, operation, text string, details map[string]any) error {
	raw, err := json.Marshal(details)
	if err != nil {
		return err
	}
	source := string(audit.ContextSource(ctx))
	if source != "cli" {
		source = "web"
	}
	if _, err = tx.Exec(ctx, `INSERT INTO work_change_records(project_id,id,object_type,object_id,operation,actor_user_id,text,created_at,details_json,source) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, project, uuid.New(), kind, target, operation, user, text, s.now(), raw, source); err != nil {
		return err
	}
	_, err = events.AppendProjectEvent(ctx, tx, project, kind+".changed", kind, target.String(), nil, map[string]any{"change": operation}, s.now())
	if err != nil {
		return err
	}
	if kind == "task" {
		_, err = events.AppendProjectEvent(ctx, tx, project, "task.activity.changed", "task", target.String(), nil, map[string]any{"change": operation}, s.now())
	}
	return err
}
func (s *Service) draftReferences(ctx context.Context, tx pgx.Tx, project uuid.UUID, ids []uuid.UUID, plan *uuid.UUID) ([]Blocker, error) {
	return draftReferenceQuery(ctx, tx, project, ids, plan)
}
func draftReferenceQuery(ctx context.Context, tx dbQuery, project uuid.UUID, ids []uuid.UUID, plan *uuid.UUID) ([]Blocker, error) {
	out := []Blocker{}
	rows, err := tx.Query(ctx, `SELECT t.id,t.title,'task' FROM tasks t WHERE t.project_id=$1 AND t.discarded_at IS NULL AND NOT(t.id=ANY($2)) AND (t.parent_task_id=ANY($2) OR EXISTS(SELECT 1 FROM task_requirements r WHERE r.task_id=t.id AND r.project_id=$1 AND r.kind='task_acceptance' AND r.target_id=ANY($2))) UNION SELECT p.id,p.title,'plan' FROM plans p JOIN plan_task_references r ON r.project_id=p.project_id AND r.plan_id=p.id WHERE p.project_id=$1 AND p.discarded_at IS NULL AND r.task_id=ANY($2) AND ($3::uuid IS NULL OR p.id<>$3) UNION SELECT d.id,d.name,'workflow' FROM workflow_definitions d JOIN workflow_versions v ON v.project_id=d.project_id AND v.id=d.published_version_id CROSS JOIN LATERAL jsonb_array_elements(COALESCE(NULLIF(v.hard_rules_json,'null'::jsonb),'[]'::jsonb)) r WHERE d.project_id=$1 AND r->>'kind'='task_acceptance' AND r->>'targetId' IN(SELECT x::text FROM unnest($2::uuid[]) x) UNION SELECT t.id,t.title,'task' FROM tasks t WHERE t.project_id=$1 AND t.id=ANY($2) AND (t.status<>'draft' OR t.latest_report_id IS NOT NULL OR t.latest_acceptance_id IS NOT NULL OR EXISTS(SELECT 1 FROM handoff_sources h WHERE h.project_id=$1 AND h.source_task_id=t.id))`, project, ids, plan)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var title, kind string
		if err = rows.Scan(&id, &title, &kind); err != nil {
			return nil, err
		}
		out = append(out, Blocker{ObjectID: id.String(), ObjectType: kind, Phase: "both", Reason: "存在有效引用或正式依据：" + strings.TrimSpace(title)})
	}
	return out, rows.Err()
}
