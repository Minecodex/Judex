package agent_test

import (
	"context"
	"strings"
	"testing"

	"github.com/kakj-go/Judex/internal/agent/runner"
	"github.com/kakj-go/Judex/internal/agent/tools"
	"github.com/kakj-go/Judex/internal/collaboration"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
)

type analysisValidationProvider func(context.Context, model.Request) (<-chan model.Event, error)

func (p analysisValidationProvider) Stream(ctx context.Context, req model.Request) (<-chan model.Event, error) {
	return p(ctx, req)
}

func TestPublicAnalysisRejectsUnresolvedTemplateWithoutRejectingQuotedEvidence(t *testing.T) {
	for _, summary := range []string{"", "   ", "__SLOT_B__", "__ROLE_A__； __ROLE_B__", "{{conclusion}}", "[[SUMMARY]]", "<|result|>", strings.Repeat("中", 8001)} {
		if err := collaboration.ValidateAnalysisSummary(summary); err == nil {
			t.Fatalf("accepted unresolved or oversized summary %q", summary[:min(len(summary), 80)])
		}
	}
	for _, summary := range []string{"无法读取来源，尚不能判断通过。", "历史模型输出为 __SLOT_B__，本次保留该失败并列出独立意见。", strings.Repeat("中", 8000)} {
		if err := collaboration.ValidateAnalysisSummary(summary); err != nil {
			t.Fatalf("rejected an actual conclusion or quoted evidence: %v", err)
		}
	}
}

func TestAnalysisPlaceholderReturnsCorrectionToModelBeforePersistence(t *testing.T) {
	registry := tools.New()
	tools.RegisterDefaults(registry)
	calls, persisted := 0, 0
	provider := analysisValidationProvider(func(_ context.Context, request model.Request) (<-chan model.Event, error) {
		calls++
		out := make(chan model.Event, 3)
		args := `{"summary":"__SLOT_B__","basis":["controlled original"]}`
		if calls == 2 {
			var correction bool
			for _, message := range request.Messages {
				correction = correction || message.Role == "tool" && strings.Contains(message.Content, "unresolved placeholder")
			}
			if !correction {
				t.Fatal("model did not receive the failed validation for correction")
			}
			args = `{"summary":"甲已核对源码，乙保留图片未读的缺口；人工验收尚未执行。","basis":["controlled source"]}`
		}
		out <- model.Event{Type: "toolCallReady", ToolCallID: "record-" + string(rune('0'+calls)), ToolName: "record_task_analysis", ArgsJSON: args}
		out <- model.Event{Type: "usage", Usage: model.UsageSnapshot{InputTokens: model.TokenCount(100), OutputTokens: model.TokenCount(20)}}
		out <- model.Event{Type: "finish"}
		close(out)
		return out, nil
	})
	journal := &checkpointJournal{}
	env := &tools.Env{RecordTaskAnalysis: func(_ context.Context, _ string, args map[string]any) (map[string]any, error) {
		persisted++
		return map[string]any{"analysisId": "valid-analysis"}, nil
	}}
	result := runner.NewRunnerForTest(provider, registry).Run(context.Background(), runner.RunRequest{PrincipalKind: "coordinator", Budget: runner.DefaultBudget(), Journal: journal, Env: env})
	if calls != 2 || persisted != 1 || result.State != "succeeded" || !strings.Contains(result.Summary, "乙保留") {
		t.Fatalf("bad public result escaped or was not corrected: calls=%d persisted=%d result=%+v", calls, persisted, result)
	}
}

func TestRequiredAnalysisCannotFinishWithOnlyATextAnswer(t *testing.T) {
	registry := tools.New()
	tools.RegisterDefaults(registry)
	calls, persisted := 0, 0
	provider := analysisValidationProvider(func(_ context.Context, req model.Request) (<-chan model.Event, error) {
		calls++
		out := make(chan model.Event, 3)
		if calls == 1 {
			out <- model.Event{Type: "textDelta", TextDelta: "信息不足，需要澄清。"}
		} else {
			if !strings.Contains(req.Messages[len(req.Messages)-1].Content, "尚未通过 record_task_analysis 保存") {
				t.Fatal("missing structured result did not get a bounded correction")
			}
			out <- model.Event{Type: "toolCallReady", ToolCallID: "record", ToolName: "record_task_analysis", ArgsJSON: `{"summary":"信息不足，已保存澄清与缺口，不安排工作。","needsClarification":true,"basis":["controlled incomplete source"]}`}
		}
		out <- model.Event{Type: "usage", Usage: model.UsageSnapshot{InputTokens: model.TokenCount(100), OutputTokens: model.TokenCount(20)}}
		out <- model.Event{Type: "finish"}
		close(out)
		return out, nil
	})
	env := &tools.Env{RecordTaskAnalysis: func(context.Context, string, map[string]any) (map[string]any, error) {
		persisted++
		return map[string]any{"analysisId": "saved-clarification"}, nil
	}}
	result := runner.NewRunnerForTest(provider, registry).Run(context.Background(), runner.RunRequest{RequireAnalysisRecord: true, PrincipalKind: "coordinator", Budget: runner.DefaultBudget(), Journal: &checkpointJournal{}, Env: env})
	if result.State != "succeeded" || calls != 2 || persisted != 1 || !strings.Contains(result.Summary, "已保存") {
		t.Fatalf("text was presented as durable analysis: calls=%d persisted=%d %+v", calls, persisted, result)
	}
}

func TestRequiredAnalysisKeepsTheOriginalAttemptBudget(t *testing.T) {
	registry := tools.New()
	tools.RegisterDefaults(registry)
	provider := analysisValidationProvider(func(context.Context, model.Request) (<-chan model.Event, error) {
		out := make(chan model.Event, 3)
		out <- model.Event{Type: "textDelta", TextDelta: "只有口头回答，未保存。"}
		out <- model.Event{Type: "usage", Usage: model.UsageSnapshot{InputTokens: model.TokenCount(100), OutputTokens: model.TokenCount(20)}}
		out <- model.Event{Type: "finish"}
		close(out)
		return out, nil
	})
	budget := runner.DefaultBudget()
	budget.MaxModelAttempts = 1
	result := runner.NewRunnerForTest(provider, registry).Run(context.Background(), runner.RunRequest{RequireAnalysisRecord: true, PrincipalKind: "coordinator", Budget: budget, Journal: &checkpointJournal{}})
	if result.State != "failed" || result.ModelCalls != 1 {
		t.Fatalf("unrecorded analysis bypassed its budget: %+v", result)
	}
}

func TestAnalysisEvidenceIsRequiredBeforeTheEffect(t *testing.T) {
	registry := tools.New()
	tools.RegisterDefaults(registry)
	persisted := 0
	env := tools.Env{RecordTaskAnalysis: func(context.Context, string, map[string]any) (map[string]any, error) {
		persisted++
		return map[string]any{"analysisId": "grounded-clarification"}, nil
	}}
	for _, basis := range []any{nil, []any{}, []any{""}, []any{" \n\t "}} {
		args := map[string]any{"summary": "Needs clarification", "needsClarification": true}
		if basis != nil {
			args["basis"] = basis
		}
		if _, err := registry.Execute(context.Background(), "record_task_analysis", "coordinator", args, env); err == nil {
			t.Fatalf("ungrounded result reached the recorder: basis=%#v", basis)
		}
	}
	if persisted != 0 {
		t.Fatalf("invalid analysis had effects: %d", persisted)
	}
	args := map[string]any{"summary": "Need an affected interface before choosing a workflow", "needsClarification": true, "basis": []any{"submission source-1 reports slowness without an interface or measurements"}}
	if _, err := registry.Execute(context.Background(), "record_task_analysis", "coordinator", args, env); err != nil || persisted != 1 {
		t.Fatalf("grounded clarification failed: persisted=%d error=%v", persisted, err)
	}
}
