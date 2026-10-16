package agent_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kakj-go/Judex/internal/agent/runner"
	"github.com/kakj-go/Judex/internal/agent/tools"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
)

func TestTruncatedOutputRetainsKnownUsageAndNeverExecutesPartialTools(t *testing.T) {
	gateway := scriptGateway(t, [][]string{{
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"partial","function":{"name":"query_work","arguments":"{\"objectType\":"}}]}}]}`,
		`{"choices":[{"delta":{},"finish_reason":"length"}]}`,
		`{"choices":[],"usage":{"prompt_tokens":100,"completion_tokens":8192}}`,
	}})
	defer gateway.Close()
	registry := tools.New()
	tools.RegisterDefaults(registry)
	provider := model.NewOpenAIProvider(model.OpenAIConfig{BaseURL: gateway.URL, APIKey: "k"})
	journal := &checkpointJournal{}
	out := runner.NewRunnerForTest(provider, registry).Run(context.Background(), runner.RunRequest{Budget: runner.DefaultBudget(), Journal: journal})
	if out.State != "failed" || !out.UsageKnown || out.TokensUsed != 8292 || out.ToolCalls != 0 || len(journal.Results) != 0 {
		t.Fatalf("truncated output was accepted, executed or lost its meter: %+v", out)
	}
}

func TestThinkingOptionIsExplicitAndDoesNotChangeTokenCap(t *testing.T) {
	for _, thinking := range []string{"", "disabled"} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["max_tokens"] != float64(8192) {
				t.Error("thinking mode changed frozen output cap")
			}
			if thinking == "" {
				if body["thinking"] != nil {
					t.Error("unconfigured provider received vendor-specific thinking")
				}
			} else if body["thinking"].(map[string]any)["type"] != thinking {
				t.Error("operator thinking choice missing")
			}
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"))
		}))
		provider, err := model.NewProvider(model.GatewayConfig{Protocol: model.ProtocolOpenAI, BaseURL: server.URL, APIKey: "test", RequestOptions: model.RequestOptions{ThinkingType: thinking}})
		if err != nil {
			t.Fatal(err)
		}
		configured := provider.(model.ConfiguredOptions).RequestOptions()
		stream, err := provider.Stream(context.Background(), model.Request{Model: "controlled", MaxOutputTokens: 8192, Options: configured})
		if err != nil {
			t.Fatal(err)
		}
		for event := range stream {
			if event.Type == "error" {
				t.Fatal(event.Err)
			}
		}
		server.Close()
	}
}
