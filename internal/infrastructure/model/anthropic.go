// SPDX-License-Identifier: Apache-2.0

package model

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// AnthropicConfig carries an Anthropic-protocol gateway entry (e.g. the GLM
// Coding Plan at https://open.bigmodel.cn/api/anthropic).
type AnthropicConfig struct {
	BaseURL string // gateway root; messages endpoint is {BaseURL}/v1/messages
	APIKey  string
	HTTP    *http.Client
}

// AnthropicProvider normalizes the Anthropic Messages API (streaming) into
// the same Event stream as the OpenAI provider: textDelta / toolCallDelta /
// toolCallReady / usage / finish / error. Thinking blocks are consumed for
// budget purposes but never surfaced as text (05 §2: 私有链路不原样推送).
type AnthropicProvider struct {
	cfg AnthropicConfig
}

func NewAnthropicProvider(cfg AnthropicConfig) *AnthropicProvider {
	if cfg.HTTP == nil {
		cfg.HTTP = &http.Client{Timeout: 10 * time.Minute}
	}
	return &AnthropicProvider{cfg: cfg}
}

// ---- wire types (Anthropic Messages API) ----

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int64              `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Stream    bool               `json:"stream"`
	Messages  []anthropicMessage `json:"messages"`
	Tools     []anthropicTool    `json:"tools,omitempty"`
}

type anthropicMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"` // string or block array
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema"`
}

// toWire converts our neutral Messages into the Anthropic wire format:
// - system messages lift into the top-level system field
// - assistant ToolCalls become tool_use content blocks
// - consecutive tool results merge into ONE user message (API requirement)
func toWire(messages []Message) (system string, wire []anthropicMessage) {
	mustContent := func(v any) json.RawMessage {
		raw, err := json.Marshal(v)
		if err != nil {
			return json.RawMessage(`""`)
		}
		return raw
	}
	appendToolResults := func(blocks []map[string]any) {
		if len(blocks) == 0 {
			return
		}
		// Merge into a trailing user message when possible.
		if len(wire) > 0 && wire[len(wire)-1].Role == "user" {
			var existing []map[string]any
			_ = json.Unmarshal(wire[len(wire)-1].Content, &existing)
			existing = append(existing, blocks...)
			wire[len(wire)-1].Content = mustContent(existing)
			return
		}
		wire = append(wire, anthropicMessage{Role: "user", Content: mustContent(blocks)})
	}
	for _, msg := range messages {
		switch msg.Role {
		case "system":
			if system != "" {
				system += "\n\n"
			}
			system += msg.Content
		case "user":
			wire = append(wire, anthropicMessage{Role: "user", Content: mustContent(msg.Content)})
		case "assistant":
			blocks := []map[string]any{}
			if msg.Content != "" {
				blocks = append(blocks, map[string]any{"type": "text", "text": msg.Content})
			}
			for _, call := range msg.ToolCalls {
				var input any
				if err := json.Unmarshal([]byte(call.Arguments), &input); err != nil {
					input = map[string]any{}
				}
				blocks = append(blocks, map[string]any{
					"type": "tool_use", "id": call.ID, "name": call.Name, "input": input,
				})
			}
			if len(blocks) == 0 {
				blocks = append(blocks, map[string]any{"type": "text", "text": ""})
			}
			wire = append(wire, anthropicMessage{Role: "assistant", Content: mustContent(blocks)})
		case "tool":
			appendToolResults([]map[string]any{{
				"type": "tool_result", "tool_use_id": msg.ToolCallID, "content": msg.Content,
			}})
		}
	}
	return system, wire
}

// Stream opens a streaming Messages request and normalizes the SSE events.
func (p *AnthropicProvider) Stream(ctx context.Context, req Request) (<-chan Event, error) {
	maxTokens := req.MaxOutputTokens
	if maxTokens <= 0 {
		maxTokens = 8192
	}
	system, wire := toWire(req.Messages)
	body := anthropicRequest{
		Model: req.Model, MaxTokens: maxTokens, Stream: true,
		System: system, Messages: wire,
	}
	for _, tool := range req.Tools {
		body.Tools = append(body.Tools, anthropicTool{
			Name: tool.Name, InputSchema: tool.InputSchema,
		})
	}
	raw, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		strings.TrimSuffix(p.cfg.BaseURL, "/")+"/v1/messages", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.cfg.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	// Transient connection failures get one immediate replay (tunnel warmup).
	httpReq.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(raw)), nil }
	resp, err := p.cfg.HTTP.Do(httpReq)
	if err != nil {
		_ = httpReq.Body.Close()
		httpReq.Body, _ = httpReq.GetBody()
		resp, err = p.cfg.HTTP.Do(httpReq)
		if err != nil {
			return nil, &ProviderError{Class: classifyStatus(0), Err: err}
		}
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, &ProviderError{
			Class: classifyStatus(resp.StatusCode),
			Err:   fmt.Errorf("网关 %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet))),
		}
	}
	events := make(chan Event, 64)
	go func() {
		defer close(events)
		defer resp.Body.Close()
		p.streamBody(ctx, resp.Body, events)
	}()
	return events, nil
}

type anthropicChunk struct {
	Type string `json:"type"`
	// message_start
	Message *struct {
		Usage *struct {
			InputTokens int64 `json:"input_tokens"`
		} `json:"usage"`
	} `json:"message"`
	// content_block_start
	Index        int `json:"index"`
	ContentBlock *struct {
		Type string `json:"type"`
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"content_block"`
	// content_block_delta AND message_delta both carry a "delta" object
	Delta *struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		Thinking    string `json:"thinking"`
		PartialJSON string `json:"partial_json"`
		StopReason  string `json:"stop_reason"`
	} `json:"delta"`
	// message_delta usage (carries the authoritative totals on GLM)
	Usage *struct {
		InputTokens  int64 `json:"input_tokens"`
		OutputTokens int64 `json:"output_tokens"`
	} `json:"usage"`
	// error frames
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// streamBody parses Anthropic SSE into normalized Events. Malformed frames
// surface as errors instead of silent truncation.
func (p *AnthropicProvider) streamBody(ctx context.Context, body io.Reader, events chan<- Event) {
	toolMeta := map[int]Event{} // block index → tool identity
	toolArgs := map[int]*strings.Builder{}
	inputTokens := int64(-1)
	reader := bufio.NewReader(body)
	for {
		line, readErr := reader.ReadString('\n')
		if line == "" && readErr != nil {
			if ctx.Err() != nil {
				events <- Event{Type: "error", Err: &ProviderError{Class: ErrorCancelled, Err: ctx.Err()}}
			} else if readErr != io.EOF {
				events <- Event{Type: "error", Err: &ProviderError{Class: ErrorTransient, Err: readErr}}
			}
			return
		}
		line = strings.TrimRight(line, "\r\n")
		if !strings.HasPrefix(line, "data: ") {
			if readErr != nil {
				if ctx.Err() != nil {
					events <- Event{Type: "error", Err: &ProviderError{Class: ErrorCancelled, Err: ctx.Err()}}
				}
				return
			}
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		var chunk anthropicChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			events <- Event{Type: "error", Err: &ProviderError{Class: ErrorInvalidRequest,
				Err: fmt.Errorf("网关返回无效 JSON 帧: %w", err)}}
			return
		}
		switch chunk.Type {
		case "message_start":
			if chunk.Message != nil && chunk.Message.Usage != nil {
				inputTokens = chunk.Message.Usage.InputTokens
			}
		case "content_block_start":
			if chunk.ContentBlock != nil && chunk.ContentBlock.Type == "tool_use" {
				toolMeta[chunk.Index] = Event{ToolCallID: chunk.ContentBlock.ID, ToolName: chunk.ContentBlock.Name}
				toolArgs[chunk.Index] = &strings.Builder{}
				events <- Event{Type: "toolCallDelta", ToolCallID: chunk.ContentBlock.ID}
			}
		case "content_block_delta":
			if chunk.Delta == nil {
				continue
			}
			switch chunk.Delta.Type {
			case "text_delta":
				events <- Event{Type: "textDelta", TextDelta: chunk.Delta.Text}
			case "thinking_delta", "signature_delta":
				// Private reasoning chain: consumed for tokens, never surfaced.
			case "input_json_delta":
				meta := toolMeta[chunk.Index]
				if toolArgs[chunk.Index] == nil {
					toolArgs[chunk.Index] = &strings.Builder{}
				}
				toolArgs[chunk.Index].WriteString(chunk.Delta.PartialJSON)
				events <- Event{Type: "toolCallDelta", ToolCallID: meta.ToolCallID,
					ArgsDelta: chunk.Delta.PartialJSON}
			}
		case "content_block_stop":
			if meta, ok := toolMeta[chunk.Index]; ok {
				args := toolArgs[chunk.Index]
				events <- Event{Type: "toolCallReady", ToolCallID: meta.ToolCallID,
					ToolName: meta.ToolName, ArgsJSON: args.String()}
				delete(toolMeta, chunk.Index)
				delete(toolArgs, chunk.Index)
			}
		case "message_delta":
			if chunk.Usage != nil {
				// GLM reports input_tokens=0 in message_start; the authoritative
				// value arrives here. Prefer the larger of the two.
				in := chunk.Usage.InputTokens
				if in < inputTokens {
					in = inputTokens
				}
				events <- Event{Type: "usage", InputTokens: in, OutputTokens: chunk.Usage.OutputTokens}
			}
			if chunk.Delta != nil && chunk.Delta.StopReason == "tool_use" {
				// Tool calls already flushed at content_block_stop.
			}
		case "message_stop":
			events <- Event{Type: "finish"}
			return
		case "error":
			msg := "网关流错误"
			class := ErrorTransient
			if chunk.Error != nil {
				msg = chunk.Error.Message
				switch chunk.Error.Type {
				case "overloaded_error":
					class = ErrorTransient
				case "invalid_request_error":
					class = ErrorInvalidRequest
				case "authentication_error":
					class = ErrorAuth
				case "rate_limit_error":
					class = ErrorQuota
				}
			}
			events <- Event{Type: "error", Err: &ProviderError{Class: class, Err: fmt.Errorf("%s", msg)}}
			return
		}
	}
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
