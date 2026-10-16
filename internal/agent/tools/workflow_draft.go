package tools

import (
	"context"
	"fmt"
)

func registerWorkflowDraft(r *Registry) {
	text := map[string]any{"type": "string"}
	id := map[string]any{"type": "string", "format": "uuid"}
	ids := map[string]any{"type": "array", "items": id}
	node := objectSchema([]string{"id", "name", "responsibility", "allowedPositionIds", "defaultApprovalPolicy"}, map[string]any{
		"id": text, "name": text, "responsibility": text,
		"kind":                  map[string]any{"type": "string", "enum": []string{"activity", "decision"}, "description": "活动或评审形状；不能使用 task/start/end。"},
		"phase":                 map[string]any{"type": "string", "maxLength": 120},
		"allowedPositionIds":    ids,
		"delegationUserIds":     map[string]any{"type": "array", "items": id, "description": "仅供 owner 明确指定人类成员的节点代批授权，使用 project member 的 userId；不是岗位任职 identityId，也不是 call_agent 委派。未要求代批时省略或为空数组。"},
		"defaultApprovalPolicy": map[string]any{"type": "string", "enum": []string{"all", "none"}},
	})
	edge := objectSchema([]string{"from", "to"}, map[string]any{"from": text, "to": text, "kind": map[string]any{"type": "string", "enum": []string{"sequence", "feedback"}, "description": "反馈必须从 decision 返回此前可达节点，并填写 label；普通顺序使用 sequence。"}, "label": text})
	rule := objectSchema([]string{"kind", "phase", "targetId"}, map[string]any{"nodeId": text, "targetId": id,
		"kind":  map[string]any{"type": "string", "enum": []string{"task_acceptance", "handoff_receipt", "material_ready"}},
		"phase": map[string]any{"type": "string", "enum": []string{"start", "accept", "both"}},
	})
	body := objectSchema([]string{"name", "instructions", "nodes", "advisoryEdges", "hardRules", "approvalPolicies"}, map[string]any{
		"name": text, "instructions": text, "nodes": map[string]any{"type": "array", "minItems": 1, "items": node},
		"advisoryEdges": map[string]any{"type": "array", "items": edge}, "hardRules": map[string]any{"type": "array", "items": rule},
		"approvalPolicies": map[string]any{"type": "object", "additionalProperties": text},
	})
	r.Register(Tool{Name: "draft_workflow", SchemaVersion: 1, AllowedFor: CoordinatorOnly, RequiresManager: true, Effect: EffectDraft,
		Description: "为当前项目创建或修订结构化工作流草稿。创建新流程只传 body，省略 workflowId 和 expectedVersion，服务端生成真实ID，不能自拟UUID。修订才传当前项目已有的 workflowId 及其当前 expectedVersion。节点 allowedPositionIds 使用 list_agent 的 positionId（职位模板），不是 identityId。hardRules 必须有当前项目已存在对象的非零 targetId；未来任务或资料条件写在职责中，取得真实ID后另定任务requirements，不能用空/全零UUID占位。返回草稿版本、hash 和图。此工具不能发布流程，发布须管理者在 Web 明确确认。",
		InputSchema: objectSchema([]string{"body"}, map[string]any{"body": body, "workflowId": map[string]any{"type": "string", "format": "uuid", "description": "仅修订时传当前项目已有的流程ID；创建必须省略，禁止编造。"}, "expectedVersion": map[string]any{"type": "integer", "minimum": 1, "description": "仅修订时传实际读取的当前版本；创建省略。"}}),
		Execute: func(ctx context.Context, args map[string]any, env Env) (Result, error) {
			if env.DraftWorkflow == nil {
				return Result{}, fmt.Errorf("workflow drafting unavailable")
			}
			data, err := env.DraftWorkflow(ctx, args)
			return Result{Data: data}, err
		},
	})
}
