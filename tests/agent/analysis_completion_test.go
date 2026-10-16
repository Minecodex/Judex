package agent_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/kakj-go/Judex/internal/agent/runner"
	"github.com/kakj-go/Judex/internal/agent/tools"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
)

type analysisCompletionProvider struct {
	t       *testing.T
	calls   int
	failure bool
}

func (p *analysisCompletionProvider) Stream(_ context.Context, req model.Request) (<-chan model.Event, error) {
	p.calls++
	out := make(chan model.Event, 3)
	if p.calls == 1 {
		out <- model.Event{Type: "toolCallReady", ToolCallID: "final-record", ToolName: "record_task_analysis", ArgsJSON: `{"summary":"原件已读，独立意见未覆盖；不代表人工验收","basis":["controlled original"]}`}
	} else {
		if !p.failure {
			p.t.Fatal("persisted final analysis made an unnecessary summary call")
		}
		out <- model.Event{Type: "textDelta", TextDelta: "结构化记录失败，未作为完成依据"}
	}
	out <- model.Event{Type: "usage", Usage: model.UsageSnapshot{InputTokens: model.TokenCount(20), OutputTokens: model.TokenCount(10)}}
	out <- model.Event{Type: "finish"}
	close(out)
	return out, nil
}

func TestPersistedAnalysisFinishesWithoutASecondModelCall(t *testing.T) {
	for _, failed := range []bool{false, true} {
		registry := tools.New()
		tools.RegisterDefaults(registry)
		provider := &analysisCompletionProvider{t: t, failure: failed}
		journal := &checkpointJournal{}
		env := &tools.Env{RecordTaskAnalysis: func(_ context.Context, _ string, args map[string]any) (map[string]any, error) {
			if failed {
				return nil, fmt.Errorf("record transaction failed")
			}
			return map[string]any{"analysisId": "persisted-analysis", "state": "draft"}, nil
		}}
		out := runner.NewRunnerForTest(provider, registry).Run(context.Background(), runner.RunRequest{PrincipalKind: "coordinator", Budget: runner.DefaultBudget(), Journal: journal, Env: env})
		if failed {
			if provider.calls != 2 || !strings.Contains(string(journal.Results[0]), "record transaction failed") {
				t.Fatal("failed persistence was treated as a successful terminal result")
			}
		} else if out.State != "succeeded" || provider.calls != 1 || out.ModelCalls != 1 || out.TokensUsed != 30 || !strings.Contains(out.Summary, "未覆盖") {
			t.Fatalf("final recorded analysis was lost or embellished: %+v", out)
		}
	}
}

type closingPositionProvider struct{ t *testing.T }

func (p closingPositionProvider) Stream(_ context.Context, req model.Request) (<-chan model.Event, error) {
	if len(req.Tools) != 0 || !strings.Contains(req.Messages[1].Content, "直接返回公开意见") {
		p.t.Fatal("position continued exploration with only enough budget to finish")
	}
	out := make(chan model.Event, 3)
	out <- model.Event{Type: "textDelta", TextDelta: "材料未读完整，意见有此缺口；未重跑测试"}
	out <- model.Event{Type: "usage", Usage: model.UsageSnapshot{InputTokens: model.TokenCount(20), OutputTokens: model.TokenCount(10)}}
	out <- model.Event{Type: "finish"}
	close(out)
	return out, nil
}

func TestPositionClosesWithinBudgetAndPreservesGap(t *testing.T) {
	registry := tools.New()
	tools.RegisterDefaults(registry)
	budget := runner.DefaultBudget()
	budget.MaxTotalTokens = 10000
	out := runner.NewRunnerForTest(closingPositionProvider{t}, registry).Run(context.Background(), runner.RunRequest{PrincipalKind: "position", Budget: budget, Journal: &checkpointJournal{}, MaxOutputTokens: 1000})
	if out.State != "succeeded" || out.ToolCalls != 0 || out.ModelCalls != 1 || !strings.Contains(out.Summary, "缺口") {
		t.Fatalf("position did not close truthfully: %+v", out)
	}
}
