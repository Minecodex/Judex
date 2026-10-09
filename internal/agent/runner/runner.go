// SPDX-License-Identifier: Apache-2.0

// Package runner implements the server-side agent harness per
// docs/plans/v1/05 §1/§3/§8: coordinator sessions per (topic, identity),
// one model loop per run, tool dispatch with prepared/unknown tracking,
// PG-persisted checkpoints, multi-layer budgets with reservations, and
// graceful cancel. Business outcomes NEVER auto-complete from run success.
package runner

import (
	"context"
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

const (
	BatchMaxModelAttempts       = 30
	BatchMaxTotalTokens   int64 = 300000
	MaxParallelChildren         = 2
)

func DefaultBudget() Budget {
	return Budget{MaxRounds: 3, MaxModelAttempts: BatchMaxModelAttempts, MaxTotalTokens: BatchMaxTotalTokens, MaxWallClock: 30 * time.Minute}
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
	Lease           string
	History         []model.Message
	StartAttempt    int
	Prepare         func(context.Context) (CallContext, error)
	RunID           uuid.UUID
	SessionID       uuid.UUID
	ProjectID       uuid.UUID
	IdentityID      uuid.UUID
	ModelName       string
	Manifest        agentcontext.Manifest
	Budget          Budget
	Env             *tools.Env
	Journal         Journal
	MaxInputTokens  int64
	MaxOutputTokens int64
}

// CallContext is refreshed before every model request, including after tools.
type CallContext struct {
	Manifest                        agentcontext.Manifest
	Provider                        model.Provider
	ModelName                       string
	MaxInputTokens, MaxOutputTokens int64
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
	outcome := Outcome{State: "running", UsageKnown: true}
	if req.Budget.MaxWallClock <= 0 {
		req.Budget.MaxWallClock = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, req.Budget.MaxWallClock)
	defer cancel()
	if req.MaxOutputTokens <= 0 {
		req.MaxOutputTokens = 8192
	}
	fail := func(err error) Outcome { outcome.State = "failed"; outcome.Summary = err.Error(); return outcome }
	system, user := req.Manifest.Prompt()
	messages := []model.Message{{Role: "system", Content: system}, {Role: "user", Content: user}}
	messages = append(messages, req.History...)
	if req.StartAttempt < 1 {
		req.StartAttempt = 1
	}
	deadline := r.now().Add(req.Budget.MaxWallClock)

	for attempt := req.StartAttempt; attempt <= req.Budget.MaxModelAttempts; attempt++ {
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
		if req.Prepare != nil {
			fresh, err := req.Prepare(ctx)
			if err != nil {
				return fail(err)
			}
			req.Manifest = fresh.Manifest
			req.ModelName = fresh.ModelName
			req.MaxInputTokens = fresh.MaxInputTokens
			req.MaxOutputTokens = fresh.MaxOutputTokens
			if fresh.Provider != nil {
				r.Provider = fresh.Provider
			}
			system, user = req.Manifest.Prompt()
			messages[0] = model.Message{Role: "system", Content: system}
			messages[1] = model.Message{Role: "user", Content: user}
		}
		schemas := r.providerTools()
		if req.MaxInputTokens > 0 && req.Journal != nil {
			compacted, changed := Compact(messages, req.RunID.String(), req.MaxInputTokens, schemas)
			if changed {
				messages = compacted
				if err := req.Journal.Checkpoint(ctx, messages); err != nil {
					return fail(err)
				}
			}
		}
		outcome.ModelCalls++
		request := model.Request{Model: req.ModelName, Messages: messages, Tools: r.providerTools(), MaxOutputTokens: req.MaxOutputTokens}
		inputBytes := RequestBytes(messages, schemas)
		if req.MaxInputTokens > 0 && inputBytes > req.MaxInputTokens {
			outcome.State = "context_blocked"
			outcome.Summary = "context requires compaction before another model call"
			return outcome
		}
		if req.Journal != nil {
			if err := req.Journal.ModelStart(ctx, attempt, request); err != nil {
				return fail(err)
			}
		}
		stream, err := r.Provider.Stream(ctx, request)
		if err != nil {
			outcome.UsageKnown = false
			outcome.TokensUsed += req.MaxOutputTokens + inputBytes
			if req.Journal != nil {
				_ = req.Journal.ModelEnd(context.WithoutCancel(ctx), attempt, req.MaxOutputTokens+inputBytes, false, err)
			}
			return r.classifyProviderError(ctx, err, outcome)
		}
		callUsage := int64(0)
		usageKnown := false
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
				callUsage = event.InputTokens + event.OutputTokens
				usageKnown = true
			case "error":
				outcome.UsageKnown = false
				outcome.TokensUsed += req.MaxOutputTokens + inputBytes
				if req.Journal != nil {
					_ = req.Journal.ModelEnd(context.WithoutCancel(ctx), attempt, req.MaxOutputTokens+inputBytes, false, event.Err)
				}
				return r.classifyProviderError(ctx, event.Err, outcome)
			case "finish":
				finish = true
			}
		}
		if !usageKnown {
			callUsage = req.MaxOutputTokens + inputBytes
			outcome.UsageKnown = false
		}
		outcome.TokensUsed += callUsage
		if req.Journal != nil {
			if err := req.Journal.ModelEnd(context.WithoutCancel(ctx), attempt, callUsage, usageKnown, nil); err != nil {
				return fail(err)
			}
		}
		if outcome.TokensUsed >= req.Budget.MaxTotalTokens {
			outcome.State = "failed"
			outcome.Summary = "token budget exhausted"
			return outcome
		}
		if !finish && len(pendingTools) == 0 {
			outcome.State = "failed"
			outcome.Summary = "model stream ended without finish"
			return outcome
		}
		messages = append(messages, model.Message{Role: "assistant", Content: text.String(), ToolCalls: pendingTools})
		if req.Journal != nil {
			if err := req.Journal.Checkpoint(ctx, messages); err != nil {
				return fail(err)
			}
		}
		if len(pendingTools) == 0 {
			outcome.State = "succeeded"
			outcome.Summary = text.String()
			return outcome
		}
		if outcome.ToolCalls+len(pendingTools) > 100 {
			outcome.State = "failed"
			outcome.Summary = "tool budget exhausted"
			return outcome
		}
		outcome.ToolCalls += len(pendingTools)
		results, unknown, err := r.executeTools(ctx, req, pendingTools)
		if err != nil {
			return fail(err)
		}
		if unknown {
			outcome.State = "waiting_human"
			outcome.Summary = "tool result unknown; verify before continuing"
			return outcome
		}
		messages = append(messages, results...)
		if req.Journal != nil {
			if err := req.Journal.Checkpoint(ctx, messages); err != nil {
				return fail(err)
			}
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
		description, _ := schema["description"].(string)
		out = append(out, model.ToolSchema{Name: name, Description: description, InputSchema: input})
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
