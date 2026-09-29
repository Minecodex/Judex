package tools

import (
	"context"
	"fmt"
)

func registerContextTools(r *Registry) {
	r.Register(Tool{Name: "read_context", Description: "按 runId 和 toolCallId 分段读取本职责会话的原始工具结果；不会读取其他岗位私有记录。", InputSchema: objectSchema([]string{"runId", "toolCallId"}, map[string]any{"runId": map[string]any{"type": "string"}, "toolCallId": map[string]any{"type": "string"}, "offset": map[string]any{"type": "integer", "minimum": 0}, "limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 16000}}), Effect: EffectRead, Execute: func(ctx context.Context, args map[string]any, env Env) (Result, error) {
		if env.ReadContext == nil {
			return Result{}, fmt.Errorf("context reader unavailable")
		}
		data, err := env.ReadContext(ctx, env.RunID, str(args, "runId"), str(args, "toolCallId"), intVal(args, "offset", 0), intVal(args, "limit", 4000))
		return Result{Data: data}, err
	}})
	r.Register(Tool{Name: "record_analysis", Description: "保存待核实的公开分析，必须提供来源和适用范围。记录不代表批准。", InputSchema: objectSchema([]string{"summary", "sourceRefs", "applicability"}, map[string]any{"summary": map[string]any{"type": "string", "minLength": 1, "maxLength": 8000}, "sourceRefs": map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "string"}}, "applicability": map[string]any{"type": "string", "minLength": 1, "maxLength": 1000}, "disagreements": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}}), Effect: EffectDraft, Execute: func(ctx context.Context, args map[string]any, env Env) (Result, error) {
		if env.RecordAnalysis == nil {
			return Result{}, fmt.Errorf("analysis recorder unavailable")
		}
		data, err := env.RecordAnalysis(ctx, env.RunID, args)
		return Result{Data: data}, err
	}})
	r.Register(Tool{Name: "query_knowledge", Description: "按关键词查询本项目公开分析；区分未核实与已确认，返回来源和适用范围。", InputSchema: objectSchema([]string{"query"}, map[string]any{"query": map[string]any{"type": "string", "maxLength": 200}, "cursor": map[string]any{"type": "string"}, "limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 50}}), Effect: EffectRead, Execute: func(ctx context.Context, args map[string]any, env Env) (Result, error) {
		if env.QueryKnowledge == nil {
			return Result{}, fmt.Errorf("knowledge query unavailable")
		}
		data, err := env.QueryKnowledge(ctx, str(args, "query"), str(args, "cursor"), intVal(args, "limit", 20))
		return Result{Data: data}, err
	}})
}
