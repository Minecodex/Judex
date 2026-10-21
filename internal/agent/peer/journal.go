package peer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	"github.com/kakj-go/Judex/internal/job"
)

func (e *Executor) claim(ctx context.Context, delivery uuid.UUID) (*run, error) {
	r := &run{Owner: uuid.NewString()}
	skip := false
	err := e.Service.Pool.Transact(ctx, func(ctx context.Context, _ postgres.Tx) error {
		var state, discussion string
		var active bool
		var lease *time.Time
		err := e.Service.Pool.QueryRow(ctx, `SELECT run.id,run.role_agent_id,run.discussion_id,d.id,g.id,g.project_id,a.user_id,d.causal_path,run.fence,run.state,run.lease_until,run.checkpoint,run.budget_model_calls,run.budget_tokens,run.budget_started_at,COALESCE(run.snapshot_version,0),dc.state,
  a.active AND cm.active AND pm.state='active' AND u.status='active'
  FROM deliveries d JOIN role_runs run ON run.delivery_id=d.id JOIN role_agents a ON a.id=d.role_agent_id JOIN groups g ON g.id=a.group_id JOIN discussions dc ON dc.id=d.discussion_id JOIN users u ON u.id=a.user_id JOIN conversation_members cm ON cm.conversation_id=g.id AND cm.user_id=a.user_id JOIN project_members pm ON pm.project_id=g.project_id AND pm.user_id=a.user_id WHERE d.id=$1 FOR UPDATE OF run`, delivery).Scan(&r.Publication.RunID, &r.Publication.RoleID, &r.Publication.DiscussionID, &r.Publication.DeliveryID, &r.Publication.GroupID, &r.ProjectID, &r.Publication.UserID, &r.Publication.Path, &r.Publication.Fence, &state, &lease, &r.Checkpoint, &r.Calls, &r.Tokens, &r.BudgetStarted, &r.SnapshotVersion, &discussion, &active)
		if errors.Is(err, pgx.ErrNoRows) {
			skip = true
			return nil
		}
		if err != nil {
			return err
		}
		if state == "completed" || state == "cancelled" {
			skip = true
			return nil
		}
		if !active || discussion == "terminated" || discussion == "settled" {
			skip = true
			if _, err = e.Service.Pool.Exec(ctx, `UPDATE role_runs SET state='cancelled',fence=fence+1 WHERE id=$1`, r.Publication.RunID); err != nil {
				return err
			}
			_, err = e.Service.Pool.Exec(ctx, `UPDATE deliveries SET state='cancelled' WHERE id=$1`, delivery)
			return err
		}
		if discussion == "paused" {
			return job.ErrBusy
		}
		if _, err = e.Service.Pool.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,1))`, r.Publication.RoleID.String()); err != nil {
			return err
		}
		var busy bool
		if err = e.Service.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM role_runs WHERE role_agent_id=$1 AND state='running' AND lease_until>now() AND id<>$2)`, r.Publication.RoleID, r.Publication.RunID).Scan(&busy); err != nil {
			return err
		}
		if busy {
			return job.ErrBusy
		}
		if state == "running" && lease != nil && lease.After(time.Now()) {
			return job.ErrBusy
		}
		if _, err = e.Service.Pool.Exec(ctx, `UPDATE role_runs SET state='paused' WHERE role_agent_id=$1 AND id<>$2 AND state='running' AND lease_until<=now()`, r.Publication.RoleID, r.Publication.RunID); err != nil {
			return err
		}
		r.Publication.Fence++
		if _, err = e.Service.Pool.Exec(ctx, `UPDATE role_runs SET state='running',fence=$2,lease_owner=$3,lease_until=now()+interval '90 seconds',updated_at=now() WHERE id=$1`, r.Publication.RunID, r.Publication.Fence, r.Owner); err != nil {
			return err
		}
		_, err = e.Service.Pool.Exec(ctx, `UPDATE deliveries SET state='running' WHERE id=$1 AND state='pending'`, delivery)
		return err
	})
	if err != nil {
		return nil, err
	}
	if skip {
		if r.Publication.GroupID != uuid.Nil {
			if err = e.Service.Settle(ctx, r.Publication.GroupID); err != nil {
				return nil, err
			}
		}
		return nil, nil
	}
	return r, nil
}
func (e *Executor) live(ctx context.Context, r *run) (bool, error) {
	var ok bool
	err := e.Service.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM role_runs run JOIN discussions d ON d.id=run.discussion_id JOIN role_agents a ON a.id=run.role_agent_id JOIN conversation_members cm ON cm.conversation_id=a.group_id AND cm.user_id=a.user_id JOIN groups g ON g.id=a.group_id JOIN project_members pm ON pm.project_id=g.project_id AND pm.user_id=a.user_id JOIN users u ON u.id=a.user_id WHERE run.id=$1 AND run.fence=$2 AND run.state='running' AND run.lease_owner=$3 AND run.lease_until>now() AND run.budget_started_at+interval '30 minutes'>clock_timestamp() AND d.state='active' AND a.active AND cm.active AND pm.state='active' AND u.status='active')`, r.Publication.RunID, r.Publication.Fence, r.Owner).Scan(&ok)
	return ok, err
}
func (e *Executor) heartbeat(ctx context.Context, r *run, cancel context.CancelFunc, done <-chan struct{}) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			ok, err := e.live(ctx, r)
			if err != nil || !ok {
				cancel()
				return
			}
			tag, err := e.Service.Pool.Exec(ctx, `UPDATE role_runs SET lease_until=now()+interval '90 seconds',updated_at=now() WHERE id=$1 AND fence=$2 AND lease_owner=$3 AND state='running'`, r.Publication.RunID, r.Publication.Fence, r.Owner)
			if err != nil || tag.RowsAffected() != 1 {
				cancel()
				return
			}
		}
	}
}
func (e *Executor) checkpoint(ctx context.Context, r *run) error {
	raw, _ := json.Marshal(r.Checkpoint)
	tag, err := e.Service.Pool.Exec(ctx, `UPDATE role_runs SET checkpoint=$3,updated_at=now() WHERE id=$1 AND fence=$2 AND state='running'`, r.Publication.RunID, r.Publication.Fence, raw)
	if err == nil && tag.RowsAffected() != 1 {
		return fmt.Errorf("run checkpoint lease lost")
	}
	return err
}
func (e *Executor) release(ctx context.Context, r *run) error {
	if ctx.Err() == nil && !time.Now().Before(r.BudgetStarted.Add(30*time.Minute)) {
		return e.fail(ctx, r, "technical_budget", errTimeBudget)
	}
	_, err := e.Service.Pool.Exec(ctx, `UPDATE role_runs SET state='pending',lease_until=NULL,lease_owner=NULL,updated_at=now() WHERE id=$1 AND fence=$2 AND state='running'`, r.Publication.RunID, r.Publication.Fence)
	if err != nil {
		return err
	}
	return job.ErrBusy
}
func (e *Executor) fail(ctx context.Context, r *run, reason string, cause error) error {
	// Stream creation and configuration I/O may fail because the process is
	// draining. That cancellation must release the lease, not persist a model
	// failure pause immediately before the process exits.
	if ctx.Err() != nil {
		return e.interrupted(ctx, r, cause)
	}
	if !time.Now().Before(r.BudgetStarted.Add(30 * time.Minute)) {
		reason = "technical_budget"
	}
	ctx = context.WithoutCancel(ctx)
	// Do not expose model request bodies, private prompts or credentials in public errors.
	err := e.Service.Pool.Transact(ctx, func(ctx context.Context, _ postgres.Tx) error {
		if _, err := e.Service.Pool.Exec(ctx, `SELECT id FROM conversations WHERE id=$1 FOR UPDATE`, r.Publication.GroupID); err != nil {
			return err
		}

		tag, err := e.Service.Pool.Exec(ctx, `UPDATE role_runs SET state='paused',lease_owner=NULL,lease_until=NULL,error_code=$3,error_message=$4,updated_at=now() WHERE id=$1 AND fence=$2 AND state='running'`, r.Publication.RunID, r.Publication.Fence, reason, "Execution paused: "+reason)
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		return e.Service.Pause(ctx, r.Publication.GroupID, r.Publication.DiscussionID, reason)
	})
	if err != nil {
		return err
	}
	_ = cause
	return job.ErrBusy
}
func (e *Executor) interrupted(ctx context.Context, r *run, cause error) error {
	if ctx.Err() == nil && !time.Now().Before(r.BudgetStarted.Add(30*time.Minute)) {
		return e.fail(ctx, r, "technical_budget", errTimeBudget)
	}
	var state string
	err := e.Service.Pool.QueryRow(context.WithoutCancel(ctx), `SELECT state FROM discussions WHERE id=$1`, r.Publication.DiscussionID).Scan(&state)
	if err != nil {
		return err
	}
	if state == "terminated" || state == "settled" {
		return nil
	}
	if state == "paused" {
		_, err = e.Service.Pool.Exec(context.WithoutCancel(ctx), `UPDATE role_runs SET state='paused',lease_owner=NULL,lease_until=NULL WHERE id=$1 AND fence=$2 AND state='running'`, r.Publication.RunID, r.Publication.Fence)
		if err != nil {
			return err
		}
		return job.ErrBusy
	}
	return e.release(context.WithoutCancel(ctx), r)
}
func (e *Executor) finish(ctx context.Context, r *run) error {
	err := e.Service.Pool.Transact(ctx, func(ctx context.Context, _ postgres.Tx) error {
		if _, err := e.Service.Pool.Exec(ctx, `SELECT id FROM conversations WHERE id=$1 FOR UPDATE`, r.Publication.GroupID); err != nil {
			return err
		}

		ok, err := e.live(ctx, r)
		if err != nil {
			return err
		}
		if !ok {
			return job.ErrBusy
		}
		if _, err = e.Service.Pool.Exec(ctx, `UPDATE role_runs SET state='completed',lease_owner=NULL,lease_until=NULL,updated_at=now() WHERE id=$1 AND fence=$2`, r.Publication.RunID, r.Publication.Fence); err != nil {
			return err
		}
		_, err = e.Service.Pool.Exec(ctx, `UPDATE deliveries SET state=CASE WHEN EXISTS(SELECT 1 FROM human_waits WHERE delivery_id=$1 AND NOT resolved) THEN 'waiting_human' ELSE 'done' END WHERE id=$1`, r.Publication.DeliveryID)
		return err
	})
	if err != nil {
		return err
	}
	return e.Service.Settle(ctx, r.Publication.GroupID)
}
