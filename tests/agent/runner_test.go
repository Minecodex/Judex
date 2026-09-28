// SPDX-License-Identifier: Apache-2.0
package agent_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	agentcontext "github.com/kakj-go/Judex/internal/agent/context"
	"github.com/kakj-go/Judex/internal/agent/runner"
	"github.com/kakj-go/Judex/internal/agent/tools"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
	"github.com/google/uuid"
)

// scriptGateway replays scripted SSE frames per model call.
func scriptGateway(t *testing.T, calls [][]string) *httptest.Server {
	t.Helper()
	index := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if index >= len(calls) {
			t.Errorf("unexpected extra model call %d", index)
			w.WriteHeader(500)
			return
		}
		frames := calls[index]
		index++
		w.Header().Set("Content-Type", "text/event-stream")
		for _, frame := range frames {
			_, _ = w.Write([]byte("data: " + frame + "\n\n"))
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
}

func newRunner(gateway string) *runner.Runner {
	registry := tools.New()
	tools.RegisterDefaults(registry)
	provider := model.NewOpenAIProvider(model.OpenAIConfig{BaseURL: gateway, APIKey: "k"})
	return runner.NewRunnerForTest(provider, registry)
}

// TestRunnerToolRoundAndFinish (E03/E06 受控): 模型请求工具 → 工具结果回填 →
// 第二轮输出结论；usage 计入；未超预算。
func TestRunnerToolRoundAndFinish(t *testing.T) {
	gateway := scriptGateway(t, [][]string{
		{ // first call: asks to query work
			`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"c1","function":{"name":"query_work"}}]}}]}`,
			`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"objectType\":\"task\"}"}}]}}]}`,
			`{"choices":[{"delta":{"finish_reason":"tool_calls"}}]}`,
			`{"choices":[],"usage":{"prompt_tokens":100,"completion_tokens":10}}`,
		},
		{ // second call: final summary
			`{"choices":[{"delta":{"content":"任务清单已核对"}}]}`,
			`{"choices":[{"delta":{"finish_reason":"stop"}}]}`,
			`{"choices":[],"usage":{"prompt_tokens":120,"completion_tokens":8}}`,
		},
	})
	defer gateway.Close()
	r := newRunner(gateway.URL)
	outcome := r.Run(context.Background(), runner.RunRequest{
		RunID: uuid.New(), SessionID: uuid.New(), ProjectID: uuid.New(), IdentityID: uuid.New(),
		ModelName: "m", Manifest: agentcontext.Build(agentcontext.Facts{}), Budget: runner.DefaultBudget(),
	})
	if outcome.State != "succeeded" {
		t.Fatalf("state = %s summary = %s", outcome.State, outcome.Summary)
	}
	if outcome.ToolCalls != 1 || outcome.ModelCalls != 2 {
		t.Fatalf("tool=%d model=%d", outcome.ToolCalls, outcome.ModelCalls)
	}
	if outcome.TokensUsed != 238 || !outcome.UsageKnown {
		t.Fatalf("tokens=%d known=%v", outcome.TokensUsed, outcome.UsageKnown)
	}
}

// TestRunnerBudgetExhausted: token 上限停止且状态如实（不批准业务）。
func TestRunnerBudgetExhausted(t *testing.T) {
	bigUsage := `{"choices":[],"usage":{"prompt_tokens":900000,"completion_tokens":1}}`
	gateway := scriptGateway(t, [][]string{
		{`{"choices":[{"delta":{"content":"…"}}]}`, `{"choices":[{"delta":{"finish_reason":"stop"}}]}`, bigUsage},
	})
	defer gateway.Close()
	r := newRunner(gateway.URL)
	outcome := r.Run(context.Background(), runner.RunRequest{
		RunID: uuid.New(), ModelName: "m", Budget: runner.DefaultBudget(),
	})
	if outcome.State != "failed" || outcome.TokensUsed < 100000 {
		t.Fatalf("budget must stop the run: %+v", outcome)
	}
}

// TestRunnerUnknownUsageHolds: usage 缺失时 UsageKnown=false（不记零）。
func TestRunnerUnknownUsageHolds(t *testing.T) {
	gateway := scriptGateway(t, [][]string{
		{`{"choices":[{"delta":{"content":"无计量网关"}}]}`, `{"choices":[{"delta":{"finish_reason":"stop"}}]}`},
	})
	defer gateway.Close()
	r := newRunner(gateway.URL)
	outcome := r.Run(context.Background(), runner.RunRequest{
		RunID: uuid.New(), ModelName: "m", Budget: runner.DefaultBudget(),
	})
	if outcome.State != "succeeded" || outcome.UsageKnown {
		t.Fatalf("usage unknown must surface: %+v", outcome)
	}
}

// TestRunnerCancellation: ctx 取消 → cancelled，不写成失败。
func TestRunnerCancellation(t *testing.T) {
	gateway := scriptGateway(t, [][]string{
		{`{"choices":[{"delta":{"content":"x"}}]}`},
	})
	defer gateway.Close()
	r := newRunner(gateway.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	outcome := r.Run(ctx, runner.RunRequest{RunID: uuid.New(), ModelName: "m", Budget: runner.DefaultBudget()})
	if outcome.State != "cancelled" {
		t.Fatalf("state = %s", outcome.State)
	}
}
