package agent_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/kakj-go/Judex/internal/agent/runner"
	"github.com/kakj-go/Judex/internal/agent/tools"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

type validationEchoProvider struct {
	t     *testing.T
	calls int
}

func (p *validationEchoProvider) Stream(_ context.Context, req model.Request) (<-chan model.Event, error) {
	p.calls++
	out := make(chan model.Event, 3)
	if p.calls == 1 {
		out <- model.Event{Type: "toolCallReady", ToolCallID: "invalid-draft", ToolName: "validate_draft", ArgsJSON: "{}"}
	} else {
		var result struct {
			ErrorCode string `json:"errorCode"`
			Details   struct {
				Fields []apierrors.FieldIssue `json:"fields"`
			} `json:"details"`
		}
		if err := json.Unmarshal([]byte(req.Messages[len(req.Messages)-1].Content), &result); err != nil {
			p.t.Fatal(err)
		}
		if result.ErrorCode != "VALIDATION_ERROR" || len(result.Details.Fields) != 1 || result.Details.Fields[0].Path != "nodes[].kind" {
			p.t.Fatalf("missing field validation detail: %+v", result)
		}
		out <- model.Event{Type: "textDelta", TextDelta: "The invalid field is identified"}
	}
	out <- model.Event{Type: "usage", Usage: model.UsageSnapshot{InputTokens: model.TokenCount(20), OutputTokens: model.TokenCount(5)}}
	out <- model.Event{Type: "finish"}
	close(out)
	return out, nil
}
func TestRunnerPreservesPublicFieldValidationDetails(t *testing.T) {
	registry := tools.New()
	registry.Register(tools.Tool{Name: "validate_draft", Effect: tools.EffectDraft, InputSchema: map[string]any{"type": "object"}, Execute: func(context.Context, map[string]any, tools.Env) (tools.Result, error) {
		return tools.Result{}, apierrors.Fields("nodes[].kind", "enum")
	}})
	out := runner.NewRunnerForTest(&validationEchoProvider{t: t}, registry).Run(context.Background(), runner.RunRequest{ModelName: "controlled", Budget: runner.DefaultBudget()})
	if out.State != "succeeded" {
		t.Fatalf("outcome %+v", out)
	}
}
