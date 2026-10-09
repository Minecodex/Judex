package collaboration

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	"github.com/kakj-go/Judex/internal/job"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/events"
	"time"
)

func (s *Service) Retry(ctx context.Context, user, project, id uuid.UUID) (Analysis, error) {
	err := s.Pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, project.String()); err != nil {
			return err
		}
		if err := Member(ctx, tx, user, project); err != nil {
			return err
		}
		var batch uuid.UUID
		var state string
		var rounds, maxRounds, attempts int
		if err := tx.QueryRow(ctx, `SELECT a.batch_id,a.state,b.rounds_reserved,b.max_rounds,b.model_attempts FROM task_analyses a JOIN discussion_batches b ON b.id=a.batch_id WHERE a.id=$1 AND a.project_id=$2 FOR UPDATE OF a,b`, id, project).Scan(&batch, &state, &rounds, &maxRounds, &attempts); err != nil {
			return apierrors.New(apierrors.NotFound, "analysis not found")
		}
		if state == "queued" || state == "running" {
			return nil
		}
		if state != "failed" && state != "cancelled" {
			return apierrors.New(apierrors.InvalidTransition, "analysis is not retryable")
		}
		if rounds >= maxRounds || attempts >= 30 {
			return apierrors.New(apierrors.BudgetExhausted, "analysis budget exhausted")
		}
		var unsafe bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tool_calls WHERE run_id IN(SELECT id FROM agent_runs WHERE batch_id=$1) AND (state IN('prepared','running','unknown') OR name IN('bash','read','write','edit')))`, batch).Scan(&unsafe); err != nil {
			return err
		}
		if unsafe {
			return apierrors.New(apierrors.InvalidTransition, "interrupted tool effects require reconciliation")
		}
		now := time.Now().UTC()
		if _, err := tx.Exec(ctx, `UPDATE task_analyses SET state='queued',error_code=NULL,updated_at=$2 WHERE id=$1`, id, now); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE discussion_batches SET state='queued',version=version+1,updated_at=$2 WHERE id=$1`, batch, now); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE agent_runs SET state='queued',lease_token=NULL,requested_by=$2,updated_at=$3 WHERE batch_id=$1 AND parent_run_id IS NULL`, batch, user, now); err != nil {
			return err
		}
		unique := fmt.Sprintf("batch:%s:retry:%s", batch, uuid.New())
		if _, err := job.Enqueue(ctx, tx.Tx, "discussion.batch", map[string]string{"projectId": project.String(), "batchId": batch.String()}, &unique, now, now); err != nil {
			return err
		}
		_, err := events.AppendProjectEvent(ctx, tx.Tx, project, "analysis.changed", "task_analysis", id.String(), nil, map[string]any{"state": "queued"}, now)
		return err
	})
	if err != nil {
		return Analysis{}, err
	}
	return s.GetAnalysis(ctx, user, project, id)
}
