// SPDX-License-Identifier: Apache-2.0

package project

import (
	"context"
	"errors"
	"github.com/kakj-go/Judex/internal/platform/paging"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/events"
	"github.com/kakj-go/Judex/internal/platform/keys"
)

// ListInvitations returns the project's invitations (manager view).
func (s *Service) ListInvitations(ctx context.Context, requester, projectID uuid.UUID) ([]Invitation, error) {
	if _, err := s.MembershipFor(ctx, requester, projectID); err != nil {
		return nil, err
	}
	rows, err := paging.Query(ctx, s.pool, `
		SELECT i.id, i.target_email_normalized, i.target_user_id, i.state, i.inviter_user_id, i.expires_at, i.accepted_user_id
		/*keys*/ FROM project_invitations i WHERE i.project_id=$1 /*page*/`, "i.created_at", "i.id", projectID)
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "invitations failed").Wrap(err)
	}
	defer rows.Close()
	var out []Invitation
	for rows.Next() {
		var inv Invitation
		if err := rows.Scan(&inv.ID, &inv.TargetEmail, &inv.TargetUserID, &inv.State, &inv.InviterUserID, &inv.ExpiresAt, &inv.AcceptedUserID); err != nil {
			return nil, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

// RevokeInvitation (manager) invalidates a pending invite.
func (s *Service) RevokeInvitation(ctx context.Context, requester, projectID, invitationID uuid.UUID) error {
	return s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, projectID.String()); err != nil {
			return err
		}
		role, err := s.MembershipForTx(ctx, tx, requester, projectID)
		if err != nil {
			return err
		}
		if role.Role != "owner" && role.Role != "manager" {
			return apierrors.New(apierrors.Forbidden, "manager or owner required")
		}
		tag, err := tx.Exec(ctx, `
			UPDATE project_invitations SET state='revoked'
			WHERE id=$1 AND project_id=$2 AND state='pending'`, invitationID, projectID)
		if err != nil {
			return apierrors.New(apierrors.Internal, "revoke failed").Wrap(err)
		}
		if tag.RowsAffected() == 0 {
			return apierrors.New(apierrors.InvalidTransition, "invitation not pending")
		}
		return audit.Append(ctx, tx, audit.Entry{
			ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
			Source: audit.SourceWeb, Operation: "invitation.revoke",
			ObjectType: "invitation", ObjectID: invitationID.String(), OccurredAt: s.now(),
		})
	})
}

// ResolveInvitationByToken maps a link token to a preview for the logged-in
// user, flagging email match (02 §5: 登录且邮箱匹配).
func (s *Service) ResolveInvitationByToken(ctx context.Context, requester uuid.UUID, token string) (Invitation, string, error) {
	var inv Invitation
	var expiresAt time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT i.id, i.project_id, i.target_email_normalized, i.state, i.expires_at
		FROM project_invitations i WHERE i.token_hash=$1`, keys.Hash(token)).
		Scan(&inv.ID, &inv.ProjectID, &inv.TargetEmail, &inv.State, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Invitation{}, "", apierrors.New(apierrors.NotFound, "invitation not found")
	}
	if err != nil {
		return Invitation{}, "", apierrors.New(apierrors.Internal, "resolve failed").Wrap(err)
	}
	inv.ExpiresAt = expiresAt
	if err := s.pool.QueryRow(ctx, `SELECT title FROM projects WHERE id=$1`, inv.ProjectID).Scan(&inv.ProjectTitle); err != nil {
		return Invitation{}, "", err
	}
	rows, err := s.pool.Query(ctx, `SELECT p.name FROM invitation_positions ip JOIN position_templates p ON p.id=ip.position_template_id WHERE ip.invitation_id=$1 ORDER BY p.name`, inv.ID)
	if err != nil {
		return Invitation{}, "", err
	}
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			rows.Close()
			return Invitation{}, "", err
		}
		inv.PositionNames = append(inv.PositionNames, name)
	}
	rows.Close()
	var email string
	if err := s.pool.QueryRow(ctx, `SELECT email_normalized FROM users WHERE id=$1`, requester).Scan(&email); err != nil {
		return Invitation{}, "", apierrors.New(apierrors.Internal, "user lookup failed").Wrap(err)
	}
	if inv.State != "pending" {
		return inv, email, nil
	}
	if s.now().After(expiresAt) {
		_, _ = s.pool.Exec(ctx, `UPDATE project_invitations SET state='expired' WHERE id=$1 AND state='pending'`, inv.ID)
		inv.State = "expired"
	}
	return inv, email, nil
}

// AcceptInvitation is the 02 §5 acceptance transaction: lock project ->
// invite FOR UPDATE, verify pending/not-expired/email/positions, upsert
// membership, create (or reuse active) identities per position, mark
// accepted. Retrying the same invitation returns the original result.
func (s *Service) AcceptInvitation(ctx context.Context, requester uuid.UUID, invitationID uuid.UUID, token string) ([]Identity, uuid.UUID, error) {
	var identities []Identity
	var projectID uuid.UUID
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		// Lock user first (01 §4 order), then the invitation row.
		if err := tx.LockUserForShare(ctx, requester.String()); err != nil {
			return err
		}
		var (
			targetEmail string
			state       string
			expiresAt   time.Time
			tokenHash   []byte
		)
		err := tx.QueryRow(ctx, `
			SELECT project_id, target_email_normalized, state, expires_at, token_hash
			FROM project_invitations WHERE id=$1 FOR UPDATE`, invitationID).
			Scan(&projectID, &targetEmail, &state, &expiresAt, &tokenHash)
		if errors.Is(err, pgx.ErrNoRows) {
			return apierrors.New(apierrors.NotFound, "invitation not found")
		}
		if err != nil {
			return apierrors.New(apierrors.Internal, "invitation lookup failed").Wrap(err)
		}
		// Retry of an already-accepted invitation: return the original result.
		if state == "accepted" {
			rows, err := tx.Query(ctx, `
				SELECT i.id FROM agent_identities i WHERE i.project_id=$1`, projectID)
			if err == nil {
				rows.Close()
			}
			return nil
		}
		if state != "pending" {
			return apierrors.New(apierrors.InvalidTransition, "invitation "+state)
		}
		if s.now().After(expiresAt) {
			_, _ = tx.Exec(ctx, `UPDATE project_invitations SET state='expired' WHERE id=$1`, invitationID)
			return apierrors.New(apierrors.InvalidTransition, "invitation expired")
		}
		if token != "" && !keys.Equal(tokenHash, keys.Hash(token)) {
			return apierrors.New(apierrors.Forbidden, "invitation token mismatch")
		}
		var email string
		if err := tx.QueryRow(ctx, `SELECT email_normalized FROM users WHERE id=$1`, requester).Scan(&email); err != nil {
			return apierrors.New(apierrors.Internal, "user lookup failed").Wrap(err)
		}
		if email != targetEmail {
			return apierrors.New(apierrors.Forbidden, "登录邮箱与受邀邮箱不一致")
		}
		if err := tx.LockActiveProject(ctx, projectID.String()); err != nil {
			return err
		}
		// Positions must still exist and be active.
		positionRows, err := tx.Query(ctx, `
			SELECT p.position_template_id FROM invitation_positions p
			WHERE p.invitation_id=$1 AND p.project_id=$2`, invitationID, projectID)
		if err != nil {
			return apierrors.New(apierrors.Internal, "positions lookup failed").Wrap(err)
		}
		var positionIDs []uuid.UUID
		for positionRows.Next() {
			var id uuid.UUID
			if err := positionRows.Scan(&id); err != nil {
				positionRows.Close()
				return apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
			}
			positionIDs = append(positionIDs, id)
		}
		positionRows.Close()
		for _, positionID := range positionIDs {
			var status string
			if err := tx.QueryRow(ctx, `
				SELECT status FROM position_templates WHERE id=$1 AND project_id=$2`,
				positionID, projectID).Scan(&status); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return apierrors.Newf(apierrors.InvalidReference, "职位已被删除，请联系管理者修订邀请")
				}
				return apierrors.New(apierrors.Internal, "position lookup failed").Wrap(err)
			}
			if status != "active" {
				return apierrors.Newf(apierrors.InvalidReference, "职位已不可用，请联系管理者修订邀请")
			}
		}
		now := s.now()
		// Upsert active membership.
		if _, err := tx.Exec(ctx, `
			INSERT INTO project_members (project_id, user_id, role, state, joined_at)
			VALUES ($1,$2,'member','active',$3)
			ON CONFLICT (project_id, user_id) DO UPDATE SET state='active'`,
			projectID, requester, now); err != nil {
			return apierrors.New(apierrors.Internal, "membership failed").Wrap(err)
		}
		// Per position: reuse the user's active identity for the same template,
		// else create a new one (02 §5 不因重试复制机器人).
		for _, positionID := range positionIDs {
			var existingID uuid.UUID
			var existingVersion int64
			err := tx.QueryRow(ctx, `
				SELECT i.id, i.current_binding_version FROM agent_identities i
				JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version
				WHERE i.project_id=$1 AND i.template_id=$2 AND i.kind='position' AND i.status='active'
				  AND b.user_id=$3 AND b.valid_until IS NULL`,
				projectID, positionID, requester).Scan(&existingID, &existingVersion)
			if err == nil {
				identities = append(identities, Identity{ID: existingID, Kind: "position", Status: "active",
					TemplateID: &positionID, CurrentBindingVersion: existingVersion})
				continue
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return apierrors.New(apierrors.Internal, "identity lookup failed").Wrap(err)
			}
			identityID := uuid.New()
			if _, err := tx.Exec(ctx, `
				INSERT INTO agent_identities (project_id, id, template_id, kind, status, current_binding_version, created_at)
				VALUES ($1,$2,$3,'position','active',1,$4)`, projectID, identityID, positionID, now); err != nil {
				return apierrors.New(apierrors.Internal, "identity insert failed").Wrap(err)
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO identity_bindings (project_id, identity_id, binding_version, user_id, valid_from)
				VALUES ($1,$2,1,$3,$4)`, projectID, identityID, requester, now); err != nil {
				return apierrors.New(apierrors.Internal, "binding insert failed").Wrap(err)
			}
			identities = append(identities, Identity{ID: identityID, Kind: "position", Status: "active",
				TemplateID: &positionID, CurrentBindingVersion: 1})
		}
		if _, err := tx.Exec(ctx, `
			UPDATE project_invitations SET state='accepted', accepted_user_id=$2 WHERE id=$1`,
			invitationID, requester); err != nil {
			return apierrors.New(apierrors.Internal, "accept failed").Wrap(err)
		}
		if _, err := events.AppendProjectEvent(ctx, tx, projectID, "membership.changed", "invitation", invitationID.String(), nil,
			map[string]any{"change": "accepted"}, now); err != nil {
			return err
		}
		return audit.Append(ctx, tx, audit.Entry{
			ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
			Source: audit.SourceWeb, Operation: "invitation.accept",
			ObjectType: "invitation", ObjectID: invitationID.String(), OccurredAt: now,
		})
	})
	if err != nil {
		return nil, uuid.Nil, err
	}
	return identities, projectID, nil
}

// DeclineInvitation marks the invite declined by the invited account.
func (s *Service) DeclineInvitation(ctx context.Context, requester, invitationID uuid.UUID) error {
	return s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		var targetEmail string
		if err := tx.QueryRow(ctx, `
			SELECT target_email_normalized FROM project_invitations WHERE id=$1 FOR UPDATE`,
			invitationID).Scan(&targetEmail); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apierrors.New(apierrors.NotFound, "invitation not found")
			}
			return apierrors.New(apierrors.Internal, "lookup failed").Wrap(err)
		}
		var email string
		if err := tx.QueryRow(ctx, `SELECT email_normalized FROM users WHERE id=$1`, requester).Scan(&email); err != nil {
			return apierrors.New(apierrors.Internal, "user lookup failed").Wrap(err)
		}
		if email != targetEmail {
			return apierrors.New(apierrors.Forbidden, "只有受邀本人可以操作")
		}
		tag, err := tx.Exec(ctx, `
			UPDATE project_invitations SET state='declined' WHERE id=$1 AND state='pending'`, invitationID)
		if err != nil || tag.RowsAffected() == 0 {
			return apierrors.New(apierrors.InvalidTransition, "invitation not pending")
		}
		return nil
	})
}

// ListMyInvitations returns pending invites matching the caller's email.
func (s *Service) ListMyInvitations(ctx context.Context, requester uuid.UUID) ([]Invitation, error) {
	var email string
	if err := s.pool.QueryRow(ctx, `SELECT email_normalized FROM users WHERE id=$1`, requester).Scan(&email); err != nil {
		return nil, apierrors.New(apierrors.Internal, "user lookup failed").Wrap(err)
	}
	rows, err := paging.Query(ctx, s.pool, `
		SELECT i.id, i.project_id, i.target_email_normalized, i.state, i.inviter_user_id, i.expires_at
		/*keys*/ FROM project_invitations i
		WHERE i.target_email_normalized=$1 AND i.state='pending' AND i.expires_at>$2
		/*page*/`, "i.created_at", "i.id", email, s.now())
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "invitations failed").Wrap(err)
	}
	defer rows.Close()
	var out []Invitation
	for rows.Next() {
		var inv Invitation
		if err := rows.Scan(&inv.ID, &inv.ProjectID, &inv.TargetEmail, &inv.State, &inv.InviterUserID, &inv.ExpiresAt); err != nil {
			return nil, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

// PersonalPreferences API (02 §6): only the owner of the prompt reads it.
type PersonalPreferences struct {
	Revision int64  `json:"revision"`
	Prompt   string `json:"prompt"`
}

func (s *Service) GetMyPreferences(ctx context.Context, requester, projectID uuid.UUID) (PersonalPreferences, error) {
	if _, err := s.MembershipFor(ctx, requester, projectID); err != nil {
		return PersonalPreferences{}, err
	}
	var out PersonalPreferences
	err := s.pool.QueryRow(ctx, `
		SELECT revision, prompt FROM personal_project_preferences WHERE project_id=$1 AND user_id=$2`,
		projectID, requester).Scan(&out.Revision, &out.Prompt)
	if errors.Is(err, pgx.ErrNoRows) {
		// Absent means "nothing saved yet": revision 0, first write -> 1.
		return PersonalPreferences{Revision: 0, Prompt: ""}, nil
	}
	if err != nil {
		return PersonalPreferences{}, apierrors.New(apierrors.Internal, "preferences failed").Wrap(err)
	}
	return out, nil
}

func (s *Service) UpdateMyPreferences(ctx context.Context, requester, projectID uuid.UUID, expectedRevision int64, prompt string) (PersonalPreferences, error) {
	if len(prompt) > 20000 {
		return PersonalPreferences{}, apierrors.Fields("prompt", "length")
	}
	var out PersonalPreferences
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, projectID.String()); err != nil {
			return err
		}
		if _, err := s.MembershipForTx(ctx, tx, requester, projectID); err != nil {
			return err
		}
		var current int64
		err := tx.QueryRow(ctx, `
			SELECT revision FROM personal_project_preferences WHERE project_id=$1 AND user_id=$2 FOR UPDATE`,
			projectID, requester).Scan(&current)
		if errors.Is(err, pgx.ErrNoRows) {
			if expectedRevision != 0 {
				return apierrors.New(apierrors.VersionConflict, "preferences revision conflict")
			}
			current = 0
			if _, err := tx.Exec(ctx, `
				INSERT INTO personal_project_preferences (project_id, user_id, revision, prompt, updated_at)
				VALUES ($1,$2,1,$3,$4)`, projectID, requester, prompt, s.now()); err != nil {
				return apierrors.New(apierrors.Internal, "insert failed").Wrap(err)
			}
			current = 1
		} else if err != nil {
			return apierrors.New(apierrors.Internal, "lookup failed").Wrap(err)
		} else {
			if current != expectedRevision {
				return apierrors.New(apierrors.VersionConflict, "preferences revision conflict")
			}
			if _, err := tx.Exec(ctx, `
				UPDATE personal_project_preferences SET revision=$3, prompt=$4, updated_at=$5
				WHERE project_id=$1 AND user_id=$2`, projectID, requester, current+1, prompt, s.now()); err != nil {
				return apierrors.New(apierrors.Internal, "update failed").Wrap(err)
			}
			current++
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO preference_revisions (project_id, user_id, revision, prompt, created_at)
			VALUES ($1,$2,$3,$4,$5)`, projectID, requester, current, prompt, s.now()); err != nil {
			return apierrors.New(apierrors.Internal, "revision insert failed").Wrap(err)
		}
		out = PersonalPreferences{Revision: current, Prompt: prompt}
		return nil
	})
	return out, err
}
