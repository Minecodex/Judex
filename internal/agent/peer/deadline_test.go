package peer

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/collab"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
	"github.com/kakj-go/Judex/internal/infrastructure/opensandbox"
	"github.com/kakj-go/Judex/internal/job"
)

// Leave time for the real database claim/snapshot/journal preparation on
// shared CI runners. The provider still waits for the persisted deadline.
const inFlightDeadlineAllowance = 3 * time.Second

func deadlineWork(t *testing.T, f fixture, remaining time.Duration) time.Time {
	t.Helper()
	if _, err := f.s.Post(f.ctx, f.group, collab.MessageInput{Body: "deadline work", Mentions: []collab.Mention{{Kind: "role_agent", TargetID: f.a}}}, false); err != nil {
		t.Fatal(err)
	}
	var started time.Time
	if err := f.s.Pool.QueryRow(f.ctx, `UPDATE role_runs SET budget_started_at=clock_timestamp()-interval '30 minutes'+$1*interval '1 millisecond' RETURNING budget_started_at`, remaining.Milliseconds()).Scan(&started); err != nil {
		t.Fatal(err)
	}
	return started
}

func assertTimePause(t *testing.T, f fixture) {
	t.Helper()
	var state, reason string
	var messages int
	if err := f.s.Pool.QueryRow(f.ctx, `SELECT state,pause_reason FROM discussions WHERE group_id=$1`, f.group).Scan(&state, &reason); err != nil {
		t.Fatal(err)
	}
	if err := f.s.Pool.QueryRow(f.ctx, `SELECT count(*) FROM messages WHERE role_agent_id IS NOT NULL`).Scan(&messages); err != nil {
		t.Fatal(err)
	}
	if state != "paused" || reason != "technical_budget" || messages != 0 {
		t.Fatalf("deadline failed: state=%s reason=%s published=%d", state, reason, messages)
	}
}

func TestPersistedDeadlineCancelsInFlightModel(t *testing.T) {
	for _, mode := range []string{"stream-creation", "stalled-stream", "continuous-no-finish", "late-result"} {
		t.Run(mode, func(t *testing.T) {
			f := setup(t)
			started := deadlineWork(t, f, inFlightDeadlineAllowance)
			called := false
			p := attemptProvider(func(ctx context.Context, req model.Request) (<-chan model.Event, error) {
				called = true
				deadline, ok := ctx.Deadline()
				if !ok || deadline.Sub(started.Add(30*time.Minute)).Abs() > time.Millisecond {
					t.Fatal("provider did not receive the persisted deadline")
				}
				switch mode {
				case "stream-creation":
					<-ctx.Done()
					return nil, ctx.Err()
				case "stalled-stream":
					return make(chan model.Event), nil // Deliberately never closes.
				case "continuous-no-finish":
					out := make(chan model.Event)
					go func() {
						defer close(out)
						for {
							select {
							case <-ctx.Done():
								return
							case out <- model.Event{Type: "textDelta", TextDelta: "unpublished"}:
								time.Sleep(time.Millisecond)
							}
						}
					}()
					return out, nil
				default:
					// A misbehaving provider returns after cancellation. Its text
					// still must not be billed as known or published.
					<-ctx.Done()
					out := make(chan model.Event, 2)
					out <- model.Event{Type: "textDelta", TextDelta: "late publication"}
					out <- model.Event{Type: "finish"}
					close(out)
					return out, nil
				}
			})
			f.step(t, f.executor(p))
			if !called {
				t.Fatal("test allowance expired before an in-flight model call was exercised")
			}
			assertTimePause(t, f)
			var unknown int
			if err := f.s.Pool.QueryRow(f.ctx, `SELECT count(*) FROM role_model_calls WHERE state='started' AND reserved_tokens>0 AND charged_tokens IS NULL`).Scan(&unknown); err != nil || unknown != 1 {
				t.Fatalf("unknown cost was lost: %d %v", unknown, err)
			}
		})
	}
}

func TestDeadlineDuringSharedCompression(t *testing.T) {
	f := setup(t)
	// Force actual Snapshot compression, preserving the original knowledge.
	for i := 0; i < 5; i++ {
		if _, err := f.s.Post(f.ctx, f.group, collab.MessageInput{Body: strings.Repeat("opposing evidence ", 2500)}, false); err != nil {
			t.Fatal(err)
		}
	}
	deadlineWork(t, f, inFlightDeadlineAllowance)
	called := false
	e := f.executor(attemptProvider(func(ctx context.Context, req model.Request) (<-chan model.Event, error) {
		called = true
		if !strings.Contains(req.Messages[0].Content, "Summarize shared group") {
			t.Fatal("expected the shared summarizer, not a role response")
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}))
	f.step(t, e)
	if !called {
		t.Fatal("compression was not exercised")
	}
	assertTimePause(t, f)
	var covered int64
	if err := f.s.Pool.QueryRow(f.ctx, `SELECT covered_through FROM context_heads WHERE group_id=$1`, f.group).Scan(&covered); err != nil || covered != 0 {
		t.Fatalf("failed summary advanced coverage: %d %v", covered, err)
	}
}

type deadlineSandbox struct{ calls int }

func (*deadlineSandbox) Create(context.Context, uuid.UUID, uuid.UUID) (string, error) {
	return "deadline-sandbox", nil
}
func (s *deadlineSandbox) Exec(ctx context.Context, _, _ string, _ time.Duration) (opensandbox.SandboxResult, error) {
	s.calls++
	<-ctx.Done()
	return opensandbox.SandboxResult{Unknown: true}, ctx.Err()
}
func (*deadlineSandbox) Kill(context.Context, string) error { return nil }

func TestDeadlineToolEffectRequiresReconciliationAfterContinue(t *testing.T) {
	f := setup(t)
	deadlineWork(t, f, inFlightDeadlineAllowance)
	provider := attemptProvider(func(context.Context, model.Request) (<-chan model.Event, error) {
		out := make(chan model.Event, 2)
		out <- model.Event{Type: "toolCallReady", ToolCallID: "shell-deadline", ToolName: "bash", ArgsJSON: `{"command":"do-something"}`}
		out <- model.Event{Type: "finish"}
		close(out)
		return out, nil
	})
	sandbox := &deadlineSandbox{}
	e := f.executor(provider)
	e.SandboxFactory = func(context.Context, collab.Publication, int64) (*opensandbox.RunSandbox, error) {
		return &opensandbox.RunSandbox{Client: sandbox}, nil
	}
	f.step(t, e)
	assertTimePause(t, f)
	var state string
	if err := f.s.Pool.QueryRow(f.ctx, `SELECT state FROM role_tool_calls WHERE call_id='shell-deadline'`).Scan(&state); err != nil || state != "unknown" {
		t.Fatalf("side effect must remain unknown: %s %v", state, err)
	}
	id, _, _, _, version := f.state(t)
	if err := f.s.Control(f.ctx, f.group, id, version, "continue", ""); err != nil {
		t.Fatal(err)
	}
	var delivery uuid.UUID
	_ = f.s.Pool.QueryRow(f.ctx, `SELECT id FROM deliveries LIMIT 1`).Scan(&delivery)
	raw, _ := json.Marshal(map[string]any{"deliveryId": delivery})
	if err := e.ExecuteJob(f.ctx, job.Job{Payload: raw}); !errors.Is(err, job.ErrBusy) {
		t.Fatalf("unknown effect must pause again: %v", err)
	}
	if sandbox.calls != 1 {
		t.Fatal("continued without reconciling shell effect")
	}
}

func TestRestartKeepsDeadlineAndContinueGrantsNewAllowance(t *testing.T) {
	f := setup(t)
	started := deadlineWork(t, f, -time.Second)
	e := f.executor(attemptProvider(func(context.Context, model.Request) (<-chan model.Event, error) {
		t.Fatal("expired recovered run must not contact model")
		return nil, nil
	}))
	f.step(t, e)
	assertTimePause(t, f)
	var persisted time.Time
	_ = f.s.Pool.QueryRow(f.ctx, `SELECT budget_started_at FROM role_runs LIMIT 1`).Scan(&persisted)
	if !persisted.Equal(started) {
		t.Fatal("claim/recovery granted more time")
	}
	id, _, _, _, version := f.state(t)
	if err := f.s.Control(f.ctx, f.group, id, version, "continue", ""); err != nil {
		t.Fatal(err)
	}
	var delivery uuid.UUID
	_ = f.s.Pool.QueryRow(f.ctx, `SELECT id FROM deliveries LIMIT 1`).Scan(&delivery)
	r, err := e.claim(f.ctx, delivery)
	if err != nil || r == nil || time.Since(r.BudgetStarted) > 2*time.Second {
		t.Fatalf("continue did not start a new allowance: %+v %v", r, err)
	}
	// Publication has its own DB deadline check, even without a cancelled ctx.
	if _, err = f.s.Pool.Exec(f.ctx, `UPDATE role_runs SET budget_started_at=now()-interval '31 minutes' WHERE id=$1`, r.Publication.RunID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.Publish(f.ctx, r.Publication, collab.MessageInput{Body: "late direct call"}); err == nil {
		t.Fatal("late publication bypassed the persisted deadline")
	}
}
