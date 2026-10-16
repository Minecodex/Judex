package batch

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/agent/tools"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/workflow"
)

func (e *Executor) workflowTools(env *tools.Env, project uuid.UUID) {
	env.ReadWorkflow = func(ctx context.Context, definitionID, versionID string) (map[string]any, error) {
		if e.Workflows == nil {
			return nil, apierrors.New(apierrors.DependencyDown, "workflow service unavailable")
		}
		authority, ok := tools.Authority(ctx)
		if !ok {
			return nil, apierrors.New(apierrors.Forbidden, "runtime authority required")
		}
		run, err := uuid.Parse(authority.Run)
		if err != nil {
			return nil, err
		}
		definition, err := uuid.Parse(definitionID)
		if err != nil {
			return nil, apierrors.Fields("workflowId", "invalid")
		}
		version, err := uuid.Parse(versionID)
		if err != nil {
			return nil, apierrors.Fields("versionId", "invalid")
		}
		out, err := e.Workflows.ReadPublishedForRun(ctx, project, run, authority.Lease, definition, version)
		if err != nil {
			return nil, err
		}
		raw, err := json.Marshal(out)
		if err != nil {
			return nil, err
		}
		var data map[string]any
		err = json.Unmarshal(raw, &data)
		return data, err
	}
	env.DraftWorkflow = func(ctx context.Context, args map[string]any) (map[string]any, error) {
		if e.Workflows == nil {
			return nil, apierrors.New(apierrors.DependencyDown, "workflow service unavailable")
		}
		var input struct {
			WorkflowID      *uuid.UUID    `json:"workflowId"`
			ExpectedVersion int64         `json:"expectedVersion"`
			Body            workflow.Body `json:"body"`
		}
		raw, err := json.Marshal(args)
		if err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &input); err != nil {
			return nil, apierrors.Fields("body", "invalid")
		}
		authority, ok := tools.Authority(ctx)
		if !ok {
			return nil, apierrors.New(apierrors.Forbidden, "runtime authority required")
		}
		run, err := uuid.Parse(authority.Run)
		if err != nil {
			return nil, err
		}
		tx, err := e.beginEffect(ctx, project)
		if err != nil {
			return nil, err
		}
		defer tx.Rollback(context.WithoutCancel(ctx))
		out, err := e.Workflows.DraftForRun(ctx, tx, project, run, authority.Lease, input.WorkflowID, input.ExpectedVersion, input.Body)
		if err != nil {
			return nil, err
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, err
		}
		return out, nil
	}
}
