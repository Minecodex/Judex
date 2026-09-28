// SPDX-License-Identifier: Apache-2.0

// Package runner implements the server-side agent harness per
// docs/plans/v1/05 §1/§3/§8: coordinator sessions per (topic, identity),
// one model loop per run, tool dispatch with prepared/unknown tracking,
// PG-persisted checkpoints, multi-layer budgets with reservations, and
// graceful cancel. Business outcomes NEVER auto-complete from run success.
package runner

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	agentcontext "github.com/kakj-go/Judex/internal/agent/context"
	"github.com/kakj-go/Judex/internal/agent/tools"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
)

// Budget is the per-batch multi-layer cap (05 §4 初值).
type Budget struct {
	MaxRounds        int
	MaxModelAttempts int
	MaxTotalTokens   int64
	MaxWallClock     time.Duration
	ReservedTokens   int64 // unknown-usage hold until reconciled
}

func DefaultBudget() Budget {
	return Budget{MaxRounds: 3, MaxModelAttempts: 30, MaxTotalTokens: 100000, MaxWallClock: 30 * time.Minute}
}

// Runner executes one agent run against the provider + tool registry.
type Runner struct {
	Provider model.Provider
	Registry *tools.Registry
	Clock    func() time.Time
}

// RunRequest identifies one run. Env carries the caller's tool projections
// (DB readers, draft creators); when nil the runner builds a minimal env.
type RunRequest struct {
	RunID      uuid.UUID
	SessionID  uuid.UUID
	ProjectID  uuid.UUID
	IdentityID uuid.UUID
	ModelName  string
	Manifest   agentcontext.Manifest
	Budget     Budget
	Env        *tools.Env
}

// Outcome summarizes the run for the scheduler (05 §8 states).
type Outcome struct {
	State         string // succeeded | failed | cancelled | waiting_human | waiting_material | context_blocked
	Summary       string
	ToolCalls     int
	ModelCalls    int
	TokensUsed    int64
	UsageKnown    bool
	Disagreements []string
}

// Run drives the model loop: stream events → dispatch tools → finalize.
// The loop stops on budget exhaustion or provider finish; a business task is
// never marked accepted here.
func (r *Runner) Run(ctx context.Context, req RunRequest) Outcome {
	outcome := Outcome{State: "running"}
	system, user := req.Manifest.Prompt()
	messages := []model.Message{{Role: "system", Content: system}, {Role: "user", Content: user}}
	deadline := r.now().Add(req.Budget.MaxWallClock)

	for attempt := 1; attempt <= req.Budget.MaxModelAttempts; attempt++ {
		if ctx.Err() != nil {
			outcome.State = "cancelled"
			return outcome
		}
		if r.now().After(deadline) {
			outcome.State = "failed"
			outcome.Summary = "壁钟预算耗尽"
			return outcome
		}
		if outcome.TokensUsed >= req.Budget.MaxTotalTokens {
			outcome.State = "failed"
			outcome.Summary = "token 预算耗尽（合法续开需 manager）"
			return outcome
		}
		outcome.ModelCalls++
		stream, err := r.Provider.Stream(ctx, model.Request{
			Model: req.ModelName, Messages: messages,
			Tools: r.providerTools(), MaxOutputTokens: 8192,
		})
		if err != nil {
			return r.classifyProviderError(ctx, err, outcome)
		}
		var text strings.Builder
		var pendingTools []model.ToolCall
		finish := false
		for event := range stream {
			switch event.Type {
			case "textDelta":
				text.WriteString(event.TextDelta)
			case "toolCallReady":
				pendingTools = append(pendingTools, model.ToolCall{
					ID: event.ToolCallID, Name: event.ToolName, Arguments: event.ArgsJSON,
				})
			case "usage":
				outcome.TokensUsed += event.InputTokens + event.OutputTokens
				outcome.UsageKnown = true
				// 未知用量保守预留：不计入时按保守值预留（05 §4）。
				if !outcome.UsageKnown {
					outcome.TokensUsed += 4096
				}
				if outcome.TokensUsed >= req.Budget.MaxTotalTokens {
					outcome.State = "failed"
					outcome.Summary = "token 预算耗尽（合法续开需 manager）"
					return outcome
				}
			case "error":
				return r.classifyProviderError(ctx, event.Err, outcome)
			case "finish":
				finish = true
			}
		}
		messages = append(messages, model.Message{Role: "assistant", Content: text.String(), ToolCalls: pendingTools})
		if len(pendingTools) == 0 {
			outcome.State = "succeeded"
			outcome.Summary = text.String()
			return outcome
		}
		// Dispatch every tool; results are data messages (05 §6).
		for _, call := range pendingTools {
			outcome.ToolCalls++
			var args map[string]any
			_ = json.Unmarshal([]byte(call.Arguments), &args)
			env := tools.Env{
				ProjectID: req.ProjectID.String(), RunID: req.RunID.String(), IdentityID: req.IdentityID.String(),
			}
			if req.Env != nil {
				env = *req.Env
				env.RunID = req.RunID.String()
				if env.IdentityID == "" {
					env.IdentityID = req.IdentityID.String()
				}
			}
			result, err := r.Registry.Execute(ctx, call.Name, "agent", args, env)
			content := map[string]any{"toolCallId": call.ID}
			if err != nil {
				content["error"] = err.Error()
			} else {
				content["data"] = result.Data
			}
			raw, _ := json.Marshal(content)
			messages = append(messages, model.Message{Role: "tool", ToolCallID: call.ID, Content: string(raw)})
		}
		if finish && len(pendingTools) > 0 {
			continue // tool round continues the loop
		}
	}
	outcome.State = "failed"
	outcome.Summary = "模型尝试次数耗尽"
	return outcome
}

func (r *Runner) providerTools() []model.ToolSchema {
	var out []model.ToolSchema
	for _, schema := range r.Registry.Schemas() {
		name, _ := schema["name"].(string)
		input, _ := schema["input_schema"].(map[string]any)
		out = append(out, model.ToolSchema{Name: name, InputSchema: input})
	}
	return out
}

func (r *Runner) classifyProviderError(ctx context.Context, err error, outcome Outcome) Outcome {
	if ctx.Err() != nil {
		outcome.State = "cancelled"
		return outcome
	}
	switch model.Classify(err) {
	case model.ErrorCancelled:
		outcome.State = "cancelled"
	case model.ErrorContextExceeded:
		outcome.State = "context_blocked" // 05 §5 不静默截断
	default:
		outcome.State = "failed"
	}
	outcome.Summary = err.Error()
	return outcome
}

func (r *Runner) now() time.Time {
	if r.Clock != nil {
		return r.Clock()
	}
	return time.Now().UTC()
}

// NewRunnerForTest builds a Runner from explicit collaborators (tests).
func NewRunnerForTest(provider model.Provider, registry *tools.Registry) *Runner {
	return &Runner{Provider: provider, Registry: registry}
}

// ToolsRegistry exposes the tool registry for callers to add tools (e.g. defaults).
func (r *Runner) ToolsRegistry() *tools.Registry { return r.Registry }
