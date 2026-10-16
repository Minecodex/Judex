package tools

import (
	"context"
	"fmt"

	"github.com/kakj-go/Judex/internal/collaboration"
)

func registerTaskAnalysis(r *Registry) {
	refs := map[string]any{"type": "array", "maxItems": 32, "description": "只引用本项目当前已发布流程和实际节点，说明公开依据；不改变正式工作。", "items": objectSchema([]string{"workflowId", "versionId", "nodeId", "reason"}, map[string]any{
		"workflowId": map[string]any{"type": "string", "format": "uuid"}, "versionId": map[string]any{"type": "string", "format": "uuid"},
		"nodeId": map[string]any{"type": "string"}, "reason": map[string]any{"type": "string", "minLength": 1, "maxLength": 4000},
	})}
	currentRefs := map[string]any{"type": refs["type"], "maxItems": refs["maxItems"], "items": refs["items"], "description": "当前工作范围内的候选：任务只列已绑定workflow/node；计划限其当前工作；职位限allowedPositionIds。没有任务/计划/职位的自由提交必须为空，参考路径放suggestedNextNodes。澄清不能绕过范围限制。"}
	r.Register(Tool{Name: "record_task_analysis", SchemaVersion: 2, Description: "保存当前任务或项目范围提交的公开分析摘要、异议和可选的独立讨论建议。创建的草稿 ID 和材料版本放入 basis。仅保存分析，不改变任务状态、不建正式会话、不批准或验收。",
		AllowedFor: CoordinatorOnly,
		InputSchema: objectSchema([]string{"summary", "basis"}, map[string]any{
			"currentNodeCandidates": currentRefs, "relatedEvidenceNodes": refs, "suggestedNextNodes": refs,
			"needsClarification": map[string]any{"type": "boolean", "description": "存在多个合理选择或与正式任务绑定冲突时为 true，公开保留分歧；不能覆盖原绑定。"},
			"summary":            map[string]any{"type": "string", "minLength": 1, "maxLength": 8000},
			"basis":              map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "string", "minLength": 1, "pattern": `\S`}, "description": "必须列出实际来源依据，注明提交、固定材料、流程版本或查询对象标识。澄清也须说明原提交和已读事实；不可用空数组、私有内容或未执行查询充当依据。数量须与完整查询/分页结果一致，无法读取的内容明确保留缺口。"},
			"disagreements":      map[string]any{"type": "array", "items": map[string]any{"type": "string", "minLength": 1}, "description": "完整表述异议与未覆盖项，提交前核对句子完整、未被截断，与摘要及来源一致。"},
			"discussion":         objectSchema([]string{"title", "reason"}, map[string]any{"title": map[string]any{"type": "string", "minLength": 1, "maxLength": 200}, "reason": map[string]any{"type": "string", "minLength": 1, "maxLength": 4000}, "existingTopicId": map[string]any{"type": "string", "format": "uuid"}}),
		}), Effect: EffectDraft, Execute: func(ctx context.Context, args map[string]any, env Env) (Result, error) {
			summary, _ := args["summary"].(string)
			if err := collaboration.ValidateAnalysisSummary(summary); err != nil {
				return Result{}, err
			}
			if env.RecordTaskAnalysis == nil {
				return Result{}, fmt.Errorf("task analysis recorder unavailable")
			}
			data, err := env.RecordTaskAnalysis(ctx, env.RunID, args)
			return Result{Data: data}, err
		}})
}
