package batch

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/kakj-go/Judex/internal/job"
	"time"
)

// Sessions use short row leases, not pool connections held across network
// calls. Parents waiting for children cannot exhaust the database pool.
func claimSession(ctx context.Context, tx pgx.Tx, session, run uuid.UUID) error {
	var active *uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT active_run_id FROM agent_sessions WHERE id=$1 FOR UPDATE`, session).Scan(&active); err != nil {
		return err
	}
	if active != nil && *active != run {
		var state string
		if err := tx.QueryRow(ctx, `SELECT state FROM agent_runs WHERE id=$1`, active).Scan(&state); err != nil {
			return err
		}
		if state == "queued" || state == "running" || state == "provisioning" || state == "waiting_children" {
			return job.ErrBusy
		}
	}
	_, err := tx.Exec(ctx, `UPDATE agent_sessions SET active_run_id=$2,lease_epoch=lease_epoch+1 WHERE id=$1`, session, run)
	return err
}
func (e *Executor) releaseSession(session, run uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, _ = e.Pool.Exec(ctx, `UPDATE agent_sessions SET active_run_id=NULL WHERE id=$1 AND active_run_id=$2`, session, run)
}
