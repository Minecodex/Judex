// SPDX-License-Identifier: Apache-2.0
package agent_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/kakj-go/Judex/internal/agent/runner"
	"github.com/kakj-go/Judex/internal/agent/tools"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
)

func TestOpenAICacheUsagePresenceAndCumulativeFrames(t *testing.T) {
	for _, tc := range []struct {
		name, detail string
		want         *int64
	}{
		{"missing", "", nil},
		{"zero", `,"prompt_tokens_details":{"cached_tokens":0}`, model.TokenCount(0)},
		{"hit", `,"prompt_tokens_details":{"cached_tokens":800}`, model.TokenCount(800)},
		{"invalid", `,"prompt_tokens_details":{"cached_tokens":1001}`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := fakeGateway(t, []string{
				`{"id":"request-id","model":"returned-model","choices":[],"usage":{"prompt_tokens":1000,"completion_tokens":1` + tc.detail + `}}`,
				`{"choices":[],"usage":{"completion_tokens":50}}`,
			}, 200)
			defer server.Close()
			stream, err := model.NewOpenAIProvider(model.OpenAIConfig{BaseURL: server.URL, APIKey: "test-key"}).Stream(context.Background(), model.Request{Model: "requested-model"})
			if err != nil {
				t.Fatal(err)
			}
			_, _, event, failed := collect(stream)
			if failed != nil || event == nil || !event.Usage.Known() || event.Usage.Total() != 1050 {
				t.Fatalf("usage=%+v error=%+v", event, failed)
			}
			u := event.Usage
			if (u.CacheReadTokens == nil) != (tc.want == nil) || tc.want != nil && *u.CacheReadTokens != *tc.want {
				t.Fatalf("cache=%v want=%v", u.CacheReadTokens, tc.want)
			}
			if u.CacheWriteTokens != nil || u.ModelName != "returned-model" || u.ProviderRequestID != "request-id" {
				t.Fatalf("metadata=%+v", u)
			}
		})
	}
}

func TestAnthropicSplitCacheTotalsAndLegacyFrames(t *testing.T) {
	for _, tc := range []struct {
		name, start, delta string
		known              bool
		input              int64
		cached             *int64
	}{
		{"native", `{"input_tokens":10,"cache_read_input_tokens":800,"cache_creation_input_tokens":200}`, `{"output_tokens":50}`, true, 1010, model.TokenCount(800)},
		{"legacy", `{"input_tokens":0}`, `{"input_tokens":1000,"output_tokens":50}`, true, 1000, nil},
		{"partial-split", `{"input_tokens":10,"cache_read_input_tokens":800}`, `{"output_tokens":50}`, false, 0, model.TokenCount(800)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := fakeAnthropicGateway(t, []string{
				`{"type":"message_start","message":{"id":"request-id","usage":` + tc.start + `}}`,
				`{"type":"message_delta","usage":` + tc.delta + `}`, `{"type":"message_stop"}`,
			}, 200)
			defer server.Close()
			stream, err := model.NewAnthropicProvider(model.AnthropicConfig{BaseURL: server.URL, APIKey: "test-key"}).Stream(context.Background(), model.Request{Model: "m"})
			if err != nil {
				t.Fatal(err)
			}
			_, _, event, failed, _ := collectAnthropic(stream)
			if failed != nil || event == nil || event.Usage.Known() != tc.known {
				t.Fatalf("usage=%+v error=%+v", event, failed)
			}
			if tc.known && *event.Usage.InputTokens != tc.input {
				t.Fatalf("input=%v want=%d", event.Usage.InputTokens, tc.input)
			}
			if (event.Usage.CacheReadTokens == nil) != (tc.cached == nil) || tc.cached != nil && *event.Usage.CacheReadTokens != *tc.cached {
				t.Fatal("cache detail lost")
			}
		})
	}
}

type usageJournal struct {
	checkpointJournal
	Usage model.UsageSnapshot
	Known bool
}

func (j *usageJournal) ModelEnd(_ context.Context, _ int, u model.UsageSnapshot, known bool, _ error) error {
	j.Usage, j.Known = u, known
	return nil
}

func TestRunnerRetainsFinalCacheSnapshotAndKnownToolOrder(t *testing.T) {
	gateway := scriptGateway(t, [][]string{{
		`{"choices":[],"usage":{"prompt_tokens":1000,"completion_tokens":1,"prompt_tokens_details":{"cached_tokens":800}}}`,
		`{"choices":[],"usage":{"prompt_tokens":1000,"completion_tokens":50,"prompt_tokens_details":{"cached_tokens":800}}}`,
		`{"choices":[{"delta":{"content":"complete","finish_reason":"stop"}}]}`,
	}})
	defer gateway.Close()
	j := &usageJournal{}
	result := newRunner(gateway.URL).Run(context.Background(), runner.RunRequest{ModelName: "m", Journal: j, Budget: runner.DefaultBudget()})
	if result.State != "succeeded" || result.TokensUsed != 1050 || !j.Known || j.Usage.CacheReadTokens == nil || *j.Usage.CacheReadTokens != 800 {
		t.Fatalf("outcome=%+v usage=%+v", result, j.Usage)
	}
	r := tools.New()
	tools.RegisterDefaults(r)
	first, _ := json.Marshal(r.Schemas())
	for i := 0; i < 30; i++ {
		next, _ := json.Marshal(r.Schemas())
		if string(next) != string(first) {
			t.Fatal("tool schemas vary between identical requests")
		}
	}
}
