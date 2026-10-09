package identity

import (
	"context"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"strings"
	"time"
	"unicode/utf8"
)

func (s *Service) GrantProjection(ctx context.Context, user, grant uuid.UUID) (User, time.Time, error) {
	var out User
	var expires time.Time
	err := s.pool.QueryRow(ctx, `SELECT u.id,u.display_name,u.email_display,u.locale,u.status,u.version,u.created_at,g.expires_at FROM users u JOIN client_grants g ON g.user_id=u.id WHERE u.id=$1 AND g.id=$2 AND u.status='active' AND u.auth_version=g.auth_version AND g.revoked_at IS NULL AND g.expires_at>now()`, user, grant).Scan(&out.ID, &out.DisplayName, &out.Email, &out.Locale, &out.Status, &out.Version, &out.CreatedAt, &expires)
	if err != nil {
		return out, expires, apierrors.New(apierrors.Unauthenticated, "grant unavailable")
	}
	return out, expires, nil
}

func (s *Service) UpdateProfile(ctx context.Context, user uuid.UUID, version int64, name, locale *string) (User, error) {
	var out User
	if name != nil && (strings.TrimSpace(*name) == "" || utf8.RuneCountInString(*name) > 80) {
		return out, apierrors.Fields("displayName", "length")
	}
	if locale != nil && *locale != "zh-CN" && *locale != "en" {
		return out, apierrors.Fields("locale", "enum")
	}
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		err := tx.QueryRow(ctx, `UPDATE users SET display_name=COALESCE($3,display_name),locale=COALESCE($4,locale),version=version+1,updated_at=now()
   WHERE id=$1 AND version=$2 AND status='active' RETURNING id,display_name,email_display,locale,status,version,created_at`, user, version, name, locale).Scan(&out.ID, &out.DisplayName, &out.Email, &out.Locale, &out.Status, &out.Version, &out.CreatedAt)
		if err != nil {
			return apierrors.New(apierrors.VersionConflict, "account changed")
		}
		return audit.Append(ctx, tx, audit.Entry{ActorType: audit.ActorUser, ActorUserID: &user, Source: audit.SourceWeb, Operation: "identity.profile.update", ObjectType: "user", ObjectID: user.String(), OccurredAt: s.now()})
	})
	return out, err
}
func (s *Service) ChangePassword(ctx context.Context, user uuid.UUID, current, password string) (User, SessionToken, error) {
	var out User
	var session SessionToken
	if len(password) < 12 || len(password) > 128 || len(current) > 128 {
		return out, session, apierrors.Fields("password", "length")
	}
	hash, err := HashPassword(password)
	if err != nil {
		return out, session, err
	}
	err = s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		var stored string
		if err := tx.QueryRow(ctx, `SELECT c.argon2id_hash FROM users u JOIN password_credentials c ON c.user_id=u.id WHERE u.id=$1 AND u.status='active' FOR UPDATE OF u,c`, user).Scan(&stored); err != nil {
			return ErrInvalidCredentials
		}
		if ok, _ := VerifyPassword(stored, current); !ok {
			return ErrInvalidCredentials
		}
		if _, err := tx.Exec(ctx, `UPDATE password_credentials SET argon2id_hash=$2,password_changed_at=now() WHERE user_id=$1`, user, hash); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `UPDATE users SET auth_version=auth_version+1,version=version+1,updated_at=now() WHERE id=$1 RETURNING id,display_name,email_display,locale,status,version,auth_version,created_at`, user).Scan(&out.ID, &out.DisplayName, &out.Email, &out.Locale, &out.Status, &out.Version, &out.AuthVersion, &out.CreatedAt); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE user_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE user_id=$1`, user); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE client_grants SET revoked_at=COALESCE(revoked_at,now()) WHERE user_id=$1`, user); err != nil {
			return err
		}
		var err error
		session, err = s.insertSession(ctx, tx, user, out.AuthVersion)
		if err != nil {
			return err
		}
		return audit.Append(ctx, tx, audit.Entry{ActorType: audit.ActorUser, ActorUserID: &user, Source: audit.SourceWeb, Operation: "identity.password.change", ObjectType: "user", ObjectID: user.String(), OccurredAt: s.now()})
	})
	return out, session, err
}
