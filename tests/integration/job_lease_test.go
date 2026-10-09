package integrationtest_test

import (
	"context"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	"github.com/kakj-go/Judex/internal/job"
	integration "github.com/kakj-go/Judex/tests/integration"
	"testing"
	"time"
)

func TestJobLeaseCannotBeImmediatelyStolen(t *testing.T) {
	fixture := integration.StartPG(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if err := fixture.Pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		_, err := job.Enqueue(ctx, tx, "long-running", nil, nil, now, now)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	first := job.NewEngine(fixture.Pool.Pool, job.Options{LeaseTTL: time.Minute}, nil, nil)
	second := job.NewEngine(fixture.Pool.Pool, job.Options{LeaseTTL: time.Minute}, nil, nil)
	claimed, err := first.Claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = second.Claim(ctx); err != job.ErrNoJob {
		t.Fatalf("active lease stolen: %v", err)
	}
	if err = first.Heartbeat(ctx, claimed); err != nil {
		t.Fatal(err)
	}
	if _, err = second.Claim(ctx); err != job.ErrNoJob {
		t.Fatalf("renewed lease stolen: %v", err)
	}
}
