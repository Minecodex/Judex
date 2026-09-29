// SPDX-License-Identifier: Apache-2.0

package work

import (
	"context"
	"encoding/json"
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
	CodeRefs            []map[string]any
	EnvironmentRefs     []map[string]any
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
	identityID, bindingVersion, err := ResolveParticipant(ctx, tx, requester, projectID, in.TaskID, in.IdentityID)
	if err != nil {
		return uuid.Nil, 0, err
	}
	in.IdentityID = &identityID
	if in.CodeRefs == nil {
		in.CodeRefs = []map[string]any{}
	}
	if in.EnvironmentRefs == nil {
		in.EnvironmentRefs = []map[string]any{}
	}
	if err := validateCodeRefs(ctx, tx, projectID, in.CodeRefs); err != nil {
		return uuid.Nil, 0, err
	}
	hash := reportHash(in)
	// Dedup: same submission never double-reports (unique constraint guards).
	if in.SubmissionID != nil {
		var existing uuid.NullUUID
		var existingHash string
		var originalVersion int64
		if err := tx.QueryRow(ctx, `
			SELECT id,payload_hash,task_version FROM work_reports WHERE task_id=$1 AND submission_id=$2 AND identity_id=COALESCE($3::uuid, '00000000-0000-0000-0000-000000000000'::uuid)`,
			in.TaskID, *in.SubmissionID, nullableUUID(in.IdentityID)).Scan(&existing, &existingHash, &originalVersion); err == nil && existing.Valid {
			if existingHash != hash {
				return uuid.Nil, 0, apierrors.New(apierrors.IdempotencyConflict, "submission already reported with different payload")
			}
			return existing.UUID, originalVersion, nil
		}
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
	if in.Kind == "progress" && status != "working" && status != "rework" {
		return uuid.Nil, 0, apierrors.New(apierrors.InvalidTransition, "progress requires working or rework")
	}
	if in.Kind == "delivery" {
		_, blockers, err := s.RequirementsFor(ctx, tx, projectID, in.TaskID)
		if err != nil {
			return uuid.Nil, 0, err
		}
		for _, b := range blockers {
			if b.Phase == "start" || b.Phase == "both" {
				return uuid.Nil, 0, apierrors.New(apierrors.RequirementUnmet, "delivery prerequisites unmet").WithDetails(map[string]any{"blockers": blockers})
			}
		}
		var missing int
		err = tx.QueryRow(ctx, `SELECT count(*) FROM task_participants p JOIN tasks t ON t.id=p.task_id
   WHERE p.task_id=$1 AND p.identity_id<>$2 AND NOT EXISTS(SELECT 1 FROM work_reports r
    WHERE r.task_id=t.id AND r.identity_id=p.identity_id AND r.agreement_version=t.agreement_version)`, in.TaskID, identityID).Scan(&missing)
		if err != nil {
			return uuid.Nil, 0, err
		}
		if missing > 0 {
			return uuid.Nil, 0, apierrors.New(apierrors.RequirementUnmet, "required participant contributions missing")
		}
	}
	for _, raw := range in.MaterialVersionIDs {
		vid, err := uuid.Parse(raw)
		if err != nil {
			return uuid.Nil, 0, apierrors.Fields("materialVersionIds", "uuid")
		}
		var ready bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM material_versions WHERE project_id=$1 AND id=$2 AND state='ready')`, projectID, vid).Scan(&ready); err != nil {
			return uuid.Nil, 0, err
		}
		if !ready {
			return uuid.Nil, 0, apierrors.New(apierrors.RequirementUnmet, "material is not ready in this project")
		}
	}
	now := s.now()
	reportID := uuid.New()
	codeRefs, _ := json.Marshal(in.CodeRefs)
	envRefs, _ := json.Marshal(in.EnvironmentRefs)
	identityColumn := uuid.Nil
	if in.IdentityID != nil {
		identityColumn = *in.IdentityID
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO work_reports (project_id, id, task_id, identity_id, binding_version, submission_id,
			report_kind, progress_hint, created_by, created_at, agreement_version,payload_hash,task_version,code_refs_json,environment_refs_json)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,(SELECT agreement_version FROM tasks WHERE id=$3),$11,$12,$13,$14)`,
		projectID, reportID, in.TaskID, identityColumn, bindingVersion, in.SubmissionID,
		in.Kind, nullableText(in.Text), requester, now, hash, currentVer+1, codeRefs, envRefs); err != nil {
		return uuid.Nil, 0, apierrors.New(apierrors.Internal, "report insert failed").Wrap(err)
	}
	for _, raw := range in.MaterialVersionIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO work_report_materials(project_id,report_id,material_version_id) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, projectID, reportID, raw); err != nil {
			return uuid.Nil, 0, err
		}
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
		IdentityID: in.IdentityID, BindingVersion: &bindingVersion, Source: audit.SourceWeb,
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
		if err := tx.LockActiveProject(ctx, projectID.String()); err != nil {
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
