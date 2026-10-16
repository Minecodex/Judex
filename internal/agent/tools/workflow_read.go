package tools

import (
	"context"
	"fmt"
	"github.com/google/uuid"
)

func registerWorkflowRead(r *Registry) {
	id := map[string]any{"type": "string", "format": "uuid"}
	r.Register(Tool{Name: "read_workflow", SchemaVersion: 1, Effect: EffectRead,
		Description: "读取本项目指定已发布流程版本的完整结构化正文、bodySha256和是否为当前版本。workflowId是流程定义UUID，versionId是流程版本UUID；二者不是材料UUID，不使用read_material/analyze_material。历史已发布正文可作来源，不能当作当前工作或正式决定。",
		InputSchema: objectSchema([]string{"workflowId", "versionId"}, map[string]any{"workflowId": id, "versionId": id}),
		Execute: func(ctx context.Context, args map[string]any, env Env) (Result, error) {
			definition, err := uuid.Parse(str(args, "workflowId"))
			if err != nil || definition == uuid.Nil {
				return Result{}, fmt.Errorf("valid workflowId required")
			}
			version, err := uuid.Parse(str(args, "versionId"))
			if err != nil || version == uuid.Nil {
				return Result{}, fmt.Errorf("valid workflow versionId required")
			}
			if env.ReadWorkflow == nil {
				return Result{}, fmt.Errorf("published workflow reader unavailable")
			}
			data, err := env.ReadWorkflow(ctx, definition.String(), version.String())
			return Result{Data: data}, err
		},
	})
}
