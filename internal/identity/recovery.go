// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/keys"
)

// RecoveryTTL is the one-time recovery code lifetime (02 §3).
const RecoveryTTL = 15 * time.Minute

// IssueRecoveryCode is the operator-audited path behind
// `judex-server account recovery-code`: it never authenticates the caller as
// the target user — deployment credentials ARE the authorization. The code is
// shown exactly once; only its hash persists.
func (s *Service) IssueRecoveryCode(ctx context.Context, email, operator, reason string) (string, error) {
	if operator == "" || reason == "" {
		return "", apierrors.New(apierrors.Validation, "operator and reason are required")
	}
	normalized := NormalizeEmail(email)
	var userID uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT id FROM users WHERE email_normalized=$1`, normalized).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		// Do not reveal whether the account exists in operator output detail;
		// the audit trail still records the attempt.
		return "", apierrors.New(apierrors.NotFound, "no such account")
	}
	if err != nil {
		return "", apierrors.New(apierrors.Internal, "lookup failed").Wrap(err)
	}
	code := keys.NewRandom()
	err = s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO recovery_tokens (id, user_id, code_hash, issued_by_operator, reason, expires_at, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			uuid.New(), userID, keys.Hash(code), operator, reason, s.now().Add(RecoveryTTL), s.now()); err != nil {
			return apierrors.New(apierrors.Internal, "recovery insert failed").Wrap(err)
		}
		return audit.Append(ctx, tx, audit.Entry{
			ActorType:   audit.ActorOperator,
			ActorUserID: &userID,
			Source:      audit.SourceOperator,
			Operation:   "identity.recovery_code.issued",
			ObjectType:  "user", ObjectID: userID.String(),
			Reason:     &reason,
			OccurredAt: s.now(),
		})
	})
	if err != nil {
		return "", err
	}
	return code, nil
}

// RecoverPassword consumes a one-time code, rotates the credential, bumps
// auth_version (invalidating every session and grant) and does NOT create a
// new session (02 §3: 不自动登录).
func (s *Service) RecoverPassword(ctx context.Context, code, newPassword string) error {
	if len(newPassword) < 12 || len(newPassword) > 128 {
		return apierrors.Fields("newPassword", "length")
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		return apierrors.New(apierrors.Internal, "hash failed").Wrap(err)
	}
	var (
		userID      uuid.UUID
		consumedAt  *time.Time
		expiresAt   time.Time
	)
	err = s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		// Advisory lock serializes concurrent use of the same code.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "recover:"+code); err != nil {
			return err
		}
		rowErr := tx.QueryRow(ctx, `
			SELECT user_id, consumed_at, expires_at FROM recovery_tokens WHERE code_hash=$1`,
			keys.Hash(code)).Scan(&userID, &consumedAt, &expiresAt)
		if errors.Is(rowErr, pgx.ErrNoRows) {
			return apierrors.New(apierrors.Unauthenticated, "恢复码无效")
		}
		if rowErr != nil {
			return apierrors.New(apierrors.Internal, "recovery lookup failed").Wrap(rowErr)
		}
		if consumedAt != nil {
			return apierrors.New(apierrors.Unauthenticated, "恢复码已使用")
		}
		if s.now().After(expiresAt) {
			return apierrors.New(apierrors.Unauthenticated, "恢复码已过期")
		}
		if err := tx.LockUserForShare(ctx, userID.String()); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `
			UPDATE recovery_tokens SET consumed_at=$2 WHERE code_hash=$1 AND consumed_at IS NULL`,
			keys.Hash(code), s.now())
		if err != nil || tag.RowsAffected() == 0 {
			return apierrors.New(apierrors.Unauthenticated, "恢复码已被使用")
		}
		if _, err := tx.Exec(ctx, `
			UPDATE password_credentials SET argon2id_hash=$2, password_changed_at=$3 WHERE user_id=$1`,
			userID, hash, s.now()); err != nil {
			return apierrors.New(apierrors.Internal, "credential update failed").Wrap(err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE users SET auth_version=auth_version+1, updated_at=$2, version=version+1 WHERE id=$1`,
			userID, s.now()); err != nil {
			return apierrors.New(apierrors.Internal, "auth_version bump failed").Wrap(err)
		}
		if _, err := tx.Exec(ctx, `UPDATE user_sessions SET revoked_at=COALESCE(revoked_at,$2) WHERE user_id=$1`, userID, s.now()); err != nil {
			return apierrors.New(apierrors.Internal, "session revoke failed").Wrap(err)
		}
		if _, err := tx.Exec(ctx, `UPDATE client_grants SET revoked_at=COALESCE(revoked_at,$2) WHERE user_id=$1`, userID, s.now()); err != nil {
			return apierrors.New(apierrors.Internal, "grant revoke failed").Wrap(err)
		}
		return audit.Append(ctx, tx, audit.Entry{
			ActorType:   audit.ActorUser,
			ActorUserID: &userID,
			Source:      audit.SourceWeb,
			Operation:   "identity.recovery.completed",
			ObjectType:  "user", ObjectID: userID.String(),
			OccurredAt: s.now(),
		})
	})
	return err
}

// DisableAccount stops a user from authenticating and kills active sessions
// and grants; history stays untouched (02 §3). Operator-audited.
func (s *Service) DisableAccount(ctx context.Context, email, operator, reason string) error {
	return s.setAccountStatus(ctx, email, operator, reason, "disabled")
}

// EnableAccount reverses a disable (kept explicit and audited).
func (s *Service) EnableAccount(ctx context.Context, email, operator, reason string) error {
	return s.setAccountStatus(ctx, email, operator, reason, "active")
}

func (s *Service) setAccountStatus(ctx context.Context, email, operator, reason, status string) error {
	if operator == "" || reason == "" {
		return apierrors.New(apierrors.Validation, "operator and reason are required")
	}
	var userID uuid.UUID
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.QueryRow(ctx,
			`SELECT id FROM users WHERE email_normalized=$1 FOR SHARE`, NormalizeEmail(email)).Scan(&userID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apierrors.New(apierrors.NotFound, "no such account")
			}
			return apierrors.New(apierrors.Internal, "lookup failed").Wrap(err)
		}
		tag, err := tx.Exec(ctx, `
			UPDATE users SET status=$2, updated_at=$3, version=version+1
			WHERE id=$1 AND status<>$2`, userID, status, s.now())
		if err != nil {
			return apierrors.New(apierrors.Internal, "status update failed").Wrap(err)
		}
		if tag.RowsAffected() == 0 {
			return apierrors.New(apierrors.InvalidTransition, "already in requested state")
		}
		if status == "disabled" {
			if _, err := tx.Exec(ctx, `UPDATE user_sessions SET revoked_at=COALESCE(revoked_at,$2) WHERE user_id=$1`, userID, s.now()); err != nil {
				return apierrors.New(apierrors.Internal, "session revoke failed").Wrap(err)
			}
			if _, err := tx.Exec(ctx, `UPDATE client_grants SET revoked_at=COALESCE(revoked_at,$2) WHERE user_id=$1`, userID, s.now()); err != nil {
				return apierrors.New(apierrors.Internal, "grant revoke failed").Wrap(err)
			}
		}
		return audit.Append(ctx, tx, audit.Entry{
			ActorType:   audit.ActorOperator,
			ActorUserID: &userID,
			Source:      audit.SourceOperator,
			Operation:   "identity.account." + status,
			ObjectType:  "user", ObjectID: userID.String(),
			Reason:     &reason,
			OccurredAt: s.now(),
		})
	})
	return err
}
