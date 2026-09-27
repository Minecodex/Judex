// SPDX-License-Identifier: Apache-2.0

package project

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/events"
)

// Member is the public member projection (06 §3).
type Member struct {
	UserID      uuid.UUID `json:"userId"`
	DisplayName string    `json:"displayName"`
	Email       string    `json:"email"`
	Role        string    `json:"role"`
	State       string    `json:"state"`
	JoinedAt    time.Time `json:"joinedAt"`
}

// ListMembers returns active members with public user fields.
func (s *Service) ListMembers(ctx context.Context, requester, projectID uuid.UUID) ([]Member, error) {
	if _, err := s.MembershipFor(ctx, requester, projectID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT m.user_id, u.display_name, u.email_display, m.role, m.state, m.joined_at
		FROM project_members m JOIN users u ON u.id = m.user_id
		WHERE m.project_id=$1 AND m.state='active'
		ORDER BY m.joined_at, m.user_id`, projectID)
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "member list failed").Wrap(err)
	}
	defer rows.Close()
	var out []Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.UserID, &m.DisplayName, &m.Email, &m.Role, &m.State, &m.JoinedAt); err != nil {
			return nil, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// UpdateMemberRole grants or revokes manager (owner only, 02 §4). The owner
// role itself is not assignable here — transfers are a separate two-party
// command.
func (s *Service) UpdateMemberRole(ctx context.Context, requester, projectID, target uuid.UUID, expectedProjectVersion int64, role string) (Member, error) {
	if role != "manager" && role != "member" {
		return Member{}, apierrors.Fields("role", "enum")
	}
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockProjectForUpdate(ctx, projectID.String()); err != nil {
			return err
		}
		m, err := s.MembershipForTx(ctx, tx, requester, projectID)
		if err != nil {
			return err
		}
		if m.Role != "owner" {
			return apierrors.New(apierrors.Forbidden, "only the owner manages roles")
		}
		targetM, err := s.MembershipForTx(ctx, tx, target, projectID)
		if err != nil {
			return err
		}
		if targetM.Role == "owner" {
			return apierrors.New(apierrors.Forbidden, "owner role changes only via transfer")
		}
		tag, err := tx.Exec(ctx, `
			UPDATE projects SET version=version+1, updated_at=$2 WHERE id=$1 AND version=$3`,
			projectID, s.now(), expectedProjectVersion)
		if err != nil {
			return apierrors.New(apierrors.Internal, "version bump failed").Wrap(err)
		}
		if tag.RowsAffected() == 0 {
			return apierrors.New(apierrors.VersionConflict, "project version conflict")
		}
		tag, err = tx.Exec(ctx, `
			UPDATE project_members SET role=$3 WHERE project_id=$1 AND user_id=$2 AND state='active'`,
			projectID, target, role)
		if err != nil || tag.RowsAffected() == 0 {
			return apierrors.New(apierrors.Internal, "role update failed").Wrap(err)
		}
		return audit.Append(ctx, tx, audit.Entry{
			ProjectID:   &projectID,
			ActorType:   audit.ActorUser,
			ActorUserID: &requester,
			Source:      audit.SourceWeb,
			Operation:   "project.member.role",
			ObjectType:  "member", ObjectID: target.String(),
			Reason: strPtr("role=" + role),
			OccurredAt: s.now(),
		})
	})
	if err != nil {
		return Member{}, err
	}
	return s.memberProjection(ctx, projectID, target)
}

func (s *Service) memberProjection(ctx context.Context, projectID, target uuid.UUID) (Member, error) {
	var m Member
	err := s.pool.QueryRow(ctx, `
		SELECT m.user_id, u.display_name, u.email_display, m.role, m.state, m.joined_at
		FROM project_members m JOIN users u ON u.id = m.user_id
		WHERE m.project_id=$1 AND m.user_id=$2`, projectID, target).
		Scan(&m.UserID, &m.DisplayName, &m.Email, &m.Role, &m.State, &m.JoinedAt)
	if err != nil {
		return Member{}, apierrors.New(apierrors.Internal, "projection failed").Wrap(err)
	}
	return m, nil
}

// AffectedResponsibility reports what an identity required handling on
// member removal/leave (populated fully once positions exist in P2-01).
type AffectedResponsibility struct {
	IdentityID uuid.UUID `json:"identityId"`
	Resolution string    `json:"resolution"`
}

// RemoveMember (manager+): owner cannot be removed; active position bindings
// must be resolved first — surfaced as blockers until P2-01 wires identities.
func (s *Service) RemoveMember(ctx context.Context, requester, projectID, target uuid.UUID, expectedProjectVersion int64, reason string) ([]AffectedResponsibility, error) {
	if strings.TrimSpace(reason) == "" {
		return nil, apierrors.Fields("reason", "required")
	}
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockProjectForUpdate(ctx, projectID.String()); err != nil {
			return err
		}
		m, err := s.MembershipForTx(ctx, tx, requester, projectID)
		if err != nil {
			return err
		}
		if m.Role == "member" {
			return apierrors.New(apierrors.Forbidden, "manager or owner required")
		}
		targetM, err := s.MembershipForTx(ctx, tx, target, projectID)
		if err != nil {
			return err
		}
		if targetM.Role == "owner" {
			return apierrors.New(apierrors.Forbidden, "owner cannot be removed")
		}
		// Active position bindings block removal until resolved (02 §4).
		var bound int
		if err := tx.QueryRow(ctx, `
			SELECT count(*) FROM identity_bindings b JOIN agent_identities i ON i.id=b.identity_id
			WHERE i.project_id=$1 AND b.user_id=$2 AND b.valid_until IS NULL`, projectID, target).Scan(&bound); err != nil {
			return apierrors.New(apierrors.Internal, "binding check failed").Wrap(err)
		}
		if bound > 0 {
			return apierrors.New(apierrors.RequirementUnmet, "成员仍有活动任职，需先替换或停用身份").
				WithDetails(map[string]any{"activeBindings": bound})
		}
		tag, err := tx.Exec(ctx, `
			UPDATE projects SET version=version+1, updated_at=$2 WHERE id=$1 AND version=$3`,
			projectID, s.now(), expectedProjectVersion)
		if err != nil || tag.RowsAffected() == 0 {
			return apierrors.New(apierrors.VersionConflict, "project version conflict")
		}
		if _, err := tx.Exec(ctx, `
			UPDATE project_members SET state='removed' WHERE project_id=$1 AND user_id=$2 AND state='active'`,
			projectID, target); err != nil {
			return apierrors.New(apierrors.Internal, "remove failed").Wrap(err)
		}
		if _, err := events.AppendProjectEvent(ctx, tx, projectID, "membership.changed", "member", target.String(), nil,
			map[string]any{"change": "removed"}, s.now()); err != nil {
			return err
		}
		return audit.Append(ctx, tx, audit.Entry{
			ProjectID:   &projectID,
			ActorType:   audit.ActorUser,
			ActorUserID: &requester,
			Source:      audit.SourceWeb,
			Operation:   "project.member.remove",
			ObjectType:  "member", ObjectID: target.String(),
			Reason:     &reason,
			OccurredAt: s.now(),
		})
	})
	if err != nil {
		return nil, err
	}
	return []AffectedResponsibility{}, nil
}

// LeaveProject: the requester leaves; the sole owner must transfer first
// (02 §4 无悬空 owner).
func (s *Service) LeaveProject(ctx context.Context, requester, projectID uuid.UUID, expectedProjectVersion int64) error {
	return s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockProjectForUpdate(ctx, projectID.String()); err != nil {
			return err
		}
		m, err := s.MembershipForTx(ctx, tx, requester, projectID)
		if err != nil {
			return err
		}
		if m.Role == "owner" {
			return apierrors.New(apierrors.InvalidTransition, "owner must transfer ownership before leaving")
		}
		var bound int
		if err := tx.QueryRow(ctx, `
			SELECT count(*) FROM identity_bindings b JOIN agent_identities i ON i.id=b.identity_id
			WHERE i.project_id=$1 AND b.user_id=$2 AND b.valid_until IS NULL`, projectID, requester).Scan(&bound); err != nil {
			return apierrors.New(apierrors.Internal, "binding check failed").Wrap(err)
		}
		if bound > 0 {
			return apierrors.New(apierrors.RequirementUnmet, "仍有活动任职，需先替换或停用身份").
				WithDetails(map[string]any{"activeBindings": bound})
		}
		tag, err := tx.Exec(ctx, `
			UPDATE projects SET version=version+1, updated_at=$2 WHERE id=$1 AND version=$3`,
			projectID, s.now(), expectedProjectVersion)
		if err != nil || tag.RowsAffected() == 0 {
			return apierrors.New(apierrors.VersionConflict, "project version conflict")
		}
		if _, err := tx.Exec(ctx, `
			UPDATE project_members SET state='left' WHERE project_id=$1 AND user_id=$2 AND state='active'`,
			projectID, requester); err != nil {
			return apierrors.New(apierrors.Internal, "leave failed").Wrap(err)
		}
		if _, err := events.AppendProjectEvent(ctx, tx, projectID, "membership.changed", "member", requester.String(), nil,
			map[string]any{"change": "left"}, s.now()); err != nil {
			return err
		}
		return audit.Append(ctx, tx, audit.Entry{
			ProjectID:   &projectID,
			ActorType:   audit.ActorUser,
			ActorUserID: &requester,
			Source:      audit.SourceWeb,
			Operation:   "project.member.leave",
			ObjectType:  "member", ObjectID: requester.String(),
			OccurredAt: s.now(),
		})
	})
}

// OwnerTransfer is the two-party transfer aggregate (06 §3).
type OwnerTransfer struct {
	ID            uuid.UUID  `json:"id"`
	ProjectID     uuid.UUID  `json:"-"`
	FromUserID    uuid.UUID  `json:"fromUserId"`
	ToUserID      uuid.UUID  `json:"toUserId"`
	State         string     `json:"state"`
	ProjectVersion int64     `json:"projectVersion"`
	ExpiresAt     time.Time  `json:"expiresAt"`
	ConfirmedAt   *time.Time `json:"confirmedAt"`
}

// RequestOwnerTransfer (owner -> active member target) creates a pending
// transfer with a 48h expiry; both parties must be active members at accept
// time (02 §4).
func (s *Service) RequestOwnerTransfer(ctx context.Context, requester, projectID, target uuid.UUID, expectedProjectVersion int64) (OwnerTransfer, error) {
	var out OwnerTransfer
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockProjectForUpdate(ctx, projectID.String()); err != nil {
			return err
		}
		m, err := s.MembershipForTx(ctx, tx, requester, projectID)
		if err != nil {
			return err
		}
		if m.Role != "owner" {
			return apierrors.New(apierrors.Forbidden, "only the owner initiates transfers")
		}
		if target == requester {
			return apierrors.Fields("targetUserId", "self")
		}
		if _, err := s.MembershipForTx(ctx, tx, target, projectID); err != nil {
			return err // 404 for non-members
		}
		// Only one pending transfer per project.
		tag, err := tx.Exec(ctx, `
			UPDATE projects SET version=version+1, updated_at=$2 WHERE id=$1 AND version=$3`,
			projectID, s.now(), expectedProjectVersion)
		if err != nil || tag.RowsAffected() == 0 {
			return apierrors.New(apierrors.VersionConflict, "project version conflict")
		}
		if _, err := tx.Exec(ctx, `
			UPDATE ownership_transfers SET state='cancelled'
			WHERE project_id=$1 AND state='pending'`, projectID); err != nil {
			return apierrors.New(apierrors.Internal, "cancel stale transfers failed").Wrap(err)
		}
		id := uuid.New()
		expires := s.now().Add(48 * time.Hour)
		if err := tx.QueryRow(ctx, `
			INSERT INTO ownership_transfers (project_id, id, from_user_id, to_user_id, project_version, expires_at, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
			RETURNING id, from_user_id, to_user_id, state, project_version, expires_at, confirmed_at`,
			projectID, id, requester, target, expectedProjectVersion+1, expires, s.now()).
			Scan(&out.ID, &out.FromUserID, &out.ToUserID, &out.State, &out.ProjectVersion, &out.ExpiresAt, &out.ConfirmedAt); err != nil {
			return apierrors.New(apierrors.Internal, "transfer insert failed").Wrap(err)
		}
		return audit.Append(ctx, tx, audit.Entry{
			ProjectID:   &projectID,
			ActorType:   audit.ActorUser,
			ActorUserID: &requester,
			Source:      audit.SourceWeb,
			Operation:   "project.owner_transfer.request",
			ObjectType:  "ownership_transfer", ObjectID: id.String(),
			OccurredAt: s.now(),
		})
	})
	return out, err
}

// GetOwnerTransfer returns the transfer to the from/to parties only.
func (s *Service) GetOwnerTransfer(ctx context.Context, requester, projectID, transferID uuid.UUID) (OwnerTransfer, error) {
	if _, err := s.MembershipFor(ctx, requester, projectID); err != nil {
		return OwnerTransfer{}, err
	}
	var out OwnerTransfer
	err := s.pool.QueryRow(ctx, `
		SELECT id, project_id, from_user_id, to_user_id, state, project_version, expires_at, confirmed_at
		FROM ownership_transfers WHERE id=$1 AND project_id=$2`, transferID, projectID).
		Scan(&out.ID, &out.ProjectID, &out.FromUserID, &out.ToUserID, &out.State, &out.ProjectVersion, &out.ExpiresAt, &out.ConfirmedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return OwnerTransfer{}, apierrors.New(apierrors.NotFound, "transfer not found")
	}
	if err != nil {
		return OwnerTransfer{}, apierrors.New(apierrors.Internal, "transfer lookup failed").Wrap(err)
	}
	if requester != out.FromUserID && requester != out.ToUserID {
		return OwnerTransfer{}, apierrors.New(apierrors.NotFound, "transfer not found")
	}
	return out, nil
}

// DecideOwnerTransfer: the TARGET accepts or declines. Accept re-locks the
// project, re-verifies both memberships and swaps roles atomically — the
// single-owner index enforces the invariant under concurrency (02 §4).
func (s *Service) DecideOwnerTransfer(ctx context.Context, requester, projectID, transferID uuid.UUID, accept bool) (OwnerTransfer, error) {
	var out OwnerTransfer
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockProjectForUpdate(ctx, projectID.String()); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `
			SELECT id, from_user_id, to_user_id, state, expires_at FROM ownership_transfers
			WHERE id=$1 AND project_id=$2 FOR UPDATE`, transferID, projectID).
			Scan(&out.ID, &out.FromUserID, &out.ToUserID, &out.State, &out.ExpiresAt); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apierrors.New(apierrors.NotFound, "transfer not found")
			}
			return apierrors.New(apierrors.Internal, "transfer lock failed").Wrap(err)
		}
		if requester != out.ToUserID {
			return apierrors.New(apierrors.Forbidden, "only the target decides")
		}
		if out.State != "pending" {
			return apierrors.New(apierrors.InvalidTransition, "transfer not pending")
		}
		if s.now().After(out.ExpiresAt) {
			if _, err := tx.Exec(ctx, `UPDATE ownership_transfers SET state='expired' WHERE id=$1`, transferID); err != nil {
				return apierrors.New(apierrors.Internal, "expire failed").Wrap(err)
			}
			return apierrors.New(apierrors.InvalidTransition, "transfer expired")
		}
		if !accept {
			if _, err := tx.Exec(ctx, `UPDATE ownership_transfers SET state='declined' WHERE id=$1`, transferID); err != nil {
				return apierrors.New(apierrors.Internal, "decline failed").Wrap(err)
			}
			out.State = "declined"
			return audit.Append(ctx, tx, audit.Entry{
				ProjectID:   &projectID,
				ActorType:   audit.ActorUser,
				ActorUserID: &requester,
				Source:      audit.SourceWeb,
				Operation:   "project.owner_transfer.decline",
				ObjectType:  "ownership_transfer", ObjectID: transferID.String(),
				OccurredAt: s.now(),
			})
		}
		// Re-verify both parties remain active members.
		fromM, err := s.MembershipForTx(ctx, tx, out.FromUserID, projectID)
		if err != nil {
			return err
		}
		if fromM.Role != "owner" {
			return apierrors.New(apierrors.InvalidTransition, "initiator is no longer owner")
		}
		if _, err := s.MembershipForTx(ctx, tx, out.ToUserID, projectID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE project_members SET role='member' WHERE project_id=$1 AND user_id=$2 AND role='owner'`,
			projectID, out.FromUserID); err != nil {
			return apierrors.New(apierrors.Internal, "demote failed").Wrap(err)
		}
		if _, err := tx.Exec(ctx, `UPDATE project_members SET role='owner' WHERE project_id=$1 AND user_id=$2 AND state='active'`,
			projectID, out.ToUserID); err != nil {
			return apierrors.New(apierrors.Internal, "promote failed").Wrap(err)
		}
		if _, err := tx.Exec(ctx, `UPDATE projects SET owner_user_id=$2, version=version+1, updated_at=$3 WHERE id=$1`,
			projectID, out.ToUserID, s.now()); err != nil {
			return apierrors.New(apierrors.Internal, "owner update failed").Wrap(err)
		}
		now := s.now()
		if _, err := tx.Exec(ctx, `UPDATE ownership_transfers SET state='accepted', confirmed_at=$2 WHERE id=$1`, transferID, now); err != nil {
			return apierrors.New(apierrors.Internal, "accept failed").Wrap(err)
		}
		if _, err := events.AppendProjectEvent(ctx, tx, projectID, "membership.changed", "project", projectID.String(), nil,
			map[string]any{"change": "owner_transferred", "newOwner": out.ToUserID}, now); err != nil {
			return err
		}
		out.State = "accepted"
		out.ConfirmedAt = &now
		return audit.Append(ctx, tx, audit.Entry{
			ProjectID:   &projectID,
			ActorType:   audit.ActorUser,
			ActorUserID: &requester,
			Source:      audit.SourceWeb,
			Operation:   "project.owner_transfer.accept",
			ObjectType:  "ownership_transfer", ObjectID: transferID.String(),
			OccurredAt: now,
		})
	})
	return out, err
}

// OperatorTransferOwner is the audited recovery command for a stuck last
// owner (02 §3): explicitly authorized manual action, never automatic.
func (s *Service) OperatorTransferOwner(ctx context.Context, projectID, toUser uuid.UUID, operator, reason string) error {
	if operator == "" || reason == "" {
		return apierrors.New(apierrors.Validation, "operator and reason are required")
	}
	return s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockProjectForUpdate(ctx, projectID.String()); err != nil {
			return err
		}
		if _, err := s.MembershipForTx(ctx, tx, toUser, projectID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE project_members SET role='member' WHERE project_id=$1 AND role='owner' AND state='active'`, projectID); err != nil {
			return apierrors.New(apierrors.Internal, "demote failed").Wrap(err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE project_members SET role='owner' WHERE project_id=$1 AND user_id=$2 AND state='active'`,
			projectID, toUser); err != nil {
			return apierrors.New(apierrors.Internal, "promote failed").Wrap(err)
		}
		if _, err := tx.Exec(ctx, `UPDATE projects SET owner_user_id=$2, version=version+1, updated_at=$3 WHERE id=$1`,
			projectID, toUser, s.now()); err != nil {
			return apierrors.New(apierrors.Internal, "owner update failed").Wrap(err)
		}
		if _, err := events.AppendProjectEvent(ctx, tx, projectID, "membership.changed", "project", projectID.String(), nil,
			map[string]any{"change": "owner_transferred_operator"}, s.now()); err != nil {
			return err
		}
		return audit.Append(ctx, tx, audit.Entry{
			ProjectID:   &projectID,
			ActorType:   audit.ActorOperator,
			ActorUserID: &toUser,
			Source:      audit.SourceOperator,
			Operation:   "project.owner_transfer.operator",
			ObjectType:  "project", ObjectID: projectID.String(),
			Reason:     &reason,
			OccurredAt: s.now(),
		})
	})
}

func strPtr(s string) *string { return &s }
