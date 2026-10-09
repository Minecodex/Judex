package batch

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/kakj-go/Judex/internal/infrastructure/opensandbox"
	"github.com/kakj-go/Judex/internal/job"
	"time"
)

func (e *Executor) RecordSandbox(ctx context.Context, project, run uuid.UUID, external string) error {
	tx, err := e.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	if _, err = tx.Exec(ctx, `UPDATE agent_runs SET sandbox_id=$3 WHERE project_id=$1 AND id=$2`, project, run, external); err != nil {
		return err
	}
	lease := uuid.New()
	expires := time.Now().UTC().Add(30 * time.Minute)
	if _, err = tx.Exec(ctx, `INSERT INTO sandbox_leases(project_id,id,run_id,external_id,expires_at,created_at) VALUES($1,$2,$3,$4,$5,now())`, project, lease, run, external, expires); err != nil {
		return err
	}
	key := "sandbox:" + lease.String()
	if _, err = job.Enqueue(ctx, tx, "sandbox.cleanup", map[string]string{"leaseId": lease.String()}, &key, expires.Add(time.Minute), time.Now().UTC()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (e *Executor) SandboxClosed(ctx context.Context, run uuid.UUID, closeErr error) {
	state, cleanup := "released", "done"
	if closeErr != nil {
		state, cleanup = "cleanup_failed", "pending"
	}
	_, _ = e.Pool.Exec(ctx, `UPDATE sandbox_leases SET state=$2,cleanup_state=$3 WHERE run_id=$1`, run, state, cleanup)
}
func (e *Executor) CleanupSandbox(client opensandbox.Sandbox) func(context.Context, job.Job) error {
	return func(ctx context.Context, j job.Job) error {
		var payload struct {
			LeaseID uuid.UUID `json:"leaseId"`
		}
		if err := json.Unmarshal(j.Payload, &payload); err != nil {
			return err
		}
		var external, state string
		err := e.Pool.QueryRow(ctx, `SELECT external_id,state FROM sandbox_leases WHERE id=$1`, payload.LeaseID).Scan(&external, &state)
		if err == pgx.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		if state == "released" {
			return nil
		}
		if err = client.Kill(ctx, external); err != nil {
			return err
		}
		_, err = e.Pool.Exec(ctx, `UPDATE sandbox_leases SET state='released',cleanup_state='done' WHERE id=$1`, payload.LeaseID)
		return err
	}
}
