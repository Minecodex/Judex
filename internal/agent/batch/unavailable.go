package batch

import (
	"context"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
)

func (s *Service) MarkUnavailable(ctx context.Context, project, batch uuid.UUID) error {
	return s.Pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockProjectForUpdate(ctx, project.String()); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE tool_calls SET state='unknown' WHERE run_id IN(SELECT id FROM agent_runs WHERE project_id=$1 AND batch_id=$2) AND state IN ('prepared','running')`, project, batch); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE agent_sessions SET active_run_id=NULL WHERE project_id=$1 AND active_run_id IN(SELECT id FROM agent_runs WHERE batch_id=$2)`, project, batch); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE agent_runs SET state=CASE WHEN state='queued' THEN 'failed' ELSE 'waiting_human' END,
    budget_snapshot=budget_snapshot||jsonb_build_object('reason','model unavailable; interrupted work requires review'),lease_token=NULL,version=version+1,updated_at=now()
    WHERE project_id=$1 AND batch_id=$2 AND state NOT IN ('succeeded','failed','cancelled','waiting_human')`, project, batch); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE discussion_batches SET state='failed',version=version+1 WHERE project_id=$1 AND id=$2 AND state IN ('queued','running')`, project, batch)
		return err
	})
}
