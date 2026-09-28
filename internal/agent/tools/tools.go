// SPDX-License-Identifier: Apache-2.0

// Package tools registers the model-facing tool set per docs/plans/v1/05 §6:
// sandbox-executed read/write/edit/bash plus server-side business tools.
// Tools declare effectClass and allowedPrincipal; results are data, never
// instructions. No tool grants approval or SQL access.
package tools

import (
	"context"
	"fmt"
	"strings"
)

// EffectClass categorizes side effects for gating and audit.
type EffectClass string

const (
	EffectSandbox EffectClass = "sandbox" // executes only inside the run sandbox
	EffectRead    EffectClass = "read"    // server-side read-only
	EffectDraft   EffectClass = "draft"   // creates drafts, never formal state
)

// Result is tool output: JSON-serializable data only.
type Result struct {
	Data map[string]any
	// Refs point at stored artifacts when output is large (05 §4 1MiB inline).
	Ref string
}

// Tool is one registered capability.
type Tool struct {
	Name          string
	SchemaVersion int
	Description   string
	InputSchema   map[string]any
	Effect        EffectClass
	AllowedFor    func(principalKind string) bool
	Execute       func(ctx context.Context, args map[string]any, env Env) (Result, error)
}

// Env carries the per-run execution context tools may use; it deliberately
// exposes NO credentials and NO write access to formal state.
type Env struct {
	ProjectID  string
	RunID      string
	IdentityID string
	// Sandbox executes read/write/edit/bash (nil → those tools refuse).
	Sandbox SandboxExec
	// ReadMaterialContent lets the live-test stub return injected Chinese
	// material without a real storage round-trip.
	ReadMaterialContent string
	// Queriers are read-only server projections.
	ListAgents   func(ctx context.Context, projectID string) ([]map[string]any, error)
	QueryWork    func(ctx context.Context, projectID string, objectType string, cursor string, limit int) ([]map[string]any, error)
	ReadMaterial func(ctx context.Context, projectID, versionID string) (map[string]any, error)
	ProposeDraft func(ctx context.Context, projectID string, draft map[string]any) (string, error)
}

// SandboxExec is the surface sandbox tools need.
type SandboxExec interface {
	Exec(ctx context.Context, command string, timeoutMs int) (exit int, stdout, stderr []byte, unknown bool, err error)
}

// Registry holds the per-deployment tool set.
type Registry struct {
	tools map[string]Tool
}

func New() *Registry { return &Registry{tools: map[string]Tool{}} }

func (r *Registry) Register(tool Tool) {
	if tool.SchemaVersion == 0 {
		tool.SchemaVersion = 1
	}
	r.tools[tool.Name] = tool
}

// Schemas returns the model-facing tool list.
func (r *Registry) Schemas() []map[string]any {
	var out []map[string]any
	for name := range r.tools {
		tool := r.tools[name]
		out = append(out, map[string]any{
			"name": tool.Name, "schema_version": tool.SchemaVersion,
			"description": tool.Description, "input_schema": tool.InputSchema,
			"effect_class": string(tool.Effect),
		})
	}
	return out
}

// Execute validates principal + schema presence and runs the tool.
func (r *Registry) Execute(ctx context.Context, name string, principalKind string, args map[string]any, env Env) (Result, error) {
	tool, ok := r.tools[name]
	if !ok {
		return Result{}, fmt.Errorf("unknown tool %q", name)
	}
	if tool.AllowedFor != nil && !tool.AllowedFor(principalKind) {
		return Result{}, fmt.Errorf("tool %q 不允许该调用方", name)
	}
	return tool.Execute(ctx, args, env)
}

func objectSchema(required []string, props map[string]any) map[string]any {
	return map[string]any{"type": "object", "required": required, "properties": props,
		"additionalProperties": false}
}

func str(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

func intVal(args map[string]any, key string, fallback int) int {
	if v, ok := args[key].(float64); ok {
		return int(v)
	}
	return fallback
}

// RegisterDefaults installs the full V1 tool set (05 §6 table).
func RegisterDefaults(r *Registry) {
	r.Register(Tool{
		Name:        "read",
		Description: "读取沙箱内文件（只读）",
		InputSchema: objectSchema([]string{"path"}, map[string]any{
			"path": map[string]any{"type": "string"},
		}),
		Effect: EffectSandbox,
		Execute: func(ctx context.Context, args map[string]any, env Env) (Result, error) {
			if env.Sandbox == nil {
				return Result{}, fmt.Errorf("沙箱未挂载")
			}
			exit, stdout, _, unknown, err := env.Sandbox.Exec(ctx, "cat -- "+shellEscape(str(args, "path")), 30000)
			if err != nil || unknown {
				return Result{}, fmt.Errorf("读取结果未知，不盲重试")
			}
			if exit != 0 {
				return Result{}, fmt.Errorf("读取失败 exit=%d", exit)
			}
			return Result{Data: map[string]any{"content": string(stdout)}}, nil
		},
	})
	r.Register(Tool{
		Name:        "bash",
		Description: "在本次运行的沙箱中执行命令（分析用；不授权生产部署）",
		InputSchema: objectSchema([]string{"command"}, map[string]any{
			"command":   map[string]any{"type": "string"},
			"timeoutMs": map[string]any{"type": "integer"},
		}),
		Effect: EffectSandbox,
		Execute: func(ctx context.Context, args map[string]any, env Env) (Result, error) {
			if env.Sandbox == nil {
				return Result{}, fmt.Errorf("沙箱未挂载")
			}
			exit, stdout, stderr, unknown, err := env.Sandbox.Exec(ctx, str(args, "command"), intVal(args, "timeoutMs", 60000))
			if err != nil {
				return Result{}, err
			}
			data := map[string]any{"exitCode": exit, "stdout": string(stdout), "stderr": string(stderr)}
			if unknown {
				data["unknown"] = true
			}
			return Result{Data: data}, nil
		},
	})
	r.Register(Tool{
		Name:        "list_agent",
		Description: "列出本项目可发现身份与公开职责（无个人提示词）",
		InputSchema: objectSchema(nil, nil),
		Effect:      EffectRead,
		Execute: func(ctx context.Context, args map[string]any, env Env) (Result, error) {
			if env.ListAgents == nil {
				return Result{}, fmt.Errorf("未接线")
			}
			agents, err := env.ListAgents(ctx, env.ProjectID)
			if err != nil {
				return Result{}, err
			}
			return Result{Data: map[string]any{"agents": agents}}, nil
		},
	})
	r.Register(Tool{
		Name:        "query_work",
		Description: "分页查询本项目工作事实（plan/task/proposal）",
		InputSchema: objectSchema([]string{"objectType"}, map[string]any{
			"objectType": map[string]any{"type": "string", "enum": []string{"plan", "task", "proposal"}},
			"cursor":     map[string]any{"type": "string"},
			"limit":      map[string]any{"type": "integer"},
		}),
		Effect: EffectRead,
		Execute: func(ctx context.Context, args map[string]any, env Env) (Result, error) {
			if env.QueryWork == nil {
				return Result{}, fmt.Errorf("未接线")
			}
			items, err := env.QueryWork(ctx, env.ProjectID, str(args, "objectType"), str(args, "cursor"), intVal(args, "limit", 20))
			if err != nil {
				return Result{}, err
			}
			return Result{Data: map[string]any{"items": items}}, nil
		},
	})
	r.Register(Tool{
		Name:        "read_material",
		Description: "读取固定材料版本的公开内容/清单",
		InputSchema: objectSchema([]string{"versionId"}, map[string]any{
			"versionId": map[string]any{"type": "string"},
		}),
		Effect: EffectRead,
		Execute: func(ctx context.Context, args map[string]any, env Env) (Result, error) {
			if env.ReadMaterial == nil {
				return Result{}, fmt.Errorf("未接线")
			}
			material, err := env.ReadMaterial(ctx, env.ProjectID, str(args, "versionId"))
			if err != nil {
				return Result{}, err
			}
			return Result{Data: material}, nil
		},
	})
	r.Register(Tool{
		Name:        "propose_changes",
		Description: "创建工作变更草稿（不提交人工投票、不验收）",
		InputSchema: objectSchema([]string{"changes"}, map[string]any{
			"changes": map[string]any{"type": "array"},
			"reason":  map[string]any{"type": "string"},
		}),
		Effect: EffectDraft,
		Execute: func(ctx context.Context, args map[string]any, env Env) (Result, error) {
			if env.ProposeDraft == nil {
				return Result{}, fmt.Errorf("未接线")
			}
			id, err := env.ProposeDraft(ctx, env.ProjectID, args)
			if err != nil {
				return Result{}, err
			}
			return Result{Data: map[string]any{"proposalId": id, "state": "draft"}}, nil
		},
	})
}

func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
