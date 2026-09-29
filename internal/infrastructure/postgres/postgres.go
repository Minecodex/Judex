// SPDX-License-Identifier: Apache-2.0

// Package postgres owns the pgx connection pool, the migration runner over
// embedded goose SQL files, and small transaction helpers. The fixed lock
// ordering from docs/plans/v1/01 §4 lives in the TxGuard helpers below.
package postgres

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.up.sql
var migrationsFS embed.FS

// Options mirrors the DB configuration contract (docs/plans/v1/11 §2).
type Options struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

func (o Options) withDefaults() Options {
	if o.MaxConns == 0 {
		o.MaxConns = 8
	}
	if o.MinConns == 0 {
		o.MinConns = 1
	}
	if o.MaxConnLifetime == 0 {
		o.MaxConnLifetime = 30 * time.Minute
	}
	if o.MaxConnIdleTime == 0 {
		o.MaxConnIdleTime = 5 * time.Minute
	}
	return o
}

// Pool wraps *pgxpool.Pool with Judex migration/lock helpers.
type Pool struct {
	*pgxpool.Pool
}

// Open creates the pool and pings it once; callers own closing.
func Open(ctx context.Context, opts Options, logger *slog.Logger) (*Pool, error) {
	if opts.URL == "" {
		return nil, errors.New("postgres: URL is required")
	}
	opts = opts.withDefaults()
	cfg, err := pgxpool.ParseConfig(opts.URL)
	if err != nil {
		return nil, fmt.Errorf("postgres: parse URL: %w", err)
	}
	cfg.MaxConns = opts.MaxConns
	cfg.MinConns = opts.MinConns
	cfg.MaxConnLifetime = opts.MaxConnLifetime
	cfg.MaxConnIdleTime = opts.MaxConnIdleTime
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: open: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	if logger != nil {
		logger.Info("postgres pool opened", "max_conns", cfg.MaxConns)
	}
	return &Pool{Pool: pool}, nil
}

// MigrationLockKey is the dedicated advisory-lock key for DDL; business
// commands never take it (docs/plans/v1/01 §6).
const MigrationLockKey int64 = 910_000_001

// Migrate applies all embedded migrations while holding the migration
// advisory lock, so concurrent migration Jobs cannot interleave DDL.
func (p *Pool) Migrate(ctx context.Context) error {
	conn, err := p.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", MigrationLockKey); err != nil {
		return fmt.Errorf("postgres: migration lock: %w", err)
	}
	defer conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", MigrationLockKey)

	db := stdlib.OpenDBFromPool(p.Pool)
	defer db.Close()
	migrationRoot, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("postgres: migration fs: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrationRoot,
		goose.WithLogger(goose.NopLogger()))
	if err != nil {
		return fmt.Errorf("postgres: goose provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("postgres: migrate up: %w", err)
	}
	return nil
}

// Transact runs fn inside a serializable-safe repeatable read transaction:
// retryable serialization failures are retried with the given budget, and fn
// must be side-effect-free outside the database. Business commands rely on
// row locks instead of isolation tricks (docs/plans/v1/01 §4).
func (p *Pool) Transact(ctx context.Context, fn func(ctx context.Context, tx Tx) error) error {
	if tx := p.transaction(ctx); tx != nil {
		return fn(ctx, Tx{tx})
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		tx, err := p.BeginTx(ctx, pgxTxOpts)
		if err != nil {
			return err
		}
		err = fn(WithTransaction(ctx, p, tx), Tx{tx})
		if err != nil {
			if apierrors.From(err).CommitResult {
				if commitErr := tx.Commit(ctx); commitErr != nil {
					return commitErr
				}
				return err
			}
			tx.Rollback(ctx)
			var ser *serializationFailure
			if errors.As(err, &ser) && attempt < 2 {
				lastErr = err
				continue
			}
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			var ser *serializationFailure
			if errors.As(err, &ser) && attempt < 2 {
				lastErr = err
				continue
			}
			return err
		}
		return nil
	}
	return lastErr
}
