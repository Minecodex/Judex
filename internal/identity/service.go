// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/keys"
)

// Options tunes identity behavior (docs/plans/v1/02 §1-§2 defaults).
type Options struct {
	SessionIdleTTL  time.Duration // default 24h
	SessionAbsTTL   time.Duration // default 7d
	LoginPerAccount int64         // default 5/min
	LoginPerIP      int64         // default 30/min
	RegisterPerIP   int64         // default 10/min
}

func (o Options) withDefaults() Options {
	if o.SessionIdleTTL == 0 {
		o.SessionIdleTTL = 24 * time.Hour
	}
	if o.SessionAbsTTL == 0 {
		o.SessionAbsTTL = 7 * 24 * time.Hour
	}
	if o.LoginPerAccount == 0 {
		o.LoginPerAccount = 5
	}
	if o.LoginPerIP == 0 {
		o.LoginPerIP = 30
	}
	if o.RegisterPerIP == 0 {
		o.RegisterPerIP = 10
	}
	return o
}

// User is the public projection (no credentials).
type User struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"displayName"`
	Email       string    `json:"email"`
	Locale      string    `json:"locale"`
	Status      string    `json:"status"`
	AuthVersion int64     `json:"-"`
	Version     int64     `json:"version"`
	CreatedAt   time.Time `json:"createdAt"`
}

// SessionToken pairs the once-only secret with its DB row id.
type SessionToken struct {
	SessionID uuid.UUID
	Secret    string // urlsafe random; only the hash is stored
	CSRFToken string // per-session CSRF token (secret itself)
	ExpiresAt time.Time
}

// Service implements accounts + credentials + sessions over PostgreSQL.
type Service struct {
	pool    *postgres.Pool
	limiter *RateLimiter
	opts    Options
	now     func() time.Time
}

func NewService(pool *postgres.Pool, limiter *RateLimiter, opts Options, now func() time.Time) *Service {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{pool: pool, limiter: limiter, opts: opts.withDefaults(), now: now}
}

// ErrInvalidCredentials is the single login failure for every cause.
var ErrInvalidCredentials = apierrors.New(apierrors.Unauthenticated, "账号或密码不正确")

// ValidateRegistration enforces the field limits of 02 §1 without inventing
// character-class rules.
func ValidateRegistration(displayName, email, password string) error {
	displayName = strings.TrimSpace(displayName)
	if l := utf8.RuneCountInString(displayName); l < 1 || l > 80 {
		return apierrors.Fields("displayName", "length")
	}
	if len(email) == 0 || len(email) > 254 || !strings.Contains(email, "@") {
		return apierrors.Fields("email", "format")
	}
	if len(password) < 12 || len(password) > 128 {
		return apierrors.Fields("password", "length")
	}
	return nil
}

// Register creates the account and its first session in one transaction
// (02 §1: 注册成功创建账号和登录会话). Concurrent duplicate emails collapse
// onto the unique index and surface EMAIL_IN_USE.
func (s *Service) Register(ctx context.Context, displayName, email, password, clientIP string) (User, SessionToken, error) {
	if s.limiter != nil && clientIP != "" {
		ok, _, err := s.limiter.Allow(ctx, "register-ip:"+clientIP, s.opts.RegisterPerIP, time.Minute)
		if err != nil {
			return User{}, SessionToken{}, apierrors.Newf(apierrors.DependencyDown, "rate limiter unavailable").WithRetryable(true).Wrap(err)
		}
		if !ok {
			return User{}, SessionToken{}, apierrors.New(apierrors.RateLimited, "注册过于频繁，请稍后再试").WithRetryable(true)
		}
	}
	if err := ValidateRegistration(displayName, email, password); err != nil {
		return User{}, SessionToken{}, err
	}
	normalized := NormalizeEmail(email)
	hash, err := HashPassword(password)
	if err != nil {
		return User{}, SessionToken{}, apierrors.New(apierrors.Internal, "hash failed").Wrap(err)
	}

	var user User
	var token SessionToken
	err = s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		userID := uuid.New()
		now := s.now()
		if err := tx.QueryRow(ctx, `
			INSERT INTO users (id, email_normalized, email_display, display_name, locale, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'zh-CN', $5, $5)
			RETURNING id, display_name, email_display, locale, status, auth_version, version, created_at`,
			userID, normalized, strings.TrimSpace(email), strings.TrimSpace(displayName), now).
			Scan(&user.ID, &user.DisplayName, &user.Email, &user.Locale, &user.Status, &user.AuthVersion, &user.Version, &user.CreatedAt); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apierrors.New(apierrors.EmailInUse, "该邮箱已被注册")
			}
			var pgErr interface{ SQLState() string }
			if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
				return apierrors.New(apierrors.EmailInUse, "该邮箱已被注册")
			}
			return apierrors.New(apierrors.Internal, "register failed").Wrap(err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO password_credentials (user_id, argon2id_hash, password_changed_at)
			VALUES ($1, $2, $3)`, userID, hash, now); err != nil {
			return apierrors.New(apierrors.Internal, "credential insert failed").Wrap(err)
		}
		var err error
		token, err = s.insertSession(ctx, tx, userID, 1)
		if err != nil {
			return err
		}
		return audit.Append(ctx, tx, audit.Entry{
			ActorType:   audit.ActorUser,
			ActorUserID: &userID,
			Source:      audit.SourceWeb,
			Operation:   "identity.register",
			ObjectType:  "user", ObjectID: userID.String(),
			OccurredAt: now,
		})
	})
	if err != nil {
		return User{}, SessionToken{}, err
	}
	return user, token, nil
}

// Login verifies credentials and creates a fresh session. Wrong password,
// unknown email and disabled account all return the same error; successful
// logins with legacy hash parameters are progressively rehashed in the same
// transaction (02 §1).
func (s *Service) Login(ctx context.Context, email, password, clientIP string) (User, SessionToken, error) {
	normalized := NormalizeEmail(email)
	if s.limiter != nil {
		if clientIP != "" {
			if ok, wait, err := s.limiter.Allow(ctx, "login-ip:"+clientIP, s.opts.LoginPerIP, time.Minute); err != nil {
				return User{}, SessionToken{}, apierrors.Newf(apierrors.DependencyDown, "rate limiter unavailable").WithRetryable(true).Wrap(err)
			} else if !ok {
				return User{}, SessionToken{}, retryAfter(apierrors.New(apierrors.RateLimited, "尝试过于频繁，请稍后再试").WithRetryable(true), wait)
			}
		}
		if ok, wait, err := s.limiter.Allow(ctx, "login-account:"+normalized, s.opts.LoginPerAccount, time.Minute); err != nil {
			return User{}, SessionToken{}, apierrors.Newf(apierrors.DependencyDown, "rate limiter unavailable").WithRetryable(true).Wrap(err)
		} else if !ok {
			return User{}, SessionToken{}, retryAfter(apierrors.New(apierrors.RateLimited, "尝试过于频繁，请稍后再试").WithRetryable(true), wait)
		}
	}

	var (
		user     User
		credHash string
		authVer  int64
	)
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.display_name, u.email_display, u.locale, u.status,
		       u.auth_version, u.version, u.created_at, p.argon2id_hash
		FROM users u JOIN password_credentials p ON p.user_id = u.id
		WHERE u.email_normalized = $1`, normalized).
		Scan(&user.ID, &user.DisplayName, &user.Email, &user.Locale, &user.Status,
			&authVer, &user.Version, &user.CreatedAt, &credHash)
	if errors.Is(err, pgx.ErrNoRows) {
		// Burn comparable CPU so unknown accounts are not timing-distinguishable.
		_, _ = VerifyPassword("$argon2id$v=19$m=65536,t=3,p=1$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", password)
		return User{}, SessionToken{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, SessionToken{}, apierrors.New(apierrors.Internal, "login query failed").Wrap(err)
	}
	if user.Status != "active" {
		return User{}, SessionToken{}, ErrInvalidCredentials
	}
	ok, needsRehash := VerifyPassword(credHash, password)
	if !ok {
		return User{}, SessionToken{}, ErrInvalidCredentials
	}

	var token SessionToken
	err = s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if needsRehash {
			newHash, err := HashPassword(password)
			if err != nil {
				return apierrors.New(apierrors.Internal, "rehash failed").Wrap(err)
			}
			if _, err := tx.Exec(ctx, `
				UPDATE password_credentials SET argon2id_hash=$2, password_changed_at=$3
				WHERE user_id=$1`, user.ID, newHash, s.now()); err != nil {
				return apierrors.New(apierrors.Internal, "rehash update failed").Wrap(err)
			}
		}
		var err error
		token, err = s.insertSession(ctx, tx, user.ID, authVer)
		if err != nil {
			return err
		}
		return audit.Append(ctx, tx, audit.Entry{
			ActorType:   audit.ActorUser,
			ActorUserID: &user.ID,
			Source:      audit.SourceWeb,
			Operation:   "identity.login",
			ObjectType:  "user", ObjectID: user.ID.String(),
			OccurredAt: s.now(),
		})
	})
	if err != nil {
		return User{}, SessionToken{}, err
	}
	user.AuthVersion = authVer
	return user, token, nil
}

func retryAfter(err *apierrors.Error, d time.Duration) *apierrors.Error {
	return err.WithDetails(map[string]any{"retryAfterSeconds": int(d.Seconds()) + 1})
}

// insertSession stores only hashes; the secret returns once.
func (s *Service) insertSession(ctx context.Context, tx pgx.Tx, userID uuid.UUID, authVersion int64) (SessionToken, error) {
	secret := keys.NewRandom()
	csrf := keys.NewRandom()
	sessionID := uuid.New()
	now := s.now()
	expiresAt := now.Add(s.opts.SessionAbsTTL)
	_, err := tx.Exec(ctx, `
		INSERT INTO user_sessions (id, user_id, secret_hash, auth_version, created_at, expires_at, last_seen_at, csrf_token)
		VALUES ($1, $2, $3, $4, $5, $6, $5, $7)`,
		sessionID, userID, keys.Hash(secret), authVersion, now, expiresAt, csrf)
	if err != nil {
		return SessionToken{}, apierrors.New(apierrors.Internal, "session insert failed").Wrap(err)
	}
	return SessionToken{SessionID: sessionID, Secret: secret, CSRFToken: csrf, ExpiresAt: expiresAt}, nil
}
