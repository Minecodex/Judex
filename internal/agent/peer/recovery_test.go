package peer

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/kakj-go/Judex/internal/job"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/collab"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
	"github.com/kakj-go/Judex/internal/platform/auth"
)

func TestUnknownModelCostSurvivesLeaseAndContinuation(t *testing.T) {
	f := setup(t)
	e := f.executor(&peerModel{a: f.a, b: f.b})
	if _, err := f.s.Post(f.ctx, f.group, collab.MessageInput{Body: "work", Mentions: []collab.Mention{{Kind: "role_agent", TargetID: f.a}}}, false); err != nil {
		t.Fatal(err)
	}
	var delivery uuid.UUID
	if err := f.s.Pool.QueryRow(f.ctx, `SELECT id FROM deliveries LIMIT 1`).Scan(&delivery); err != nil {
		t.Fatal(err)
	}
	r, err := e.claim(f.ctx, delivery)
	if err != nil {
		t.Fatal(err)
	}
	call := uuid.New()
	if err = e.reserveCall(f.ctx, r, call, 1, "controlled", "hash", 1200); err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.Pool.Exec(f.ctx, `UPDATE role_runs SET lease_until=now()-interval '1 second' WHERE id=$1`, r.Publication.RunID); err != nil {
		t.Fatal(err)
	}
	recovered, err := e.claim(f.ctx, delivery)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Calls != 1 || recovered.Tokens != 1200 {
		t.Fatal("unknown call reservation was lost")
	}
	if err = e.reserveCall(f.ctx, recovered, uuid.New(), 1, "controlled", "hash", 300000); err == nil {
		t.Fatal("oversized budget accepted")
	}
	if err = f.s.Pause(f.ctx, f.group, r.Publication.DiscussionID, "technical_budget"); err != nil {
		t.Fatal(err)
	}
	id, _, _, _, version := f.state(t)
	if err = f.s.Control(f.ctx, f.group, id, version, "continue", ""); err != nil {
		t.Fatal(err)
	}
	continued, err := e.claim(f.ctx, delivery)
	if err != nil {
		t.Fatal(err)
	}
	if continued.Calls != 0 || continued.Tokens != 0 {
		t.Fatal("new technical allowance not independent")
	}
	next := uuid.New()
	if err = e.reserveCall(f.ctx, continued, next, 1, "controlled", "hash", 700); err != nil {
		t.Fatal(err)
	}
	if err = e.chargeCall(f.ctx, r, call, model.UsageSnapshot{InputTokens: model.TokenCount(150), OutputTokens: model.TokenCount(50)}, 0); err != nil {
		t.Fatal(err)
	}
	var total, budget int64
	var calls int
	if err = f.s.Pool.QueryRow(f.ctx, `SELECT tokens,budget_tokens,model_calls FROM role_runs WHERE id=$1`, r.Publication.RunID).Scan(&total, &budget, &calls); err != nil {
		t.Fatal(err)
	}
	if total != 900 || budget != 700 || calls != 2 {
		t.Fatalf("total=%d budget=%d calls=%d", total, budget, calls)
	}
}
func TestUncertainShellRequiresOwnerReconciliation(t *testing.T) {
	f := setup(t)
	e := f.executor(&peerModel{a: f.a, b: f.b})
	if _, err := f.s.Post(f.ctx, f.group, collab.MessageInput{Body: "work", Mentions: []collab.Mention{{Kind: "role_agent", TargetID: f.a}}}, false); err != nil {
		t.Fatal(err)
	}
	var delivery uuid.UUID
	_ = f.s.Pool.QueryRow(f.ctx, `SELECT id FROM deliveries LIMIT 1`).Scan(&delivery)
	r, err := e.claim(f.ctx, delivery)
	if err != nil {
		t.Fatal(err)
	}
	call := model.ToolCall{ID: "unknown-shell", Name: "bash", Arguments: `{"command":"touch /workspace/effect"}`}
	r.Checkpoint = []model.Message{{Role: "assistant", ToolCalls: []model.ToolCall{call}}}
	if err = e.checkpoint(f.ctx, r); err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.Pool.Exec(f.ctx, `INSERT INTO role_tool_calls(run_id,call_id,name,args,state) VALUES($1,$2,'bash',$3,'unknown')`, r.Publication.RunID, call.ID, json.RawMessage(call.Arguments)); err != nil {
		t.Fatal(err)
	}
	if err = e.recoverPending(f.ctx, r, nil); err == nil {
		t.Fatal("unknown shell was silently replayed")
	}
	_ = e.fail(f.ctx, r, "unknown_effect", err)
	other := auth.WithPrincipal(f.ctx, &auth.Principal{Kind: auth.KindWeb, UserID: f.other})
	if _, err = f.s.ToolCalls(other, f.group, r.Publication.RunID); err == nil {
		t.Fatal("another member read private journal")
	}
	var fence int64
	_ = f.s.Pool.QueryRow(f.ctx, `SELECT fence FROM role_runs WHERE id=$1`, r.Publication.RunID).Scan(&fence)
	if err = f.s.Reconcile(f.ctx, f.group, r.Publication.RunID, call.ID, collab.ReconcileInput{ExpectedVersion: fence, Resolution: "completed", Reason: "Verified the workspace effect", Result: json.RawMessage(`{"verified":true}`)}); err != nil {
		t.Fatal(err)
	}
	id, state, _, _, version := f.state(t)
	if state != "paused" {
		t.Fatal("reconciliation resumed discussion")
	}
	if err = f.s.Control(f.ctx, f.group, id, version, "continue", ""); err != nil {
		t.Fatal(err)
	}
	resumed, err := e.claim(f.ctx, delivery)
	if err != nil {
		t.Fatal(err)
	}
	if err = e.recoverPending(f.ctx, resumed, nil); err != nil {
		t.Fatal(err)
	}
	if resumed.Checkpoint[len(resumed.Checkpoint)-1].Role != "tool" {
		t.Fatal("verified result not paired")
	}
}

func TestLongExecutionUsesPublicReferencesWithoutDroppingPendingEffects(t *testing.T) {
	r := &run{}
	r.Publication.RunID = uuid.New()
	call := model.ToolCall{ID: "large-result", Name: "read_context", Arguments: `{}`}
	r.Checkpoint = []model.Message{{Role: "assistant", ToolCalls: []model.ToolCall{call}}, {Role: "tool", ToolCallID: call.ID, Content: strings.Repeat("original evidence ", 10000)}}
	if !compactExecution(r, 20000) || !strings.Contains(r.Checkpoint[0].Content, r.Publication.RunID.String()+":"+call.ID) {
		t.Fatal("completed source reference lost")
	}
	r.Checkpoint = []model.Message{{Role: "assistant", Content: strings.Repeat("execution ", 10000), ToolCalls: []model.ToolCall{call}}}
	if compactExecution(r, 20000) {
		t.Fatal("pending effect was removed from recovery protocol")
	}
}

type cancelledStartModel struct{ started chan struct{} }

func (p cancelledStartModel) Stream(ctx context.Context, _ model.Request) (<-chan model.Event, error) {
	close(p.started)
	<-ctx.Done()
	return nil, ctx.Err()
}
func TestShutdownDuringModelStreamCreationPreservesRunnableDiscussion(t *testing.T) {
	f := setup(t)
	started := make(chan struct{})
	e := f.executor(cancelledStartModel{started: started})
	if _, err := f.s.Post(f.ctx, f.group, collab.MessageInput{Body: "work", Mentions: []collab.Mention{{Kind: "role_agent", TargetID: f.a}}}, false); err != nil {
		t.Fatal(err)
	}
	var delivery uuid.UUID
	_ = f.s.Pool.QueryRow(f.ctx, `SELECT id FROM deliveries LIMIT 1`).Scan(&delivery)
	ctx, cancel := context.WithCancel(f.ctx)
	done := make(chan error, 1)
	raw, _ := json.Marshal(map[string]any{"deliveryId": delivery})
	go func() { done <- e.ExecuteJob(ctx, job.Job{Payload: raw}) }()
	<-started
	cancel()
	if err := <-done; err != job.ErrBusy {
		t.Fatalf("shutdown result: %v", err)
	}
	_, state, _, _, _ := f.state(t)
	if state != "active" {
		t.Fatal("process cancellation persisted a model failure pause")
	}
	var runState string
	var calls int
	var reserved int64
	if err := f.s.Pool.QueryRow(f.ctx, `SELECT state,model_calls,tokens FROM role_runs WHERE delivery_id=$1`, delivery).Scan(&runState, &calls, &reserved); err != nil {
		t.Fatal(err)
	}
	if runState != "pending" || calls != 1 || reserved == 0 {
		t.Fatalf("state=%s calls=%d reserved=%d", runState, calls, reserved)
	}
}

func TestRepeatedContextDiscoveryDoesNotRecursivelyDuplicateHistory(t *testing.T) {
	f := setup(t)
	e := f.executor(&peerModel{a: f.a, b: f.b})
	id, err := f.s.Post(f.ctx, f.group, collab.MessageInput{Body: strings.Repeat("Original evidence. ", 1000), Mentions: []collab.Mention{{Kind: "role_agent", TargetID: f.a}}}, false)
	if err != nil {
		t.Fatal(err)
	}
	_ = id
	var delivery uuid.UUID
	_ = f.s.Pool.QueryRow(f.ctx, `SELECT id FROM deliveries LIMIT 1`).Scan(&delivery)
	r, err := e.claim(f.ctx, delivery)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 25; i++ {
		index, err := f.s.KnowledgeIndex(f.ctx, f.group, 0, 20)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(index)
		if len(raw) > 24000 {
			t.Fatalf("index recursively expanded to %d bytes", len(raw))
		}
		if err = f.s.ToolKnowledge(f.ctx, r.Publication, fmt.Sprint(i), "read_context", index); err != nil {
			t.Fatal(err)
		}
	}
	var size int
	if err = f.s.Pool.QueryRow(f.ctx, `SELECT max(octet_length(content::text)) FROM knowledge_entries WHERE group_id=$1 AND kind='tool'`, f.group).Scan(&size); err != nil {
		t.Fatal(err)
	}
	if size > 25000 {
		t.Fatal("public tool results expanded recursively")
	}
}
