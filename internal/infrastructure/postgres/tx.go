// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var pgxTxOpts = pgx.TxOptions{}

// serializationFailure matches PG 40001/40P01 so Transact can retry.
type serializationFailure struct{ cause error }

func (s *serializationFailure) Error() string { return s.cause.Error() }
func (s *serializationFailure) Unwrap() error { return s.cause }

func asSerialization(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "40001" || pgErr.Code == "40P01") {
		return &serializationFailure{cause: err}
	}
	return err
}

// Tx is the transaction handle passed into Transact; it exposes lock helpers
// implementing the fixed ordering of docs/plans/v1/01 §4. Every formal
// business command must lock user rows FOR SHARE first, then the project row
// FOR UPDATE, then aggregate rows ordered by id — never a different order.
type Tx struct {
	pgx.Tx
}

// LockUserForShare pins the acting user's row (status/auth_version checks
// then read the locked row). Must be called before LockProjectForUpdate.
func (t Tx) LockUserForShare(ctx context.Context, userID string) error {
	_, err := t.Exec(ctx, "SELECT id FROM users WHERE id = $1 FOR SHARE", userID)
	return err
}

// LockProjectForUpdate serializes formal writes for one project. Different
// projects proceed in parallel (E05).
func (t Tx) LockProjectForUpdate(ctx context.Context, projectID string) error {
	_, err := t.Exec(ctx, "SELECT id FROM projects WHERE id = $1 FOR UPDATE", projectID)
	return err
}

// AdvisoryLock takes a session advisory lock inside this transaction
// (released at commit/rollback) for cross-row coordination that row locks
// cannot express (e.g. per-identity session serialization).
func (t Tx) AdvisoryLock(ctx context.Context, key int64) error {
	_, err := t.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", key)
	return err
}

// QueryRow/Exec passthroughs are inherited from pgx.Tx; helpers below add
// the retry classification so Transact can see serialization failures.
func (t Tx) checked(err error) error { return asSerialization(err) }
