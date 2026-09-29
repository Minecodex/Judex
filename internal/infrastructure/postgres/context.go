package postgres

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type transactionKey struct{}
type transactionContext struct {
	pool *Pool
	tx   pgx.Tx
}

// WithTransaction joins domain commands and their reads to one application
// transaction. Ownership is explicit; another pool never inherits this tx.
func WithTransaction(ctx context.Context, pool *Pool, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, transactionKey{}, transactionContext{pool, tx})
}
func (p *Pool) transaction(ctx context.Context) pgx.Tx {
	if v, ok := ctx.Value(transactionKey{}).(transactionContext); ok && v.pool == p {
		return v.tx
	}
	return nil
}
func (p *Pool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if tx := p.transaction(ctx); tx != nil {
		return tx.Exec(ctx, sql, args...)
	}
	return p.Pool.Exec(ctx, sql, args...)
}
func (p *Pool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if tx := p.transaction(ctx); tx != nil {
		return tx.Query(ctx, sql, args...)
	}
	return p.Pool.Query(ctx, sql, args...)
}
func (p *Pool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if tx := p.transaction(ctx); tx != nil {
		return tx.QueryRow(ctx, sql, args...)
	}
	return p.Pool.QueryRow(ctx, sql, args...)
}
