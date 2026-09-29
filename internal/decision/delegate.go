// SPDX-License-Identifier: Apache-2.0

package decision

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/kakj-go/Judex/internal/platform/paging"
	"github.com/kakj-go/Judex/internal/work"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	"github.com/kakj-go/Judex/internal/job"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/events"
)

// Delegate covers only the explicitly selected pending seats. The project owner
// is the project's designated responsible person; manager membership alone is
// not a delegation grant.
func (s *Service) Delegate(ctx context.Context, requester, projectID, proposalID uuid.UUID, reviewHash, reason string, slotIDs ...uuid.UUID) (Review, error) {
	var out Review
	if reason == "" {
		return out, apierrors.Fields("reason", "required")
	}
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, projectID.String()); err != nil {
			return err
		}
		role, err := memberTx(ctx, tx, projectID, requester)
		if err != nil {
			return err
		}

		if len(slotIDs) == 0 {
			if role != "owner" {
				return apierrors.New(apierrors.Forbidden, "explicit delegation required")
			}
			return apierrors.Fields("slotIds", "required")
		}
		var status string
		var reviewID uuid.UUID
		var hash string
		var deadline, first *time.Time
		var timeout int
		err = tx.QueryRow(ctx, `SELECT p.status,v.id,v.review_hash,v.deadline_at,v.first_approval_at,v.timeout_seconds_snapshot FROM proposals p JOIN proposal_versions v ON v.id=p.current_review_id WHERE p.id=$1 AND p.project_id=$2 FOR UPDATE OF p`, proposalID, projectID).Scan(&status, &reviewID, &hash, &deadline, &first, &timeout)
		if err != nil {
			return apierrors.New(apierrors.NotFound, "pending review not found")
		}
		if status != "pending" {
			return apierrors.New(apierrors.InvalidTransition, "proposal is "+status)
		}
		if hash != reviewHash {
			return apierrors.New(apierrors.ReviewStale, "review changed")
		}
		if deadline != nil && !s.now().Before(*deadline) {
			if err = s.SettleTimeout(ctx, projectID, proposalID, reviewID); err != nil {
				return err
			}
			out, err = s.GetReview(ctx, requester, projectID, proposalID)
			return err
		}
		allowed := map[uuid.UUID]bool{}
		if role != "owner" {
			var raw []byte
			if err = tx.QueryRow(ctx, `SELECT review_manifest_json FROM proposal_versions WHERE id=$1`, reviewID).Scan(&raw); err != nil {
				return err
			}
			var manifest struct {
				Constraints []work.WorkflowConstraint `json:"workflowConstraints"`
			}
			if err = json.Unmarshal(raw, &manifest); err != nil {
				return err
			}
			for _, constraint := range manifest.Constraints {
				current, e := work.LoadWorkflowConstraint(ctx, tx, projectID, constraint.WorkflowID, constraint.NodeID)
				if e != nil {
					return e
				}
				if current.Hash != constraint.Hash {
					return apierrors.New(apierrors.ReviewStale, "delegation policy changed")
				}
				if containsUUID(constraint.DelegationUserIDs, requester) {
					for _, identity := range constraint.IdentityIDs {
						allowed[identity] = true
					}
				}
			}
		}
		selected := map[uuid.UUID]bool{}
		delegatedFor := []map[string]any{}
		for _, id := range slotIDs {
			if selected[id] {
				return apierrors.Fields("slotIds", "duplicate")
			}
			selected[id] = true
			var state, authority string
			var authorityID uuid.UUID
			if err = tx.QueryRow(ctx, `SELECT state,authority_type,authority_id FROM approval_slots WHERE id=$1 AND review_id=$2 FOR UPDATE`, id, reviewID).Scan(&state, &authority, &authorityID); err != nil {
				return apierrors.New(apierrors.InvalidReference, "selected seat outside review")
			}
			if role != "owner" && (authority != "identity" || !allowed[authorityID]) {
				return apierrors.New(apierrors.Forbidden, "selected seat is outside delegated node authority")
			}
			if state != "pending" {
				return apierrors.New(apierrors.InvalidTransition, "only pending seats can be delegated")
			}
			delegatedFor = append(delegatedFor, map[string]any{"slotId": id, "authorityType": authority, "authorityId": authorityID})
		}
		now := s.now()
		decisionID := uuid.New()
		details, _ := json.Marshal(delegatedFor)
		if _, err = tx.Exec(ctx, `INSERT INTO approval_decisions(project_id,id,review_id,decision,actor_user_id,acting_bindings_json,decision_source,reason,decided_at) VALUES($1,$2,$3,'approve',$4,$5,'delegate',$6,$7)`, projectID, decisionID, reviewID, requester, details, reason, now); err != nil {
			return err
		}
		for _, id := range slotIDs {
			if _, err = tx.Exec(ctx, `INSERT INTO approval_decision_slots(decision_id,slot_id) VALUES($1,$2)`, decisionID, id); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `UPDATE approval_slots SET state='delegated' WHERE id=$1`, id); err != nil {
				return err
			}
		}
		if first == nil {
			until := now.Add(time.Duration(timeout) * time.Second)
			if _, err = tx.Exec(ctx, `UPDATE proposal_versions SET first_approval_at=$2,deadline_at=$3 WHERE id=$1`, reviewID, now, until); err != nil {
				return err
			}
			if _, err = job.Enqueue(ctx, tx, "proposal.timeout", map[string]any{"projectId": projectID, "proposalId": proposalID, "reviewId": reviewID}, strPtr("review:"+reviewID.String()), until, now); err != nil {
				return err
			}
		}
		var remaining int
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM approval_slots WHERE review_id=$1 AND state NOT IN ('approved','delegated','timeout')`, reviewID).Scan(&remaining); err != nil {
			return err
		}
		if remaining == 0 {
			if _, err = s.applyReviewed(ctx, tx, projectID, requester, reviewID); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `UPDATE proposals SET status='approved',updated_at=$2 WHERE id=$1`, proposalID, now); err != nil {
				return err
			}
		}
		if _, err = events.AppendProjectEvent(ctx, tx, projectID, "proposal.changed", "proposal", proposalID.String(), nil, map[string]any{"change": "delegated", "slotIds": slotIDs}, now); err != nil {
			return err
		}
		if err = audit.Append(ctx, tx, audit.Entry{ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester, Source: audit.SourceDelegate, Operation: "proposal.delegate", ObjectType: "proposal", ObjectID: proposalID.String(), ReviewID: &reviewID, Reason: &reason, OccurredAt: now}); err != nil {
			return err
		}
		out, err = s.GetReview(ctx, requester, projectID, proposalID)
		return err
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
		if err := tx.LockActiveProject(ctx, projectID.String()); err != nil {
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
	role, err := memberTx(ctx, s.pool, projectID, requester)
	if err != nil {
		return Review{}, err
	}
	var (
		reviewID    uuid.NullUUID
		status      string
		reviewHash  *string
		submitted   *time.Time
		firstAt     *time.Time
		deadline    *time.Time
		changesRaw  []byte
		createdRaw  []byte
		manifestRaw []byte
		version     int64
	)
	err = s.pool.QueryRow(ctx, `
		SELECT v.id, p.status, v.review_hash, v.submitted_at, v.first_approval_at,
		       v.deadline_at, v.changes_json,v.created_ids_json,v.review_manifest_json,p.version
		FROM proposals p LEFT JOIN proposal_versions v ON v.id=COALESCE(p.current_review_id,(SELECT id FROM proposal_versions WHERE proposal_id=p.id ORDER BY revision DESC LIMIT 1))
		WHERE p.id=$1 AND p.project_id=$2`, proposalID, projectID).
		Scan(&reviewID, &status, &reviewHash, &submitted, &firstAt, &deadline, &changesRaw, &createdRaw, &manifestRaw, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return Review{}, apierrors.New(apierrors.NotFound, "proposal not found")
	}
	if err != nil {
		return Review{}, apierrors.New(apierrors.Internal, "review lookup failed").Wrap(err)
	}
	out := Review{ProposalID: proposalID, Status: status, Version: version,
		SubmittedAt: submitted, FirstApprovalAt: firstAt, DeadlineAt: deadline}
	_ = json.Unmarshal(createdRaw, &out.CreatedIDs)
	var frozen struct {
		Constraints []work.WorkflowConstraint `json:"workflowConstraints"`
	}
	_ = json.Unmarshal(manifestRaw, &frozen)
	out.WorkflowConstraints = frozen.Constraints
	delegated := map[uuid.UUID]bool{}
	for _, constraint := range frozen.Constraints {
		if containsUUID(constraint.DelegationUserIDs, requester) {
			for _, id := range constraint.IdentityIDs {
				delegated[id] = true
			}
		}
	}
	if reviewID.Valid {
		out.ReviewID = reviewID.UUID
	}
	if reviewHash != nil {
		out.ReviewHash = *reviewHash
	}
	if changesRaw != nil {
		_ = json.Unmarshal(changesRaw, &out.Changes)
	}
	if status == "draft" {
		err = s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
			specs, err := s.computeSlots(ctx, tx, projectID, out.Changes, requester)
			if err != nil {
				return err
			}
			var timeout int
			if err = tx.QueryRow(ctx, `SELECT approval_timeout_seconds FROM projects WHERE id=$1`, projectID).Scan(&timeout); err != nil {
				return err
			}
			constraints, err := work.ChangeWorkflowConstraints(ctx, tx, projectID, out.Changes)
			if err != nil {
				return err
			}
			out.WorkflowConstraints = constraints
			out.ReviewHash = canonicalHash(out.Changes, specs, timeout, out.ReviewID, constraints)
			for _, spec := range specs {
				out.Slots = append(out.Slots, Slot{AuthorityType: spec.AuthorityType, AuthorityID: spec.AuthorityID, DisplayName: spec.DisplayName, PositionName: spec.PositionName, State: "pending"})
			}
			return nil
		})
		return out, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT s.id, s.authority_type, s.authority_id, s.state,
		       COALESCE(hu.display_name, COALESCE(u.display_name,'')),
		       (SELECT name FROM position_templates t WHERE t.id=i.template_id), CASE WHEN s.authority_type='user' THEN s.authority_id ELSE b.user_id END,COALESCE(i.current_binding_version,0)
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
			&slot.DisplayName, &slot.PositionName, &slot.CurrentUserID, &slot.BindingVersion); err != nil {
			return Review{}, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		slot.CanDelegate = status == "pending" && slot.State == "pending" && (role == "owner" || (slot.AuthorityType == "identity" && delegated[slot.AuthorityID]))
		slot.CanDecide = slot.State == "pending" && slot.CurrentUserID != nil && *slot.CurrentUserID == requester
		out.Slots = append(out.Slots, slot)
	}
	return out, rows.Err()
}

// ListProposals pages proposals for the project (06 §5).
func (s *Service) ListProposals(ctx context.Context, requester, projectID uuid.UUID) ([]map[string]any, error) {
	if _, err := memberTx(ctx, s.pool, projectID, requester); err != nil {
		return nil, err
	}
	rows, err := paging.Query(ctx, s.pool, `
		SELECT id, kind, status, current_review_id, version, created_at /*keys*/
		FROM proposals WHERE project_id=$1 /*page*/`, "created_at", "id", projectID)
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

func (h TimeoutJobHandler) Kind() string     { return "proposal.timeout" }
func (h TimeoutJobHandler) MaxAttempts() int { return 5 }
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
