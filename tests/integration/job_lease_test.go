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

func TestPreviewWorkerDoesNotCompeteForModelSlots(t *testing.T) {
	fixture := integration.StartPG(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if err := fixture.Pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		for _, kind := range []string{"role.delivery", "role.delivery", "file.preview"} {
			if _, err := job.Enqueue(ctx, tx, kind, nil, nil, now, now); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	roles := job.NewEngine(fixture.Pool.Pool, job.Options{Kinds: []string{"role.delivery"}}, nil, nil)
	for i := 0; i < 2; i++ {
		j, err := roles.Claim(ctx)
		if err != nil || j.Kind != "role.delivery" {
			t.Fatalf("role claim: %v", err)
		}
	}
	if _, err := roles.Claim(ctx); err != job.ErrNoJob {
		t.Fatal("role worker stole preview")
	}
	previews := job.NewEngine(fixture.Pool.Pool, job.Options{Kinds: []string{"file.preview"}}, nil, nil)
	j, err := previews.Claim(ctx)
	if err != nil || j.Kind != "file.preview" {
		t.Fatalf("preview blocked by active model leases: %v", err)
	}
}
