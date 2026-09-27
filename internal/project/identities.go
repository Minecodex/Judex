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
	"github.com/kakj-go/Judex/internal/platform/keys"
)

// Identity is the stable identity projection (06 §3).
type Identity struct {
	ID                    uuid.UUID       `json:"id"`
	Kind                  string          `json:"kind"`
	Status                string          `json:"status"`
	TemplateID            *uuid.UUID      `json:"templateId"`
	PositionName          *string         `json:"positionName"`
	CurrentBindingVersion int64           `json:"currentBindingVersion"`
	CurrentBinding        *BindingSummary `json:"currentBinding"`
}

// BindingSummary shows who currently holds an identity.
type BindingSummary struct {
	BindingVersion int64     `json:"bindingVersion"`
	UserID         uuid.UUID `json:"userId"`
	DisplayName    string    `json:"displayName"`
	ValidFrom      time.Time `json:"validFrom"`
}

// ListIdentities returns the project's stable identities with bindings.
func (s *Service) ListIdentities(ctx context.Context, requester, projectID uuid.UUID) ([]Identity, error) {
	if _, err := s.MembershipFor(ctx, requester, projectID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT i.id, i.kind, i.status, i.template_id, i.current_binding_version,
		       COALESCE((SELECT name FROM position_templates t WHERE t.id=i.template_id), '')
		FROM agent_identities i WHERE i.project_id=$1 ORDER BY i.created_at`, projectID)
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "identities failed").Wrap(err)
	}
	defer rows.Close()
	var out []Identity
	for rows.Next() {
		var id Identity
		var templateName string
		if err := rows.Scan(&id.ID, &id.Kind, &id.Status, &id.TemplateID, &id.CurrentBindingVersion, &templateName); err != nil {
			return nil, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		if id.TemplateID != nil && templateName != "" {
			name := templateName
			id.PositionName = &name
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Attach current bindings.
	for idx := range out {
		if out[idx].CurrentBindingVersion == 0 {
			continue
		}
		var b BindingSummary
		err := s.pool.QueryRow(ctx, `
			SELECT b.binding_version, b.user_id, u.display_name, b.valid_from
			FROM identity_bindings b JOIN users u ON u.id=b.user_id
			WHERE b.identity_id=$1 AND b.binding_version=$2`,
			out[idx].ID, out[idx].CurrentBindingVersion).Scan(&b.BindingVersion, &b.UserID, &b.DisplayName, &b.ValidFrom)
		if err == nil {
			out[idx].CurrentBinding = &b
		}
	}
	return out, nil
}

// CreateIdentity binds a member to a position as a NEW stable identity
// (manager; 02 §5). Reuse-active-identity semantics live in invitation
// acceptance; this is the explicit multi-seat path.
func (s *Service) CreateIdentity(ctx context.Context, requester, projectID, positionID, userID uuid.UUID) (Identity, error) {
	var out Identity
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockProjectForUpdate(ctx, projectID.String()); err != nil {
			return err
		}
		role, err := s.MembershipForTx(ctx, tx, requester, projectID)
		if err != nil {
			return err
		}
		if role.Role != "owner" && role.Role != "manager" {
			return apierrors.New(apierrors.Forbidden, "manager or owner required")
		}
		if _, err := s.MembershipForTx(ctx, tx, userID, projectID); err != nil {
			return apierrors.New(apierrors.InvalidReference, "target must be an active member")
		}
		var name string
		if err := tx.QueryRow(ctx, `
			SELECT name FROM position_templates WHERE id=$1 AND project_id=$2 AND status='active'`,
			positionID, projectID).Scan(&name); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apierrors.New(apierrors.InvalidReference, "position not found")
			}
			return apierrors.New(apierrors.Internal, "position lookup failed").Wrap(err)
		}
		now := s.now()
		identityID := uuid.New()
		if _, err := tx.Exec(ctx, `
			INSERT INTO agent_identities (project_id, id, template_id, kind, status, current_binding_version, created_at)
			VALUES ($1,$2,$3,'position','active',1,$4)`, projectID, identityID, positionID, now); err != nil {
			return apierrors.New(apierrors.Internal, "identity insert failed").Wrap(err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO identity_bindings (project_id, identity_id, binding_version, user_id, valid_from)
			VALUES ($1,$2,1,$3,$4)`, projectID, identityID, userID, now); err != nil {
			return apierrors.New(apierrors.Internal, "binding insert failed").Wrap(err)
		}
		if _, err := events.AppendProjectEvent(ctx, tx, projectID, "membership.changed", "identity", identityID.String(), nil,
			map[string]any{"change": "created"}, now); err != nil {
			return err
		}
		positionPtr := &positionID
		namePtr := &name
		out = Identity{ID: identityID, Kind: "position", Status: "active", TemplateID: positionPtr,
			PositionName: namePtr, CurrentBindingVersion: 1}
		return audit.Append(ctx, tx, audit.Entry{
			ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
			Source: audit.SourceWeb, Operation: "identity.create",
			ObjectType: "identity", ObjectID: identityID.String(), OccurredAt: now,
		})
	})
	return out, err
}

// ReplaceIdentityBinding ends the old binding and appends a new version
// while preserving the identity and all history (02 §5 替换任职).
func (s *Service) ReplaceIdentityBinding(ctx context.Context, requester, projectID, identityID, newUserID uuid.UUID, expectedBindingVersion int64, reason string) (Identity, error) {
	if reason == "" {
		return Identity{}, apierrors.Fields("reason", "required")
	}
	var out Identity
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockProjectForUpdate(ctx, projectID.String()); err != nil {
			return err
		}
		role, err := s.MembershipForTx(ctx, tx, requester, projectID)
		if err != nil {
			return err
		}
		if role.Role != "owner" && role.Role != "manager" {
			return apierrors.New(apierrors.Forbidden, "manager or owner required")
		}
		if _, err := s.MembershipForTx(ctx, tx, newUserID, projectID); err != nil {
			return apierrors.New(apierrors.InvalidReference, "target must be an active member")
		}
		var current int64
		var templateID *uuid.UUID
		if err := tx.QueryRow(ctx, `
			SELECT current_binding_version, template_id FROM agent_identities
			WHERE id=$1 AND project_id=$2 AND kind='position' FOR UPDATE`,
			identityID, projectID).Scan(&current, &templateID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apierrors.New(apierrors.NotFound, "identity not found")
			}
			return apierrors.New(apierrors.Internal, "identity lookup failed").Wrap(err)
		}
		if current != expectedBindingVersion {
			return apierrors.New(apierrors.VersionConflict, "binding version conflict")
		}
		now := s.now()
		next := current + 1
		if _, err := tx.Exec(ctx, `
			UPDATE identity_bindings SET valid_until=$3
			WHERE identity_id=$1 AND binding_version=$2 AND valid_until IS NULL`,
			identityID, current, now); err != nil {
			return apierrors.New(apierrors.Internal, "close binding failed").Wrap(err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO identity_bindings (project_id, identity_id, binding_version, user_id, valid_from)
			VALUES ($1,$2,$3,$4,$5)`, projectID, identityID, next, newUserID, now); err != nil {
			return apierrors.New(apierrors.Internal, "new binding failed").Wrap(err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE agent_identities SET current_binding_version=$2 WHERE id=$1`, identityID, next); err != nil {
			return apierrors.New(apierrors.Internal, "head update failed").Wrap(err)
		}
		if _, err := events.AppendProjectEvent(ctx, tx, projectID, "membership.changed", "identity", identityID.String(), &next,
			map[string]any{"change": "rebound"}, now); err != nil {
			return err
		}
		out = Identity{ID: identityID, Kind: "position", Status: "active", TemplateID: templateID,
			CurrentBindingVersion: next}
		return audit.Append(ctx, tx, audit.Entry{
			ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
			Source: audit.SourceWeb, Operation: "identity.replace",
			ObjectType: "identity", ObjectID: identityID.String(), Reason: &reason, OccurredAt: now,
		})
	})
	return out, err
}

// Invitation is the invite aggregate (06 §3).
type Invitation struct {
	projectID      uuid.UUID      `json:"-"`
	ID             uuid.UUID      `json:"id"`
	TargetEmail    string         `json:"targetEmail"`
	TargetUserID   *uuid.UUID     `json:"targetUserId"`
	State          string         `json:"state"`
	InviterUserID  uuid.UUID      `json:"inviterUserId"`
	Positions      []PositionLite `json:"positions"`
	ExpiresAt      time.Time      `json:"expiresAt"`
	AcceptedUserID *uuid.UUID     `json:"acceptedUserId"`
	Token          string         `json:"-"`
}

// PositionLite is a position reference inside invitations.
type PositionLite struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// CreateInvitation (manager) targets an exact email with a fixed position
// set; the one-time link token is returned once (02 §5).
func (s *Service) CreateInvitation(ctx context.Context, requester, projectID uuid.UUID, targetEmail string, positionIDs []uuid.UUID) (Invitation, error) {
	if len(positionIDs) == 0 {
		return Invitation{}, apierrors.Fields("positionIds", "required")
	}
	token := keys.NewRandom()
	var out Invitation
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockProjectForUpdate(ctx, projectID.String()); err != nil {
			return err
		}
		role, err := s.MembershipForTx(ctx, tx, requester, projectID)
		if err != nil {
			return err
		}
		if role.Role != "owner" && role.Role != "manager" {
			return apierrors.New(apierrors.Forbidden, "manager or owner required")
		}
		// Positions must stay valid until acceptance is re-verified later.
		for _, positionID := range positionIDs {
			var status string
			if err := tx.QueryRow(ctx, `
				SELECT status FROM position_templates WHERE id=$1 AND project_id=$2`,
				positionID, projectID).Scan(&status); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return apierrors.Newf(apierrors.InvalidReference, "position %s not found", positionID)
				}
				return apierrors.New(apierrors.Internal, "position lookup failed").Wrap(err)
			}
			if status != "active" {
				return apierrors.Newf(apierrors.InvalidReference, "position %s is not active", positionID)
			}
		}
		normalized := NormalizeEmailForProject(targetEmail)
		now := s.now()
		id := uuid.New()
		if _, err := tx.Exec(ctx, `
			INSERT INTO project_invitations (project_id, id, target_email_normalized, inviter_user_id, token_hash, expires_at, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			projectID, id, normalized, requester, keys.Hash(token), now.Add(7*24*time.Hour), now); err != nil {
			return apierrors.New(apierrors.Internal, "invitation insert failed").Wrap(err)
		}
		for _, positionID := range positionIDs {
			if _, err := tx.Exec(ctx, `
				INSERT INTO invitation_positions (project_id, invitation_id, position_template_id)
				VALUES ($1,$2,$3)`, projectID, id, positionID); err != nil {
				return apierrors.New(apierrors.Internal, "invitation position failed").Wrap(err)
			}
		}
		if _, err := events.AppendProjectEvent(ctx, tx, projectID, "membership.changed", "invitation", id.String(), nil,
			map[string]any{"change": "invited"}, now); err != nil {
			return err
		}
		out = Invitation{ID: id, TargetEmail: normalized, State: "pending",
			InviterUserID: requester, ExpiresAt: now.Add(7 * 24 * time.Hour), Token: token}
		return audit.Append(ctx, tx, audit.Entry{
			ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
			Source: audit.SourceWeb, Operation: "invitation.create",
			ObjectType: "invitation", ObjectID: id.String(), OccurredAt: now,
		})
	})
	return out, err
}

// NormalizeEmailForProject shares the identity normalization rules.
func NormalizeEmailForProject(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
