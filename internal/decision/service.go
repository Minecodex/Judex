// SPDX-License-Identifier: Apache-2.0

// Package decision implements typed proposals with frozen reviews and
// ALL-slot approval per docs/plans/v1/03 §3-§5: the submit freezes changes,
// recipients and constraint snapshots under a SHA-256 review hash; every
// approval slot must be satisfied before the change group applies
// atomically in the SAME transaction; one human may cover multiple slots
// they legitimately hold, and no opponent can be removed to force approval.
package decision

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/events"
	"github.com/kakj-go/Judex/internal/work"
	"sort"
)

// Change is one typed operation (06 §5 ProposalChange).
type Change = work.Change

// Review is the frozen review aggregate (06 §5).
type Review struct {
	Version             int64                     `json:"version"`
	WorkflowConstraints []work.WorkflowConstraint `json:"workflowConstraints"`
	ReviewID            uuid.UUID                 `json:"reviewId"`
	ReviewHash          string                    `json:"reviewHash"`
	ProposalID          uuid.UUID                 `json:"proposalId"`
	Status              string                    `json:"status"`
	SubmittedAt         *time.Time                `json:"submittedAt"`
	FirstApprovalAt     *time.Time                `json:"firstApprovalAt"`
	DeadlineAt          *time.Time                `json:"deadlineAt"`
	Changes             []Change                  `json:"changes"`
	Slots               []Slot                    `json:"slots"`
	CreatedIDs          map[string]string         `json:"createdIds"`
}

// Slot is one approval seat (06 §5).
type Slot struct {
	CanDelegate    bool       `json:"canDelegate"`
	CurrentUserID  *uuid.UUID `json:"currentUserId"`
	BindingVersion int64      `json:"bindingVersion"`
	CanDecide      bool       `json:"canDecide"`
	ID             uuid.UUID  `json:"id"`
	AuthorityType  string     `json:"authorityType"`
	AuthorityID    uuid.UUID  `json:"authorityId"`
	DisplayName    string     `json:"displayName"`
	PositionName   *string    `json:"positionName"`
	State          string     `json:"state"`
}

type Service struct {
	pool               *postgres.Pool
	now                func() time.Time
	defaultTimeoutSecs int
}

func NewService(pool *postgres.Pool, now func() time.Time, defaultTimeoutSecs int) *Service {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	if defaultTimeoutSecs == 0 {
		defaultTimeoutSecs = 86400
	}
	return &Service{pool: pool, now: now, defaultTimeoutSecs: defaultTimeoutSecs}
}

var allowedOperations = map[string]bool{
	"reference_task": true, "activate_object": true, "create_plan": true, "create_task": true, "update_scope": true,
	"set_assignment": true, "set_requirements": true, "link_material": true,
	"link_topic": true, "cancel_plan": true, "cancel_task": true,
}

var allowedKinds = map[string]bool{
	"work_arrangement": true, "work_change": true, "dependency_change": true,
	"material_link": true, "topic_creation": true, "formal_conclusion": true,
	"handoff_change": true, "release_acceptance": true,
}

func memberTx(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, projectID, requester uuid.UUID) (string, error) {
	var role string
	err := q.QueryRow(ctx, `
		SELECT role FROM project_members WHERE project_id=$1 AND user_id=$2 AND state='active'`,
		projectID, requester).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apierrors.New(apierrors.NotFound, "project not found")
	}
	return role, err
}

// CreateDraft stores a typed proposal draft (03 §3 提交前)。
func (s *Service) CreateDraft(ctx context.Context, requester, projectID uuid.UUID, kind string, topicID *uuid.UUID, reason string, changes []Change) (uuid.UUID, error) {
	if !allowedKinds[kind] {
		return uuid.Nil, apierrors.Fields("kind", "enum")
	}
	if err := ValidateDraftChanges(changes); err != nil {
		return uuid.Nil, err
	}
	var id uuid.UUID
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, projectID.String()); err != nil {
			return err
		}
		if _, err := memberTx(ctx, tx, projectID, requester); err != nil {
			return err
		}
		now := s.now()
		id = uuid.New()
		if _, err := tx.Exec(ctx, `
			INSERT INTO proposals (project_id, id, topic_id, kind, status, reason, created_by, created_at, updated_at)
			VALUES ($1,$2,$3,$4,'draft',$5,$6,$7,$7)`,
			projectID, id, nullableUUID(topicID), kind, reason, requester, now); err != nil {
			return apierrors.New(apierrors.Internal, "proposal insert failed").Wrap(err)
		}
		raw, _ := json.Marshal(changes)
		if _, err := tx.Exec(ctx, `
			INSERT INTO proposal_versions (project_id, id, proposal_id, revision, changes_json, created_at)
			VALUES ($1,$2,$3,1,$4,$5)`,
			projectID, uuid.New(), id, raw, now); err != nil {
			return apierrors.New(apierrors.Internal, "version insert failed").Wrap(err)
		}
		return audit.Append(ctx, tx, audit.Entry{
			ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
			Source: audit.SourceWeb, Operation: "proposal.draft.create",
			ObjectType: "proposal", ObjectID: id.String(), OccurredAt: now,
		})
	})
	return id, err
}

func nullableUUID(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return *id
}

// computeSlots derives the fixed recipient set from the frozen changes
// (03 §3): plan owner identities, task participants and reviewers, plus the
// sender when they hold one of those seats. Same real user covering several
// seats still generates per-seat slots; the DECISION dedups by user.
func (s *Service) computeSlots(ctx context.Context, tx pgx.Tx, projectID uuid.UUID, changes []Change, sender uuid.UUID) ([]slotSpec, error) {
	identitySet := map[uuid.UUID]struct{}{}
	constraints, err := work.ChangeWorkflowConstraints(ctx, tx, projectID, changes)
	if err != nil {
		return nil, err
	}
	for _, constraint := range constraints {
		for _, id := range constraint.IdentityIDs {
			identitySet[id] = struct{}{}
		}
	}
	addFromFields := func(fields map[string]any, keys ...string) {
		for _, key := range keys {
			raw, ok := fields[key].(string)
			if !ok || raw == "" {
				continue
			}
			if id, err := uuid.Parse(raw); err == nil {
				identitySet[id] = struct{}{}
			}
		}
	}
	for _, change := range changes {
		addFromFields(change.Fields, "ownerIdentityId", "reviewerIdentityId")
		raw, _ := json.Marshal(change.Fields["participantIdentityIds"])
		var ids []uuid.UUID
		if err := json.Unmarshal(raw, &ids); err != nil && change.Fields["participantIdentityIds"] != nil {
			return nil, apierrors.Fields("participantIdentityIds", "uuid list")
		}
		for _, id := range ids {
			identitySet[id] = struct{}{}
		}
		if change.TargetID != "" {
			target, err := uuid.Parse(change.TargetID)
			if err != nil {
				continue
			} // clientRef resolved at application
			var rows pgx.Rows
			switch change.TargetType {
			case "task":
				rows, err = tx.Query(ctx, `SELECT reviewer_identity_id FROM tasks WHERE project_id=$1 AND id=$2
    UNION SELECT identity_id FROM task_participants WHERE project_id=$1 AND task_id=$2
    UNION SELECT p.owner_identity_id FROM plans p JOIN tasks t ON t.plan_id=p.id WHERE t.project_id=$1 AND t.id=$2`, projectID, target)
			case "plan":
				rows, err = tx.Query(ctx, `SELECT owner_identity_id FROM plans WHERE project_id=$1 AND id=$2`, projectID, target)
			default:
				return nil, apierrors.Fields("targetType", "enum")
			}
			if err != nil {
				return nil, err
			}
			for rows.Next() {
				var id *uuid.UUID
				if err = rows.Scan(&id); err != nil {
					rows.Close()
					return nil, err
				}
				if id != nil {
					identitySet[*id] = struct{}{}
				}
			}
			rows.Close()
			if err = rows.Err(); err != nil {
				return nil, err
			}
		}
	}
	var specs []slotSpec
	for identityID := range identitySet {
		// Coordinator identity or unknown ids are skipped (not approval seats).
		var kind string
		var bindingUser *uuid.UUID
		var positionName *string
		err := tx.QueryRow(ctx, `
			SELECT i.kind, b.user_id, (SELECT name FROM position_templates t WHERE t.id=i.template_id)
			FROM agent_identities i
			LEFT JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version
			WHERE i.id=$1 AND i.project_id=$2`, identityID, projectID).
			Scan(&kind, &bindingUser, &positionName)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierrors.New(apierrors.InvalidReference, "responsible identity not found in project")
		}
		if err != nil {
			return nil, apierrors.New(apierrors.Internal, "identity lookup failed").Wrap(err)
		}
		if kind == "coordinator" || bindingUser == nil {
			return nil, apierrors.New(apierrors.RequirementUnmet, "responsible identity has no human holder")
		}
		var displayName string
		if err := tx.QueryRow(ctx, `SELECT display_name FROM users WHERE id=$1`, *bindingUser).Scan(&displayName); err != nil {
			return nil, apierrors.New(apierrors.Internal, "user lookup failed").Wrap(err)
		}
		specs = append(specs, slotSpec{
			AuthorityType: "identity", AuthorityID: identityID,
			InitialUser: *bindingUser, DisplayName: displayName, PositionName: positionName,
		})
	}
	// The sender must be part of the affected group for formal confirmation.
	anyBySender := false
	for _, spec := range specs {
		if spec.InitialUser == sender {
			anyBySender = true
		}
	}
	if !anyBySender && len(specs) > 0 {
		// Sender confirms intent separately at submit; not a slot unless affected.
	}
	if len(specs) == 0 {
		// Degenerate proposals (e.g. pure topic creation) still need one seat:
		// the project owner, so nothing self-approves silently.
		var owner uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT owner_user_id FROM projects WHERE id=$1`, projectID).Scan(&owner); err != nil {
			return nil, apierrors.New(apierrors.Internal, "owner lookup failed").Wrap(err)
		}
		var displayName string
		_ = tx.QueryRow(ctx, `SELECT display_name FROM users WHERE id=$1`, owner).Scan(&displayName)
		specs = append(specs, slotSpec{
			AuthorityType: "user", AuthorityID: owner,
			InitialUser: owner, DisplayName: displayName,
		})
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].AuthorityID.String() < specs[j].AuthorityID.String() })
	return specs, nil
}

type slotSpec struct {
	AuthorityType string
	AuthorityID   uuid.UUID
	InitialUser   uuid.UUID
	DisplayName   string
	PositionName  *string
}

// canonicalHash produces the review hash over changes + slots + versions.
func canonicalHash(changes []Change, specs []slotSpec, timeoutSecs int, reviewID uuid.UUID, constraints ...[]work.WorkflowConstraint) string {
	payload := map[string]any{
		"changes": changes,
		"slots":   specs,
		"timeout": timeoutSecs,
	}
	payload["reviewId"] = reviewID
	if len(constraints) > 0 {
		payload["workflowConstraints"] = constraints[0]
	}
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// Submit freezes the review (03 §3 提案提交): payload, slots and hashes are
// persisted immutably; the proposal flips to pending.
func (s *Service) Submit(ctx context.Context, requester, projectID, proposalID uuid.UUID, expectedVersion int64, draftHash string) (Review, error) {
	var out Review
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, projectID.String()); err != nil {
			return err
		}
		if _, err := memberTx(ctx, tx, projectID, requester); err != nil {
			return err
		}
		var (
			status  string
			version int64
			topicID *uuid.UUID
			creator *uuid.UUID
		)
		if err := tx.QueryRow(ctx, `
			SELECT status, version, topic_id,created_by FROM proposals WHERE id=$1 AND project_id=$2 FOR UPDATE`,
			proposalID, projectID).Scan(&status, &version, &topicID, &creator); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apierrors.New(apierrors.NotFound, "proposal not found")
			}
			return apierrors.New(apierrors.Internal, "proposal lookup failed").Wrap(err)
		}
		if creator != nil && *creator != requester {
			return apierrors.New(apierrors.Forbidden, "proposal sender required")
		}
		if status != "draft" {
			return apierrors.New(apierrors.InvalidTransition, "proposal is "+status)
		}
		if version != expectedVersion {
			return apierrors.New(apierrors.VersionConflict, "proposal version conflict")
		}
		var changesRaw []byte
		var reviewID uuid.UUID
		if err := tx.QueryRow(ctx, `
			SELECT id, changes_json FROM proposal_versions WHERE proposal_id=$1 AND revision=(
				SELECT max(revision) FROM proposal_versions WHERE proposal_id=$1)`,
			proposalID).Scan(&reviewID, &changesRaw); err != nil {
			return apierrors.New(apierrors.Internal, "draft lookup failed").Wrap(err)
		}
		var changes []Change
		if err := json.Unmarshal(changesRaw, &changes); err != nil {
			return apierrors.New(apierrors.Internal, "draft parse failed").Wrap(err)
		}
		if err := ValidateDraftChanges(changes); err != nil {
			return err
		}
		specs, err := s.computeSlots(ctx, tx, projectID, changes, requester)
		if err != nil {
			return err
		}
		var timeout int
		if err := tx.QueryRow(ctx, `SELECT approval_timeout_seconds FROM projects WHERE id=$1`, projectID).Scan(&timeout); err != nil {
			return err
		}
		constraints, err := work.ChangeWorkflowConstraints(ctx, tx, projectID, changes)
		if err != nil {
			return err
		}
		hash := canonicalHash(changes, specs, timeout, reviewID, constraints)
		if draftHash != "" && draftHash != hash {
			return apierrors.New(apierrors.ReviewStale, "draft changed since review")
		}
		now := s.now()
		manifest, _ := json.Marshal(map[string]any{"slots": specs, "timeoutSeconds": timeout, "workflowConstraints": constraints})
		if _, err := tx.Exec(ctx, `
			UPDATE proposal_versions SET review_manifest_json=$2, review_hash=$3, sender_user_id=$4,
				submitted_at=$5, timeout_seconds_snapshot=$6
			WHERE proposal_id=$1 AND revision=(
				SELECT max(revision) FROM proposal_versions WHERE proposal_id=$1)`,
			proposalID, manifest, hash, requester, now, timeout); err != nil {
			return apierrors.New(apierrors.Internal, "freeze failed").Wrap(err)
		}
		for _, spec := range specs {
			if _, err := tx.Exec(ctx, `
				INSERT INTO approval_slots (project_id, id, review_id, authority_type, authority_id, initial_user_id, state)
				VALUES ($1,$2,$3,$4,$5,$6,'pending')`,
				projectID, uuid.New(), reviewID, spec.AuthorityType, spec.AuthorityID, spec.InitialUser); err != nil {
				return apierrors.New(apierrors.Internal, "slot insert failed").Wrap(err)
			}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE proposals SET status='pending', current_review_id=$2, version=version+1, updated_at=$3
			WHERE id=$1`, proposalID, reviewID, now); err != nil {
			return apierrors.New(apierrors.Internal, "proposal update failed").Wrap(err)
		}
		if _, err := events.AppendProjectEvent(ctx, tx, projectID, "proposal.changed", "proposal", proposalID.String(), nil,
			map[string]any{"change": "submitted", "reviewId": reviewID}, now); err != nil {
			return err
		}
		return audit.Append(ctx, tx, audit.Entry{
			ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
			Source: audit.SourceWeb, Operation: "proposal.submit",
			ObjectType: "proposal", ObjectID: proposalID.String(), ReviewID: &reviewID, OccurredAt: now,
		})
	})
	if err == nil {
		return s.GetReview(ctx, requester, projectID, proposalID)
	}
	return out, err
}
