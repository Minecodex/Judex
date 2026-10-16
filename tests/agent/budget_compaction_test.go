package agent_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	agentcontext "github.com/kakj-go/Judex/internal/agent/context"
	"github.com/kakj-go/Judex/internal/agent/runner"
	"github.com/kakj-go/Judex/internal/agent/tools"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
)

type sharedBudgetJournal struct {
	checkpointJournal
	reads int
}

func (j *sharedBudgetJournal) RemainingBudget(context.Context) (int64, int, error) {
	j.reads++
	if j.reads == 1 {
		return 150000, 8, nil
	}
	return 20000, 2, nil
}

type budgetEvidenceProvider struct {
	t     *testing.T
	calls int
}

func (p *budgetEvidenceProvider) Stream(_ context.Context, req model.Request) (<-chan model.Event, error) {
	p.calls++
	out := make(chan model.Event, 3)
	if p.calls == 1 {
		out <- model.Event{Type: "toolCallReady", ToolCallID: "parsed-source", ToolName: "analyze_material", ArgsJSON: `{}`}
	} else {
		if runner.RequestBytes(req.Messages, req.Tools)+req.MaxOutputTokens > 20000 {
			p.t.Fatal("shared batch reservation would exceed remaining budget")
		}
		var text strings.Builder
		for _, message := range req.Messages {
			text.WriteString(message.Content)
		}
		for _, required := range []string{"archived", "parsed-source", "反对未经测试直接验收", "最多剩余 2 次尝试"} {
			if !strings.Contains(text.String(), required) {
				p.t.Fatalf("lost budget or protected evidence: %s", required)
			}
		}
		out <- model.Event{Type: "textDelta", TextDelta: "保留来源及异议，分析完成不等于验收"}
	}
	out <- model.Event{Type: "usage", Usage: model.UsageSnapshot{InputTokens: model.TokenCount(100), OutputTokens: model.TokenCount(50)}}
	out <- model.Event{Type: "finish"}
	close(out)
	return out, nil
}

func TestSharedBudgetCompactsParsedEvidenceWithoutChangingCap(t *testing.T) {
	registry := tools.New()
	original := strings.Repeat("有来源的解析记录。", 8000)
	registry.Register(tools.Tool{Name: "analyze_material", InputSchema: map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false}, Effect: tools.EffectRead, Execute: func(context.Context, map[string]any, tools.Env) (tools.Result, error) {
		return tools.Result{Data: map[string]any{"records": original, "versionId": "immutable-original"}}, nil
	}})
	provider := &budgetEvidenceProvider{t: t}
	journal := &sharedBudgetJournal{}
	out := runner.NewRunnerForTest(provider, registry).Run(context.Background(), runner.RunRequest{RunID: uuid.New(), Budget: runner.DefaultBudget(), Journal: journal, MaxInputTokens: 128000, MaxOutputTokens: 1000, Manifest: agentcontext.Build(agentcontext.Facts{Disagreements: []string{"反对未经测试直接验收"}})})
	if out.State != "succeeded" || provider.calls != 2 || runner.BatchMaxTotalTokens != 300000 {
		t.Fatalf("budget-aware completion failed: %+v", out)
	}
	if len(journal.Results) != 1 || !strings.Contains(string(journal.Results[0]), original) {
		t.Fatal("the complete parsed original was not retained")
	}
}

type exhaustedBudgetJournal struct{ checkpointJournal }

func (*exhaustedBudgetJournal) RemainingBudget(context.Context) (int64, int, error) {
	return 500, 1, nil
}

func TestExhaustedSharedBudgetStopsBeforeProvider(t *testing.T) {
	provider := &budgetEvidenceProvider{t: t}
	out := runner.NewRunnerForTest(provider, tools.New()).Run(context.Background(), runner.RunRequest{Budget: runner.DefaultBudget(), Journal: &exhaustedBudgetJournal{}, MaxOutputTokens: 1000})
	if out.State != "failed" || provider.calls != 0 || out.TokensUsed != 0 {
		t.Fatal(fmt.Sprintf("exhausted batch called provider or invented usage: %+v", out))
	}
}
