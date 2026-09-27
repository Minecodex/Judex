// SPDX-License-Identifier: Apache-2.0

package decision

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	"github.com/kakj-go/Judex/internal/job"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/events"
)

// Delegate lets a project owner/manager settle still-PENDING seats on
// behalf of holders (03 §4 代批): only pending seats are covered, never an
// established reject; the real actor and delegated seats are recorded
// (decision_source=delegate, actor != 原审批人). Completing the quorum
// applies the change group in the same transaction.
func (s *Service) Delegate(ctx context.Context, requester, projectID, proposalID uuid.UUID, reviewHash, reason string) (Review, error) {
	if reason == "" {
		return Review{}, apierrors.Fields("reason", "required")
	}
	var out Review
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockProjectForUpdate(ctx, projectID.String()); err != nil {
			return err
		}
		role, err := memberTx(ctx, tx, projectID, requester)
		if err != nil {
			return err
		}
		if role != "owner" && role != "manager" {
			return apierrors.New(apierrors.Forbidden, "代批需要项目负责人或管理者权限")
		}
		var (
			status   string
			reviewID uuid.UUID
		)
		if err := tx.QueryRow(ctx, `
			SELECT status, current_review_id FROM proposals WHERE id=$1 AND project_id=$2 FOR UPDATE`,
			proposalID, projectID).Scan(&status, &reviewID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apierrors.New(apierrors.NotFound, "proposal not found")
			}
			return apierrors.New(apierrors.Internal, "proposal lookup failed").Wrap(err)
		}
		if status != "pending" {
			return apierrors.New(apierrors.InvalidTransition, "proposal is "+status)
		}
		var hash *string
		if err := tx.QueryRow(ctx, `SELECT review_hash FROM proposal_versions WHERE id=$1`, reviewID).Scan(&hash); err != nil {
			return apierrors.New(apierrors.Internal, "review lookup failed").Wrap(err)
		}
		if hash == nil || *hash != reviewHash {
			return apierrors.New(apierrors.ReviewStale, "review hash mismatch")
		}
		if _, err := tx.Exec(ctx, `SELECT 1 FROM approval_slots WHERE review_id=$1 AND state='pending' FOR UPDATE`, reviewID); err != nil {
			return apierrors.New(apierrors.Internal, "slots lock failed").Wrap(err)
		}
		rows, err := tx.Query(ctx, `
			SELECT id, state FROM approval_slots WHERE review_id=$1`, reviewID)
		if err != nil {
			return apierrors.New(apierrors.Internal, "slots query failed").Wrap(err)
		}
		var all, pending []uuid.UUID
		for rows.Next() {
			var id uuid.UUID
			var state string
			if err := rows.Scan(&id, &state); err != nil {
				rows.Close()
				return apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
			}
			all = append(all, id)
			if state == "pending" {
				pending = append(pending, id)
			}
		}
		rows.Close()
		if len(pending) == 0 {
			return apierrors.New(apierrors.InvalidTransition, "没有可代批的待回应职责位")
		}
		now := s.now()
		decisionID := uuid.New()
		if _, err := tx.Exec(ctx, `
			INSERT INTO approval_decisions (project_id, id, review_id, decision, actor_user_id,
				acting_bindings_json, decision_source, reason, decided_at)
			VALUES ($1,$2,$3,'approve',$4,'[]','delegate',$5,$6)`,
			projectID, decisionID, reviewID, requester, reason, now); err != nil {
			return apierrors.New(apierrors.Internal, "delegate insert failed").Wrap(err)
		}
		for _, slotID := range pending {
			if _, err := tx.Exec(ctx, `
				INSERT INTO approval_decision_slots (decision_id, slot_id) VALUES ($1,$2)`, decisionID, slotID); err != nil {
				return apierrors.New(apierrors.Internal, "decision slot failed").Wrap(err)
			}
			if _, err := tx.Exec(ctx, `UPDATE approval_slots SET state='delegated' WHERE id=$1`, slotID); err != nil {
				return apierrors.New(apierrors.Internal, "slot update failed").Wrap(err)
			}
		}
		// Delegation counts as approval: quorum complete -> apply now.
		created, err := s.applyChanges(ctx, tx, projectID, requester, reviewID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE proposals SET status='approved', updated_at=$2 WHERE id=$1`, proposalID, now); err != nil {
			return apierrors.New(apierrors.Internal, "approve failed").Wrap(err)
		}
		if _, err := events.AppendProjectEvent(ctx, tx, projectID, "proposal.changed", "proposal", proposalID.String(), nil,
			map[string]any{"change": "delegated_applied", "createdIds": created}, now); err != nil {
			return err
		}
		out = Review{ReviewID: reviewID, ReviewHash: reviewHash, ProposalID: proposalID, CreatedIDs: created}
		return audit.Append(ctx, tx, audit.Entry{
			ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
			Source: audit.SourceDelegate, Operation: "proposal.delegate",
			ObjectType: "proposal", ObjectID: proposalID.String(), ReviewID: &reviewID,
			Reason: &reason, OccurredAt: now,
		})
	})
	return out, err
}

// CreateRevision turns a terminal proposal back into a NEW draft revision
// (03 §2 修订): the old tickets are never inherited; resubmitting computes a
// fresh review hash and slot set.
func (s *Service) CreateRevision(ctx context.Context, requester, projectID, proposalID uuid.UUID, changes []Change, reason string) (int64, error) {
	if reason == "" {
		return 0, apierrors.Fields("reason", "required")
	}
	for i, change := range changes {
		if !allowedOperations[change.Operation] {
			return 0, apierrors.Newf(apierrors.Validation, "changes[%d].operation not whitelisted", i)
		}
	}
	var revision int64
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockProjectForUpdate(ctx, projectID.String()); err != nil {
			return err
		}
		if _, err := memberTx(ctx, tx, projectID, requester); err != nil {
			return err
		}
		var status string
		var createdBy uuid.UUID
		if err := tx.QueryRow(ctx, `
			SELECT status, created_by FROM proposals WHERE id=$1 AND project_id=$2 FOR UPDATE`,
			proposalID, projectID).Scan(&status, &createdBy); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apierrors.New(apierrors.NotFound, "proposal not found")
			}
			return apierrors.New(apierrors.Internal, "proposal lookup failed").Wrap(err)
		}
		if status == "pending" {
			return apierrors.New(apierrors.InvalidTransition, "pending proposal cannot be revised; reject it first")
		}
		if createdBy != requester {
			return apierrors.New(apierrors.Forbidden, "只有发起人可以修订")
		}
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(max(revision),0)+1 FROM proposal_versions WHERE proposal_id=$1`,
			proposalID).Scan(&revision); err != nil {
			return apierrors.New(apierrors.Internal, "revision lookup failed").Wrap(err)
		}
		raw, _ := json.Marshal(changes)
		if _, err := tx.Exec(ctx, `
			INSERT INTO proposal_versions (project_id, id, proposal_id, revision, changes_json, created_at)
			VALUES ($1,$2,$3,$4,$5,$6)`,
			projectID, uuid.New(), proposalID, revision, raw, s.now()); err != nil {
			return apierrors.New(apierrors.Internal, "revision insert failed").Wrap(err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE proposals SET status='draft', current_review_id=NULL, version=version+1,
				previous_proposal_id=previous_proposal_id, updated_at=$2 WHERE id=$1`,
			proposalID, s.now()); err != nil {
			return apierrors.New(apierrors.Internal, "proposal reset failed").Wrap(err)
		}
		return audit.Append(ctx, tx, audit.Entry{
			ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
			Source: audit.SourceWeb, Operation: "proposal.revision.create",
			ObjectType: "proposal", ObjectID: proposalID.String(),
			Reason: &reason, OccurredAt: s.now(),
		})
	})
	return revision, err
}

// GetReview loads the frozen review for display (06 §5).
func (s *Service) GetReview(ctx context.Context, requester, projectID, proposalID uuid.UUID) (Review, error) {
	if _, err := memberTx(ctx, s.pool, projectID, requester); err != nil {
		return Review{}, err
	}
	var (
		reviewID   uuid.NullUUID
		status     string
		reviewHash *string
		submitted  *time.Time
		firstAt    *time.Time
		deadline   *time.Time
		changesRaw []byte
	)
	err := s.pool.QueryRow(ctx, `
		SELECT p.current_review_id, p.status, v.review_hash, v.submitted_at, v.first_approval_at,
		       v.deadline_at, v.changes_json
		FROM proposals p LEFT JOIN proposal_versions v ON v.id=p.current_review_id
		WHERE p.id=$1 AND p.project_id=$2`, proposalID, projectID).
		Scan(&reviewID, &status, &reviewHash, &submitted, &firstAt, &deadline, &changesRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return Review{}, apierrors.New(apierrors.NotFound, "proposal not found")
	}
	if err != nil {
		return Review{}, apierrors.New(apierrors.Internal, "review lookup failed").Wrap(err)
	}
	out := Review{ProposalID: proposalID, Status: status,
		SubmittedAt: submitted, FirstApprovalAt: firstAt, DeadlineAt: deadline}
	if reviewID.Valid {
		out.ReviewID = reviewID.UUID
	}
	if reviewHash != nil {
		out.ReviewHash = *reviewHash
	}
	if changesRaw != nil {
		_ = json.Unmarshal(changesRaw, &out.Changes)
	}
	rows, err := s.pool.Query(ctx, `
		SELECT s.id, s.authority_type, s.authority_id, s.state,
		       COALESCE(u.display_name, COALESCE(hu.display_name,'')),
		       (SELECT name FROM position_templates t WHERE t.id=i.template_id)
		FROM approval_slots s
		LEFT JOIN agent_identities i ON i.id=s.authority_id AND s.authority_type='identity'
		LEFT JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version
		LEFT JOIN users u ON u.id=s.initial_user_id
		LEFT JOIN users hu ON hu.id=b.user_id
		WHERE s.review_id=$1`, out.ReviewID)
	if err != nil {
		return Review{}, apierrors.New(apierrors.Internal, "slots query failed").Wrap(err)
	}
	defer rows.Close()
	for rows.Next() {
		var slot Slot
		if err := rows.Scan(&slot.ID, &slot.AuthorityType, &slot.AuthorityID, &slot.State,
			&slot.DisplayName, &slot.PositionName); err != nil {
			return Review{}, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		out.Slots = append(out.Slots, slot)
	}
	return out, rows.Err()
}

// ListProposals pages proposals for the project (06 §5).
func (s *Service) ListProposals(ctx context.Context, requester, projectID uuid.UUID) ([]map[string]any, error) {
	if _, err := memberTx(ctx, s.pool, projectID, requester); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, kind, status, current_review_id, version, created_at
		FROM proposals WHERE project_id=$1 ORDER BY created_at DESC LIMIT 100`, projectID)
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "proposals failed").Wrap(err)
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var (
			id       uuid.UUID
			kind     string
			status   string
			reviewID uuid.NullUUID
			version  int64
			created  time.Time
		)
		if err := rows.Scan(&id, &kind, &status, &reviewID, &version, &created); err != nil {
			return nil, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		out = append(out, map[string]any{
			"id": id, "kind": kind, "status": status,
			"currentReviewId": reviewID.UUID, "version": version, "createdAt": created,
		})
	}
	return out, rows.Err()
}

// TimeoutJobHandler adapts SettleTimeout to the job engine (P3-05 挂载).
type TimeoutJobHandler struct {
	Service *Service
}

func (h TimeoutJobHandler) Kind() string        { return "proposal.timeout" }
func (h TimeoutJobHandler) MaxAttempts() int    { return 5 }
func (h TimeoutJobHandler) Execute(ctx context.Context, j job.Job) error {
	payload := struct {
		ProjectID  string `json:"projectId"`
		ProposalID string `json:"proposalId"`
		ReviewID   string `json:"reviewId"`
	}{}
	if err := json.Unmarshal(j.Payload, &payload); err != nil {
		return apierrors.New(apierrors.Validation, "bad payload").Wrap(err)
	}
	projectID, err1 := uuid.Parse(payload.ProjectID)
	proposalID, err2 := uuid.Parse(payload.ProposalID)
	reviewID, err3 := uuid.Parse(payload.ReviewID)
	if err1 != nil || err2 != nil || err3 != nil {
		return apierrors.New(apierrors.Validation, "bad payload ids")
	}
	return h.Service.SettleTimeout(ctx, projectID, proposalID, reviewID)
}
