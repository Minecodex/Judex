// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RateLimiter is a PG-backed fixed-window counter shared by all replicas
// (docs/plans/v1/02 §1). Buckets look like "login-account:<normalized>",
// "login-ip:<ip>", "register-ip:<ip>". Windows never lock accounts forever —
// they expire and recover automatically.
type RateLimiter struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

func NewRateLimiter(pool *pgxpool.Pool, now func() time.Time) *RateLimiter {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &RateLimiter{pool: pool, now: now}
}

// Allow increments the bucket for one window and reports whether the action
// is still under limit. Buckets reset at each window boundary.
func (r *RateLimiter) Allow(ctx context.Context, bucket string, limit int64, window time.Duration) (bool, time.Duration, error) {
	now := r.now()
	windowStart := now.Truncate(window)
	nextWindow := windowStart.Add(window)
	var count int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO rate_limit_windows (bucket, window_start, count)
		VALUES ($1, $2, 1)
		ON CONFLICT (bucket) DO UPDATE SET
			count = CASE WHEN rate_limit_windows.window_start = $2
			             THEN rate_limit_windows.count + 1
			             ELSE 1 END,
			window_start = $2
		RETURNING count`, bucket, windowStart).Scan(&count)
	if err != nil {
		return false, 0, err
	}
	return count <= limit, nextWindow.Sub(now), nil
}

// Cleanup drops windows older than the given horizon; callers schedule it as
// a periodic job.
func (r *RateLimiter) Cleanup(ctx context.Context, horizon time.Duration) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM rate_limit_windows WHERE window_start < $1`, r.now().Add(-horizon))
	return err
}
