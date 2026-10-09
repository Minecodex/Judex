package tools

import (
	"context"
	"fmt"
)

func registerTaskAnalysis(r *Registry) {
	r.Register(Tool{Name: "record_task_analysis", Description: "保存当前任务上报的公开分析摘要、异议和可选的独立讨论建议。仅创建建议草稿，不改变任务状态、不建正式会话、不批准或验收。",
		InputSchema: objectSchema([]string{"summary"}, map[string]any{
			"summary":       map[string]any{"type": "string", "minLength": 1, "maxLength": 8000},
			"basis":         map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "依据原记录、不可变材料版本及当前正式事实逐条列出公开依据，注明来源标识；无法读取的内容明确保留缺口。"},
			"disagreements": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"discussion":    objectSchema([]string{"title", "reason"}, map[string]any{"title": map[string]any{"type": "string", "minLength": 1, "maxLength": 200}, "reason": map[string]any{"type": "string", "minLength": 1, "maxLength": 4000}, "existingTopicId": map[string]any{"type": "string", "format": "uuid"}}),
		}), Effect: EffectDraft, Execute: func(ctx context.Context, args map[string]any, env Env) (Result, error) {
			if env.RecordTaskAnalysis == nil {
				return Result{}, fmt.Errorf("task analysis recorder unavailable")
			}
			data, err := env.RecordTaskAnalysis(ctx, env.RunID, args)
			return Result{Data: data}, err
		}})
}
