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
		Description: "调用指定岗位的子Agent进行分析（仅coordinator可用）。提供具体问题与已取得的材料，子Agent依据当前职责给出意见。失败不代表已有岗位结论；预算耗尽或超时须保留缺口，不以重复调用冒充完成。",
		InputSchema: objectSchema([]string{"position", "question"}, map[string]any{
			"position": map[string]any{
				"type":        "string",
				"description": "优先传 list_agent 返回的 identityId。岗位名称仅在本项目恰好一名任职人时可用；同岗多人必须分别使用各自 identityId。",
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
		Effect:     EffectRead,
		AllowedFor: CoordinatorOnly,
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
				return Result{Data: map[string]any{"position": position}}, fmt.Errorf("position analysis %s failed: %w", position, err)
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
