// SPDX-License-Identifier: Apache-2.0
package integrationtest_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	"github.com/kakj-go/Judex/internal/job"
	"github.com/kakj-go/Judex/internal/platform/clock"
	"github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/events"
	"github.com/kakj-go/Judex/internal/platform/idempotency"
	integration "github.com/kakj-go/Judex/tests/integration"
)

func seedProject(t *testing.T, pool *postgres.Pool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO projects (id, title, creator_user_id, owner_user_id, created_at, updated_at)
		VALUES ($1,'测试项目',$2,$2,now(),now())`, id, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// TestEventSequenceDenseAndUnique: concurrent appends under the project lock
// produce a dense, duplicate-free (project_id, seq) sequence across two real
// connections (P0-04 exit criterion).
func TestEventSequenceDenseAndUnique(t *testing.T) {
	fixture := integration.StartPG(t)
	ctx := context.Background()
	projectID := seedProject(t, fixture.Pool)

	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				err := fixture.Pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
					if err := tx.LockProjectForUpdate(ctx, projectID.String()); err != nil {
						return err
					}
					_, err := events.AppendProjectEvent(ctx, tx, projectID,
						"task.changed", "task", uuid.NewString(), nil,
						map[string]any{"n": i}, clock.System.Now())
					return err
				})
				if err != nil {
					t.Errorf("append: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()

	var maxSeq, count int64
	if err := fixture.Pool.QueryRow(ctx,
		`SELECT max(seq), count(*) FROM project_events WHERE project_id=$1`, projectID).Scan(&maxSeq, &count); err != nil {
		t.Fatal(err)
	}
	if count != 40 || maxSeq != 40 {
		t.Fatalf("expected dense sequence 1..40, got count=%d max=%d", count, maxSeq)
	}
	var outbox int
	if err := fixture.Pool.QueryRow(ctx,
		`SELECT count(*) FROM outbox_events WHERE project_id=$1 AND delivered_at IS NULL`, projectID).Scan(&outbox); err != nil {
		t.Fatal(err)
	}
	if outbox != 40 {
		t.Fatalf("expected 40 pending outbox rows, got %d", outbox)
	}
}

// TestOutboxDeliveredOnce: claiming pending rows marks them delivered; a
// second claim returns nothing (wake-ups are at-least-once markers, never the
// source of truth — consumers replay from project_events).
func TestOutboxDeliveredOnce(t *testing.T) {
	fixture := integration.StartPG(t)
	ctx := context.Background()
	projectID := seedProject(t, fixture.Pool)
	err := fixture.Pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		_, err := events.AppendProjectEvent(ctx, tx, projectID, "plan.changed", "plan", uuid.NewString(), nil, nil, clock.System.Now())
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	var first, second []events.OutboxRow
	err = fixture.Pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		var err error
		first, err = events.ClaimPendingOutbox(ctx, tx, clock.System.Now(), 10)
		return err
	})
	if err != nil || len(first) != 1 {
		t.Fatalf("first claim: rows=%d err=%v", len(first), err)
	}
	err = fixture.Pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		var err error
		second, err = events.ClaimPendingOutbox(ctx, tx, clock.System.Now(), 10)
		return err
	})
	if err != nil || len(second) != 0 {
		t.Fatalf("second claim must be empty: rows=%d err=%v", len(second), err)
	}
}

// TestJobsSkipLockedDisjointClaims: two engines never claim the same job.
func TestJobsSkipLockedDisjointClaims(t *testing.T) {
	fixture := integration.StartPG(t)
	ctx := context.Background()
	now := clock.System.Now()
	// Enqueue 20 jobs directly.
	for i := 0; i < 20; i++ {
		err := fixture.Pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
			_, err := job.Enqueue(ctx, tx, "probe", map[string]any{"i": i}, nil, now, now)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	e1 := job.NewEngine(fixture.Pool.Pool, job.Options{}, nil, nil)
	e2 := job.NewEngine(fixture.Pool.Pool, job.Options{}, nil, nil)

	seen := map[uuid.UUID]int{}
	var mu sync.Mutex
	claim := func(e *job.Engine) int {
		n := 0
		for {
			j, err := e.Claim(ctx)
			if err == job.ErrNoJob {
				return n
			}
			if err != nil {
				t.Errorf("claim: %v", err)
				return n
			}
			mu.Lock()
			seen[j.ID]++
			mu.Unlock()
			if err := e.Finish(ctx, j, nil); err != nil {
				t.Errorf("finish: %v", err)
			}
			n++
		}
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); claim(e1) }()
	// Give engine1 a head start mid-flight, then race engine2.
	time.Sleep(50 * time.Millisecond)
	go func() { defer wg.Done(); claim(e2) }()
	wg.Wait()

	if len(seen) != 20 {
		t.Fatalf("expected 20 distinct jobs claimed, got %d", len(seen))
	}
	for id, n := range seen {
		if n != 1 {
			t.Fatalf("job %s claimed %d times", id, n)
		}
	}
}

// TestFencingBlocksStaleWorker: a worker whose lease expired loses to the
// successor — its finish() writes zero rows and cannot corrupt the outcome.
func TestFencingBlocksStaleWorker(t *testing.T) {
	fixture := integration.StartPG(t)
	ctx := context.Background()
	now := clock.System.Now()
	var jobID uuid.UUID
	err := fixture.Pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		id, err := job.Enqueue(ctx, tx, "probe", map[string]any{}, nil, now, now)
		jobID = id
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	stale := job.NewEngine(fixture.Pool.Pool, job.Options{LeaseTTL: 1, Heartbeat: time.Hour}, nil, nil)
	successor := job.NewEngine(fixture.Pool.Pool, job.Options{}, nil, nil)

	claimed1, err := stale.Claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Wait past the tiny lease so the successor may take over.
	time.Sleep(1200 * time.Millisecond)
	claimed2, err := successor.Claim(ctx)
	if err != nil {
		t.Fatalf("successor claim: %v", err)
	}
	if claimed2.ID != jobID || claimed2.FencingToken <= claimed1.FencingToken {
		t.Fatalf("successor must reclaim with higher fencing token: t1=%d t2=%d", claimed1.FencingToken, claimed2.FencingToken)
	}
	// Stale worker finishes late: guarded write affects 0 rows; the successor's
	// result stands.
	if err := stale.Finish(ctx, claimed1, nil); err != nil {
		t.Fatalf("stale finish error: %v", err)
	}
	var state string
	var attempts int
	if err := fixture.Pool.QueryRow(ctx,
		`SELECT state, attempt FROM background_jobs WHERE id=$1`, jobID).Scan(&state, &attempts); err != nil {
		t.Fatal(err)
	}
	if state != "running" || attempts != 2 {
		t.Fatalf("stale finish must not change state, got state=%s attempts=%d", state, attempts)
	}
	if err := successor.Finish(ctx, claimed2, nil); err != nil {
		t.Fatal(err)
	}
	if err := fixture.Pool.QueryRow(ctx,
		`SELECT state FROM background_jobs WHERE id=$1`, jobID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != "succeeded" {
		t.Fatalf("successor finish must win, got %s", state)
	}
}

// TestJobRetryAndDeadLetter: transient failures requeue with future run_after
// and land in dead after exhausting attempts; non-retryable business errors
// fail immediately.
func TestJobRetryAndDeadLetter(t *testing.T) {
	fixture := integration.StartPG(t)
	ctx := context.Background()
	now := clock.System.Now()

	var transient uuid.UUID
	var fatal uuid.UUID
	err := fixture.Pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		id, err := job.Enqueue(ctx, tx, "retry.probe", map[string]any{}, nil, now, now)
		transient = id
		if err != nil {
			return err
		}
		id, err = job.Enqueue(ctx, tx, "fatal.probe", map[string]any{}, nil, now, now)
		fatal = id
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	engine := job.NewEngine(fixture.Pool.Pool, job.Options{BackoffBase: 150 * time.Millisecond, BackoffCap: time.Second}, nil, nil)
	transientErr := errors.Newf(errors.DependencyDown, "connection refused").WithRetryable(true)
	fatalErr := errors.New(errors.InvalidTransition, "not allowed")

	// Claim order follows run_after, so dispatch on kind instead of assuming
	// which job comes first. Retryable path: 3 attempts then dead. Retried
	// jobs requeue with a short future run_after, so poll until drained.
	const maxAttempts = 3
	transientAttempts := 0
	fatalDone := false
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if transientAttempts >= maxAttempts && fatalDone {
			break
		}
		j, err := engine.Claim(ctx)
		if err == job.ErrNoJob {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		switch j.Kind {
		case "retry.probe":
			transientAttempts++
			j.MaxAttempts = maxAttempts
			if err := engine.Finish(ctx, j, transientErr); err != nil {
				t.Fatal(err)
			}
		case "fatal.probe":
			fatalDone = true
			j.MaxAttempts = 5
			if err := engine.Finish(ctx, j, fatalErr); err != nil {
				t.Fatal(err)
			}
		default:
			t.Fatalf("unexpected kind %s", j.Kind)
		}
	}
	if transientAttempts != maxAttempts {
		t.Fatalf("expected %d transient attempts, got %d", maxAttempts, transientAttempts)
	}

	var state string
	if err := fixture.Pool.QueryRow(ctx,
		`SELECT state FROM background_jobs WHERE id=$1`, transient).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != "dead" {
		t.Fatalf("expected dead after exhausting attempts, got %s", state)
	}

	if !fatalDone {
		t.Fatal("fatal job was never claimed")
	}
	if err := fixture.Pool.QueryRow(ctx,
		`SELECT state FROM background_jobs WHERE id=$1`, fatal).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != "failed" {
		t.Fatalf("non-retryable error must fail immediately, got %s", state)
	}
}

// TestJobSameTxAsIdempotentBusinessEffect: enqueue + idempotency + event all
// commit or roll back together — no orphan wake-ups (01 §5).
func TestJobSameTxAsIdempotentBusinessEffect(t *testing.T) {
	fixture := integration.StartPG(t)
	ctx := context.Background()
	projectID := seedProject(t, fixture.Pool)
	now := clock.System.Now()

	err := fixture.Pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if _, _, _, err := idempotency.Begin(ctx, tx, "txactor", "jobs.tx", "k1", []byte("p"), time.Hour, now); err != nil {
			return err
		}
		if _, err := events.AppendProjectEvent(ctx, tx, projectID, "plan.changed", "plan", uuid.NewString(), nil, nil, now); err != nil {
			return err
		}
		_, err := job.Enqueue(ctx, tx, "probe", map[string]any{}, nil, now, now)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	err = fixture.Pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if _, _, _, err := idempotency.Begin(ctx, tx, "txactor", "jobs.tx", "k2", []byte("p"), time.Hour, now); err != nil {
			return err
		}
		if _, err := events.AppendProjectEvent(ctx, tx, projectID, "plan.changed", "plan", uuid.NewString(), nil, nil, now); err != nil {
			return err
		}
		if _, err := job.Enqueue(ctx, tx, "probe", map[string]any{}, nil, now, now); err != nil {
			return err
		}
		return errIntentionalRollback
	})
	if err == nil {
		t.Fatal("expected rollback")
	}

	var jobs, evts int
	if err := fixture.Pool.QueryRow(ctx, `SELECT count(*) FROM background_jobs`).Scan(&jobs); err != nil {
		t.Fatal(err)
	}
	if err := fixture.Pool.QueryRow(ctx, `SELECT count(*) FROM project_events WHERE project_id=$1`, projectID).Scan(&evts); err != nil {
		t.Fatal(err)
	}
	if jobs != 1 || evts != 1 {
		t.Fatalf("rollback must remove job+event together: jobs=%d events=%d", jobs, evts)
	}
}

var errIntentionalRollback = &rollbackError{}

type rollbackError struct{}

func (*rollbackError) Error() string { return "intentional rollback" }
