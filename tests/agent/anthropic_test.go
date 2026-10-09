// SPDX-License-Identifier: Apache-2.0
package agent_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kakj-go/Judex/internal/infrastructure/model"
)

// fakeAnthropicGateway serves Anthropic-protocol SSE frames.
func fakeAnthropicGateway(t *testing.T, frames []string, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-key" {
			w.WriteHeader(401)
			return
		}
		if status != 200 {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"type":"error","error":{"type":"rate_limit_error","message":"quota"}}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, frame := range frames {
			_, _ = w.Write([]byte("event: " + frameType(frame) + "\n"))
			_, _ = w.Write([]byte("data: " + frame + "\n\n"))
		}
	}))
}

func frameType(frame string) string {
	var probe struct {
		Type string `json:"type"`
	}
	_ = json.Unmarshal([]byte(frame), &probe)
	return probe.Type
}

func collectAnthropic(ch <-chan model.Event) (text string, tools []model.ToolCall, usage *model.Event, errEvent *model.Event, finish bool) {
	for event := range ch {
		switch event.Type {
		case "textDelta":
			text += event.TextDelta
		case "toolCallReady":
			tools = append(tools, model.ToolCall{ID: event.ToolCallID, Name: event.ToolName, Arguments: event.ArgsJSON})
		case "usage":
			u := event
			usage = &u
		case "finish":
			finish = true
		case "error":
			e := event
			errEvent = &e
		}
	}
	return
}

func TestAnthropicStreamTextThinkingUsage(t *testing.T) {
	server := fakeAnthropicGateway(t, []string{
		`{"type":"message_start","message":{"usage":{"input_tokens":42}}}`,
		`{"type":"ping"}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"thinking"}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"内部推理不应外泄"}}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"content_block_start","index":1,"content_block":{"type":"text"}}`,
		`{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"结论"}}`,
		`{"type":"content_block_stop","index":1}`,
		`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":7}}`,
		`{"type":"message_stop"}`,
	}, 200)
	defer server.Close()
	provider := model.NewAnthropicProvider(model.AnthropicConfig{BaseURL: server.URL, APIKey: "test-key"})
	ch, err := provider.Stream(context.Background(), model.Request{Model: "glm-5.3", MaxOutputTokens: 64,
		Messages: []model.Message{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatal(err)
	}
	text, _, usage, errEvent, finish := collectAnthropic(ch)
	if errEvent != nil {
		t.Fatalf("unexpected error: %v", errEvent.Err)
	}
	if !finish {
		t.Fatal("finish missing")
	}
	if text != "结论" {
		t.Fatalf("text=%q (thinking must be filtered)", text)
	}
	if usage == nil || usage.InputTokens != 42 || usage.OutputTokens != 7 {
		t.Fatalf("usage=%+v", usage)
	}
}

func TestAnthropicToolAccumulation(t *testing.T) {
	server := fakeAnthropicGateway(t, []string{
		`{"type":"message_start","message":{"usage":{"input_tokens":10}}}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"call_1","name":"read_material"}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"ver"}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"sionId\":\"v9\"}"}}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":5}}`,
		`{"type":"message_stop"}`,
	}, 200)
	defer server.Close()
	provider := model.NewAnthropicProvider(model.AnthropicConfig{BaseURL: server.URL, APIKey: "test-key"})
	ch, err := provider.Stream(context.Background(), model.Request{Model: "m", MaxOutputTokens: 64,
		Messages: []model.Message{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatal(err)
	}
	_, tools, _, errEvent, _ := collectAnthropic(ch)
	if errEvent != nil {
		t.Fatalf("unexpected error: %v", errEvent.Err)
	}
	if len(tools) != 1 || tools[0].ID != "call_1" || tools[0].Name != "read_material" {
		t.Fatalf("tools=%+v", tools)
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(tools[0].Arguments), &args); err != nil || args["versionId"] != "v9" {
		t.Fatalf("tool args invalid: %v %v", tools[0].Arguments, err)
	}
}

func TestAnthropicWireConversion(t *testing.T) {
	var captured []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured, _ = readAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"type":"message_stop"}` + "\n\n"))
	}))
	defer server.Close()
	provider := model.NewAnthropicProvider(model.AnthropicConfig{BaseURL: server.URL, APIKey: "test-key"})
	ch, err := provider.Stream(context.Background(), model.Request{
		Model: "m", MaxOutputTokens: 128,
		Messages: []model.Message{
			{Role: "system", Content: "系统职责"},
			{Role: "user", Content: "查看文件"},
			{Role: "assistant", Content: "", ToolCalls: []model.ToolCall{{ID: "c1", Name: "read", Arguments: `{"path":"a.md"}`}}},
			{Role: "tool", ToolCallID: "c1", Content: `{"content":"正文"}`},
			{Role: "tool", ToolCallID: "c1", Content: `{"content":"第二段"}`},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for range ch {
	}
	var sent struct {
		System   string `json:"system"`
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(captured, &sent); err != nil {
		t.Fatal(err)
	}
	if sent.System != "系统职责" {
		t.Fatalf("system=%q", sent.System)
	}
	if len(sent.Messages) != 3 {
		t.Fatalf("messages=%d (assistant + merged tool results)", len(sent.Messages))
	}
	// Tool results must merge into ONE user message with two tool_result blocks.
	last := sent.Messages[len(sent.Messages)-1]
	if last.Role != "user" {
		t.Fatalf("last role=%s", last.Role)
	}
	var blocks []map[string]any
	if err := json.Unmarshal(last.Content, &blocks); err != nil || len(blocks) != 2 {
		t.Fatalf("tool_result blocks=%d err=%v", len(blocks), err)
	}
	if blocks[0]["type"] != "tool_result" || blocks[0]["tool_use_id"] != "c1" {
		t.Fatalf("block0=%+v", blocks[0])
	}
}

func readAll(r io.Reader) ([]byte, error) {
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 1024)
	for {
		n, err := r.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			return buf, nil
		}
	}
}

func TestAnthropicErrorClassification(t *testing.T) {
	server := fakeAnthropicGateway(t, nil, 429)
	defer server.Close()
	provider := model.NewAnthropicProvider(model.AnthropicConfig{BaseURL: server.URL, APIKey: "test-key"})
	_, err := provider.Stream(context.Background(), model.Request{Model: "m", MaxOutputTokens: 8,
		Messages: []model.Message{{Role: "user", Content: "x"}}})
	if err == nil || model.Classify(err) != model.ErrorQuota {
		t.Fatalf("429 must classify quota, got %v", err)
	}
}

func TestAnthropicInvalidJSONFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {broken\n\n"))
	}))
	defer server.Close()
	provider := model.NewAnthropicProvider(model.AnthropicConfig{BaseURL: server.URL, APIKey: "test-key"})
	ch, err := provider.Stream(context.Background(), model.Request{Model: "m", MaxOutputTokens: 8,
		Messages: []model.Message{{Role: "user", Content: "x"}}})
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, errEvent, _ := collectAnthropic(ch)
	if errEvent == nil {
		t.Fatal("invalid JSON must surface as error")
	}
}

func TestAnthropicCancelPropagates(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"type":"message_start","message":{"usage":{"input_tokens":1}}}` + "\n\n"))
		w.(http.Flusher).Flush()
		time.Sleep(2 * time.Second)
	}))
	defer slow.Close()
	provider := model.NewAnthropicProvider(model.AnthropicConfig{BaseURL: slow.URL, APIKey: "test-key"})
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := provider.Stream(ctx, model.Request{Model: "m", MaxOutputTokens: 8,
		Messages: []model.Message{{Role: "user", Content: "x"}}})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	cancel()
	deadline := time.After(3 * time.Second)
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatal("closed without cancelled error")
			}
			if ev.Type == "error" {
				if model.Classify(ev.Err) != model.ErrorCancelled {
					t.Fatalf("expected cancelled, got %v", ev.Err)
				}
				return
			}
		case <-deadline:
			t.Fatal("cancel not propagated")
		}
	}
}
