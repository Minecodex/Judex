// SPDX-License-Identifier: Apache-2.0

// Package multiagent implements coordinator + position sub-agent coordination
// (docs/plans/v1/05 §1): the coordinator can invoke position agents via
// call_agent, each runs with its own duty prompt, results flow back for
// synthesis. No free peer-to-peer or recursive delegation.
package multiagent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	agentcontext "github.com/kakj-go/Judex/internal/agent/context"
	"github.com/kakj-go/Judex/internal/agent/runner"
	"github.com/kakj-go/Judex/internal/agent/tools"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
)

// Coordinator orchestrates position sub-agents and synthesizes their results.
type Coordinator struct {
	Runner    *runner.Runner
	ModelName string
	Positions *tools.PositionRegistry
	Provider  model.Provider
}

// CallPositionAgent implements tools.AgentCaller — runs a single position
// agent with its duty prompt and the provided material.
func (c *Coordinator) CallPositionAgent(ctx context.Context, projectID, positionName, question, material string) (string, error) {
	prompt, ok := c.Positions.Prompt(positionName)
	if !ok {
		return "", fmt.Errorf("岗位 %q 不存在", positionName)
	}
	facts := agentcontext.Facts{
		PositionPrompt: prompt,
		NewMaterial:    material,
	}
	if question != "" {
		facts.RecentMessages = []string{"问题：" + question}
	}
	// Sub-agents get their own runner WITHOUT call_agent (no recursion).
	subRunner := runner.NewRunnerForTest(c.Provider, tools.New())
	tools.RegisterDefaults(subRunner.ToolsRegistry())
	outcome := subRunner.Run(ctx, runner.RunRequest{
		RunID:     uuid.New(),
		ProjectID: uuid.MustParse(projectID),
		ModelName: c.ModelName,
		Manifest:  agentcontext.Build(facts),
		Budget:    runner.Budget{MaxRounds: 1, MaxModelAttempts: 6, MaxTotalTokens: 100000, MaxWallClock: 2 * time.Minute},
	})
	if outcome.State != "succeeded" {
		return "", fmt.Errorf("岗位 %q 分析失败: %s", positionName, outcome.Summary)
	}
	return outcome.Summary, nil
}

// AnalyzeWithPositions runs the coordinator which may call position agents
// via the call_agent tool, then returns the coordinator's synthesis.
func (c *Coordinator) AnalyzeWithPositions(ctx context.Context, projectID, material string, positionNames []string) (CoordinatorResult, error) {
	// Build a registry that includes call_agent wired to this coordinator.
	registry := tools.New()
	tools.RegisterDefaults(registry)
	tools.RegisterCallAgent(registry, c)

	// Add position info to the coordinator's context.
	var positionInfo []string
	for _, name := range positionNames {
		prompt, ok := c.Positions.Prompt(name)
		if !ok {
			continue
		}
		positionInfo = append(positionInfo, fmt.Sprintf("岗位「%s」: %s", name, prompt))
	}

	r := runner.NewRunnerForTest(c.Provider, registry)
	facts := agentcontext.Facts{
		PositionPrompt: "你是项目协调者。你可以使用 call_agent 工具调用岗位子Agent进行分析。请先让各岗位分析材料，再汇总他们的意见给出综合结论。",
		NewMaterial:    material,
		WorkFacts:      positionInfo,
	}

	outcome := r.Run(ctx, runner.RunRequest{
		RunID:     uuid.New(),
		ProjectID: uuid.MustParse(projectID),
		ModelName: c.ModelName,
		Manifest:  agentcontext.Build(facts),
		Budget:    runner.Budget{MaxRounds: 1, MaxModelAttempts: 12, MaxTotalTokens: runner.BatchMaxTotalTokens, MaxWallClock: 5 * time.Minute},
	})

	result := CoordinatorResult{
		Summary:    outcome.Summary,
		State:      outcome.State,
		TokensUsed: outcome.TokensUsed,
		ToolCalls:  outcome.ToolCalls,
	}
	if outcome.State != "succeeded" {
		return result, fmt.Errorf("coordinator state=%s: %s", outcome.State, outcome.Summary)
	}
	return result, nil
}

// CoordinatorResult captures the coordinator's synthesis outcome.
type CoordinatorResult struct {
	Summary    string
	State      string
	TokensUsed int64
	ToolCalls  int
}

// HasMultiplePerspectives checks if the summary references multiple positions
// (a heuristic for verifying the coordinator actually called sub-agents).
func (r CoordinatorResult) HasMultiplePerspectives(positionNames []string) bool {
	found := 0
	for _, name := range positionNames {
		if strings.Contains(r.Summary, name) {
			found++
		}
	}
	return found >= 2
}
