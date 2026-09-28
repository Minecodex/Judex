// SPDX-License-Identifier: Apache-2.0
package agent_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kakj-go/Judex/internal/infrastructure/model"
)

// fakeGateway serves controlled SSE frames for contract tests (05 §2:
// fixture 不代替真实模型，但必须覆盖拆包/无效 JSON/工具累积/usage 缺失/429/5xx/取消).
func fakeGateway(t *testing.T, frames []string, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != 200 {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":{"message":"boom"}}`))
			return
		}
		// Verify auth + shape without logging secrets.
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(401)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, frame := range frames {
			_, _ = w.Write([]byte("data: " + frame + "\n\n"))
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
}

func collect(ch <-chan model.Event) (text string, tools []model.ToolCall, usage *model.Event, errEvent *model.Event) {
	for event := range ch {
		switch event.Type {
		case "textDelta":
			text += event.TextDelta
		case "toolCallReady":
			tools = append(tools, model.ToolCall{ID: event.ToolCallID, Name: event.ToolName, Arguments: event.ArgsJSON})
		case "usage":
			u := event
			usage = &u
		case "error":
			e := event
			errEvent = &e
		}
	}
	return
}

func TestStreamTextAndUsage(t *testing.T) {
	server := fakeGateway(t, []string{
		`{"choices":[{"delta":{"content":"你"}}]}`,
		`{"choices":[{"delta":{"content":"好"}}]}`,
		`{"choices":[{"delta":{"finish_reason":"stop"}}]}`,
		`{"choices":[],"usage":{"prompt_tokens":128,"completion_tokens":7}}`,
	}, 200)
	defer server.Close()
	provider := model.NewOpenAIProvider(model.OpenAIConfig{BaseURL: server.URL, APIKey: "test-key"})
	ch, err := provider.Stream(context.Background(), model.Request{Model: "test-model"})
	if err != nil {
		t.Fatal(err)
	}
	text, _, usage, errEvent := collect(ch)
	if errEvent != nil {
		t.Fatalf("unexpected error: %v", errEvent.Err)
	}
	if text != "你好" {
		t.Fatalf("text = %q", text)
	}
	if usage == nil || usage.InputTokens != 128 || usage.OutputTokens != 7 {
		t.Fatalf("usage = %+v", usage)
	}
}

func TestStreamToolCallAccumulates(t *testing.T) {
	server := fakeGateway(t, []string{
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"read_material"}}]}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"pa"}}]}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"th\":\"x\"}"}}]}}]}`,
		`{"choices":[{"delta":{"finish_reason":"tool_calls"}}]}`,
	}, 200)
	defer server.Close()
	provider := model.NewOpenAIProvider(model.OpenAIConfig{BaseURL: server.URL, APIKey: "test-key"})
	ch, err := provider.Stream(context.Background(), model.Request{Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	_, tools, _, errEvent := collect(ch)
	if errEvent != nil {
		t.Fatalf("unexpected error: %v", errEvent.Err)
	}
	if len(tools) != 1 || tools[0].ID != "call_1" || tools[0].Name != "read_material" {
		t.Fatalf("tools = %+v", tools)
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(tools[0].Arguments), &args); err != nil || args["path"] != "x" {
		t.Fatalf("tool args invalid: %v %v", tools[0].Arguments, err)
	}
}

func TestStreamInvalidJSONFails(t *testing.T) {
	server := fakeGateway(t, []string{`{not-json`}, 200)
	defer server.Close()
	provider := model.NewOpenAIProvider(model.OpenAIConfig{BaseURL: server.URL, APIKey: "test-key"})
	ch, err := provider.Stream(context.Background(), model.Request{Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, errEvent := collect(ch)
	if errEvent == nil || model.Classify(errEvent.Err) != model.ErrorInvalidRequest {
		t.Fatalf("invalid JSON must surface as error, got %+v", errEvent)
	}
}

func TestErrorClassification(t *testing.T) {
	for status, want := range map[int]model.ErrorClass{
		401: model.ErrorAuth, 429: model.ErrorQuota, 500: model.ErrorTransient,
	} {
		server := fakeGateway(t, nil, status)
		provider := model.NewOpenAIProvider(model.OpenAIConfig{BaseURL: server.URL, APIKey: "test-key"})
		_, err := provider.Stream(context.Background(), model.Request{Model: "m"})
		if err == nil || model.Classify(err) != want {
			t.Fatalf("status %d: class = %v err = %v", status, model.Classify(err), err)
		}
		server.Close()
	}
}

func TestCancelPropagates(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"a\"}}]}\n\n"))
		w.(http.Flusher).Flush()
		time.Sleep(2 * time.Second)
	}))
	defer slow.Close()
	provider := model.NewOpenAIProvider(model.OpenAIConfig{BaseURL: slow.URL, APIKey: "test-key",
		HTTP: &http.Client{Timeout: 5 * time.Second}})
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := provider.Stream(ctx, model.Request{Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	cancel()
	deadline := time.After(3 * time.Second)
	for {
		select {
		case event, ok := <-ch:
			if !ok {
				t.Fatal("channel closed without cancelled error")
			}
			if event.Type == "error" {
				if model.Classify(event.Err) != model.ErrorCancelled && !errors.Is(event.Err, context.Canceled) {
					t.Fatalf("expected cancelled, got %v", event.Err)
				}
				return
			}
		case <-deadline:
			t.Fatal("cancel not propagated in time")
		}
	}
}

func TestMissingUsageNotZeroCost(t *testing.T) {
	server := fakeGateway(t, []string{
		`{"choices":[{"delta":{"content":"done"}}]}`,
		`{"choices":[{"delta":{"finish_reason":"stop"}}]}`,
	}, 200)
	defer server.Close()
	provider := model.NewOpenAIProvider(model.OpenAIConfig{BaseURL: server.URL, APIKey: "test-key"})
	ch, err := provider.Stream(context.Background(), model.Request{Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	text, _, usage, _ := collect(ch)
	if text != "done" {
		t.Fatalf("text = %q", text)
	}
	if usage != nil && usage.InputTokens == 0 && usage.OutputTokens == 0 {
		t.Fatal("usage 缺失时不得记零成本：应保持 nil 由调用方标记 usageKnown=false")
	}
	if usage != nil {
		t.Fatalf("usage expected nil, got %+v", usage)
	}
	_ = strings.TrimSpace
}

func TestOpenAIWireToolCallShape(t *testing.T) {
	var requests []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var doc map[string]any
		_ = json.Unmarshal(raw, &doc)
		requests = append(requests, doc)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()
	provider := model.NewOpenAIProvider(model.OpenAIConfig{BaseURL: server.URL, APIKey: "test-key"})
	// 1) initial request with tools; 2) follow-up carrying assistant tool_calls + tool result
	_, err := provider.Stream(context.Background(), model.Request{
		Model: "m", Messages: []model.Message{{Role: "user", Content: "x"}},
		Tools: []model.ToolSchema{{Name: "read", InputSchema: map[string]any{"type": "object"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.Stream(context.Background(), model.Request{
		Model: "m",
		Messages: []model.Message{
			{Role: "user", Content: "x"},
			{Role: "assistant", ToolCalls: []model.ToolCall{{ID: "c1", Name: "read", Arguments: `{"path":"a"}`}}},
			{Role: "tool", ToolCallID: "c1", Content: `{"content":"a"}`},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 2 {
		t.Fatalf("requests=%d", len(requests))
	}
	msgs := requests[1]["messages"].([]any)
	assistant := msgs[1].(map[string]any)
	calls := assistant["tool_calls"].([]any)
	call := calls[0].(map[string]any)
	if call["type"] != "function" {
		t.Fatalf("tool_call.type=%v (must be \"function\")", call["type"])
	}
	fn := call["function"].(map[string]any)
	if fn["name"] != "read" {
		t.Fatalf("function.name=%v", fn["name"])
	}
}
