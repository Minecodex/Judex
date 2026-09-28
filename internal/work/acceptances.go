// SPDX-License-Identifier: Apache-2.0

package work

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/events"
)

// AcceptanceReview is the frozen snapshot shown to the reviewer (06 §5).
type AcceptanceReview struct {
	ReviewID          string           `json:"reviewId"`
	ReviewHash        string           `json:"reviewHash"`
	TargetType        string           `json:"targetType"`
	TargetID          uuid.UUID        `json:"targetId"`
	TargetVersion     int64            `json:"targetVersion"`
	Reports           []map[string]any `json:"reports"`
	TaskAcceptanceIDs []string         `json:"taskAcceptanceIds"`
	Blockers          []Blocker        `json:"blockers"`
}

// TaskAcceptanceReview builds the task snapshot: current version, latest
// report, satisfied requirements; the hash covers exactly what the reviewer
// sees (03 §7 弹窗展示"验收的是哪份成果")。
func (s *Service) TaskAcceptanceReview(ctx context.Context, requester, projectID, taskID uuid.UUID) (AcceptanceReview, error) {
	if _, err := memberTx(ctx, s.pool, projectID, requester); err != nil {
		return AcceptanceReview{}, err
	}
	var (
		version          int64
		status           string
		latestReport     uuid.NullUUID
		reviewerIdentity uuid.NullUUID
	)
	err := s.pool.QueryRow(ctx, `
		SELECT version, status, latest_report_id, reviewer_identity_id
		FROM tasks WHERE id=$1 AND project_id=$2`, taskID, projectID).
		Scan(&version, &status, &latestReport, &reviewerIdentity)
	if errors.Is(err, pgx.ErrNoRows) {
		return AcceptanceReview{}, apierrors.New(apierrors.NotFound, "task not found")
	}
	if err != nil {
		return AcceptanceReview{}, apierrors.New(apierrors.Internal, "task lookup failed").Wrap(err)
	}
	_, blockers, err := s.RequirementsFor(ctx, poolAsQuery{s.pool}, projectID, taskID)
	if err != nil {
		return AcceptanceReview{}, err
	}
	hasher := sha256.New()
	hasher.Write([]byte("task|" + taskID.String() + "|" + toString(version)))
	if latestReport.Valid {
		hasher.Write([]byte("|report:" + latestReport.UUID.String()))
	}
	review := AcceptanceReview{
		ReviewID:      uuid.NewString(),
		ReviewHash:    hex.EncodeToString(hasher.Sum(nil)),
		TargetType:    "task",
		TargetID:      taskID,
		TargetVersion: version,
		Blockers:      blockers,
	}
	if latestReport.Valid {
		review.Reports = append(review.Reports, map[string]any{"reportId": latestReport.UUID})
	}
	return review, nil
}

func toString(v int64) string {
	return time.Unix(v, 0).Format("") + itoa(v)
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [24]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// DecideTaskAcceptance is the reviewer's human decision (03 §2 任务验收/
// 验收退回). A hash mismatch means the evidence moved under the reviewer
// (REVIEW_STALE); accept requires accept-phase hard requirements satisfied.
func (s *Service) DecideTaskAcceptance(ctx context.Context, requester, projectID, taskID uuid.UUID, reviewHash string, accept bool, reason string, expectedVersion int64) (Task, error) {
	if !accept && reason == "" {
		return Task{}, apierrors.Fields("reason", "required")
	}
	var out Task
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		return s.decideTaskAcceptanceTx(ctx, tx, requester, projectID, taskID, reviewHash, accept, reason, expectedVersion, &out)
	})
	return out, err
}

// DecideTaskAcceptanceTx is the in-transaction variant used by intent
// confirmation (07 §5): eligibility is fully re-verified here.
func (s *Service) DecideTaskAcceptanceTx(ctx context.Context, tx pgx.Tx, requester, projectID, taskID uuid.UUID, reviewHash string, accept bool, reason string, expectedVersion int64) (Task, error) {
	var out Task
	if err := s.decideTaskAcceptanceTx(ctx, tx, requester, projectID, taskID, reviewHash, accept, reason, expectedVersion, &out); err != nil {
		return Task{}, err
	}
	return out, nil
}

func (s *Service) decideTaskAcceptanceTx(ctx context.Context, tx pgx.Tx, requester, projectID, taskID uuid.UUID, reviewHash string, accept bool, reason string, expectedVersion int64, out *Task) error {
	{
		if err := txLockProject(ctx, tx, projectID); err != nil {
			return err
		}
		if _, err := memberTxRow(ctx, tx, projectID, requester); err != nil {
			return err
		}
		var (
			version          int64
			status           string
			latestReport     uuid.NullUUID
			reviewerIdentity uuid.NullUUID
		)
		if err := tx.QueryRow(ctx, `
			SELECT version, status, latest_report_id, reviewer_identity_id
			FROM tasks WHERE id=$1 AND project_id=$2 FOR UPDATE`, taskID, projectID).
			Scan(&version, &status, &latestReport, &reviewerIdentity); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apierrors.New(apierrors.NotFound, "task not found")
			}
			return apierrors.New(apierrors.Internal, "task lookup failed").Wrap(err)
		}
		// Only the current holder of the reviewer identity decides.
		if reviewerIdentity.Valid {
			var holder *uuid.UUID
			if err := tx.QueryRow(ctx, `
				SELECT b.user_id FROM agent_identities i
				LEFT JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version
				WHERE i.id=$1`, reviewerIdentity.UUID).Scan(&holder); err != nil {
				return apierrors.New(apierrors.Internal, "binding lookup failed").Wrap(err)
			}
			if holder == nil || *holder != requester {
				return apierrors.New(apierrors.Forbidden, "只有当前验收人可以决定")
			}
		}
		if status != "delivered" {
			return apierrors.New(apierrors.InvalidTransition, "task is "+status)
		}
		if expectedVersion != version {
			return apierrors.New(apierrors.VersionConflict, "task version conflict")
		}
		// Stale review: evidence moved (new report) since the reviewer opened it.
		hasher := sha256.New()
		hasher.Write([]byte("task|" + taskID.String() + "|" + itoa(version)))
		if latestReport.Valid {
			hasher.Write([]byte("|report:" + latestReport.UUID.String()))
		}
		if hex.EncodeToString(hasher.Sum(nil)) != reviewHash {
			return apierrors.New(apierrors.ReviewStale, "验收对象已变化，请重新读取")
		}
		now := s.now()
		nextStatus := "rework"
		if accept {
			_, blockers, err := s.RequirementsFor(ctx, tx, projectID, taskID)
			if err != nil {
				return err
			}
			for _, b := range blockers {
				if b.Phase == "accept" || b.Phase == "both" {
					return apierrors.New(apierrors.RequirementUnmet, "验收前置未满足").
						WithDetails(map[string]any{"blockers": blockers})
				}
			}
			nextStatus = "accepted"
			acceptanceID := uuid.New()
			manifest := map[string]any{"taskVersion": version, "latestReportId": latestReport.UUID}
			rawManifest, _ := jsonMarshal(manifest)
			if _, err := tx.Exec(ctx, `
				INSERT INTO task_acceptances (project_id, id, task_id, task_version, reviewer_identity_id,
					actor_user_id, review_manifest, evidence_hash, created_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8,$9)`,
				projectID, acceptanceID, taskID, version, nullableNullUUID(reviewerIdentity),
				requester, rawManifest, reviewHash, now); err != nil {
				return apierrors.New(apierrors.Internal, "acceptance insert failed").Wrap(err)
			}
			if _, err := tx.Exec(ctx, `UPDATE tasks SET latest_acceptance_id=$2 WHERE id=$1`, taskID, acceptanceID); err != nil {
				return apierrors.New(apierrors.Internal, "acceptance pointer failed").Wrap(err)
			}
		}
		tag, err := tx.Exec(ctx, `
			UPDATE tasks SET status=$2, version=version+1, updated_at=$3
			WHERE id=$1 AND version=$4`, taskID, nextStatus, now, version)
		if err != nil || tag.RowsAffected() == 0 {
			return apierrors.New(apierrors.VersionConflict, "task version conflict")
		}
		if _, err := events.AppendProjectEvent(ctx, tx, projectID, "task.changed", "task", taskID.String(), nil,
			map[string]any{"change": nextStatus}, now); err != nil {
			return err
		}
		if err := audit.Append(ctx, tx, audit.Entry{
			ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
			IdentityID: nullUUIDPtr(reviewerIdentity), Source: audit.SourceWeb,
			Operation:  "task." + map[bool]string{true: "accept", false: "reject"}[accept],
			ObjectType: "task", ObjectID: taskID.String(), Reason: reasonPtr(reason), OccurredAt: now,
		}); err != nil {
			return err
		}
		*out = Task{ID: taskID, Status: nextStatus, Version: version + 1}
		return nil
	}
}

// txLockProject/memberTxRow adapt pgx.Tx to the helpers' interfaces.
func txLockProject(ctx context.Context, tx pgx.Tx, projectID uuid.UUID) error {
	_, err := tx.Exec(ctx, `SELECT id FROM projects WHERE id=$1 FOR UPDATE`, projectID)
	return err
}

func memberTxRow(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, projectID, requester uuid.UUID) (string, error) {
	return memberTx(ctx, q, projectID, requester)
}

// ReopenTask returns an accepted task to rework (03 §2 任务重开): reviewer
// only, explicit reason, references the superseded acceptance; history rows
// stay untouched.
func (s *Service) ReopenTask(ctx context.Context, requester, projectID, taskID uuid.UUID, acceptanceID uuid.UUID, reason string, expectedVersion int64) (Task, error) {
	if reason == "" {
		return Task{}, apierrors.Fields("reason", "required")
	}
	var out Task
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockProjectForUpdate(ctx, projectID.String()); err != nil {
			return err
		}
		if _, err := memberTx(ctx, tx, projectID, requester); err != nil {
			return err
		}
		var (
			status           string
			reviewerIdentity uuid.NullUUID
		)
		if err := tx.QueryRow(ctx, `
			SELECT status, reviewer_identity_id FROM tasks WHERE id=$1 AND project_id=$2 FOR UPDATE`,
			taskID, projectID).Scan(&status, &reviewerIdentity); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apierrors.New(apierrors.NotFound, "task not found")
			}
			return apierrors.New(apierrors.Internal, "task lookup failed").Wrap(err)
		}
		if reviewerIdentity.Valid {
			var holder *uuid.UUID
			if err := tx.QueryRow(ctx, `
				SELECT b.user_id FROM agent_identities i
				LEFT JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version
				WHERE i.id=$1`, reviewerIdentity.UUID).Scan(&holder); err != nil {
				return apierrors.New(apierrors.Internal, "binding lookup failed").Wrap(err)
			}
			if holder == nil || *holder != requester {
				return apierrors.New(apierrors.Forbidden, "只有当前验收人可以重开")
			}
		}
		if status != "accepted" {
			return apierrors.New(apierrors.InvalidTransition, "only accepted tasks reopen")
		}
		now := s.now()
		tag, err := tx.Exec(ctx, `
			UPDATE tasks SET status='rework', version=version+1, updated_at=$3
			WHERE id=$1 AND project_id=$2 AND version=$4`, taskID, projectID, now, expectedVersion)
		if err != nil || tag.RowsAffected() == 0 {
			return apierrors.New(apierrors.VersionConflict, "task version conflict")
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO reopen_records (project_id, id, target_type, target_id, previous_acceptance_id, reason, actor_user_id, created_at)
			VALUES ($1,$2,'task',$3,$4,$5,$6,$7)`,
			projectID, uuid.New(), taskID, acceptanceID, reason, requester, now); err != nil {
			return apierrors.New(apierrors.Internal, "reopen record failed").Wrap(err)
		}
		if _, err := events.AppendProjectEvent(ctx, tx, projectID, "task.changed", "task", taskID.String(), nil,
			map[string]any{"change": "reopened"}, now); err != nil {
			return err
		}
		if err := audit.Append(ctx, tx, audit.Entry{
			ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
			Source: audit.SourceWeb, Operation: "task.reopen",
			ObjectType: "task", ObjectID: taskID.String(), Reason: &reason, OccurredAt: now,
		}); err != nil {
			return err
		}
		out = Task{ID: taskID, Status: "rework", Version: expectedVersion + 1}
		return nil
	})
	return out, err
}

// PlanAcceptanceReview snapshots the plan for its owner (03 §7 计划验收引用
// 各任务 acceptanceId)：任务全部验收仅产生待办，不自动完成计划。
func (s *Service) PlanAcceptanceReview(ctx context.Context, requester, projectID, planID uuid.UUID) (AcceptanceReview, error) {
	if _, err := memberTx(ctx, s.pool, projectID, requester); err != nil {
		return AcceptanceReview{}, err
	}
	var (
		version int64
		status  string
		owner   uuid.NullUUID
	)
	err := s.pool.QueryRow(ctx, `
		SELECT version, status, owner_identity_id FROM plans WHERE id=$1 AND project_id=$2`,
		planID, projectID).Scan(&version, &status, &owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return AcceptanceReview{}, apierrors.New(apierrors.NotFound, "plan not found")
	}
	if err != nil {
		return AcceptanceReview{}, apierrors.New(apierrors.Internal, "plan lookup failed").Wrap(err)
	}
	rows, err := s.pool.Query(ctx, `
		SELECT t.id, t.status, t.latest_acceptance_id FROM tasks t
		WHERE t.plan_id=$1 AND t.project_id=$2 AND t.status<>'cancelled'`, planID, projectID)
	if err != nil {
		return AcceptanceReview{}, apierrors.New(apierrors.Internal, "tasks failed").Wrap(err)
	}
	defer rows.Close()
	accepted, total := 0, 0
	var acceptanceIDs []string
	hasher := sha256.New()
	hasher.Write([]byte("plan|" + planID.String() + "|" + itoa(version)))
	for rows.Next() {
		var (
			id         uuid.UUID
			status     string
			acceptance uuid.NullUUID
		)
		if err := rows.Scan(&id, &status, &acceptance); err != nil {
			return AcceptanceReview{}, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		total++
		if status == "accepted" {
			accepted++
			if acceptance.Valid {
				acceptanceIDs = append(acceptanceIDs, acceptance.UUID.String())
				hasher.Write([]byte("|acc:" + acceptance.UUID.String()))
			}
		}
	}
	var blockers []Blocker
	if accepted < total {
		blockers = append(blockers, Blocker{Phase: "accept", ObjectType: "plan", ObjectID: planID.String(),
			Reason: itoa(int64(accepted)) + "/" + itoa(int64(total)) + " 任务已验收"})
	}
	return AcceptanceReview{
		ReviewID:          uuid.NewString(),
		ReviewHash:        hex.EncodeToString(hasher.Sum(nil)),
		TargetType:        "plan",
		TargetID:          planID,
		TargetVersion:     version,
		TaskAcceptanceIDs: acceptanceIDs,
		Blockers:          blockers,
	}, nil
}

// DecidePlanAcceptance: only the plan owner's current holder accepts; tasks
// all accepted is REQUIRED (REQUIREMENT_UNMET otherwise); the acceptance
// snapshots the referenced task acceptance ids (03 §7).
func (s *Service) DecidePlanAcceptance(ctx context.Context, requester, projectID, planID uuid.UUID, reviewHash string, accept bool, reason string) (Plan, error) {
	if !accept && reason == "" {
		return Plan{}, apierrors.Fields("reason", "required")
	}
	var out Plan
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockProjectForUpdate(ctx, projectID.String()); err != nil {
			return err
		}
		if _, err := memberTx(ctx, tx, projectID, requester); err != nil {
			return err
		}
		var (
			version  int64
			status   string
			owner    uuid.NullUUID
			criteria string
		)
		if err := tx.QueryRow(ctx, `
			SELECT version, status, owner_identity_id, acceptance_criteria FROM plans
			WHERE id=$1 AND project_id=$2 FOR UPDATE`, planID, projectID).
			Scan(&version, &status, &owner, &criteria); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apierrors.New(apierrors.NotFound, "plan not found")
			}
			return apierrors.New(apierrors.Internal, "plan lookup failed").Wrap(err)
		}
		if owner.Valid {
			var holder *uuid.UUID
			if err := tx.QueryRow(ctx, `
				SELECT b.user_id FROM agent_identities i
				LEFT JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version
				WHERE i.id=$1`, owner.UUID).Scan(&holder); err != nil {
				return apierrors.New(apierrors.Internal, "binding lookup failed").Wrap(err)
			}
			if holder == nil || *holder != requester {
				return apierrors.New(apierrors.Forbidden, "只有计划负责人可以整体验收")
			}
		}
		if status != "active" {
			return apierrors.New(apierrors.InvalidTransition, "plan is "+status)
		}
		rows, err := tx.Query(ctx, `
			SELECT t.status, t.latest_acceptance_id FROM tasks t
			WHERE t.plan_id=$1 AND t.project_id=$2 AND t.status<>'cancelled'`, planID, projectID)
		if err != nil {
			return apierrors.New(apierrors.Internal, "tasks failed").Wrap(err)
		}
		accepted, total := 0, 0
		var acceptanceIDs []string
		hasher := sha256.New()
		hasher.Write([]byte("plan|" + planID.String() + "|" + itoa(version)))
		for rows.Next() {
			var status string
			var acceptance uuid.NullUUID
			if err := rows.Scan(&status, &acceptance); err != nil {
				rows.Close()
				return apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
			}
			total++
			if status == "accepted" {
				accepted++
				if acceptance.Valid {
					acceptanceIDs = append(acceptanceIDs, acceptance.UUID.String())
					hasher.Write([]byte("|acc:" + acceptance.UUID.String()))
				}
			}
		}
		rows.Close()
		if hex.EncodeToString(hasher.Sum(nil)) != reviewHash {
			return apierrors.New(apierrors.ReviewStale, "计划验收对象已变化")
		}
		now := s.now()
		if accept {
			if accepted < total {
				return apierrors.New(apierrors.RequirementUnmet, "任务未全部验收").
					WithDetails(map[string]any{"accepted": accepted, "total": total})
			}
			acceptanceID := uuid.New()
			refs, _ := jsonMarshal(acceptanceIDs)
			if _, err := tx.Exec(ctx, `
				INSERT INTO plan_acceptances (project_id, id, plan_id, plan_version, owner_identity_id,
					actor_user_id, task_acceptance_refs, criteria_snapshot, created_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8,$9)`,
				projectID, acceptanceID, planID, version, nullableNullUUID(owner), requester,
				refs, criteria, now); err != nil {
				return apierrors.New(apierrors.Internal, "plan acceptance failed").Wrap(err)
			}
			if _, err := tx.Exec(ctx, `UPDATE plans SET status='accepted', latest_acceptance_id=$2 WHERE id=$1`,
				planID, acceptanceID); err != nil {
				return apierrors.New(apierrors.Internal, "plan update failed").Wrap(err)
			}
		}
		if _, err := events.AppendProjectEvent(ctx, tx, projectID, "plan.changed", "plan", planID.String(), nil,
			map[string]any{"change": map[bool]string{true: "accepted", false: "rejected"}[accept]}, now); err != nil {
			return err
		}
		if err := audit.Append(ctx, tx, audit.Entry{
			ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
			Source: audit.SourceWeb, Operation: "plan." + map[bool]string{true: "accept", false: "reject"}[accept],
			ObjectType: "plan", ObjectID: planID.String(), Reason: reasonPtr(reason), OccurredAt: now,
		}); err != nil {
			return err
		}
		out = Plan{ID: planID, Status: map[bool]string{true: "accepted", false: "active"}[accept]}
		return nil
	})
	return out, err
}

// ReopenPlan: plan owner reopens an accepted plan with a reason (03 §7).
func (s *Service) ReopenPlan(ctx context.Context, requester, projectID, planID uuid.UUID, acceptanceID uuid.UUID, reason string) (Plan, error) {
	if reason == "" {
		return Plan{}, apierrors.Fields("reason", "required")
	}
	var out Plan
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockProjectForUpdate(ctx, projectID.String()); err != nil {
			return err
		}
		if _, err := memberTx(ctx, tx, projectID, requester); err != nil {
			return err
		}
		var status string
		if err := tx.QueryRow(ctx, `SELECT status FROM plans WHERE id=$1 AND project_id=$2 FOR UPDATE`,
			planID, projectID).Scan(&status); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apierrors.New(apierrors.NotFound, "plan not found")
			}
			return apierrors.New(apierrors.Internal, "plan lookup failed").Wrap(err)
		}
		if status != "accepted" {
			return apierrors.New(apierrors.InvalidTransition, "only accepted plans reopen")
		}
		now := s.now()
		if _, err := tx.Exec(ctx, `UPDATE plans SET status='active', version=version+1, updated_at=$2 WHERE id=$1`,
			planID, now); err != nil {
			return apierrors.New(apierrors.Internal, "plan reopen failed").Wrap(err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO reopen_records (project_id, id, target_type, target_id, previous_acceptance_id, reason, actor_user_id, created_at)
			VALUES ($1,$2,'plan',$3,$4,$5,$6,$7)`,
			projectID, uuid.New(), planID, acceptanceID, reason, requester, now); err != nil {
			return apierrors.New(apierrors.Internal, "reopen record failed").Wrap(err)
		}
		if _, err := events.AppendProjectEvent(ctx, tx, projectID, "plan.changed", "plan", planID.String(), nil,
			map[string]any{"change": "reopened"}, now); err != nil {
			return err
		}
		if err := audit.Append(ctx, tx, audit.Entry{
			ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
			Source: audit.SourceWeb, Operation: "plan.reopen",
			ObjectType: "plan", ObjectID: planID.String(), Reason: &reason, OccurredAt: now,
		}); err != nil {
			return err
		}
		out = Plan{ID: planID, Status: "active"}
		return nil
	})
	return out, err
}

func reasonPtr(reason string) *string {
	if reason == "" {
		return nil
	}
	return &reason
}

func nullableNullUUID(n uuid.NullUUID) any {
	if !n.Valid {
		return nil
	}
	return n.UUID
}

func nullUUIDPtr(n uuid.NullUUID) *uuid.UUID {
	if !n.Valid {
		return nil
	}
	return &n.UUID
}
