// SPDX-License-Identifier: Apache-2.0
package integrationtest_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	"github.com/kakj-go/Judex/internal/platform/clock"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/idempotency"
	"github.com/kakj-go/Judex/internal/platform/keys"
	integration "github.com/kakj-go/Judex/tests/integration"
)

// TestMigrationIsRepeatable proves the goose migration set is idempotent:
// running Up twice on a live database succeeds without DDL drift (G03 basics).
func TestMigrationIsRepeatable(t *testing.T) {
	fixture := integration.StartPG(t)
	ctx := context.Background()
	if err := fixture.Pool.Migrate(ctx); err != nil {
		t.Fatalf("second migrate run must succeed: %v", err)
	}
	var count int
	if err := fixture.Pool.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.tables WHERE table_name = 'users'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("users table missing after migrations")
	}
}

func runIdempotentCommand(t *testing.T, pool *postgres.Pool, actor, operation, key string, hash []byte,
	effect func()) (idempotency.Decision, int, []byte) {
	t.Helper()
	var decision idempotency.Decision
	var status int
	var body []byte
	err := pool.Transact(context.Background(), func(ctx context.Context, tx postgres.Tx) error {
		d, s, b, err := idempotency.Begin(ctx, tx, actor, operation, key, hash, time.Hour, clock.System.Now())
		decision, status, body = d, s, b
		if err != nil {
			return err
		}
		switch d {
		case idempotency.Proceed:
			effect()
			return idempotency.Complete(ctx, tx, actor, operation, key, 200, []byte(`{"ok":true}`))
		}
		return nil
	})
	if err != nil && !apierrors.IsCode(err, apierrors.IdempotencyConflict) {
		t.Errorf("transaction failed: %v", err)
	}
	return decision, status, body
}

// TestIdempotencySameKeySingleEffect covers 01 §4: same key+payload replays
// the stored response without a new effect; a different payload under the
// same key is rejected with IDEMPOTENCY_CONFLICT.
func TestIdempotencySameKeySingleEffect(t *testing.T) {
	fixture := integration.StartPG(t)
	actor := "user-" + uuid.NewString()
	key := idempotency.NewKey()
	payloadA := keys.Hash("payload-a")
	payloadB := keys.Hash("payload-b")

	var effects int64
	effect := func() { atomic.AddInt64(&effects, 1) }

	if d, _, _ := runIdempotentCommand(t, fixture.Pool, actor, "test.command", key, payloadA, effect); d != idempotency.Proceed {
		t.Fatalf("first run should proceed, got %v", d)
	}
	if d, status, body := runIdempotentCommand(t, fixture.Pool, actor, "test.command", key, payloadA, effect); d != idempotency.Replay || status != 200 || string(body) != `{"ok":true}` {
		t.Fatalf("replay mismatch: %v %d %s", d, status, body)
	}
	if got := atomic.LoadInt64(&effects); got != 1 {
		t.Fatalf("expected exactly one effect, got %d", got)
	}
	if d, _, _ := runIdempotentCommand(t, fixture.Pool, actor, "test.command", key, payloadB, effect); d != idempotency.Conflict {
		t.Fatalf("expected conflict, got %v", d)
	}
	if got := atomic.LoadInt64(&effects); got != 1 {
		t.Fatalf("conflict must not create effects, got %d", got)
	}
}

// TestIdempotencyConcurrentRacers: the FOR UPDATE row lock serializes racing
// transactions on the same key; exactly one Proceed ever happens.
func TestIdempotencyConcurrentRacers(t *testing.T) {
	fixture := integration.StartPG(t)
	ctx := context.Background()
	actor := "user-" + uuid.NewString()
	key := idempotency.NewKey()
	payload := keys.Hash("same")

	var effects int64
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := fixture.Pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
				d, _, _, err := idempotency.Begin(ctx, tx, actor, "race.command", key, payload, time.Hour, clock.System.Now())
				if err != nil {
					return err
				}
				if d == idempotency.Proceed {
					atomic.AddInt64(&effects, 1)
					return idempotency.Complete(ctx, tx, actor, "race.command", key, 200, []byte(`{}`))
				}
				return nil
			})
			if err != nil {
				t.Errorf("racer tx: %v", err)
			}
		}()
	}
	wg.Wait()
	if got := atomic.LoadInt64(&effects); got != 1 {
		t.Fatalf("exactly one effect expected under concurrency, got %d", got)
	}
}

// TestAuditIsTransactionalWithBusinessEffect: rollback removes the audit row
// together with the business row — no half audit trail can exist (P0-03 exit
// criterion).
func TestAuditIsTransactionalWithBusinessEffect(t *testing.T) {
	fixture := integration.StartPG(t)
	ctx := context.Background()
	userID := uuid.NewString()

	err := fixture.Pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if _, err := tx.Exec(ctx,
			`INSERT INTO users (id, email_normalized, email_display, display_name, created_at, updated_at)
			 VALUES ($1,'a@b.c','a@b.c','A',now(),now())`, userID); err != nil {
			return err
		}
		if err := audit.Append(ctx, tx, audit.Entry{
			ActorType: audit.ActorUser, Source: audit.SourceWeb,
			Operation: "test.insert", ObjectType: "user", ObjectID: userID,
			OccurredAt: clock.System.Now(),
		}); err != nil {
			return err
		}
		return fmt.Errorf("intentional rollback")
	})
	if err == nil || err.Error() != "intentional rollback" {
		t.Fatalf("expected intentional rollback, got %v", err)
	}
	var n int
	if err := fixture.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE object_id=$1`, userID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("audit row survived a rolled-back business transaction")
	}

	err = fixture.Pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if _, err := tx.Exec(ctx,
			`INSERT INTO users (id, email_normalized, email_display, display_name, created_at, updated_at)
			 VALUES ($1,'a2@b.c','a2@b.c','A2',now(),now())`, userID); err != nil {
			return err
		}
		return audit.Append(ctx, tx, audit.Entry{
			ActorType: audit.ActorUser, Source: audit.SourceWeb,
			Operation: "test.insert", ObjectType: "user", ObjectID: userID,
			OccurredAt: clock.System.Now(),
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE object_id=$1`, userID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatal("committed audit row missing")
	}
}

// TestLockOrderingSerializesProjectGuard: two transactions taking the
// project guard on the same (future) project row serialize — the second only
// proceeds after the first commits. Users table has no projects yet in M001,
// so this exercises the FOR UPDATE primitive itself via users rows.
func TestLockOrderingSerializesProjectGuard(t *testing.T) {
	fixture := integration.StartPG(t)
	ctx := context.Background()
	rowID := uuid.NewString()
	if _, err := fixture.Pool.Exec(ctx,
		`INSERT INTO users (id, email_normalized, email_display, display_name, created_at, updated_at)
		 VALUES ($1,'lock@b.c','lock@b.c','Lock',now(),now())`, rowID); err != nil {
		t.Fatal(err)
	}

	release := make(chan struct{})
	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		err := fixture.Pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
			if _, err := tx.Exec(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, rowID); err != nil {
				return err
			}
			<-release // hold the lock until the racing reader proves blocking
			return nil
		})
		if err != nil {
			t.Errorf("first tx: %v", err)
		}
	}()

	// Give the first transaction time to acquire the lock.
	time.Sleep(400 * time.Millisecond)
	blocked := make(chan error, 1)
	go func() {
		blocked <- fixture.Pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
			_, err := tx.Exec(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, rowID)
			return err
		})
	}()

	select {
	case err := <-blocked:
		t.Fatalf("second lock holder must block until commit, got %v", err)
	case <-time.After(600 * time.Millisecond):
		// Still blocked as expected; release and confirm completion.
		close(release)
		select {
		case err := <-blocked:
			if err != nil {
				t.Fatalf("second tx after release: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("second transaction never completed after lock release")
		}
	}
	<-firstDone
}
