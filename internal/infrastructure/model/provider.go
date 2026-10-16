// SPDX-License-Identifier: Apache-2.0

// Package model implements the OpenAI-compatible streaming ModelProvider
// per docs/plans/v1/05 §2: normalized events, error classification, usage
// accounting (unknown usage is NOT zero cost), and contract tests against a
// controlled fake gateway.
package model

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Event is the normalized stream event (05 §2).
type Event struct {
	Type         string // textDelta | toolCallDelta | toolCallReady | usage | finish | error
	TextDelta    string
	ToolCallID   string
	ToolName     string
	ArgsDelta    string
	ArgsJSON     string
	Usage        UsageSnapshot
	FinishReason string
	Err          error
}

// ErrorClass classifies request failures (05 §2) for retry/budget logic.
type ErrorClass int

const (
	ErrorTransient ErrorClass = iota
	ErrorQuota
	ErrorAuth
	ErrorInvalidRequest
	ErrorContextExceeded
	ErrorCancelled
	ErrorUnknown
	ErrorRateLimited
	ErrorOutputTruncated
)

// ProviderError carries the class for retry decisions.
type ProviderError struct {
	Class ErrorClass
	Err   error
}

func (e *ProviderError) Error() string { return e.Err.Error() }
func (e *ProviderError) Unwrap() error { return e.Err }

func Classify(err error) ErrorClass {
	var pe *ProviderError
	if errors.As(err, &pe) {
		return pe.Class
	}
	return ErrorUnknown
}

// Message is one chat message.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

// ToolCall is a model-requested tool invocation.
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolSchema describes a tool for the request (05 §6).
type ToolSchema struct {
	Description   string         `json:"description,omitempty"`
	Name          string         `json:"name"`
	SchemaVersion int            `json:"schema_version"`
	InputSchema   map[string]any `json:"input_schema"`
}

// Request is one model call.
type Request struct {
	Model           string
	Messages        []Message
	Tools           []ToolSchema
	MaxOutputTokens int64
	Temperature     float64
	Options         RequestOptions
}

// Provider streams model events. Implementations must never log secrets.
type Provider interface {
	Stream(ctx context.Context, req Request) (<-chan Event, error)
}

// OpenAIConfig carries the operator-configured entry (02 §8): baseUrl,
// apiKey (env-resolved), model name, token limits.
type OpenAIConfig struct {
	BaseURL        string
	APIKey         string
	HTTP           *http.Client
	RequestOptions RequestOptions
}

type OpenAIProvider struct {
	cfg OpenAIConfig
}

func NewOpenAIProvider(cfg OpenAIConfig) *OpenAIProvider {
	if cfg.HTTP == nil {
		cfg.HTTP = &http.Client{Timeout: 10 * time.Minute}
	}
	return &OpenAIProvider{cfg: cfg}
}

type chatRequest struct {
	Model           string         `json:"model"`
	Messages        []wireMessage  `json:"messages"`
	Tools           []toolWire     `json:"tools,omitempty"`
	Stream          bool           `json:"stream"`
	StreamOptions   *streamOptions `json:"stream_options,omitempty"`
	MaxTokens       int64          `json:"max_tokens,omitempty"`
	Temperature     float64        `json:"temperature,omitempty"`
	Thinking        *thinkingWire  `json:"thinking,omitempty"`
	ReasoningEffort string         `json:"reasoning_effort,omitempty"`
}

type thinkingWire struct {
	Type string `json:"type"`
}

// wireMessage serializes assistant tool calls in the OpenAI nested shape
// [{id, type:"function", function:{name, arguments}}] — sending the flat
// neutral shape is rejected by strict gateways (GLM error 1214).
type wireMessage struct {
	Role       string         `json:"role"`
	Content    any            `json:"content"` // string; nil when tool_calls present
	ToolCallID string         `json:"tool_call_id,omitempty"`
	ToolCalls  []wireToolCall `json:"tool_calls,omitempty"`
}

type wireToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"` // always "function"
	Function wireToolFunction `json:"function"`
}

type wireToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

func toWireMessages(messages []Message) []wireMessage {
	out := make([]wireMessage, 0, len(messages))
	for _, msg := range messages {
		wire := wireMessage{Role: msg.Role, ToolCallID: msg.ToolCallID}
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			wire.Content = nil
			for _, call := range msg.ToolCalls {
				wire.ToolCalls = append(wire.ToolCalls, wireToolCall{
					ID: call.ID, Type: "function",
					Function: wireToolFunction{Name: call.Name, Arguments: call.Arguments},
				})
			}
		} else {
			wire.Content = msg.Content
		}
		out = append(out, wire)
	}
	return out
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type toolWire struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters"`
}

// Stream opens a streaming chat completion and normalizes SSE into Events.
func (p *OpenAIProvider) Stream(ctx context.Context, req Request) (<-chan Event, error) {
	options := req.Options
	if options.ThinkingType == "" && options.ReasoningEffort == "" {
		options = p.cfg.RequestOptions
	}
	if err := options.Validate(ProtocolOpenAI); err != nil {
		return nil, err
	}
	body := chatRequest{
		Model: req.Model, Messages: toWireMessages(req.Messages), Stream: true,
		StreamOptions: &streamOptions{IncludeUsage: true},
		MaxTokens:     req.MaxOutputTokens, Temperature: req.Temperature,
	}
	if options.ThinkingType != "" {
		body.Thinking = &thinkingWire{Type: options.ThinkingType}
	}
	body.ReasoningEffort = options.ReasoningEffort
	for _, tool := range req.Tools {
		body.Tools = append(body.Tools, toolWire{Type: "function", Function: ToolFunction{
			Name: tool.Name, Description: tool.Description, Parameters: tool.InputSchema,
		}})
	}
	raw, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		strings.TrimSuffix(p.cfg.BaseURL, "/")+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	// Transient connection failures (e.g. proxy route establishment) get one
	// immediate retry; the request body is replayed from the original bytes.
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
			Class: classifyGatewayError(resp.StatusCode, snippet),
			Err:   fmt.Errorf("网关 %d: %s", resp.StatusCode, strings.ReplaceAll(strings.TrimSpace(string(snippet)), p.cfg.APIKey, "[redacted]")),
		}
	}
	events := make(chan Event, 64)
	go func() {
		defer close(events)
		defer resp.Body.Close()
		p.streamBody(ctx, resp.Body, events, req.Model)
	}()
	return events, nil
}

func classifyStatus(status int) ErrorClass {
	switch {
	case status == 0:
		return ErrorTransient
	case status == 401 || status == 403:
		return ErrorAuth
	case status == 400:
		return ErrorInvalidRequest
	case status == 429:
		return ErrorRateLimited
	case status >= 500:
		return ErrorTransient
	default:
		return ErrorUnknown
	}
}

type sseChunk struct {
	Error     json.RawMessage `json:"error"`
	ID        string          `json:"id"`
	RequestID string          `json:"request_id"`
	Model     string          `json:"model"`
	Choices   []struct {
		FinishReason *string `json:"finish_reason"`
		Delta        struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     *int64 `json:"prompt_tokens"`
		CompletionTokens *int64 `json:"completion_tokens"`
		PromptDetails    *struct {
			CachedTokens        *int64 `json:"cached_tokens"`
			CacheWriteTokens    *int64 `json:"cache_write_tokens"`
			CacheCreationTokens *int64 `json:"cache_creation_tokens"`
		} `json:"prompt_tokens_details"`
	} `json:"usage"`
}

// streamBody parses gateway SSE; malformed frames surface as error events
// instead of silently truncating (05 §2 受控测试覆盖).
func (p *OpenAIProvider) streamBody(ctx context.Context, body io.Reader, events chan<- Event, modelName string) {
	toolArgs := map[int]*strings.Builder{}
	toolMeta := map[int]Event{}
	usage := UsageSnapshot{ModelName: modelName}
	finishReason := ""
	flushTools := func() {
		indices := make([]int, 0, len(toolArgs))
		for index := range toolArgs {
			indices = append(indices, index)
		}
		sort.Ints(indices)
		for _, index := range indices {
			meta := toolMeta[index]
			events <- Event{Type: "toolCallReady", ToolCallID: meta.ToolCallID,
				ToolName: meta.ToolName, ArgsJSON: toolArgs[index].String()}
			delete(toolArgs, index)
			delete(toolMeta, index)
		}
	}
	reader := bufio.NewReader(body)
	for {
		line, err := reader.ReadString('\n')
		if line == "" && err != nil {
			if ctx.Err() != nil {
				events <- Event{Type: "error", Err: &ProviderError{Class: ErrorCancelled, Err: ctx.Err()}}
			} else if err != io.EOF {
				events <- Event{Type: "error", Err: &ProviderError{Class: ErrorTransient, Err: err}}
			}
			return
		}
		line = strings.TrimRight(line, "\r\n")
		if !strings.HasPrefix(line, "data: ") {
			if err != nil {
				if ctx.Err() != nil {
					events <- Event{Type: "error", Err: &ProviderError{Class: ErrorCancelled, Err: ctx.Err()}}
				}
				return
			}
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			if finishReason == "length" {
				events <- Event{Type: "error", Err: &ProviderError{Class: ErrorOutputTruncated, Err: fmt.Errorf("model output truncated at max output tokens; incomplete tools were not executed")}}
				return
			}
			// Flush accumulated tool calls before finishing.
			flushTools()
			events <- Event{Type: "finish", FinishReason: finishReason}
			return
		}
		var chunk sseChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			events <- Event{Type: "error", Err: &ProviderError{Class: ErrorInvalidRequest,
				Err: fmt.Errorf("网关返回无效 JSON 帧: %w", err)}}
			return
		}
		if len(chunk.Error) > 0 && string(chunk.Error) != "null" {
			events <- Event{Type: "error", Err: &ProviderError{Class: classifyGatewayError(0, []byte(payload)), Err: fmt.Errorf("provider returned a stream error")}}
			return
		}
		if chunk.ID != "" {
			usage.ProviderRequestID = chunk.ID
		}
		if chunk.RequestID != "" {
			usage.ProviderRequestID = chunk.RequestID
		}
		if chunk.Model != "" {
			usage.ModelName = chunk.Model
		}
		if chunk.Usage != nil {
			updateCounter(&usage.InputTokens, chunk.Usage.PromptTokens)
			updateCounter(&usage.OutputTokens, chunk.Usage.CompletionTokens)
			if chunk.Usage.PromptDetails != nil {
				updateCounter(&usage.CacheReadTokens, chunk.Usage.PromptDetails.CachedTokens)
				details := chunk.Usage.PromptDetails
				if details.CacheWriteTokens != nil || details.CacheCreationTokens != nil {
					usage.CacheWriteTokens = cacheWriteCounter(details.CacheWriteTokens, details.CacheCreationTokens)
				}
			}
			events <- Event{Type: "usage", Usage: usage.Validated()}
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				events <- Event{Type: "textDelta", TextDelta: choice.Delta.Content}
			}
			for _, call := range choice.Delta.ToolCalls {
				if call.ID != "" {
					toolMeta[call.Index] = Event{ToolCallID: call.ID, ToolName: call.Function.Name}
					toolArgs[call.Index] = &strings.Builder{}
				}
				if call.Function.Arguments != "" {
					if toolArgs[call.Index] == nil {
						toolArgs[call.Index] = &strings.Builder{}
					}
					toolArgs[call.Index].WriteString(call.Function.Arguments)
					events <- Event{Type: "toolCallDelta", ToolCallID: toolMeta[call.Index].ToolCallID,
						ArgsDelta: call.Function.Arguments}
				}
			}
			if choice.FinishReason != nil {
				finishReason = *choice.FinishReason
				// Gateway finish: flush tools, then wait for possible usage.
				if finishReason != "length" {
					flushTools()
				}
			}
		}
	}
}
