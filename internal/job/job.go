// SPDX-License-Identifier: Apache-2.0

// Package job implements the PG-backed durable job queue from docs/plans/v1
// 01 §5: FOR UPDATE SKIP LOCKED claiming, fencing tokens so a stalled worker
// can never overwrite a successor's result, lease heartbeat, exponential
// backoff with jitter, and per-kind handlers registered by later stages.
package job

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

// Job is a claimed unit of background work.
type Job struct {
	ID            uuid.UUID
	Kind          string
	Payload       json.RawMessage
	Attempt       int
	MaxAttempts   int
	FencingToken  int64
	LeaseOwner    string
	RunAfter      time.Time
	LastErrorCode *string
}

// Handler executes one job kind. Returning an *apierrors.Error with
// Retryable=false and a non-INTERNAL code fails the job immediately; any
// other outcome retries with backoff until MaxAttempts.
type Handler interface {
	Kind() string
	MaxAttempts() int
	Execute(ctx context.Context, j Job) error
}

// HandlerFunc adapts plain functions to Handler.
type HandlerFunc struct {
	KindName  string
	Attempts  int
	ExecuteFn func(ctx context.Context, j Job) error
}

func (h HandlerFunc) Kind() string                             { return h.KindName }
func (h HandlerFunc) MaxAttempts() int                         { return h.Attempts }
func (h HandlerFunc) Execute(ctx context.Context, j Job) error { return h.ExecuteFn(ctx, j) }

// Options tunes the engine; zero fields fall back to 01 §5 defaults.
type Options struct {
	PollInterval time.Duration
	LeaseTTL     time.Duration
	Heartbeat    time.Duration
	BatchSize    int
	BackoffBase  time.Duration
	BackoffCap   time.Duration
}

func (o Options) withDefaults() Options {
	if o.PollInterval == 0 {
		o.PollInterval = 2 * time.Second
	}
	if o.LeaseTTL == 0 {
		o.LeaseTTL = 60 * time.Second
	}
	if o.Heartbeat == 0 {
		o.Heartbeat = 20 * time.Second
	}
	if o.BatchSize == 0 {
		o.BatchSize = 10
	}
	if o.BackoffBase == 0 {
		o.BackoffBase = 2 * time.Second
	}
	if o.BackoffCap == 0 {
		o.BackoffCap = 10 * time.Minute
	}
	return o
}

// Enqueue inserts a job in the caller's business transaction (docs/plans/v1
// 01 §5: 同事务入队). uniqueKey (kind-scoped) deduplicates logical work.
func Enqueue(ctx context.Context, tx pgx.Tx, kind string, payload any, uniqueKey *string, runAfter, now time.Time) (uuid.UUID, error) {
	id := uuid.New()
	raw, err := json.Marshal(payload)
	if err != nil {
		return id, fmt.Errorf("job: marshal payload: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO background_jobs (id, kind, payload, unique_key, run_after, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$6)`,
		id, kind, raw, uniqueKey, runAfter, now)
	if err != nil {
		return id, fmt.Errorf("job: enqueue: %w", err)
	}
	return id, nil
}

// Engine claims and executes jobs; multiple engines (processes/goroutines)
// cooperate through SKIP LOCKED + fencing.
type Engine struct {
	pool     *pgxpool.Pool
	handlers map[string]Handler
	opts     Options
	logger   *slog.Logger
	owner    string
	now      func() time.Time
}

// NewEngine builds the engine over an existing pool.
func NewEngine(pool *pgxpool.Pool, opts Options, logger *slog.Logger, now func() time.Time) *Engine {
	if logger == nil {
		logger = slog.Default()
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Engine{
		pool:     pool,
		handlers: map[string]Handler{},
		opts:     opts.withDefaults(),
		logger:   logger,
		owner:    fmt.Sprintf("worker-%s", uuid.NewString()[:8]),
		now:      now,
	}
}

// Register makes a handler available; duplicate kinds panic at wiring time.
func (e *Engine) Register(h Handler) {
	if _, exists := e.handlers[h.Kind()]; exists {
		panic("job: duplicate handler kind " + h.Kind())
	}
	e.handlers[h.Kind()] = h
}

// ErrNoJob is returned by Claim when the queue has nothing runnable.
var ErrNoJob = errors.New("job: nothing to claim")

// Claim takes the next runnable job: a short transaction locks candidate rows
// FOR UPDATE SKIP LOCKED, flips them to running, bumps the fencing token and
// takes a lease (01 §5).
func (e *Engine) Claim(ctx context.Context) (*Job, error) {
	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	j := &Job{}
	err = tx.QueryRow(ctx, `
		UPDATE background_jobs SET
			state = 'running',
			attempt = attempt + 1,
			lease_owner = $1,
			lease_until = $2,
			fencing_token = fencing_token + 1,
			updated_at = $2
		WHERE id = (
			SELECT id FROM background_jobs
			WHERE (state = 'queued' AND run_after <= $2)
			   OR (state = 'running' AND lease_until < $2)
			ORDER BY run_after
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING id, kind, payload, attempt, max_attempts, fencing_token, lease_owner, run_after, last_error_code`,
		e.owner, e.now()).Scan(
		&j.ID, &j.Kind, &j.Payload, &j.Attempt, &j.MaxAttempts,
		&j.FencingToken, &j.LeaseOwner, &j.RunAfter, &j.LastErrorCode)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoJob
	}
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return j, nil
}

// Heartbeat extends the lease; it must never resurrect a lost job, so it
// matches on both id and fencing token.
func (e *Engine) Heartbeat(ctx context.Context, j *Job) error {
	tag, err := e.pool.Exec(ctx, `
		UPDATE background_jobs SET lease_until = $2, updated_at = $2
		WHERE id = $1 AND fencing_token = $3 AND state = 'running'`,
		j.ID, e.now().Add(e.opts.LeaseTTL), j.FencingToken)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("job %s heartbeat lost (fencing token %d)", j.ID, j.FencingToken)
	}
	return nil
}

// finish records the terminal/near-terminal state guarded by the fencing
// token: a stale worker whose lease was taken over writes zero rows.
func (e *Engine) Finish(ctx context.Context, j *Job, execErr error) error {
	now := e.now()
	if execErr == nil {
		_, err := e.pool.Exec(ctx, `
			UPDATE background_jobs SET state='succeeded', lease_owner=NULL, lease_until=NULL,
			       updated_at=$2, last_error=NULL, last_error_code=NULL
			WHERE id=$1 AND fencing_token=$3`, j.ID, now, j.FencingToken)
		return err
	}
	apiErr := apierrors.From(execErr)
	nonRetryable := apiErr.Code != apierrors.Internal && !apiErr.Retryable
	if nonRetryable || j.Attempt >= j.MaxAttempts {
		state := "failed"
		if j.Attempt >= j.MaxAttempts {
			state = "dead"
		}
		code := string(apiErr.Code)
		_, err := e.pool.Exec(ctx, `
			UPDATE background_jobs SET state=$4, lease_owner=NULL, lease_until=NULL,
			       updated_at=$2, last_error=$5, last_error_code=$6, run_after=$2
			WHERE id=$1 AND fencing_token=$3`, j.ID, now, j.FencingToken, state, execErr.Error(), code)
		return err
	}
	backoff := e.backoff(j.Attempt)
	_, err := e.pool.Exec(ctx, `
		UPDATE background_jobs SET state='queued', lease_owner=NULL, lease_until=NULL,
		       updated_at=$2, run_after=$4, last_error=$5, last_error_code=$6
		WHERE id=$1 AND fencing_token=$3`, j.ID, now, j.FencingToken, now.Add(backoff), execErr.Error(), string(apiErr.Code))
	return err
}

func (e *Engine) backoff(attempt int) time.Duration {
	d := e.opts.BackoffBase
	for i := 1; i < attempt; i++ {
		d *= 2
		if d > e.opts.BackoffCap {
			d = e.opts.BackoffCap
			break
		}
	}
	jitter := time.Duration(rand.Int63n(int64(d / 4)))
	return d + jitter
}

// Run claims and executes jobs until ctx ends. Intended to run in several
// goroutines/processes; each iteration handles one job with heartbeats.
func (e *Engine) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		j, err := e.Claim(ctx)
		if err != nil {
			if !errors.Is(err, ErrNoJob) {
				e.logger.Error("job claim failed", "error", err)
			}
			sleep(ctx, e.opts.PollInterval)
			continue
		}
		handler, ok := e.handlers[j.Kind]
		if !ok {
			e.Finish(ctx, j, apierrors.Newf(apierrors.Internal, "no handler for kind %s", j.Kind))
			e.logger.Error("job without handler", "kind", j.Kind, "id", j.ID)
			continue
		}
		if j.MaxAttempts == 0 {
			j.MaxAttempts = handler.MaxAttempts()
		}
		e.execute(ctx, handler, j)
	}
}

func (e *Engine) execute(ctx context.Context, handler Handler, j *Job) {
	hbCtx, stop := context.WithCancel(ctx)
	defer stop()
	go func() {
		ticker := time.NewTicker(e.opts.Heartbeat)
		defer ticker.Stop()
		for {
			select {
			case <-hbCtx.Done():
				return
			case <-ticker.C:
				if err := e.Heartbeat(hbCtx, j); err != nil {
					e.logger.Warn("job heartbeat lost", "id", j.ID, "error", err)
					stop()
					return
				}
			}
		}
	}()
	execCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	err := handler.Execute(execCtx, *j)
	if err := e.Finish(context.WithoutCancel(ctx), j, err); err != nil {
		e.logger.Error("job finish failed", "id", j.ID, "error", err)
	}
}

func sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
