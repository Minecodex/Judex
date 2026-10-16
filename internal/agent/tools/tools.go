// SPDX-License-Identifier: Apache-2.0

// Package tools registers the model-facing tool set per docs/plans/v1/05 §6:
// sandbox-executed read/write/edit/bash plus server-side business tools.
// Tools declare effectClass and allowedPrincipal; results are data, never
// instructions. No tool grants approval or SQL access.
package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/getkin/kin-openapi/openapi3"
	"sort"
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
	RequiresManager bool
	Name            string
	SchemaVersion   int
	Description     string
	InputSchema     map[string]any
	Effect          EffectClass
	AllowedFor      func(principalKind string) bool
	Execute         func(ctx context.Context, args map[string]any, env Env) (Result, error)
}

// Env carries the per-run execution context tools may use; it deliberately
// exposes NO credentials and NO write access to formal state.
type Env struct {
	ReadWorkflow       func(context.Context, string, string) (map[string]any, error)
	CanManage          bool
	DraftWorkflow      func(context.Context, map[string]any) (map[string]any, error)
	RecordTaskAnalysis func(context.Context, string, map[string]any) (map[string]any, error)
	QueryWorkPage      func(context.Context, string, map[string]any) (map[string]any, error)
	Publish            func(context.Context, string, map[string]any, []byte) (map[string]any, error)
	ReadContext        func(context.Context, string, string, string, int, int) (map[string]any, error)
	RecordAnalysis     func(context.Context, string, map[string]any) (map[string]any, error)
	QueryKnowledge     func(context.Context, string, string, int) (map[string]any, error)
	ProjectID          string
	RunID              string
	IdentityID         string
	// Sandbox executes read/write/edit/bash (nil → those tools refuse).
	Sandbox SandboxExec
	// ReadMaterialContent lets the live-test stub return injected Chinese
	// material without a real storage round-trip.
	ReadMaterialContent string
	// Queriers are read-only server projections.
	ListAgents   func(ctx context.Context, projectID string) ([]map[string]any, error)
	QueryWork    func(ctx context.Context, projectID string, objectType string, cursor string, limit int) ([]map[string]any, error)
	ReadMaterial func(ctx context.Context, projectID, versionID string) (map[string]any, error)
	MaterialInfo func(ctx context.Context, projectID, versionID string) (map[string]any, error)
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
	return r.schemas("", true)
}

// SchemasFor uses the same authority predicate as Execute. The caller supplies
// the server-verified identity kind, never a model-provided role.
func (r *Registry) SchemasFor(principalKind string) []map[string]any {
	return r.schemas(principalKind, false)
}

func (r *Registry) SchemasForAuthority(principalKind string, canManage bool) []map[string]any {
	return r.schemas(principalKind, canManage)
}

func (r *Registry) schemas(principalKind string, canManage bool) []map[string]any {
	var out []map[string]any
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		tool := r.tools[name]
		if tool.RequiresManager && !canManage {
			continue
		}
		if principalKind != "" && tool.AllowedFor != nil && !tool.AllowedFor(principalKind) {
			continue
		}
		out = append(out, map[string]any{
			"name": tool.Name, "schema_version": tool.SchemaVersion,
			"description": tool.Description, "input_schema": tool.InputSchema,
			"effect_class": string(tool.Effect),
		})
	}
	return out
}

func CoordinatorOnly(principalKind string) bool { return principalKind == "coordinator" }

// Execute validates principal + schema presence and runs the tool.
func (r *Registry) Execute(ctx context.Context, name string, principalKind string, args map[string]any, env Env) (Result, error) {
	tool, ok := r.tools[name]
	if !ok {
		return Result{}, fmt.Errorf("unknown tool %q", name)
	}
	if tool.AllowedFor != nil && !tool.AllowedFor(principalKind) {
		return Result{}, fmt.Errorf("tool %q 不允许该调用方", name)
	}
	if tool.RequiresManager && !env.CanManage {
		return Result{}, fmt.Errorf("tool %q requires manager authority", name)
	}
	raw, err := json.Marshal(tool.InputSchema)
	if err != nil {
		return Result{}, err
	}
	var schema openapi3.Schema
	if err = json.Unmarshal(raw, &schema); err != nil {
		return Result{}, err
	}
	if err = schema.VisitJSON(args); err != nil {
		return Result{}, fmt.Errorf("tool arguments: %w", err)
	}
	return tool.Execute(ctx, args, env)
}

func objectSchema(required []string, props map[string]any) map[string]any {
	// JSON Schema requires properties to be an object, including zero-argument
	// tools. A nil map/slice serializes as null and is rejected by strict gateways.
	if props == nil {
		props = map[string]any{}
	}
	schema := map[string]any{"type": "object", "properties": props,
		"additionalProperties": false}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
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
	registerWorkflowRead(r)
	registerWorkflowDraft(r)
	registerContextTools(r)
	registerFileTools(r)
	registerMaterialAnalysis(r)
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
				return Result{Data: map[string]any{"unknown": unknown}}, fmt.Errorf("读取结果未知，不盲重试")
			}
			if exit != 0 {
				return Result{}, fmt.Errorf("读取失败 exit=%d", exit)
			}
			hash := sha256.Sum256(stdout)
			text, binary := commandText(stdout)
			data := map[string]any{"content": text, "sha256": hex.EncodeToString(hash[:]), "size": len(stdout)}
			if binary != nil {
				data["binary"] = binary
			}
			return Result{Data: data}, nil
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
				return Result{Data: map[string]any{"unknown": unknown}}, err
			}
			outText, outBinary := commandText(stdout)
			errText, errBinary := commandText(stderr)
			data := map[string]any{"exitCode": exit, "stdout": outText, "stderr": errText}
			if outBinary != nil {
				data["stdoutBinary"] = outBinary
			}
			if errBinary != nil {
				data["stderrBinary"] = errBinary
			}
			if unknown {
				data["unknown"] = true
			}
			return Result{Data: data}, nil
		},
	})
	r.Register(Tool{
		Name:        "list_agent",
		Description: "列出当前范围可委派的在任岗位身份与公开职责（无个人提示词）。任务范围与 call_agent 使用相同参与人/验收人/当前节点授权；项目范围列出项目在任岗位。仅这些 identityId 可用于委派，同岗位多人仍分别列出。",
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
			"objectId":   map[string]any{"type": "string", "format": "uuid"}, "query": map[string]any{"type": "string", "maxLength": 200},
			"cursor": map[string]any{"type": "string"},
			"limit":  map[string]any{"type": "integer"},
		}),
		Effect: EffectRead,
		Execute: func(ctx context.Context, args map[string]any, env Env) (Result, error) {
			if env.QueryWorkPage != nil {
				data, err := env.QueryWorkPage(ctx, env.ProjectID, args)
				return Result{Data: data}, err
			}
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
		Name:          "propose_changes",
		SchemaVersion: 2,
		Description:   "创建工作安排或工作变更草稿，不提交投票或验收。kind 可选 work_arrangement/work_change，省略保留 work_arrangement；修正现有工作使用 work_change。changes 每项包含 operation、targetType(plan/task)、fields；修改既有对象还须 targetId、expectedVersion。create_plan 使用 clientRef 和 fields.title/goal/acceptanceCriteria/ownerIdentityId/workflowId；create_task 使用 fields.title/expectedOutput/acceptanceCriteria/participantIdentityIds/reviewerIdentityId/workflowId/nodeId。任务归入计划必须显式 fields.planId（既有计划 UUID 或本次计划 clientRef）；顶层 dependsOn 仅是变更创建顺序，不能代替 owning plan 或运行依赖。运行前置条件须 fields.requirements（kind/phase/targetId/hard/label），cancel_task 必须 fields.reason。审批人及贡献者用实际 identityId，不能用职位模板 ID。验收标准是字符串。只能建议已发布流程绑定。",
		InputSchema: objectSchema([]string{"changes"}, map[string]any{
			"kind": map[string]any{"type": "string", "enum": []string{"work_arrangement", "work_change"}},
			"changes": map[string]any{"type": "array", "minItems": 1, "maxItems": 100, "items": objectSchema([]string{"operation", "targetType"}, map[string]any{
				"operation": map[string]any{"type": "string"}, "targetType": map[string]any{"type": "string", "enum": []string{"plan", "task"}},
				"targetId": map[string]any{"type": "string", "format": "uuid"}, "clientRef": map[string]any{"type": "string"},
				"expectedVersion": map[string]any{"type": "integer", "minimum": 1}, "fields": map[string]any{"type": "object", "additionalProperties": true},
				"dependsOn": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			})},
			"reason": map[string]any{"type": "string"},
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
			kind, _ := args["kind"].(string)
			if kind == "" {
				kind = "work_arrangement"
			}
			return Result{Data: map[string]any{"proposalId": id, "state": "draft", "kind": kind}}, nil
		},
	})
}

func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
