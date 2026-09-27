// SPDX-License-Identifier: Apache-2.0

package work

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/events"
)

// ReportInput carries the unified work-report payload shared by the
// submission entry and POST /tasks/{id}/reports (04 §1 防双写：两个入口共用
// 本用例)。
type ReportInput struct {
	SubmissionID        *uuid.UUID
	TaskID              uuid.UUID
	IdentityID          *uuid.UUID
	Kind                string // progress | delivery
	Text                string
	MaterialVersionIDs  []string
	ExpectedTaskVersion int64
}

// ReportInTx records one work report inside the caller's transaction:
// participant/binding check, task status gates (03 §2), immutable report
// row, task version+1, latest pointer, event + audit. Delivery moves
// ready/working/rework -> delivered; progress keeps the state.
func (s *Service) ReportInTx(ctx context.Context, tx pgx.Tx, requester, projectID uuid.UUID, in ReportInput) (uuid.UUID, int64, error) {
	if in.Kind != "progress" && in.Kind != "delivery" {
		return uuid.Nil, 0, apierrors.Fields("kind", "enum")
	}
	if in.ExpectedTaskVersion <= 0 {
		return uuid.Nil, 0, apierrors.Fields("expectedTaskVersion", "required")
	}
	var (
		status       string
		currentVer   int64
		latestReport uuid.NullUUID
	)
	if err := tx.QueryRow(ctx, `
		SELECT status, version, latest_report_id FROM tasks
		WHERE id=$1 AND project_id=$2 FOR UPDATE`, in.TaskID, projectID).
		Scan(&status, &currentVer, &latestReport); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, 0, apierrors.New(apierrors.InvalidReference, "task not found")
		}
		return uuid.Nil, 0, apierrors.New(apierrors.Internal, "task lookup failed").Wrap(err)
	}
	if status == "accepted" {
		return uuid.Nil, 0, apierrors.New(apierrors.InvalidTransition, "已验收任务不能追加报告；需 reviewer 重开")
	}
	if status == "cancelled" || status == "draft" {
		return uuid.Nil, 0, apierrors.New(apierrors.InvalidTransition, "task is "+status)
	}
	if in.Kind == "delivery" && status != "ready" && status != "working" && status != "rework" {
		return uuid.Nil, 0, apierrors.New(apierrors.InvalidTransition, "delivery from "+status)
	}
	// Identity check: the reporter must currently hold the identity (or the
	// task has no identity requirement and the member reports as themselves).
	bindingVersion := int64(0)
	if in.IdentityID != nil {
		var holder *uuid.UUID
		if err := tx.QueryRow(ctx, `
			SELECT b.user_id FROM agent_identities i
			LEFT JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version
			WHERE i.id=$1 AND i.project_id=$2`, *in.IdentityID, projectID).Scan(&holder); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return uuid.Nil, 0, apierrors.New(apierrors.InvalidReference, "identity not found")
			}
			return uuid.Nil, 0, apierrors.New(apierrors.Internal, "identity lookup failed").Wrap(err)
		}
		if holder == nil || *holder != requester {
			return uuid.Nil, 0, apierrors.New(apierrors.Forbidden, "只有当前身份绑定人可以上报")
		}
		var version int64
		if err := tx.QueryRow(ctx, `SELECT current_binding_version FROM agent_identities WHERE id=$1`, *in.IdentityID).Scan(&version); err != nil {
			return uuid.Nil, 0, apierrors.New(apierrors.Internal, "binding lookup failed").Wrap(err)
		}
		bindingVersion = version
	}
	// Dedup: same submission never double-reports (unique constraint guards).
	if in.SubmissionID != nil {
		var existing uuid.NullUUID
		if err := tx.QueryRow(ctx, `
			SELECT id FROM work_reports WHERE task_id=$1 AND submission_id=$2 AND identity_id=COALESCE($3::uuid, '00000000-0000-0000-0000-000000000000'::uuid)`,
			in.TaskID, *in.SubmissionID, nullableUUID(in.IdentityID)).Scan(&existing); err == nil && existing.Valid {
			return existing.UUID, currentVer, nil
		}
	}
	now := s.now()
	reportID := uuid.New()
	identityColumn := uuid.Nil
	if in.IdentityID != nil {
		identityColumn = *in.IdentityID
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO work_reports (project_id, id, task_id, identity_id, binding_version, submission_id,
			report_kind, progress_hint, created_by, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		projectID, reportID, in.TaskID, identityColumn, bindingVersion, in.SubmissionID,
		in.Kind, nullableText(in.Text), requester, now); err != nil {
		return uuid.Nil, 0, apierrors.New(apierrors.Internal, "report insert failed").Wrap(err)
	}
	nextStatus := status
	if in.Kind == "delivery" {
		nextStatus = "delivered"
	}
	tag, err := tx.Exec(ctx, `
		UPDATE tasks SET status=$2, latest_report_id=$3, version=version+1, updated_at=$4
		WHERE id=$1 AND version=$5`,
		in.TaskID, nextStatus, reportID, now, in.ExpectedTaskVersion)
	if err != nil {
		return uuid.Nil, 0, apierrors.New(apierrors.Internal, "task update failed").Wrap(err)
	}
	if tag.RowsAffected() == 0 {
		return uuid.Nil, 0, apierrors.New(apierrors.VersionConflict, "task version conflict")
	}
	if _, err := events.AppendProjectEvent(ctx, tx, projectID, "task.changed", "task", in.TaskID.String(), nil,
		map[string]any{"change": "report_" + in.Kind, "reportId": reportID}, now); err != nil {
		return uuid.Nil, 0, err
	}
	if err := audit.Append(ctx, tx, audit.Entry{
		ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
		IdentityID: in.IdentityID, Source: audit.SourceWeb,
		Operation:  "work.report." + in.Kind,
		ObjectType: "task", ObjectID: in.TaskID.String(), OccurredAt: now,
	}); err != nil {
		return uuid.Nil, 0, err
	}
	return reportID, currentVer + 1, nil
}

// Report is the standalone use case used by POST /tasks/{id}/reports: it
// wraps ReportInTx in its own transaction.
func (s *Service) Report(ctx context.Context, requester, projectID uuid.UUID, in ReportInput) (uuid.UUID, int64, error) {
	var reportID uuid.UUID
	var version int64
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockProjectForUpdate(ctx, projectID.String()); err != nil {
			return err
		}
		if _, err := memberTx(ctx, tx, projectID, requester); err != nil {
			return err
		}
		var err error
		reportID, version, err = s.ReportInTx(ctx, tx, requester, projectID, in)
		return err
	})
	return reportID, version, err
}

func nullableText(v string) any {
	if v == "" {
		return nil
	}
	return v
}
