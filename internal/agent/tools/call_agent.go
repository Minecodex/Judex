// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// AgentCaller runs a position sub-agent and returns its result (05 §1).
type AgentCaller interface {
	CallPositionAgent(ctx context.Context, projectID, positionName, prompt, material string) (string, error)
}

// RegisterCallAgent installs the coordinator-only tool that invokes position
// sub-agents (05 §6: 只有 coordinator，无递归自由互调).
func RegisterCallAgent(r *Registry, caller AgentCaller) {
	r.Register(Tool{
		Name:        "call_agent",
		Description: "调用指定岗位的子Agent进行分析（仅coordinator可用）。子Agent将基于其岗位职责和提供的材料给出分析意见。",
		InputSchema: objectSchema([]string{"position", "question"}, map[string]any{
			"position": map[string]any{
				"type":        "string",
				"description": "岗位名称，如：后端工程师、质量验收",
			},
			"question": map[string]any{
				"type":        "string",
				"description": "要子Agent分析的问题或材料",
			},
			"material": map[string]any{
				"type":        "string",
				"description": "可选：附带给子Agent的材料内容",
			},
		}),
		Effect: EffectRead,
		AllowedFor: func(principalKind string) bool {
			return principalKind == "agent" // only agents can call sub-agents
		},
		Execute: func(ctx context.Context, args map[string]any, env Env) (Result, error) {
			if caller == nil {
				return Result{}, fmt.Errorf("子Agent调用未接线")
			}
			position, _ := args["position"].(string)
			question, _ := args["question"].(string)
			material, _ := args["material"].(string)
			if position == "" || question == "" {
				return Result{}, fmt.Errorf("position 和 question 为必填")
			}
			// Timeout the sub-agent call.
			callCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()
			answer, err := caller.CallPositionAgent(callCtx, env.ProjectID, position, question, material)
			if err != nil {
				return Result{Data: map[string]any{"position": position, "error": err.Error()}}, nil
			}
			return Result{Data: map[string]any{
				"position": position,
				"answer":   answer,
			}}, nil
		},
	})
}

// PositionRegistry resolves position names to their prompts.
type PositionRegistry struct {
	positions map[string]string // name → duty prompt
}

func NewPositionRegistry() *PositionRegistry {
	return &PositionRegistry{positions: map[string]string{}}
}

func (r *PositionRegistry) Register(name, prompt string) {
	r.positions[strings.TrimSpace(name)] = prompt
}

func (r *PositionRegistry) Prompt(name string) (string, bool) {
	p, ok := r.positions[strings.TrimSpace(name)]
	return p, ok
}

func (r *PositionRegistry) Names() []string {
	var out []string
	for name := range r.positions {
		out = append(out, name)
	}
	return out
}
