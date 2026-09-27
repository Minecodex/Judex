// SPDX-License-Identifier: Apache-2.0

// Package idempotency implements the in-transaction idempotency protocol of
// docs/plans/v1/01 §4: same key + same payload replay returns the stored
// result (after access re-check), same key + different payload is 409, and
// the record is written in the SAME transaction as the business effect.
package idempotency

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/keys"
)

// Decision tells the command how to continue after Begin.
type Decision int

const (
	// Proceed means no prior record existed (or it was failed/expired): the
	// command runs and MUST call Complete/Fail in the same transaction.
	Proceed Decision = iota
	// Replay means a completed record matched: return the stored response
	// after re-checking current access (handler responsibility).
	Replay
	// Conflict means the same key carried a different payload.
	Conflict
)

// Begin checks/inserts the idempotency record inside the caller's tx.
// actorKey derives from the server-resolved principal (never client input).
// A transaction-scoped advisory lock on the key serializes racing inserts:
// without it two transactions can both see "no row" and both proceed.
func Begin(ctx context.Context, tx pgx.Tx, actorKey, operation, key string, requestHash []byte, ttl time.Duration, now time.Time) (Decision, int, []byte, error) {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1 || ':' || $2 || ':' || $3))`, actorKey, operation, key); err != nil {
		return 0, 0, nil, err
	}
	var (
		storedHash  []byte
		status      *int
		body        []byte
		expiresAt   time.Time
		recordState string
	)
	err := tx.QueryRow(ctx, `
		SELECT request_hash, status, response_status, response_body, expires_at
		FROM idempotency_records
		WHERE actor_key = $1 AND operation = $2 AND key = $3
		FOR UPDATE`, actorKey, operation, key).Scan(&storedHash, &recordState, &status, &body, &expiresAt)
	switch {
	case err == nil:
		if !keys.Equal(storedHash, requestHash) {
			return Conflict, 0, nil, apierrors.New(apierrors.IdempotencyConflict,
				"idempotency key was already used with a different payload")
		}
		if recordState == "completed" {
			if expiresAt.Before(now) {
				// Expired record: reuse the slot by overwriting below.
				break
			}
			statusCode := 0
			if status != nil {
				statusCode = *status
			}
			return Replay, statusCode, body, nil
		}
		// in_flight or failed: the command may proceed and overwrite.
	case err == pgx.ErrNoRows:
		// no record yet
	default:
		return 0, 0, nil, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO idempotency_records (actor_key, operation, key, request_hash, status, created_at, expires_at)
		VALUES ($1, $2, $3, $4, 'in_flight', $5, $6)
		ON CONFLICT (actor_key, operation, key) DO UPDATE
			SET request_hash = EXCLUDED.request_hash,
			    status = 'in_flight',
			    response_status = NULL,
			    response_body = NULL,
			    created_at = EXCLUDED.created_at,
			    expires_at = EXCLUDED.expires_at`,
		actorKey, operation, key, requestHash, now, now.Add(ttl))
	if err != nil {
		return 0, 0, nil, err
	}
	return Proceed, 0, nil, nil
}

// Complete stores the successful response in the same transaction.
func Complete(ctx context.Context, tx pgx.Tx, actorKey, operation, key string, status int, body []byte) error {
	_, err := tx.Exec(ctx, `
		UPDATE idempotency_records
		SET status = 'completed', response_status = $4, response_body = $5
		WHERE actor_key = $1 AND operation = $2 AND key = $3`,
		actorKey, operation, key, status, body)
	return err
}

// Fail marks the attempt failed; the same key may then be retried cleanly.
func Fail(ctx context.Context, tx pgx.Tx, actorKey, operation, key string) error {
	_, err := tx.Exec(ctx, `
		UPDATE idempotency_records SET status = 'failed'
		WHERE actor_key = $1 AND operation = $2 AND key = $3`,
		actorKey, operation, key)
	return err
}

// NewKey returns a fresh client-side idempotency key (UUIDv4).
func NewKey() string { return uuid.NewString() }
