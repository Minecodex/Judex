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
	"github.com/kakj-go/Judex/internal/work"
)

// Decide records one human decision covering the slots the requester
// currently holds (03 §3): acting binding versions must be current; the
// final approving decision applies the whole change group atomically; any
// reject cancels the proposal and stops its timer.
func (s *Service) Decide(ctx context.Context, requester, projectID, proposalID uuid.UUID, reviewHash string, approve bool, reason string, selection ...DecisionSelection) (Review, error) {
	var out Review
	if !approve && reason == "" {
		return out, apierrors.Fields("reason", "required")
	}
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, projectID.String()); err != nil {
			return err
		}
		if _, err := memberTx(ctx, tx, projectID, requester); err != nil {
			return apierrors.New(apierrors.Forbidden, "active membership required")
		}
		var (
			status   string
			reviewID uuid.UUID
			revHash  *string
			firstAt  *time.Time
			deadline *time.Time
			topicID  *uuid.UUID
		)
		if err := tx.QueryRow(ctx, `
			SELECT status, current_review_id, version, topic_id FROM proposals
			WHERE id=$1 AND project_id=$2 FOR UPDATE`, proposalID, projectID).
			Scan(&status, &reviewID, new(int64), &topicID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apierrors.New(apierrors.NotFound, "proposal not found")
			}
			return apierrors.New(apierrors.Internal, "proposal lookup failed").Wrap(err)
		}
		_ = topicID
		if status != "pending" {
			return apierrors.New(apierrors.InvalidTransition, "proposal is "+status)
		}
		if err := tx.QueryRow(ctx, `
			SELECT review_hash, first_approval_at, deadline_at FROM proposal_versions WHERE id=$1`,
			reviewID).Scan(&revHash, &firstAt, &deadline); err != nil {
			return apierrors.New(apierrors.Internal, "review lookup failed").Wrap(err)
		}
		if revHash == nil || *revHash != reviewHash {
			return apierrors.New(apierrors.ReviewStale, "review hash mismatch")
		}
		if deadline != nil && !s.now().Before(*deadline) {
			if err := s.SettleTimeout(ctx, projectID, proposalID, reviewID); err != nil {
				return err
			}
			var err error
			out, err = s.GetReview(ctx, requester, projectID, proposalID)
			return err
		}
		// Lock the seats with a join-free FOR UPDATE (FOR UPDATE combined with
		// LEFT JOIN silently returns no rows in PostgreSQL), then read the
		// eligibility projection without locking — the project row lock above
		// already serializes concurrent decisions.
		if _, err := tx.Exec(ctx, `SELECT 1 FROM approval_slots WHERE review_id=$1 FOR UPDATE`, reviewID); err != nil {
			return apierrors.New(apierrors.Internal, "slots lock failed").Wrap(err)
		}
		rows, err := tx.Query(ctx, `
			SELECT s.id, s.authority_type, s.authority_id, s.state,
			       COALESCE(b.user_id::text,''), COALESCE(u.display_name,''),
			       (SELECT name FROM position_templates t WHERE t.id=i.template_id),COALESCE(i.current_binding_version,0)
			FROM approval_slots s
			LEFT JOIN agent_identities i ON i.id=s.authority_id AND s.authority_type='identity'
			LEFT JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version
			LEFT JOIN users u ON u.id=b.user_id
			WHERE s.review_id=$1`, reviewID)
		if err != nil {
			return apierrors.New(apierrors.Internal, "slots query failed").Wrap(err)
		}
		type slotRow struct {
			id             uuid.UUID
			authority      string
			authorityID    uuid.UUID
			state          string
			holder         string
			displayName    string
			positionName   *string
			bindingVersion int64
		}
		var slots []slotRow
		for rows.Next() {
			var sr slotRow
			if err := rows.Scan(&sr.id, &sr.authority, &sr.authorityID, &sr.state, &sr.holder, &sr.displayName, &sr.positionName, &sr.bindingVersion); err != nil {
				rows.Close()
				return apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
			}
			slots = append(slots, sr)
		}
		rows.Close()
		selected := map[uuid.UUID]bool{}
		bindings := map[uuid.UUID]int64{}
		if len(selection) > 0 {
			for _, id := range selection[0].SlotIDs {
				selected[id] = true
			}
			for _, b := range selection[0].Bindings {
				bindings[b.IdentityID] = b.BindingVersion
			}
		}
		acting := []ActingBinding{}
		var covered []uuid.UUID
		for _, sr := range slots {

			if len(selection) > 0 && !selected[sr.id] {
				continue
			}
			if sr.state != "pending" {
				continue
			}
			if sr.authority == "user" && sr.authorityID == requester {
				covered = append(covered, sr.id)
			} else if sr.authority == "identity" && sr.holder == requester.String() {
				if len(selection) > 0 && bindings[sr.authorityID] != sr.bindingVersion {
					return apierrors.New(apierrors.ReviewStale, "acting binding changed")
				}
				acting = append(acting, ActingBinding{IdentityID: sr.authorityID, BindingVersion: sr.bindingVersion})
				covered = append(covered, sr.id)
			}
		}
		if len(covered) == 0 || (len(selection) > 0 && len(covered) != len(selected)) {
			return apierrors.New(apierrors.Forbidden, "当前登录用户不持有任何待审批职责位")
		}
		now := s.now()
		decisionID := uuid.New()
		bindingJSON, _ := json.Marshal(acting)
		source := "human_web"
		if audit.ContextSource(ctx) == audit.SourceCLI {
			source = "human_cli"
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO approval_decisions (project_id, id, review_id, decision, actor_user_id,
				acting_bindings_json, decision_source, reason, decided_at)
			VALUES ($1,$2,$3,$4,$5,$9::jsonb,$6,$7,$8)`,
			projectID, decisionID, reviewID, approveText(approve), requester, source, nullable(reason), now, bindingJSON); err != nil {
			return apierrors.New(apierrors.Internal, "decision insert failed").Wrap(err)
		}
		for _, slotID := range covered {
			if _, err := tx.Exec(ctx, `
				INSERT INTO approval_decision_slots (decision_id, slot_id) VALUES ($1,$2)`, decisionID, slotID); err != nil {
				return apierrors.New(apierrors.Internal, "decision slot failed").Wrap(err)
			}
			state := "approved"
			if !approve {
				state = "rejected"
			}
			if _, err := tx.Exec(ctx, `UPDATE approval_slots SET state=$2 WHERE id=$1`, slotID, state); err != nil {
				return apierrors.New(apierrors.Internal, "slot update failed").Wrap(err)
			}
		}
		if !approve {
			if _, err := tx.Exec(ctx, `UPDATE proposals SET status='cancelled', updated_at=$2 WHERE id=$1`, proposalID, now); err != nil {
				return apierrors.New(apierrors.Internal, "cancel failed").Wrap(err)
			}
			if _, err := tx.Exec(ctx, `UPDATE proposal_versions SET deadline_at=NULL WHERE id=$1`, reviewID); err != nil {
				return apierrors.New(apierrors.Internal, "deadline clear failed").Wrap(err)
			}
			if _, err := events.AppendProjectEvent(ctx, tx, projectID, "proposal.changed", "proposal", proposalID.String(), nil,
				map[string]any{"change": "rejected"}, now); err != nil {
				return err
			}
			return audit.Append(ctx, tx, audit.Entry{
				ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
				Source: audit.SourceWeb, Operation: "proposal.reject",
				ObjectType: "proposal", ObjectID: proposalID.String(), ReviewID: &reviewID,
				Reason: strPtrOrNil(reason), OccurredAt: now,
			})
		}
		// First approval starts the countdown (03 §4).
		if firstAt == nil {
			var timeout int
			if err := tx.QueryRow(ctx, `SELECT timeout_seconds_snapshot FROM proposal_versions WHERE id=$1`, reviewID).Scan(&timeout); err != nil {
				return err
			}
			deadlineAt := now.Add(time.Duration(timeout) * time.Second)
			if _, err := tx.Exec(ctx, `
				UPDATE proposal_versions SET first_approval_at=$2, deadline_at=$3 WHERE id=$1`,
				reviewID, now, deadlineAt); err != nil {
				return apierrors.New(apierrors.Internal, "first approval failed").Wrap(err)
			}
			// Persistent timeout job — at-least-once wake; correctness lives in
			// the settlement command itself.
			if _, err := job.Enqueue(ctx, tx, "proposal.timeout", map[string]any{
				"projectId": projectID.String(), "proposalId": proposalID.String(), "reviewId": reviewID.String(),
			}, strPtr("review:"+reviewID.String()), deadlineAt, now); err != nil {
				return apierrors.New(apierrors.Internal, "timeout enqueue failed").Wrap(err)
			}
		}
		// All seats satisfied? Apply atomically NOW.
		pending := 0
		for _, sr := range slots {
			if sr.state == "pending" && !containsUUID(covered, sr.id) {
				pending++
			}
		}
		if pending == 0 {
			created, err := s.applyReviewed(ctx, tx, projectID, requester, reviewID)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE proposals SET status='approved', updated_at=$2 WHERE id=$1`, proposalID, now); err != nil {
				return apierrors.New(apierrors.Internal, "approve failed").Wrap(err)
			}
			if _, err := events.AppendProjectEvent(ctx, tx, projectID, "proposal.changed", "proposal", proposalID.String(), nil,
				map[string]any{"change": "approved", "createdIds": created}, now); err != nil {
				return err
			}
			out.CreatedIDs = created
		}
		out.ReviewID = reviewID
		out.ReviewHash = reviewHash
		out.ProposalID = proposalID
		return audit.Append(ctx, tx, audit.Entry{
			ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
			Source: audit.SourceWeb, Operation: "proposal.approve",
			ObjectType: "proposal", ObjectID: proposalID.String(), ReviewID: &reviewID, OccurredAt: now,
		})
	})
	if err == nil {
		return s.GetReview(ctx, requester, projectID, proposalID)
	}
	return out, err
}

func approveText(ok bool) string {
	if ok {
		return "approve"
	}
	return "reject"
}

func nullable(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func strPtr(v string) *string { return &v }

func containsUUID(list []uuid.UUID, v uuid.UUID) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

// applyChanges executes the frozen change group in THIS transaction — a
// single failure rolls the whole application back (03 §3 原子应用).
func (s *Service) applyChanges(ctx context.Context, tx pgx.Tx, projectID, actor uuid.UUID, reviewID uuid.UUID) (map[string]string, error) {
	var changesRaw []byte
	if err := tx.QueryRow(ctx, `SELECT changes_json FROM proposal_versions WHERE id=$1`, reviewID).
		Scan(&changesRaw); err != nil {
		return nil, apierrors.New(apierrors.Internal, "changes load failed").Wrap(err)
	}
	var changes []Change
	if err := json.Unmarshal(changesRaw, &changes); err != nil {
		return nil, apierrors.New(apierrors.Internal, "changes parse failed").Wrap(err)
	}
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT review_manifest_json FROM proposal_versions WHERE id=$1`, reviewID).Scan(&raw); err != nil {
		return nil, err
	}
	var manifest struct {
		Constraints []work.WorkflowConstraint `json:"workflowConstraints"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, err
	}
	current, err := work.ChangeWorkflowConstraints(ctx, tx, projectID, changes)
	if err != nil {
		return nil, err
	}
	if len(current) != len(manifest.Constraints) {
		return nil, apierrors.New(apierrors.ReviewStale, "workflow references changed")
	}
	for i, ref := range current {
		old := manifest.Constraints[i]
		if ref.WorkflowID != old.WorkflowID || ref.NodeID != old.NodeID || ref.Hash != old.Hash {
			return nil, apierrors.New(apierrors.ReviewStale, "workflow constraints changed")
		}
	}
	return work.ApplyReviewedChanges(ctx, tx, projectID, actor, changes, reviewID)
}

// SettleTimeout is the deadline command (03 §4): locks project→proposal→
// slots, re-verifies pending + deadline, records timeout for un-responded
// seats (actor is NOT a human) and applies if that completes the quorum.
// Human decisions racing the timer are serialized by the project lock.
func (s *Service) SettleTimeout(ctx context.Context, projectID, proposalID, reviewID uuid.UUID) error {
	return s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, projectID.String()); err != nil {
			return err
		}
		var status string
		if err := tx.QueryRow(ctx, `SELECT status FROM proposals WHERE id=$1 AND project_id=$2 FOR UPDATE`,
			proposalID, projectID).Scan(&status); err != nil {
			return nil // gone: idempotent no-op
		}
		if status != "pending" {
			return nil
		}
		var deadline *time.Time
		if err := tx.QueryRow(ctx, `SELECT deadline_at FROM proposal_versions WHERE id=$1`, reviewID).Scan(&deadline); err != nil {
			return nil
		}
		if deadline == nil || s.now().Before(*deadline) {
			return nil // not due yet or never approved once
		}
		// Timeout every still-pending seat.
		if _, err := tx.Exec(ctx, `SELECT 1 FROM approval_slots WHERE review_id=$1 FOR UPDATE`, reviewID); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT id FROM approval_slots WHERE review_id=$1 AND state='pending'`, reviewID)
		if err != nil {
			return err
		}
		var pending []uuid.UUID
		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			pending = append(pending, id)
		}
		rows.Close()
		now := s.now()
		for _, slotID := range pending {
			if _, err := tx.Exec(ctx, `UPDATE approval_slots SET state='timeout' WHERE id=$1`, slotID); err != nil {
				return err
			}
		}
		if len(pending) > 0 {
			decisionID := uuid.New()
			if _, err := tx.Exec(ctx, `
				INSERT INTO approval_decisions (project_id, id, review_id, decision, actor_user_id,
					acting_bindings_json, decision_source, reason, decided_at)
				VALUES ($1,$2,$3,'approve',NULL,'[]','timeout','到期自动通过',$4)`,
				projectID, decisionID, reviewID, now); err != nil {
				return err
			}
		}
		created, err := s.applyReviewed(ctx, tx, projectID, uuid.Nil, reviewID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE proposals SET status='approved', updated_at=$2 WHERE id=$1`, proposalID, now); err != nil {
			return err
		}
		_, _ = events.AppendProjectEvent(ctx, tx, projectID, "proposal.changed", "proposal", proposalID.String(), nil,
			map[string]any{"change": "timeout_applied", "createdIds": created}, now)
		return audit.Append(ctx, tx, audit.Entry{
			ProjectID: &projectID, ActorType: audit.ActorSystem,
			Source: audit.SourceTimeout, Operation: "proposal.timeout.settle",
			ObjectType: "proposal", ObjectID: proposalID.String(), ReviewID: &reviewID, OccurredAt: now,
		})
	})
}

func strPtrOrNil(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

type ActingBinding struct {
	IdentityID     uuid.UUID `json:"identityId"`
	BindingVersion int64     `json:"bindingVersion"`
}
type DecisionSelection struct {
	SlotIDs  []uuid.UUID
	Bindings []ActingBinding
}
