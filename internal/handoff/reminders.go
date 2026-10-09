package handoff

import (
	"context"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/events"
)

func (s *Service) Remind(ctx context.Context, user, project, handoff uuid.UUID) (int64, error) {
	var count int64
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, project.String()); err != nil {
			return err
		}
		if err := memberTx(ctx, tx, project, user); err != nil {
			return err
		}
		var receiver uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT receiver_identity_id FROM handoffs WHERE id=$1 AND project_id=$2`, handoff, project).Scan(&receiver); err != nil {
			return apierrors.New(apierrors.NotFound, "handoff not found")
		}
		event, err := events.AppendProjectEvent(ctx, tx, project, "handoff.changed", "handoff", handoff.String(), nil, map[string]any{"change": "reminded"}, s.now())
		if err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `INSERT INTO notifications(project_id,id,user_id,type,object_ref,event_id,created_at)
   SELECT DISTINCT $1,gen_random_uuid(),b.user_id,'handoff.reminder',jsonb_build_object('type','handoff','id',$2::text),$3,now()
   FROM agent_identities i JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version
   JOIN project_members m ON m.project_id=i.project_id AND m.user_id=b.user_id AND m.state='active'
   WHERE i.project_id=$1 AND i.id=$4`, project, handoff, event.EventID, receiver)
		if err != nil {
			return err
		}
		count = tag.RowsAffected()
		return nil
	})
	return count, err
}
