// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"errors"
	"github.com/kakj-go/Judex/internal/platform/paging"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/keys"
)

// ResolvedSession is the verified session context for one request. The CSRF
// token is a plain per-session value echoed by same-origin writes; it grants
// nothing on its own (the cookie secret stays HttpOnly).
type ResolvedSession struct {
	SessionID         uuid.UUID
	User              User
	CSRFToken         string
	ExpiresAt         time.Time
	LastSeen          time.Time
	lastSeenStaleness time.Duration
}

// ResolveSession verifies the opaque cookie secret against the stored hash
// and the account state: revoked/expired sessions, disabled users and stale
// auth_version all fail as UNAUTHENTICATED (02 §2).
func (s *Service) ResolveSession(ctx context.Context, secret string) (ResolvedSession, error) {
	if secret == "" {
		return ResolvedSession{}, apierrors.New(apierrors.Unauthenticated, "no session")
	}
	var (
		rs             ResolvedSession
		revoked        *time.Time
		sessionAuthVer int64
	)
	err := s.pool.QueryRow(ctx, `
		SELECT ses.id, ses.csrf_token, ses.expires_at, ses.last_seen_at,
		       ses.revoked_at, ses.auth_version,
		       u.id, u.display_name, u.email_display, u.locale, u.status,
		       u.auth_version, u.version, u.created_at
		FROM user_sessions ses JOIN users u ON u.id = ses.user_id
		WHERE ses.secret_hash = $1`, keys.Hash(secret)).
		Scan(&rs.SessionID, &rs.CSRFToken, &rs.ExpiresAt, &rs.LastSeen,
			&revoked, &sessionAuthVer,
			&rs.User.ID, &rs.User.DisplayName, &rs.User.Email, &rs.User.Locale, &rs.User.Status,
			&rs.User.AuthVersion, &rs.User.Version, &rs.User.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ResolvedSession{}, apierrors.New(apierrors.Unauthenticated, "invalid session")
	}
	if err != nil {
		return ResolvedSession{}, apierrors.New(apierrors.Internal, "session lookup failed").Wrap(err)
	}
	now := s.now()
	switch {
	case revoked != nil:
		return ResolvedSession{}, apierrors.New(apierrors.Unauthenticated, "session revoked")
	case now.After(rs.ExpiresAt):
		return ResolvedSession{}, apierrors.New(apierrors.SessionExpired, "session expired")
	case rs.User.Status != "active":
		return ResolvedSession{}, apierrors.New(apierrors.GrantRevoked, "account disabled")
	case sessionAuthVer != rs.User.AuthVersion:
		return ResolvedSession{}, apierrors.New(apierrors.GrantRevoked, "credentials rotated")
	}
	rs.lastSeenStaleness = now.Sub(rs.LastSeen)
	return rs, nil
}

// TouchSession updates last_seen at most once per minute (02 §2 节流).
func (s *Service) TouchSession(ctx context.Context, sessionID uuid.UUID, lastSeenStaleness time.Duration) {
	if lastSeenStaleness < time.Minute {
		return
	}
	_, _ = s.pool.Exec(ctx, `UPDATE user_sessions SET last_seen=$2 WHERE id=$1`, sessionID, s.now())
}

// Logout revokes one session; idempotent for already-ended sessions.
func (s *Service) Logout(ctx context.Context, sessionID uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE user_sessions SET revoked_at=COALESCE(revoked_at,$2) WHERE id=$1`, sessionID, s.now())
	if err != nil {
		return apierrors.New(apierrors.Internal, "logout failed").Wrap(err)
	}
	return nil
}

// LogoutAll revokes every OTHER session and all CLI grants for the user
// after verifying the expected auth version (02 §2).
func (s *Service) LogoutAll(ctx context.Context, userID, keepSessionID uuid.UUID, expectedAuthVersion int64) (int, int, error) {
	revokedSessions, revokedGrants := 0, 0
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockUserForShare(ctx, userID.String()); err != nil {
			return err
		}
		var current int64
		if err := tx.QueryRow(ctx, `SELECT auth_version FROM users WHERE id=$1`, userID).Scan(&current); err != nil {
			return apierrors.New(apierrors.Internal, "user lookup failed").Wrap(err)
		}
		if current != expectedAuthVersion {
			return apierrors.New(apierrors.VersionConflict, "auth version changed")
		}
		tag, err := tx.Exec(ctx, `
			UPDATE user_sessions SET revoked_at=COALESCE(revoked_at,$3)
			WHERE user_id=$1 AND id<>$2 AND revoked_at IS NULL`, userID, keepSessionID, s.now())
		if err != nil {
			return apierrors.New(apierrors.Internal, "session revoke failed").Wrap(err)
		}
		revokedSessions = int(tag.RowsAffected())
		tag, err = tx.Exec(ctx, `
			UPDATE client_grants SET revoked_at=COALESCE(revoked_at,$2)
			WHERE user_id=$1 AND revoked_at IS NULL`, userID, s.now())
		if err != nil {
			return apierrors.New(apierrors.Internal, "grant revoke failed").Wrap(err)
		}
		revokedGrants = int(tag.RowsAffected())
		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	return revokedSessions, revokedGrants, nil
}

// SessionProjection re-reads the mutable user projection for GET /auth/session
// (display name / locale may have changed since the middleware resolved it).
func (s *Service) SessionProjection(ctx context.Context, sessionID uuid.UUID) (User, time.Time, error) {
	var (
		user      User
		expiresAt time.Time
		revoked   *time.Time
		authVer   int64
	)
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.display_name, u.email_display, u.locale, u.status,
		       u.auth_version, u.version, u.created_at,
		       ses.expires_at, ses.revoked_at, ses.auth_version
		FROM user_sessions ses JOIN users u ON u.id = ses.user_id
		WHERE ses.id = $1`, sessionID).
		Scan(&user.ID, &user.DisplayName, &user.Email, &user.Locale, &user.Status,
			&user.AuthVersion, &user.Version, &user.CreatedAt,
			&expiresAt, &revoked, &authVer)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, time.Time{}, apierrors.New(apierrors.Unauthenticated, "invalid session")
	}
	if err != nil {
		return User{}, time.Time{}, apierrors.New(apierrors.Internal, "session lookup failed").Wrap(err)
	}
	if revoked != nil || s.now().After(expiresAt) || authVer != user.AuthVersion || user.Status != "active" {
		return User{}, time.Time{}, apierrors.New(apierrors.Unauthenticated, "session no longer valid")
	}
	return user, expiresAt, nil
}

// SessionSummary is the safe projection for GET /me/sessions (06 §2).
type SessionSummary struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	LastSeen  time.Time `json:"lastSeenAt"`
	Current   bool      `json:"current"`
}

// ListSessions returns the user's active sessions without secrets.
func (s *Service) ListSessions(ctx context.Context, user, currentSession uuid.UUID) ([]SessionSummary, error) {
	rows, err := paging.Query(ctx, s.pool, `
		SELECT id, created_at, expires_at, last_seen_at /*keys*/ FROM user_sessions
		WHERE user_id=$1 AND revoked_at IS NULL AND expires_at>$2
		/*page*/`, "created_at", "id", user, s.now())
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "session list failed").Wrap(err)
	}
	defer rows.Close()
	var out []SessionSummary
	for rows.Next() {
		var sum SessionSummary
		if err := rows.Scan(&sum.ID, &sum.CreatedAt, &sum.ExpiresAt, &sum.LastSeen); err != nil {
			return nil, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		sum.Current = sum.ID == currentSession
		out = append(out, sum)
	}
	return out, rows.Err()
}

// RevokeSession revokes one of the user's own sessions (idempotent).
func (s *Service) RevokeSession(ctx context.Context, user, sessionID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE user_sessions SET revoked_at=COALESCE(revoked_at,$3) WHERE id=$1 AND user_id=$2`,
		sessionID, user, s.now())
	if err != nil {
		return apierrors.New(apierrors.Internal, "revoke failed").Wrap(err)
	}
	if tag.RowsAffected() == 0 {
		return apierrors.New(apierrors.NotFound, "session not found")
	}
	return nil
}
