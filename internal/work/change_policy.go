package work

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/kakj-go/Judex/internal/collaboration"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

func orderChanges(changes []Change) ([]Change, error) {
	refs := map[string]bool{}
	for _, c := range changes {
		if c.ClientRef != "" {
			if refs[c.ClientRef] {
				return nil, apierrors.Fields("clientRef", "duplicate")
			}
			refs[c.ClientRef] = true
		}
	}
	done := map[string]bool{}
	remaining := append([]Change(nil), changes...)
	out := []Change{}
	for len(remaining) > 0 {
		next := []Change{}
		for _, c := range remaining {
			deps := append([]string(nil), c.DependsOn...)
			for _, key := range []string{"planId", "parentTaskId"} {
				if raw, ok := c.Fields[key].(string); ok && refs[raw] {
					deps = append(deps, raw)
				}
			}
			ready := true
			for _, d := range deps {
				if !refs[d] {
					return nil, apierrors.New(apierrors.InvalidReference, "unknown change dependency")
				}
				if !done[d] {
					ready = false
				}
			}
			if !ready {
				next = append(next, c)
				continue
			}
			out = append(out, c)
			if c.ClientRef != "" {
				done[c.ClientRef] = true
			}
		}
		if len(next) == len(remaining) {
			return nil, apierrors.New(apierrors.DependencyChanged, "change dependency cycle")
		}
		remaining = next
	}
	return out, nil
}

func identityExists(ctx context.Context, tx pgx.Tx, project uuid.UUID, raw any) error {
	text, ok := raw.(string)
	if !ok {
		return apierrors.Fields("identityId", "required")
	}
	id, err := uuid.Parse(text)
	if err != nil {
		return apierrors.Fields("identityId", "uuid")
	}
	var valid bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM agent_identities i
 JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version AND b.valid_until IS NULL
 JOIN project_members m ON m.project_id=i.project_id AND m.user_id=b.user_id AND m.state='active'
 WHERE i.project_id=$1 AND i.id=$2 AND i.kind='position' AND i.status='active')`, project, id).Scan(&valid)
	if err != nil {
		return err
	}
	if !valid {
		return apierrors.New(apierrors.InvalidReference, "active bound identity required")
	}
	return nil
}
func participantFields(fields map[string]any) ([]uuid.UUID, error) {
	raw, _ := json.Marshal(fields["participantIdentityIds"])
	var ids []uuid.UUID
	if err := json.Unmarshal(raw, &ids); err != nil {
		return nil, apierrors.Fields("participantIdentityIds", "uuid list")
	}
	if len(ids) == 0 {
		return nil, apierrors.Fields("participantIdentityIds", "required")
	}
	seen := map[uuid.UUID]bool{}
	for _, id := range ids {
		if seen[id] {
			return nil, apierrors.Fields("participantIdentityIds", "duplicate")
		}
		seen[id] = true
	}
	return ids, nil
}
func validateNewWork(ctx context.Context, tx pgx.Tx, project uuid.UUID, c Change, created map[string]string) error {
	allowed := map[string]bool{"title": true, "acceptanceCriteria": true, "workflowId": true}
	if c.Operation == "create_plan" {
		allowed["goal"] = true
		allowed["ownerIdentityId"] = true
		allowed["sourceTopicId"] = true
		allowed["forkAfterSeq"] = true
	} else {
		for _, key := range []string{"expectedOutput", "reviewerIdentityId", "participantIdentityIds", "planId", "parentTaskId", "nodeId", "kind", "requirements", "bugDetails"} {
			allowed[key] = true
		}
	}
	for key := range c.Fields {
		if !allowed[key] {
			return apierrors.Fields("fields."+key, "unsupported")
		}
	}
	title := strOrDefault(c.Fields, "title")
	if strings.TrimSpace(title) == "" || utf8.RuneCountInString(title) > 200 {
		return apierrors.Fields("title", "length")
	}
	if c.Operation == "create_plan" {
		if _, err := ForkOriginFromFields(ctx, tx, project, c.Fields); err != nil {
			return err
		}
		return identityExists(ctx, tx, project, c.Fields["ownerIdentityId"])
	}
	if err := identityExists(ctx, tx, project, c.Fields["reviewerIdentityId"]); err != nil {
		return err
	}
	ids, err := participantFields(c.Fields)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err = identityExists(ctx, tx, project, id.String()); err != nil {
			return err
		}
	}
	resolved := c
	resolved.Fields = map[string]any{}
	for key, value := range c.Fields {
		resolved.Fields[key] = value
	}
	if raw := strOrDefault(c.Fields, "planId"); raw != "" {
		if id, ok := created[raw]; ok {
			resolved.Fields["planId"] = id
		}
	}
	refs, err := ChangeWorkflowConstraints(ctx, tx, project, []Change{resolved})
	if err != nil {
		return err
	}
	if err = validateWorkflowParticipants(ctx, tx, project, refs, ids); err != nil {
		return err
	}
	if raw := strOrDefault(c.Fields, "planId"); raw != "" {
		var valid bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM plans WHERE project_id=$1 AND id=$2 AND status='active')`, project, resolveTarget(raw, created)).Scan(&valid); err != nil {
			return err
		}
		if !valid {
			return apierrors.New(apierrors.InvalidReference, "active plan in project required")
		}
	}
	return nil
}

func applyExisting(ctx context.Context, tx pgx.Tx, project, actor, target uuid.UUID, c Change, created map[string]string) error {
	table := map[string]string{"plan": "plans", "task": "tasks"}[c.TargetType]
	if table == "" || target == uuid.Nil {
		return apierrors.Fields("target", "invalid")
	}
	var version int64
	var state string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT version,status FROM %s WHERE project_id=$1 AND id=$2 FOR UPDATE`, table), project, target).Scan(&version, &state); err != nil {
		return apierrors.New(apierrors.InvalidReference, "target not found")
	}
	if c.ExpectedVersion < 1 || version != c.ExpectedVersion {
		return apierrors.New(apierrors.ReviewStale, "target version changed")
	}
	if state == "accepted" || state == "cancelled" {
		return apierrors.New(apierrors.InvalidTransition, "terminal work must be reopened before changing it")
	}
	switch c.Operation {
	case "cancel_task", "cancel_plan":
		if strings.TrimSpace(strOrDefault(c.Fields, "reason")) == "" {
			return apierrors.Fields("reason", "required")
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s SET status='cancelled' WHERE id=$1`, table), target); err != nil {
			return err
		}
	case "activate_object":
		if state != "draft" {
			return apierrors.New(apierrors.InvalidTransition, "only drafts activate")
		}
		var owner *uuid.UUID
		col := "owner_identity_id"
		next := "active"
		if c.TargetType == "task" {
			col = "reviewer_identity_id"
			next = "ready"
		}
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT %s FROM %s WHERE id=$1`, col, table), target).Scan(&owner); err != nil {
			return err
		}
		if owner == nil {
			return apierrors.New(apierrors.RequirementUnmet, "responsible identity required")
		}
		if err := identityExists(ctx, tx, project, owner.String()); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s SET status=$2 WHERE id=$1`, table), target, next); err != nil {
			return err
		}
	case "update_scope":
		allowed := map[string]string{"title": "title", "acceptanceCriteria": "acceptance_criteria"}
		if c.TargetType == "task" {
			allowed["expectedOutput"] = "expected_output"
		} else {
			allowed["goal"] = "goal"
		}
		if len(c.Fields) == 0 {
			return apierrors.Fields("fields", "required")
		}
		for key, value := range c.Fields {
			column, ok := allowed[key]
			if !ok {
				return apierrors.Fields("fields."+key, "unsupported")
			}
			text, ok := value.(string)
			if !ok {
				return apierrors.Fields(key, "string")
			}
			if key == "title" && (strings.TrimSpace(text) == "" || utf8.RuneCountInString(text) > 200) {
				return apierrors.Fields(key, "length")
			}
			if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s SET %s=$2 WHERE id=$1`, table, column), target, text); err != nil {
				return err
			}
		}
	case "set_assignment":
		if len(c.Fields) == 0 {
			return apierrors.Fields("fields", "required")
		}
		for key := range c.Fields {
			if (c.TargetType == "plan" && key != "ownerIdentityId") || (c.TargetType == "task" && key != "reviewerIdentityId" && key != "participantIdentityIds") {
				return apierrors.Fields("fields."+key, "unsupported")
			}
		}
		key := "ownerIdentityId"
		col := "owner_identity_id"
		if c.TargetType == "task" {
			key = "reviewerIdentityId"
			col = "reviewer_identity_id"
		}
		if v, ok := c.Fields[key]; ok {
			if err := identityExists(ctx, tx, project, v); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s SET %s=$2 WHERE id=$1`, table, col), target, v); err != nil {
				return err
			}
		}
		if _, ok := c.Fields["participantIdentityIds"]; ok {
			if c.TargetType != "task" {
				return apierrors.Fields("participantIdentityIds", "task only")
			}
			ids, err := participantFields(c.Fields)
			if err != nil {
				return err
			}
			for _, id := range ids {
				if err = identityExists(ctx, tx, project, id.String()); err != nil {
					return err
				}
			}
			if _, err = tx.Exec(ctx, `DELETE FROM task_participants WHERE project_id=$1 AND task_id=$2`, project, target); err != nil {
				return err
			}
			for _, id := range ids {
				if _, err = tx.Exec(ctx, `INSERT INTO task_participants(project_id,task_id,identity_id) VALUES($1,$2,$3)`, project, target, id); err != nil {
					return err
				}
			}
		}
	case "set_requirements":
		if value, ok := c.Fields["requirements"]; !ok || value == nil {
			return apierrors.Fields("requirements", "required")
		}
		if c.TargetType != "task" {
			return apierrors.Fields("targetType", "task required")
		}
		raw, _ := json.Marshal(c.Fields["requirements"])
		var reqs []Requirement
		if err := json.Unmarshal(raw, &reqs); err != nil {
			return apierrors.Fields("requirements", "invalid")
		}
		if err := replaceRequirements(ctx, tx, project, target, reqs); err != nil {
			return err
		}
	case "link_material":
		id, err := uuid.Parse(strOrDefault(c.Fields, "materialVersionId"))
		if err != nil {
			return apierrors.Fields("materialVersionId", "uuid")
		}
		var ok bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM material_versions WHERE id=$1 AND project_id=$2 AND state='ready')`, id, project).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return apierrors.New(apierrors.RequirementUnmet, "material not ready")
		}
		var actual any
		if actor != uuid.Nil {
			actual = actor
		}
		if _, err = tx.Exec(ctx, `INSERT INTO material_links(project_id,id,version_id,object_type,object_id,purpose,confirmed_by,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,now())`, project, uuid.New(), id, c.TargetType, target, strOrDefault2(c.Fields, "purpose", "reference"), actual); err != nil {
			return err
		}
	case "link_topic":
		id, err := uuid.Parse(strOrDefault(c.Fields, "topicId"))
		if err != nil {
			return apierrors.Fields("topicId", "uuid")
		}
		var ok bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM topics WHERE id=$1 AND project_id=$2)`, id, project).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return apierrors.New(apierrors.InvalidReference, "topic not found")
		}
		if err = collaboration.ChangeTopicLinks(ctx, tx, project, id, nil, []collaboration.Link{{ObjectType: c.TargetType, ObjectID: target}}, false, time.Now().UTC()); err != nil {
			return err
		}
	case "reference_task":
		if c.TargetType != "plan" {
			return apierrors.Fields("targetType", "plan required")
		}
		id, err := uuid.Parse(strOrDefault(c.Fields, "taskId"))
		if err != nil {
			return apierrors.Fields("taskId", "uuid")
		}
		var valid bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tasks WHERE id=$1 AND project_id=$2)`, id, project).Scan(&valid); err != nil {
			return err
		}
		if !valid {
			return apierrors.New(apierrors.InvalidReference, "referenced task outside project")
		}
		if _, err = tx.Exec(ctx, `INSERT INTO plan_task_references(project_id,plan_id,task_id) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, project, target, id); err != nil {
			return err
		}
	default:
		return apierrors.Fields("operation", "unsupported")
	}
	extra := ""
	_, outputChanged := c.Fields["expectedOutput"]
	_, criteriaChanged := c.Fields["acceptanceCriteria"]
	if c.TargetType == "task" && (c.Operation == "set_assignment" || c.Operation == "set_requirements" || (c.Operation == "update_scope" && (outputChanged || criteriaChanged))) {
		extra = ",agreement_version=agreement_version+1"
		if state == "delivered" {
			extra += ",status='rework'"
		}
	}
	_, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s SET version=version+1,updated_at=now()%s WHERE id=$1`, table, extra), target)
	return err
}

func replaceRequirements(ctx context.Context, tx pgx.Tx, project, task uuid.UUID, reqs []Requirement) error {
	if err := validateRequirementReferences(ctx, tx, project, reqs); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM task_requirements WHERE project_id=$1 AND task_id=$2`, project, task); err != nil {
		return err
	}
	for _, r := range reqs {
		if _, err := tx.Exec(ctx, `INSERT INTO task_requirements(project_id,id,task_id,phase,kind,target_id,material_version_id,hard,label) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, project, uuid.New(), task, r.Phase, r.Kind, r.TargetID, r.MaterialVersionID, r.Hard, r.Label); err != nil {
			return err
		}
	}
	return checkRequirementCycle(ctx, tx, project, nil)
}

func validateRequirementReferences(ctx context.Context, tx pgx.Tx, project uuid.UUID, reqs []Requirement) error {
	for _, r := range reqs {
		if r.Phase != "start" && r.Phase != "accept" && r.Phase != "both" {
			return apierrors.Fields("requirements.phase", "enum")
		}
		table := map[string]string{"task_acceptance": "tasks", "handoff_receipt": "handoff_sources", "material_ready": "material_versions"}[r.Kind]
		if table == "" {
			return apierrors.Fields("requirements.kind", "enum")
		}
		var exists bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s WHERE project_id=$1 AND id=$2)`, table), project, r.TargetID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return apierrors.New(apierrors.InvalidReference, "requirement target outside project or missing")
		}
	}
	return nil
}
