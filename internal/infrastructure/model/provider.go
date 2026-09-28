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
	"strings"
	"time"
)

// Event is the normalized stream event (05 §2).
type Event struct {
	Type        string // textDelta | toolCallDelta | toolCallReady | usage | finish | error
	TextDelta   string
	ToolCallID  string
	ToolName    string
	ArgsDelta   string
	ArgsJSON    string
	InputTokens int64
	OutputTokens int64
	FinishReason string
	Err         error
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
	Name         string         `json:"name"`
	SchemaVersion int            `json:"schema_version"`
	InputSchema  map[string]any `json:"input_schema"`
}

// Request is one model call.
type Request struct {
	Model         string
	Messages      []Message
	Tools         []ToolSchema
	MaxOutputTokens int64
	Temperature   float64
}

// Provider streams model events. Implementations must never log secrets.
type Provider interface {
	Stream(ctx context.Context, req Request) (<-chan Event, error)
}

// OpenAIConfig carries the operator-configured entry (02 §8): baseUrl,
// apiKey (env-resolved), model name, token limits.
type OpenAIConfig struct {
	BaseURL         string
	APIKey          string
	HTTP            *http.Client
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
	Model    string        `json:"model"`
	Messages []Message     `json:"messages"`
	Tools    []toolWire    `json:"tools,omitempty"`
	Stream   bool          `json:"stream"`
	StreamOptions *streamOptions `json:"stream_options,omitempty"`
	MaxTokens int64         `json:"max_tokens,omitempty"`
	Temperature float64     `json:"temperature,omitempty"`
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
	body := chatRequest{
		Model: req.Model, Messages: req.Messages, Stream: true,
		StreamOptions: &streamOptions{IncludeUsage: true},
		MaxTokens: req.MaxOutputTokens, Temperature: req.Temperature,
	}
	for _, tool := range req.Tools {
		body.Tools = append(body.Tools, toolWire{Type: "function", Function: ToolFunction{
			Name: tool.Name, Parameters: tool.InputSchema,
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
	resp, err := p.cfg.HTTP.Do(httpReq)
	if err != nil {
		return nil, &ProviderError{Class: classifyStatus(0), Err: err}
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

func classifyStatus(status int) ErrorClass {
	switch {
	case status == 0:
		return ErrorTransient
	case status == 401 || status == 403:
		return ErrorAuth
	case status == 400:
		return ErrorInvalidRequest
	case status == 429:
		return ErrorQuota
	case status >= 500:
		return ErrorTransient
	default:
		return ErrorUnknown
	}
}

type sseChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
			FinishReason *string `json:"finish_reason"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
	} `json:"usage"`
}

// streamBody parses gateway SSE; malformed frames surface as error events
// instead of silently truncating (05 §2 受控测试覆盖).
func (p *OpenAIProvider) streamBody(ctx context.Context, body io.Reader, events chan<- Event) {
	toolArgs := map[int]*strings.Builder{}
	toolMeta := map[int]Event{}
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
			// Flush accumulated tool calls before finishing.
			for idx, args := range toolArgs {
				meta := toolMeta[idx]
				events <- Event{Type: "toolCallReady", ToolCallID: meta.ToolCallID,
					ToolName: meta.ToolName, ArgsJSON: args.String()}
			}
			events <- Event{Type: "finish"}
			return
		}
		var chunk sseChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			events <- Event{Type: "error", Err: &ProviderError{Class: ErrorInvalidRequest,
				Err: fmt.Errorf("网关返回无效 JSON 帧: %w", err)}}
			return
		}
		if chunk.Usage != nil {
			events <- Event{Type: "usage", InputTokens: chunk.Usage.PromptTokens,
				OutputTokens: chunk.Usage.CompletionTokens}
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
			if choice.Delta.FinishReason != nil {
				// Gateway finish: flush tools, then wait for possible usage.
				for idx, args := range toolArgs {
					meta := toolMeta[idx]
					events <- Event{Type: "toolCallReady", ToolCallID: meta.ToolCallID,
						ToolName: meta.ToolName, ArgsJSON: args.String()}
					delete(toolArgs, idx)
				}
			}
		}
	}
}
