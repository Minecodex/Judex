// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"errors"
	"github.com/kakj-go/Judex/internal/platform/paging"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	"github.com/kakj-go/Judex/internal/platform/auth"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/keys"
)

// Device flow defaults (07 §3): access 15min, refresh 30d.
const (
	DeviceAccessTTL  = 15 * time.Minute
	DeviceRefreshTTL = 30 * 24 * time.Hour
	DeviceAuthTTL    = 15 * time.Minute
	DevicePollMin    = 5
)

// StartDeviceAuthorization creates the device/user code pair (06 §2). Codes
// are random; only hashes persist. The user code is short and typed by hand.
func (s *Service) StartDeviceAuthorization(ctx context.Context, deviceName string, requestedScopes, requestedProjectScope []string) (deviceCode, userCode string, expiresIn, interval int, err error) {
	if strings.TrimSpace(deviceName) == "" || len(deviceName) > 120 {
		return "", "", 0, 0, apierrors.Fields("deviceName", "length")
	}
	for _, scope := range requestedScopes {
		switch scope {
		case auth.ScopeProjectsRead, auth.ScopeProjectsCreate, auth.ScopeContextRead, auth.ScopeMaterialsRead, auth.ScopeMaterialsWrite,
			auth.ScopeSubmissionsWrite, auth.ScopeReportsWrite, auth.ScopeProposalsDraft, auth.ScopeAgentRequest,
			auth.ScopeEventsRead, auth.ScopeIntentsCreate:
		default:
			return "", "", 0, 0, apierrors.Newf(apierrors.Validation, "scope %q 不在支持列表", scope)
		}
	}
	deviceCode = keys.NewRandom()
	userCode = keys.NewRandom()[:8]
	_, err = s.pool.Exec(ctx, `
		INSERT INTO device_authorizations (id, device_code_hash, user_code_hash, device_name,
			requested_scopes, requested_project_scope, interval_seconds, expires_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		uuid.New(), keys.Hash(deviceCode), keys.Hash(userCode), deviceName,
		requestedScopes, nonNilUUIDs(requestedProjectScope), DevicePollMin, s.now().Add(DeviceAuthTTL), s.now())
	if err != nil {
		return "", "", 0, 0, apierrors.New(apierrors.Internal, "device auth insert failed").Wrap(err)
	}
	return deviceCode, userCode, int(DeviceAuthTTL.Seconds()), DevicePollMin, nil
}

// ConfirmDeviceAuthorization is the browser-side decision (07 §3): the
// logged-in user approves/denies; scopes can only be narrowed, never
// expanded beyond the request.
func (s *Service) ConfirmDeviceAuthorization(ctx context.Context, requester uuid.UUID, userCode string, approved bool, scopes []string, selectedProjects ...[]uuid.UUID) error {
	return s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockUserForShare(ctx, requester.String()); err != nil {
			return err
		}
		var (
			id              uuid.UUID
			requestedScopes []string
			status          string
			expiresAt       time.Time
		)
		if err := tx.QueryRow(ctx, `
			SELECT id, requested_scopes, status, expires_at FROM device_authorizations
			WHERE user_code_hash=$1 FOR UPDATE`, keys.Hash(userCode)).
			Scan(&id, &requestedScopes, &status, &expiresAt); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apierrors.New(apierrors.NotFound, "设备码无效")
			}
			return apierrors.New(apierrors.Internal, "device lookup failed").Wrap(err)
		}
		if status != "pending" {
			return apierrors.New(apierrors.InvalidTransition, "该授权已处理")
		}
		if s.now().After(expiresAt) {
			_, _ = tx.Exec(ctx, `UPDATE device_authorizations SET status='expired' WHERE id=$1`, id)
			return apierrors.New(apierrors.InvalidTransition, "设备码已过期")
		}
		// Narrowed scopes must be a subset of the request.
		requested := map[string]bool{}
		for _, scope := range requestedScopes {
			requested[scope] = true
		}
		granted := requestedScopes
		if len(scopes) > 0 {
			granted = scopes
			for _, scope := range granted {
				if !requested[scope] {
					return apierrors.Newf(apierrors.Validation, "scope %q 未在请求中", scope)
				}
			}
		}
		if !approved {
			if _, err := tx.Exec(ctx, `UPDATE device_authorizations SET status='denied' WHERE id=$1`, id); err != nil {
				return apierrors.New(apierrors.Internal, "deny failed").Wrap(err)
			}
			return audit.Append(ctx, tx, audit.Entry{
				ActorType: audit.ActorUser, ActorUserID: &requester,
				Source: audit.SourceWeb, Operation: "device.denied",
				ObjectType: "device_authorization", ObjectID: id.String(), OccurredAt: s.now(),
			})
		}
		if len(selectedProjects) > 0 && selectedProjects[0] != nil {
			var requested []uuid.UUID
			if err := tx.QueryRow(ctx, `SELECT requested_project_scope FROM device_authorizations WHERE id=$1`, id).Scan(&requested); err != nil {
				return err
			}
			for _, pid := range selectedProjects[0] {
				allowed := len(requested) == 0
				for _, r := range requested {
					if r == pid {
						allowed = true
					}
				}
				var member bool
				if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM project_members WHERE project_id=$1 AND user_id=$2 AND state='active')`, pid, requester).Scan(&member); err != nil {
					return err
				}
				if !allowed || !member {
					return apierrors.New(apierrors.Forbidden, "project scope cannot be expanded")
				}
			}
			if _, err := tx.Exec(ctx, `UPDATE device_authorizations SET requested_project_scope=$2 WHERE id=$1`, id, selectedProjects[0]); err != nil {
				return err
			}
		}
		grantID := uuid.New()
		now := s.now()
		if _, err := tx.Exec(ctx, `
			INSERT INTO client_grants (id, user_id, device_name, scopes, project_scope, auth_version, created_at, expires_at)
			SELECT $1, $2, device_name, $3::text[], requested_project_scope, (SELECT auth_version FROM users WHERE id=$2), $4, $5
			FROM device_authorizations WHERE id=$6`,
			grantID, requester, granted, now, now.Add(DeviceRefreshTTL), id); err != nil {
			return apierrors.New(apierrors.Internal, "grant insert failed").Wrap(err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE device_authorizations SET status='consumed', user_id=$2, grant_id=$3 WHERE id=$1`,
			id, requester, grantID); err != nil {
			return apierrors.New(apierrors.Internal, "consume failed").Wrap(err)
		}
		return audit.Append(ctx, tx, audit.Entry{
			ActorType: audit.ActorUser, ActorUserID: &requester,
			Source: audit.SourceWeb, Operation: "device.approved",
			ObjectType: "client_grant", ObjectID: grantID.String(), OccurredAt: now,
		})
	})
}

// PollState mirrors RFC 8628 polling outcomes (06 §2).
type PollState int

const (
	PollPending PollState = iota
	PollSlowDown
	PollDenied
	PollExpired
	PollCompleted
)

type TokenPair struct {
	AccessToken      string    `json:"accessToken"`
	AccessExpiresAt  time.Time `json:"accessExpiresAt"`
	RefreshToken     string    `json:"refreshToken"`
	RefreshExpiresAt time.Time `json:"refreshExpiresAt"`
	Scopes           []string  `json:"scopes"`
}

// PollDeviceToken exchanges one approved device code once under a row lock.
func (s *Service) PollDeviceToken(ctx context.Context, deviceCode string) (PollState, TokenPair, error) {
	state := PollPending
	var pair TokenPair
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		var id uuid.UUID
		var status string
		var expires time.Time
		var grant *uuid.UUID
		var last, issued *time.Time
		var interval int
		if err := tx.QueryRow(ctx, `SELECT id,status,expires_at,grant_id,last_polled_at,tokens_issued_at,interval_seconds FROM device_authorizations WHERE device_code_hash=$1 FOR UPDATE`, keys.Hash(deviceCode)).Scan(&id, &status, &expires, &grant, &last, &issued, &interval); err != nil {
			return apierrors.New(apierrors.Unauthenticated, "invalid device code")
		}
		now := s.now()
		if !now.Before(expires) {
			return apierrors.New(apierrors.Unauthenticated, "device code expired")
		}
		if issued != nil {
			return apierrors.New(apierrors.GrantRevoked, "device code already exchanged; sign in again")
		}
		if status == "pending" {
			if last != nil && now.Sub(*last) < time.Duration(interval)*time.Second {
				state = PollSlowDown
				interval += 5
			}
			_, err := tx.Exec(ctx, `UPDATE device_authorizations SET last_polled_at=$2,interval_seconds=$3 WHERE id=$1`, id, now, interval)
			return err
		}
		if status != "consumed" || grant == nil {
			return apierrors.New(apierrors.Unauthenticated, "device authorization not approved")
		}
		var err error
		pair, err = s.issueTokens(ctx, tx, *grant)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE device_authorizations SET tokens_issued_at=$2 WHERE id=$1`, id, now); err != nil {
			return err
		}
		state = PollCompleted
		return nil
	})
	return state, pair, err
}
func (s *Service) issueTokens(ctx context.Context, tx pgx.Tx, grantID uuid.UUID) (TokenPair, error) {
	now := s.now()
	out := TokenPair{AccessToken: keys.NewRandom(), RefreshToken: keys.NewRandom(), AccessExpiresAt: now.Add(DeviceAccessTTL), RefreshExpiresAt: now.Add(DeviceRefreshTTL)}
	var expiry time.Time
	if err := tx.QueryRow(ctx, `SELECT g.scopes,g.expires_at FROM client_grants g JOIN users u ON u.id=g.user_id WHERE g.id=$1 AND g.revoked_at IS NULL AND g.expires_at>$2 AND u.status='active' AND u.auth_version=g.auth_version`, grantID, now).Scan(&out.Scopes, &expiry); err != nil {
		return TokenPair{}, apierrors.New(apierrors.GrantRevoked, "grant is no longer active")
	}
	if expiry.Before(out.RefreshExpiresAt) {
		out.RefreshExpiresAt = expiry
	}
	if expiry.Before(out.AccessExpiresAt) {
		out.AccessExpiresAt = expiry
	}
	_, err := tx.Exec(ctx, `INSERT INTO client_tokens(id,grant_id,access_hash,access_expires_at,refresh_hash,refresh_family_id,refresh_expires_at) VALUES($1,$2,$3,$4,$5,$2,$6)`, uuid.New(), grantID, keys.Hash(out.AccessToken), out.AccessExpiresAt, keys.Hash(out.RefreshToken), out.RefreshExpiresAt)
	return out, err
}

// Rotation and replay revocation share a row lock; concurrent use of an old
// refresh credential can never mint two independently valid successors.
func (s *Service) RotateRefreshToken(ctx context.Context, oldRefresh string) (TokenPair, error) {
	var out TokenPair
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		var token, grant, family uuid.UUID
		var revoked, rotated *time.Time
		var expiry time.Time
		if err := tx.QueryRow(ctx, `SELECT id,grant_id,refresh_family_id,revoked_at,refresh_expires_at,rotated_at FROM client_tokens WHERE refresh_hash=$1 FOR UPDATE`, keys.Hash(oldRefresh)).Scan(&token, &grant, &family, &revoked, &expiry, &rotated); err != nil {
			return apierrors.New(apierrors.GrantRevoked, "invalid refresh token")
		}
		now := s.now()
		if revoked != nil || !now.Before(expiry) {
			return apierrors.New(apierrors.GrantRevoked, "refresh token revoked or expired")
		}
		if rotated != nil {
			if _, err := tx.Exec(ctx, `UPDATE client_tokens SET revoked_at=$2 WHERE refresh_family_id=$1 AND revoked_at IS NULL`, family, now); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE client_grants SET revoked_at=$2 WHERE id=$1 AND revoked_at IS NULL`, grant, now); err != nil {
				return err
			}
			return apierrors.New(apierrors.GrantRevoked, "refresh replay revoked the token family").WithCommittedResult()
		}
		if _, err := tx.Exec(ctx, `UPDATE client_tokens SET rotated_at=$2 WHERE id=$1`, token, now); err != nil {
			return err
		}
		var err error
		out, err = s.issueTokens(ctx, tx, grant)
		return err
	})
	return out, err
}

// ResolveGrant authenticates a short-lived Bearer access secret into a CLI principal:
// user must stay active with the same auth_version, the grant unrevoked and
// within its project scope (07 §3 服务端每次调用核对).
func (s *Service) ResolveGrant(ctx context.Context, secret string) (*auth.Principal, error) {
	var (
		grantID      uuid.UUID
		accessID     uuid.UUID
		userID       uuid.UUID
		scopes       []string
		projectScope []uuid.UUID
		authVersion  int64
		grantRevoked *time.Time
		grantExpiry  time.Time
		tokenRevoked *time.Time
		tokenExpiry  time.Time
		tokenRotated *time.Time
	)
	err := s.pool.QueryRow(ctx, `
		SELECT g.id, g.user_id, g.scopes, g.project_scope, g.auth_version, g.revoked_at, g.expires_at,
		       t.revoked_at, t.access_expires_at, t.rotated_at,t.id
		FROM client_grants g
		JOIN client_tokens t ON t.grant_id=g.id
		WHERE t.access_hash=$1`, keys.Hash(secret)).
		Scan(&grantID, &userID, &scopes, &projectScope, &authVersion, &grantRevoked, &grantExpiry,
			&tokenRevoked, &tokenExpiry, &tokenRotated, &accessID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apierrors.New(apierrors.Unauthenticated, "无效凭据")
	}
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "grant lookup failed").Wrap(err)
	}
	now := s.now()
	switch {
	case grantRevoked != nil || tokenRevoked != nil || tokenRotated != nil:
		return nil, apierrors.New(apierrors.GrantRevoked, "授权已撤销")
	case !now.Before(grantExpiry) || !now.Before(tokenExpiry):
		return nil, apierrors.New(apierrors.GrantRevoked, "授权已过期")
	}
	var (
		status     string
		currentVer int64
	)
	if err := s.pool.QueryRow(ctx, `SELECT status, auth_version FROM users WHERE id=$1`, userID).
		Scan(&status, &currentVer); err != nil {
		return nil, apierrors.New(apierrors.Unauthenticated, "账号不可用")
	}
	if status != "active" || currentVer != authVersion {
		return nil, apierrors.New(apierrors.GrantRevoked, "凭据已轮换")
	}
	return &auth.Principal{
		Kind: auth.KindCLI, AccessTokenID: accessID,
		UserID:       userID,
		AuthVersion:  int(authVersion),
		GrantID:      grantID,
		Scopes:       scopes,
		ProjectScope: projectScope,
	}, nil
}

// ListGrants returns the caller's CLI device grants (safe projection).
func (s *Service) ListGrants(ctx context.Context, user uuid.UUID) ([]map[string]any, error) {
	rows, err := paging.Query(ctx, s.pool, `
		SELECT id, device_name, scopes, project_scope, created_at, expires_at, revoked_at
		/*keys*/ FROM client_grants WHERE user_id=$1 /*page*/`, "created_at", "id", user)
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "grants failed").Wrap(err)
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var (
			id           uuid.UUID
			device       string
			scopes       []string
			projectScope []uuid.UUID
			createdAt    time.Time
			expiresAt    time.Time
			revokedAt    *time.Time
		)
		if err := rows.Scan(&id, &device, &scopes, &projectScope, &createdAt, &expiresAt, &revokedAt); err != nil {
			return nil, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		out = append(out, map[string]any{
			"id": id, "deviceName": device, "scopes": scopes, "projectScope": projectScope,
			"createdAt": createdAt, "expiresAt": expiresAt, "revokedAt": revokedAt,
		})
	}
	return out, rows.Err()
}

// RevokeGrant revokes the caller's own grant (CLI logout, 07 §3).
func (s *Service) RevokeGrant(ctx context.Context, user, grantID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE client_grants SET revoked_at=COALESCE(revoked_at,$3)
		WHERE id=$1 AND user_id=$2`, grantID, user, s.now())
	if err != nil {
		return apierrors.New(apierrors.Internal, "revoke failed").Wrap(err)
	}
	if tag.RowsAffected() == 0 {
		return apierrors.New(apierrors.NotFound, "grant not found")
	}
	_, _ = s.pool.Exec(ctx, `UPDATE client_tokens SET revoked_at=now() WHERE grant_id=$1 AND revoked_at IS NULL`, grantID)
	return nil
}

func nonNilUUIDs(ids []string) []string {
	if ids == nil {
		return []string{}
	}
	return ids
}
