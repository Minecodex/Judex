package work

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"strings"
)

func (s *Service) ChangeExecutionException(ctx context.Context, user, project, task uuid.UUID, operation string, in ExceptionCommand) error {
	return s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, project.String()); err != nil {
			return err
		}
		return s.ChangeExecutionExceptionTx(ctx, tx, user, project, task, operation, in)
	})
}
func (s *Service) ChangeExecutionExceptionTx(ctx context.Context, tx pgx.Tx, user, project, task uuid.UUID, operation string, in ExceptionCommand) error {
	var version int64
	if err := tx.QueryRow(ctx, `SELECT version FROM tasks WHERE project_id=$1 AND id=$2 FOR UPDATE`, project, task).Scan(&version); err != nil {
		return apierrors.New(apierrors.NotFound, "task not found")
	}
	if version != in.ExpectedVersion {
		return apierrors.New(apierrors.VersionConflict, "task changed")
	}
	review, err := s.exceptionReview(ctx, tx, user, project, task, operation)
	if err != nil {
		return err
	}
	if review.ReviewHash != in.ReviewHash {
		return apierrors.New(apierrors.ReviewStale, "execution exception impact changed")
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return apierrors.Fields("reason", "required")
	}
	if operation == "skip" {
		waivers, e := validateWaivers(review, in.Waivers)
		if e != nil {
			return e
		}
		raw, e := json.Marshal(waivers)
		if e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, `INSERT INTO task_execution_exceptions(project_id,id,task_id,previous_status,reason,actor_user_id,created_at,waivers_json) SELECT $1,$2,id,status,$4,$5,$6,$7 FROM tasks WHERE project_id=$1 AND id=$3`, project, uuid.New(), task, reason, user, s.now(), raw); e != nil {
			return e
		}
	} else {
		for _, v := range review.AffectedTasks {
			if v.AlreadyStarted && !in.AcknowledgeStarted {
				return apierrors.New(apierrors.RequirementUnmet, "started successors require explicit acknowledgement")
			}
		}
		if _, err = tx.Exec(ctx, `UPDATE task_execution_exceptions SET restored_at=$3,restored_by=$4,restore_reason=$5 WHERE project_id=$1 AND task_id=$2 AND restored_at IS NULL`, project, task, s.now(), user, reason); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE tasks SET version=version+1,updated_at=$3 WHERE project_id=$1 AND id=$2`, project, task, s.now()); err != nil {
		return err
	}
	label := "管理员临时跳过任务："
	if operation == "restore" {
		label = "管理员恢复任务执行："
	}
	if err = s.recordWorkChange(ctx, tx, project, user, task, "task", "execution."+operation, label+reason, map[string]any{"reviewHash": in.ReviewHash, "waivers": in.Waivers, "acknowledgeStarted": in.AcknowledgeStarted}); err != nil {
		return err
	}
	return audit.Append(ctx, tx, audit.Entry{ProjectID: &project, ActorType: audit.ActorUser, ActorUserID: &user, Source: audit.SourceWeb, Operation: "task.execution." + operation, ObjectType: "task", ObjectID: task.String(), OccurredAt: s.now()})
}
