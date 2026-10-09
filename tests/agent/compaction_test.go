package agent_test

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	agentcontext "github.com/kakj-go/Judex/internal/agent/context"
	"github.com/kakj-go/Judex/internal/agent/runner"
	"github.com/kakj-go/Judex/internal/agent/tools"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
	"strings"
	"testing"
)

type checkpointJournal struct {
	Results   [][]byte
	Snapshots [][]model.Message
}

func (j *checkpointJournal) ModelStart(context.Context, int, model.Request) error    { return nil }
func (j *checkpointJournal) ModelEnd(context.Context, int, int64, bool, error) error { return nil }
func (j *checkpointJournal) ToolStart(context.Context, model.ToolCall) error         { return nil }
func (j *checkpointJournal) ToolEnd(_ context.Context, _ model.ToolCall, raw []byte, _ bool) error {
	j.Results = append(j.Results, raw)
	return nil
}
func (j *checkpointJournal) Checkpoint(_ context.Context, m []model.Message) error {
	j.Snapshots = append(j.Snapshots, append([]model.Message{}, m...))
	return nil
}

type compactProvider struct {
	t     *testing.T
	calls int
}

func (p *compactProvider) Stream(_ context.Context, request model.Request) (<-chan model.Event, error) {
	p.calls++
	out := make(chan model.Event, 3)
	if p.calls == 1 {
		out <- model.Event{Type: "toolCallReady", ToolCallID: "large-evidence", ToolName: "read_material", ArgsJSON: `{"versionId":"fixed-material"}`}
	} else {
		raw, _ := json.Marshal(request.Messages)
		if !strings.Contains(string(raw), "反对未经测试直接验收") || !strings.Contains(string(raw), "archived") || !strings.Contains(string(raw), "large-evidence") {
			p.t.Fatalf("compaction dropped a protected objection or original reference: %s", raw)
		}
		if runner.RequestBytes(request.Messages, request.Tools) > 12000 {
			p.t.Fatal("compacted request still exceeds window")
		}
		out <- model.Event{Type: "textDelta", TextDelta: "保留反对，等待核对原件"}
	}
	out <- model.Event{Type: "usage", InputTokens: 100, OutputTokens: 100}
	out <- model.Event{Type: "finish"}
	close(out)
	return out, nil
}
func TestLongToolContextCompactsWithoutLosingObjection(t *testing.T) {
	registry := tools.New()
	tools.RegisterDefaults(registry)
	provider := &compactProvider{t: t}
	journal := &checkpointJournal{}
	original := strings.Repeat("中文探索原始证据。", 10000)
	env := &tools.Env{ReadMaterial: func(context.Context, string, string) (map[string]any, error) {
		return map[string]any{"content": original, "versionId": "fixed-material"}, nil
	}}
	harness := runner.Runner{Provider: provider, Registry: registry}
	result := harness.Run(context.Background(), runner.RunRequest{RunID: uuid.New(), ProjectID: uuid.New(), Manifest: agentcontext.Build(agentcontext.Facts{Disagreements: []string{"反对未经测试直接验收"}, NewMaterial: "核对固定材料，不得代人决定"}), Budget: runner.DefaultBudget(), Journal: journal, MaxInputTokens: 12000, MaxOutputTokens: 1000, Env: env})
	if result.State != "succeeded" || provider.calls != 2 {
		t.Fatalf("compaction did not resume: %+v", result)
	}
	if len(journal.Results) != 1 || !strings.Contains(string(journal.Results[0]), original) {
		t.Fatal("raw result was lost")
	}
}
func TestHardContextIsNeverSilentlyTruncated(t *testing.T) {
	registry := tools.New()
	tools.RegisterDefaults(registry)
	provider := &compactProvider{t: t}
	harness := runner.Runner{Provider: provider, Registry: registry}
	result := harness.Run(context.Background(), runner.RunRequest{Manifest: agentcontext.Build(agentcontext.Facts{Disagreements: []string{strings.Repeat("不能验收", 10000)}}), Budget: runner.DefaultBudget(), MaxInputTokens: 12000, Journal: &checkpointJournal{}})
	if result.State != "context_blocked" || provider.calls != 0 {
		t.Fatalf("hard context was truncated: %+v", result)
	}
}
