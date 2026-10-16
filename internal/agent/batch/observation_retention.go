// SPDX-License-Identifier: Apache-2.0
package batch

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const ModelObservationRetention = 24 * time.Hour
const ModelObservationSweepInterval = time.Minute

// PruneModelObservations expires only temporary measurement fields. The call
// row, attempt, reservation and total cost are needed by recovery/accounting.
// A partial expiry index and bounded SKIP LOCKED batches keep maintenance
// independent of active runs and safe across multiple worker processes.
func PruneModelObservations(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	const batchSize = 500
	var removed int64
	for pass := 0; pass < 16; pass++ {
		tag, err := pool.Exec(ctx, `WITH expired AS (
 SELECT id FROM model_calls
 WHERE ended_at <= now()-($1::bigint*interval '1 second') AND usage_json ? 'observation'
 ORDER BY ended_at,id LIMIT $2 FOR UPDATE SKIP LOCKED
)
UPDATE model_calls c SET usage_json=c.usage_json-'observation',provider_request_id=NULL
FROM expired e WHERE c.id=e.id`, int64(ModelObservationRetention/time.Second), batchSize)
		if err != nil {
			return removed, err
		}
		removed += tag.RowsAffected()
		if tag.RowsAffected() < batchSize {
			break
		}
	}
	return removed, nil
}

// MaintainModelObservations runs once at worker startup, then every minute.
// No maintenance job rows accumulate, and a restart catches up with expiry.
func MaintainModelObservations(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) {
	if logger == nil {
		logger = slog.Default()
	}
	ticker := time.NewTicker(ModelObservationSweepInterval)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		sweepCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		removed, err := PruneModelObservations(sweepCtx, pool)
		cancel()
		if err != nil && ctx.Err() == nil {
			logger.Warn("model observation expiry failed", "error", err)
		} else if removed > 0 {
			logger.Info("expired temporary model observations", "calls", removed)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
