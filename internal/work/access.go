package work

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"time"
)

type WorkCapabilities struct {
	EditDraft     bool `json:"editDraft"`
	DiscardDraft  bool `json:"discardDraft"`
	ProposeChange bool `json:"proposeChange"`
	Skip          bool `json:"skip"`
	Restore       bool `json:"restore"`
}
type ExecutionException struct {
	ID             uuid.UUID         `json:"id"`
	Reason         string            `json:"reason"`
	PreviousStatus string            `json:"previousStatus"`
	ActorUserID    uuid.UUID         `json:"actorUserId"`
	CreatedAt      time.Time         `json:"createdAt"`
	Waivers        []ExceptionWaiver `json:"waivers"`
}
type ExceptionWaiver struct {
	TaskID        uuid.UUID `json:"taskId"`
	RequirementID uuid.UUID `json:"requirementId"`
	Fingerprint   string    `json:"fingerprint"`
}
type accessMetadata struct {
	Author       *uuid.UUID
	DiscardedAt  *time.Time
	OwnsPlan     bool
	Capabilities WorkCapabilities
}

func workAccess(ctx context.Context, q dbQuery, project, user, target uuid.UUID, kind string) (accessMetadata, error) {
	var out accessMetadata
	role, err := memberTx(ctx, q, project, user)
	if err != nil {
		return out, err
	}
	sql := `SELECT p.created_by,p.discarded_at,p.status,EXISTS(SELECT 1 FROM agent_identities i JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version AND b.valid_until IS NULL WHERE i.project_id=p.project_id AND i.id=p.owner_identity_id AND i.status='active' AND b.user_id=$3) FROM plans p WHERE p.project_id=$1 AND p.id=$2`
	if kind == "task" {
		sql = `SELECT t.created_by,t.discarded_at,t.status,EXISTS(SELECT 1 FROM plans p JOIN agent_identities i ON i.project_id=p.project_id AND i.id=p.owner_identity_id JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version AND b.valid_until IS NULL WHERE p.project_id=t.project_id AND p.id=t.plan_id AND i.status='active' AND b.user_id=$3) FROM tasks t WHERE t.project_id=$1 AND t.id=$2`
	}
	var status string
	if err = q.QueryRow(ctx, sql, project, target, user).Scan(&out.Author, &out.DiscardedAt, &status, &out.OwnsPlan); err != nil {
		return out, apierrors.New(apierrors.NotFound, "work not found")
	}
	manager := role == "owner" || role == "manager"
	edit := manager || out.OwnsPlan || out.Author != nil && *out.Author == user
	out.Capabilities = WorkCapabilities{EditDraft: status == "draft" && out.DiscardedAt == nil && edit, DiscardDraft: status == "draft" && out.DiscardedAt == nil && edit, ProposeChange: status != "accepted" && status != "cancelled" && out.DiscardedAt == nil}
	if kind == "task" {
		active, err := activeException(ctx, q, project, target)
		if err != nil {
			return out, err
		}
		out.Capabilities.Skip = manager && active == nil && (status == "ready" || status == "working" || status == "delivered" || status == "rework") && out.DiscardedAt == nil
		out.Capabilities.Restore = manager && active != nil
	}
	return out, nil
}
func activeException(ctx context.Context, q dbQuery, project, task uuid.UUID) (*ExecutionException, error) {
	rows, err := q.Query(ctx, `SELECT id,reason,previous_status,actor_user_id,created_at,waivers_json FROM task_execution_exceptions WHERE project_id=$1 AND task_id=$2 AND restored_at IS NULL`, project, task)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	out := &ExecutionException{Waivers: []ExceptionWaiver{}}
	var raw []byte
	if err = rows.Scan(&out.ID, &out.Reason, &out.PreviousStatus, &out.ActorUserID, &out.CreatedAt, &raw); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(raw, &out.Waivers); err != nil {
		return nil, err
	}
	return out, nil
}
func ensureTaskExecutable(ctx context.Context, q dbQuery, project, task uuid.UUID) error {
	e, err := activeException(ctx, q, project, task)
	if err != nil {
		return err
	}
	if e != nil {
		return apierrors.New(apierrors.InvalidTransition, "task is temporarily skipped")
	}
	return nil
}
func (s *Service) decorateTasks(ctx context.Context, user, project uuid.UUID, tasks []Task) error {
	return s.taskAccessPage(ctx, user, project, tasks)
}
func (s *Service) decoratePlans(ctx context.Context, user, project uuid.UUID, plans []Plan) error {
	for i := range plans {
		m, err := workAccess(ctx, poolAsQuery{s.pool}, project, user, plans[i].ID, "plan")
		if err != nil {
			return err
		}
		plans[i].Capabilities = m.Capabilities
		plans[i].DiscardedAt = m.DiscardedAt
		var required, drafts, skipped int
		err = s.pool.QueryRow(ctx, `SELECT count(*) FILTER(WHERE t.status NOT IN('draft','cancelled') AND (x.id IS NULL OR t.plan_id IS DISTINCT FROM $2)),count(*) FILTER(WHERE t.status='draft'),count(*) FILTER(WHERE x.id IS NOT NULL) FROM tasks t LEFT JOIN task_execution_exceptions x ON x.project_id=t.project_id AND x.task_id=t.id AND x.restored_at IS NULL WHERE t.project_id=$1 AND t.discarded_at IS NULL AND (t.plan_id=$2 OR EXISTS(SELECT 1 FROM plan_task_references r WHERE r.project_id=$1 AND r.plan_id=$2 AND r.task_id=t.id))`, project, plans[i].ID).Scan(&required, &drafts, &skipped)
		if err != nil {
			return err
		}
		plans[i].TaskStats.Required = required
		plans[i].TaskStats.Draft = drafts
		plans[i].TaskStats.Skipped = skipped
	}
	return nil
}
