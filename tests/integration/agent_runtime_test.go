package integrationtest_test

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/agent/batch"
	"github.com/kakj-go/Judex/internal/discussion"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
	"github.com/kakj-go/Judex/internal/project"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type runtimeProvider struct {
	calls      atomic.Int32
	position   string
	positions  []string
	children   atomic.Int32
	rendezvous chan struct{}
}

func (p *runtimeProvider) Stream(ctx context.Context, req model.Request) (<-chan model.Event, error) {
	p.calls.Add(1)
	out := make(chan model.Event, 4)
	system := req.Messages[0].Content
	if strings.Contains(system, "runtime-test-duty") {
		if p.rendezvous != nil {
			if p.children.Add(1) == 2 {
				close(p.rendezvous)
			}
			select {
			case <-p.rendezvous:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		out <- model.Event{Type: "textDelta", TextDelta: "岗位核对结果"}
	} else if len(req.Messages) == 2 {
		positions := p.positions
		if len(positions) == 0 {
			positions = []string{p.position}
		}
		for i, position := range positions {
			args, _ := json.Marshal(map[string]string{"position": position, "question": "检查需求"})
			out <- model.Event{Type: "toolCallReady", ToolCallID: position + string(rune('a'+i)), ToolName: "call_agent", ArgsJSON: string(args)}
		}
	} else {
		out <- model.Event{Type: "textDelta", TextDelta: "已汇总岗位意见，等待人工决定"}
	}
	out <- model.Event{Type: "usage", InputTokens: 100, OutputTokens: 50}
	out <- model.Event{Type: "finish"}
	close(out)
	return out, nil
}
func TestProductionAgentPersistsAndCallsPosition(t *testing.T) {
	_, projects, ids, pool := newDecisionEnv(t)
	ctx := context.Background()
	user, _, err := ids.Register(ctx, "Runtime", "runtime@test.local", "runtime-password-123", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	projectValue, err := projects.Create(ctx, user.ID, project.CreateRequest{Title: "Runtime"})
	if err != nil {
		t.Fatal(err)
	}
	pos, err := projects.CreatePosition(ctx, user.ID, projectValue.ID, project.PositionDraft{Name: "工程师", Prompt: "runtime-test-duty"})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := projects.CreateIdentity(ctx, user.ID, projectValue.ID, pos.ID, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	var topic uuid.UUID
	if err = pool.QueryRow(ctx, `SELECT id FROM topics WHERE project_id=$1`, projectValue.ID).Scan(&topic); err != nil {
		t.Fatal(err)
	}
	sub, err := discussion.NewService(pool, nil).CreateSubmission(ctx, user.ID, projectValue.ID, discussion.Submission{ClientSubmissionID: uuid.NewString(), Purpose: "message", Source: "web", Text: "中文研发材料", TopicID: &topic})
	if err != nil {
		t.Fatal(err)
	}
	svc := batch.Service{Pool: pool}
	batchID, runID, err := svc.Start(ctx, user.ID, projectValue.ID, topic, sub.ID)
	if err != nil {
		t.Fatal(err)
	}
	before, err := svc.Get(ctx, user.ID, projectValue.ID, runID)
	if err != nil || before.State != "queued" {
		t.Fatalf("durable queued run: %+v %v", before, err)
	}
	position2, err := projects.CreatePosition(ctx, user.ID, projectValue.ID, project.PositionDraft{Name: "质量", Prompt: "runtime-test-duty"})
	if err != nil {
		t.Fatal(err)
	}
	identity2, err := projects.CreateIdentity(ctx, user.ID, projectValue.ID, position2.ID, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	provider := &runtimeProvider{positions: []string{identity.ID.String(), identity2.ID.String()}, rendezvous: make(chan struct{})}
	executor := batch.Executor{Pool: pool.Pool, Provider: provider, ModelName: "test"}
	runtimeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err = executor.ExecuteBatch(runtimeCtx, projectValue.ID, batchID); err != nil {
		t.Fatal(err)
	}
	after, err := svc.Get(ctx, user.ID, projectValue.ID, runID)
	if err != nil || after.State != "succeeded" {
		t.Fatalf("finished run: %+v %v", after, err)
	}
	if provider.calls.Load() != 4 {
		t.Fatalf("expected coordinator, two parallel children, synthesis; calls=%d", provider.calls.Load())
	}
	for _, table := range []string{"model_calls", "tool_calls", "context_checkpoints", "run_events"} {
		var count int
		if err = pool.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE project_id=$1", projectValue.ID).Scan(&count); err != nil || count == 0 {
			t.Fatalf("missing %s evidence: %d %v", table, count, err)
		}
	}
	if err = executor.ExecuteBatch(ctx, projectValue.ID, batchID); err != nil {
		t.Fatal(err)
	}
	if provider.calls.Load() != 4 {
		t.Fatal("completed batch repeated model calls")
	}

	// Crash after durable tool results, before final summary commit. Resume the
	// model from the transcript; do not run the already completed children again.
	if _, err = pool.Exec(ctx, `DELETE FROM context_checkpoints WHERE run_id=$1 AND manifest->'messages'->-1->>'role'='assistant' AND NOT ((manifest->'messages'->-1) ? 'tool_calls')`, runID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `DELETE FROM messages WHERE run_id=$1`, runID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE agent_runs SET state='running' WHERE id=$1`, runID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE discussion_batches SET state='running' WHERE id=$1`, batchID); err != nil {
		t.Fatal(err)
	}
	if err = executor.ExecuteBatch(ctx, projectValue.ID, batchID); err != nil {
		t.Fatal(err)
	}
	if provider.calls.Load() != 5 || provider.children.Load() != 2 {
		t.Fatalf("recovery replayed completed children: calls=%d children=%d", provider.calls.Load(), provider.children.Load())
	}
	var cursor int64
	if err = pool.QueryRow(ctx, `SELECT s.last_consumed_seq FROM agent_sessions s JOIN agent_runs r ON r.session_id=s.id WHERE r.id=$1`, runID).Scan(&cursor); err != nil || cursor == 0 {
		t.Fatalf("consumption cursor not advanced: %d %v", cursor, err)
	}
	var version int64
	var attempts, tokens int
	if err = pool.QueryRow(ctx, `SELECT version,model_attempts,used_tokens FROM discussion_batches WHERE id=$1`, batchID).Scan(&version, &attempts, &tokens); err != nil {
		t.Fatal(err)
	}
	if err = svc.Extend(ctx, user.ID, projectValue.ID, batchID, version, 1, "继续评审未决事项"); err != nil {
		t.Fatal(err)
	}
	var queued, currentAttempts, currentTokens int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM agent_runs WHERE batch_id=$1 AND state='queued'`, batchID).Scan(&queued); err != nil || queued != 1 {
		t.Fatalf("continuation not queued: %d %v", queued, err)
	}
	if err = pool.QueryRow(ctx, `SELECT model_attempts,used_tokens FROM discussion_batches WHERE id=$1`, batchID).Scan(&currentAttempts, &currentTokens); err != nil || currentAttempts != attempts || currentTokens != tokens {
		t.Fatal("continuation reset shared budget")
	}
	if err = svc.Extend(ctx, user.ID, projectValue.ID, batchID, version, 1, "重复扩展"); err == nil {
		t.Fatal("stale extension repeated")
	}
}

func TestInterruptedAgentNeverReplaysPreparedTool(t *testing.T) {
	_, projects, ids, pool := newDecisionEnv(t)
	ctx := context.Background()
	user, _, err := ids.Register(ctx, "Recovery", "recovery-agent@test.local", "runtime-password-123", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	p, err := projects.Create(ctx, user.ID, project.CreateRequest{Title: "Recovery"})
	if err != nil {
		t.Fatal(err)
	}
	var topic uuid.UUID
	if err = pool.QueryRow(ctx, `SELECT id FROM topics WHERE project_id=$1`, p.ID).Scan(&topic); err != nil {
		t.Fatal(err)
	}
	sub, err := discussion.NewService(pool, nil).CreateSubmission(ctx, user.ID, p.ID, discussion.Submission{ClientSubmissionID: uuid.NewString(), Purpose: "message", Source: "web", Text: "恢复验证", TopicID: &topic})
	if err != nil {
		t.Fatal(err)
	}
	svc := batch.Service{Pool: pool}
	bid, rid, err := svc.Start(ctx, user.ID, p.ID, topic, sub.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE discussion_batches SET state='running',rounds_reserved=1 WHERE id=$1`, bid); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE agent_runs SET state='running',lease_token='old-worker' WHERE id=$1`, rid); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO tool_calls(project_id,id,run_id,tool_call_id,name,effect_class,state) VALUES($1,$2,$3,'unknown-shell','bash','sandbox','prepared')`, p.ID, uuid.New(), rid); err != nil {
		t.Fatal(err)
	}
	provider := &runtimeProvider{}
	executor := batch.Executor{Pool: pool.Pool, Provider: provider, ModelName: "test"}
	if err = executor.ExecuteBatch(ctx, p.ID, bid); err != nil {
		t.Fatal(err)
	}
	result, err := svc.Get(ctx, user.ID, p.ID, rid)
	if err != nil || result.State != "waiting_human" {
		t.Fatalf("recovery state: %+v %v", result, err)
	}
	var state string
	pool.QueryRow(ctx, `SELECT state FROM tool_calls WHERE run_id=$1`, rid).Scan(&state)
	if state != "unknown" || provider.calls.Load() != 0 {
		t.Fatalf("unknown tool replayed: state=%s calls=%d", state, provider.calls.Load())
	}
	var version int64
	if err = pool.QueryRow(ctx, `SELECT version FROM discussion_batches WHERE id=$1`, bid).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err = svc.Extend(ctx, user.ID, p.ID, bid, version, 1, "不得重放未知工具"); err == nil {
		t.Fatal("unknown tool allowed continuation")
	}

}
