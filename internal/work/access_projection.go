package work

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"time"
)

func (s *Service) taskAccessPage(ctx context.Context, user, project uuid.UUID, tasks []Task) error {
	if len(tasks) == 0 {
		return nil
	}
	role, err := memberTx(ctx, s.pool, project, user)
	if err != nil {
		return err
	}
	manager := role == "owner" || role == "manager"
	ids := []uuid.UUID{}
	indexes := map[uuid.UUID]int{}
	for i, t := range tasks {
		ids = append(ids, t.ID)
		indexes[t.ID] = i
	}
	rows, err := s.pool.Query(ctx, `SELECT t.id,t.created_by,t.discarded_at,t.status,EXISTS(SELECT 1 FROM plans p JOIN agent_identities i ON i.project_id=p.project_id AND i.id=p.owner_identity_id JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version AND b.valid_until IS NULL WHERE p.project_id=t.project_id AND p.id=t.plan_id AND i.status='active' AND b.user_id=$3),CASE WHEN x.id IS NULL THEN NULL ELSE jsonb_build_object('id',x.id,'reason',x.reason,'previousStatus',x.previous_status,'actorUserId',x.actor_user_id,'createdAt',x.created_at,'waivers',x.waivers_json) END FROM tasks t LEFT JOIN task_execution_exceptions x ON x.project_id=t.project_id AND x.task_id=t.id AND x.restored_at IS NULL WHERE t.project_id=$1 AND t.id=ANY($2)`, project, ids, user)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var author *uuid.UUID
		var owns bool
		var status string
		var discarded *time.Time
		var raw []byte
		if err = rows.Scan(&id, &author, &discarded, &status, &owns, &raw); err != nil {
			return err
		}
		i := indexes[id]
		tasks[i].DiscardedAt = discarded
		if len(raw) > 0 {
			tasks[i].ExecutionException = &ExecutionException{}
			if err = json.Unmarshal(raw, tasks[i].ExecutionException); err != nil {
				return err
			}
		}
		live := tasks[i].DiscardedAt == nil
		draft := live && status == "draft" && (manager || owns || author != nil && *author == user)
		tasks[i].Capabilities = WorkCapabilities{EditDraft: draft, DiscardDraft: draft, ProposeChange: live && status != "accepted" && status != "cancelled", Skip: live && manager && tasks[i].ExecutionException == nil && (status == "ready" || status == "working" || status == "delivered" || status == "rework"), Restore: manager && tasks[i].ExecutionException != nil}
	}
	return rows.Err()
}
