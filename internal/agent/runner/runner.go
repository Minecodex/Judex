// SPDX-License-Identifier: Apache-2.0

// Package runner implements the server-side agent harness per
// docs/plans/v1/05 §1/§3/§8: coordinator sessions per (topic, identity),
// one model loop per run, tool dispatch with prepared/unknown tracking,
// PG-persisted checkpoints, multi-layer budgets with reservations, and
// graceful cancel. Business outcomes NEVER auto-complete from run success.
package runner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	agentcontext "github.com/kakj-go/Judex/internal/agent/context"
	"github.com/kakj-go/Judex/internal/agent/tools"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
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
	RequireAnalysisRecord bool
	Lease                 string
	History               []model.Message
	StartAttempt          int
	Prepare               func(context.Context) (CallContext, error)
	RunID                 uuid.UUID
	SessionID             uuid.UUID
	ProjectID             uuid.UUID
	IdentityID            uuid.UUID
	ModelName             string
	PrincipalKind         string
	CanManage             bool
	Manifest              agentcontext.Manifest
	Budget                Budget
	Env                   *tools.Env
	Journal               Journal
	MaxInputTokens        int64
	MaxOutputTokens       int64
}

// CallContext is refreshed before every model request, including after tools.
type CallContext struct {
	CanManage                       bool
	PrincipalKind                   string
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
	analysisCorrectionSent := false
	if req.Budget.MaxWallClock <= 0 {
		req.Budget.MaxWallClock = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, req.Budget.MaxWallClock)
	defer cancel()
	if req.MaxOutputTokens <= 0 {
		req.MaxOutputTokens = 8192
	}
	fail := func(err error) Outcome {
		outcome.State = "failed"
		outcome.Summary = err.Error()
		if req.RequireAnalysisRecord {
			outcome.Summary = "模型未保存结构化公开分析：" + outcome.Summary
		}
		return outcome
	}
	system, user := req.Manifest.Prompt()
	privateOutput := agentcontext.PublicOutputGuard{}
	privateOutput.Add(req.Manifest)
	messages := []model.Message{{Role: "system", Content: system}, {Role: "user", Content: user}}
	messages = append(messages, req.History...)
	if req.StartAttempt < 1 {
		req.StartAttempt = 1
	}
	deadline := r.now().Add(req.Budget.MaxWallClock)
	quotaExhausted := map[string]bool{}
	configuredProvider := r.Provider

attempts:
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
			privateOutput.Add(fresh.Manifest)
			req.CanManage = fresh.CanManage
			if fresh.PrincipalKind != "" {
				req.PrincipalKind = fresh.PrincipalKind
			}
			req.ModelName = fresh.ModelName
			req.MaxInputTokens = fresh.MaxInputTokens
			req.MaxOutputTokens = fresh.MaxOutputTokens
			if fresh.Provider != nil {
				configuredProvider = fresh.Provider
			}
			system, user = req.Manifest.Prompt()
			messages[0] = model.Message{Role: "system", Content: system}
			messages[1] = model.Message{Role: "user", Content: user}
		}
		r.Provider = configuredProvider
		var candidates []model.Target
		if policy, ok := configuredProvider.(model.QuotaCandidates); ok {
			candidates = policy.QuotaTargets()
			for _, target := range candidates {
				if !quotaExhausted[target.Name] {
					r.Provider, req.ModelName = target.Provider, target.Name
					req.MaxInputTokens, req.MaxOutputTokens = target.MaxInputTokens, target.MaxOutputTokens
					break
				}
			}
		}
		canFallback := func(err error) bool {
			if model.Classify(err) != model.ErrorQuota || ctx.Err() != nil {
				return false
			}
			quotaExhausted[req.ModelName] = true
			for _, target := range candidates {
				if !quotaExhausted[target.Name] {
					return true
				}
			}
			return false
		}
		schemas := r.providerTools(req.PrincipalKind, req.CanManage)
		remaining := req.Budget.MaxTotalTokens - outcome.TokensUsed
		attemptsLeft := req.Budget.MaxModelAttempts - attempt + 1
		if budget, ok := req.Journal.(BudgetReader); ok {
			sharedTokens, sharedAttempts, err := budget.RemainingBudget(ctx)
			if err != nil {
				return fail(err)
			}
			remaining = min(remaining, sharedTokens)
			attemptsLeft = min(attemptsLeft, sharedAttempts)
		}
		if remaining <= req.MaxOutputTokens || attemptsLeft <= 0 {
			return fail(apierrors.New(apierrors.BudgetExhausted, "batch model/token budget exhausted"))
		}
		_, user = req.Manifest.Prompt()
		messages[1].Content = user
		finalizeOnly := remaining < 2*(RequestBytes(messages, schemas)+req.MaxOutputTokens) || attemptsLeft <= 1
		if finalizeOnly {
			messages[1].Content += fmt.Sprintf("\n运行预算提示：当前最多可预留 %d token，最多剩余 %d 次尝试（含同批次岗位），按现有上下文预留只能保证本次收尾调用。依据已经实际读取的来源总结；没核对的内容必须作为缺口保留，不再探测或重跑成员测试。岗位本次直接返回公开意见；协调者仅调用 record_task_analysis 保存必要摘要与来源。此提示不代表材料已核对或业务已通过。", remaining, attemptsLeft)
			finalTools := []model.ToolSchema{}
			for _, schema := range schemas {
				if req.PrincipalKind == "coordinator" && schema.Name == "record_task_analysis" {
					finalTools = append(finalTools, schema)
				}
			}
			schemas = finalTools
		}
		contextLimit := remaining - req.MaxOutputTokens
		if req.MaxInputTokens > 0 {
			contextLimit = min(contextLimit, req.MaxInputTokens)
		}
		if req.Journal != nil {
			compacted, changed := Compact(messages, req.RunID.String(), contextLimit, schemas)
			if changed {
				messages = compacted
				if err := req.Journal.Checkpoint(ctx, messages); err != nil {
					return fail(err)
				}
			}
		}
		outcome.ModelCalls++
		request := model.Request{Model: req.ModelName, Messages: messages, Tools: schemas, MaxOutputTokens: req.MaxOutputTokens}
		if configured, ok := r.Provider.(model.ConfiguredOptions); ok {
			request.Options = configured.RequestOptions()
		}
		inputBytes := RequestBytes(messages, schemas)
		if req.MaxInputTokens > 0 && inputBytes > req.MaxInputTokens {
			outcome.State = "context_blocked"
			outcome.Summary = "context requires compaction before another model call"
			return outcome
		}
		if inputBytes+req.MaxOutputTokens > remaining {
			return fail(apierrors.New(apierrors.BudgetExhausted, "protected context cannot fit remaining model/token budget"))
		}
		if req.Journal != nil {
			if err := req.Journal.ModelStart(ctx, attempt, request); err != nil {
				return fail(err)
			}
		}
		callCtx, cancelCall := context.WithCancel(ctx)
		defer cancelCall()
		stream, err := r.Provider.Stream(callCtx, request)
		if err != nil {
			outcome.UsageKnown = false
			outcome.TokensUsed += req.MaxOutputTokens + inputBytes
			if req.Journal != nil {
				if journalErr := req.Journal.ModelEnd(context.WithoutCancel(ctx), attempt, model.UsageSnapshot{ModelName: req.ModelName}, false, err); journalErr != nil {
					return fail(journalErr)
				}
			}
			if canFallback(err) {
				cancelCall()
				continue attempts
			}
			return r.classifyProviderError(ctx, err, outcome)
		}
		callUsage := int64(0)
		usage := model.UsageSnapshot{ModelName: req.ModelName}
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
				usage = event.Usage.Validated()
				if usage.ModelName == "" {
					usage.ModelName = req.ModelName
				}
			case "error":
				known := model.Classify(event.Err) == model.ErrorOutputTruncated && usage.Known()
				used := req.MaxOutputTokens + inputBytes
				if known {
					used = usage.Total()
				} else {
					outcome.UsageKnown = false
				}
				outcome.TokensUsed += used
				if req.Journal != nil {
					if journalErr := req.Journal.ModelEnd(context.WithoutCancel(ctx), attempt, usage, known, event.Err); journalErr != nil {
						return fail(journalErr)
					}
				}
				if canFallback(event.Err) {
					cancelCall()
					continue attempts
				}
				return r.classifyProviderError(ctx, event.Err, outcome)
			case "finish":
				finish = true
			}
		}
		cancelCall()
		if !finish {
			outcome.UsageKnown = false
			outcome.TokensUsed += req.MaxOutputTokens + inputBytes
			err := &model.ProviderError{Class: model.ErrorTransient, Err: fmt.Errorf("model stream ended without finish")}
			if req.Journal != nil {
				_ = req.Journal.ModelEnd(context.WithoutCancel(ctx), attempt, usage, false, err)
			}
			return r.classifyProviderError(ctx, err, outcome)
		}
		usageKnown := usage.Known()
		callUsage = usage.Total()
		if !usageKnown {
			callUsage = req.MaxOutputTokens + inputBytes
			outcome.UsageKnown = false
		}
		outcome.TokensUsed += callUsage
		if req.Journal != nil {
			if err := req.Journal.ModelEnd(context.WithoutCancel(ctx), attempt, usage, usageKnown, nil); err != nil {
				return fail(err)
			}
		}
		if outcome.TokensUsed >= req.Budget.MaxTotalTokens {
			outcome.State = "failed"
			outcome.Summary = "token budget exhausted"
			return outcome
		}
		// Account for a successful provider response first, then reject private
		// echoes before checkpointing, dispatching tools or returning an opinion.
		// The rejected content is never included in the error or durable journal.
		if err := privateOutput.Check(text.String()); err != nil {
			return fail(err)
		}
		for _, call := range pendingTools {
			if err := privateOutput.CheckJSON(call.Arguments); err != nil {
				return fail(err)
			}
		}
		messages = append(messages, model.Message{Role: "assistant", Content: text.String(), ToolCalls: pendingTools})
		if req.Journal != nil {
			if err := req.Journal.Checkpoint(ctx, messages); err != nil {
				return fail(err)
			}
		}
		if len(pendingTools) == 0 {
			if req.RequireAnalysisRecord {
				if analysisCorrectionSent {
					return fail(apierrors.Fields("summary", "结构化公开分析未保存；原记录保留，可在原预算内显式重试"))
				}
				analysisCorrectionSent = true
				messages = append(messages, model.Message{Role: "user", Content: "当前回答尚未通过 record_task_analysis 保存，不能作为公开分析完成。只将实际已读来源、结论、异议和未覆盖项存入该工具；信息不足可明确 needsClarification，别重跑测试或新增工作。必须纠正工具校验错误，不能把口头总结当作保存成功。"})
				if req.Journal != nil {
					if err := req.Journal.Checkpoint(ctx, messages); err != nil {
						return fail(err)
					}
				}
				continue
			}
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
		if finalizeOnly {
			for _, call := range pendingTools {
				if req.PrincipalKind != "coordinator" || call.Name != "record_task_analysis" {
					return fail(apierrors.New(apierrors.BudgetExhausted, "remaining budget permits analysis finalization only"))
				}
			}
		}
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
		if summary, complete := completedAnalysis(pendingTools, results); complete {
			outcome.State, outcome.Summary = "succeeded", summary
			return outcome
		}
		if finish && len(pendingTools) > 0 {
			continue // tool round continues the loop
		}
	}
	outcome.State = "failed"
	outcome.Summary = "模型尝试次数耗尽"
	return outcome
}

func (r *Runner) providerTools(principalKind string, canManage bool) []model.ToolSchema {
	var out []model.ToolSchema
	if principalKind == "" {
		principalKind = "agent"
	}
	for _, schema := range r.Registry.SchemasForAuthority(principalKind, canManage) {
		name, _ := schema["name"].(string)
		input, _ := schema["input_schema"].(map[string]any)
		description, _ := schema["description"].(string)
		version, _ := schema["schema_version"].(int)
		out = append(out, model.ToolSchema{Name: name, SchemaVersion: version, Description: description, InputSchema: input})
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
