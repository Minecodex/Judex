package runner

import (
	"context"
	"encoding/json"
	"github.com/kakj-go/Judex/internal/agent/tools"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
)

func (r *Runner) executeTool(ctx context.Context, req RunRequest, call model.ToolCall) (model.Message, bool, error) {
	var args map[string]any
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		return model.Message{}, false, err
	}
	if req.Journal != nil {
		if err := req.Journal.ToolStart(ctx, call); err != nil {
			return model.Message{}, false, err
		}
	}
	env := tools.Env{ProjectID: req.ProjectID.String(), RunID: req.RunID.String(), IdentityID: req.IdentityID.String()}
	if req.Env != nil {
		env = *req.Env
		env.RunID = req.RunID.String()
		if env.IdentityID == "" {
			env.IdentityID = req.IdentityID.String()
		}
	}
	ctx = tools.WithRuntimeAuthority(ctx, tools.RuntimeAuthority{Project: req.ProjectID.String(), Run: req.RunID.String(), Lease: req.Lease, ToolCall: call.ID})
	result, callErr := r.Registry.Execute(ctx, call.Name, "agent", args, env)
	content := map[string]any{"toolCallId": call.ID}
	if callErr != nil {
		content["error"] = callErr.Error()
	} else {
		content["data"] = result.Data
	}
	unknown, _ := result.Data["unknown"].(bool)
	raw, err := json.Marshal(content)
	if err != nil {
		return model.Message{}, unknown, err
	}
	if req.Journal != nil {
		if err := req.Journal.ToolEnd(context.WithoutCancel(ctx), call, raw, unknown); err != nil {
			return model.Message{}, unknown, err
		}
	}
	return model.Message{Role: "tool", ToolCallID: call.ID, Content: string(raw)}, unknown, nil
}

// Only independent position calls overlap. Shell/file operations preserve
// model order. Results return to the model in their original call order.
func (r *Runner) executeTools(ctx context.Context, req RunRequest, calls []model.ToolCall) ([]model.Message, bool, error) {
	out := []model.Message{}
	for start := 0; start < len(calls); {
		end := start + 1
		if calls[start].Name == "call_agent" {
			for end < len(calls) && calls[end].Name == "call_agent" {
				end++
			}
		}
		if end-start == 1 {
			message, unknown, err := r.executeTool(ctx, req, calls[start])
			if err != nil || unknown {
				return out, unknown, err
			}
			out = append(out, message)
		} else {
			type completed struct {
				index   int
				message model.Message
				unknown bool
				err     error
			}
			done := make(chan completed, end-start)
			slots := make(chan struct{}, MaxParallelChildren)
			for index := start; index < end; index++ {
				go func(index int) {
					select {
					case slots <- struct{}{}:
						defer func() { <-slots }()
					case <-ctx.Done():
						done <- completed{index: index, err: ctx.Err()}
						return
					}
					message, unknown, err := r.executeTool(ctx, req, calls[index])
					done <- completed{index, message, unknown, err}
				}(index)
			}
			messages := make([]model.Message, end-start)
			var first error
			unknown := false
			for count := start; count < end; count++ {
				result := <-done
				messages[result.index-start] = result.message
				if first == nil {
					first = result.err
				}
				unknown = unknown || result.unknown
			}
			if first != nil || unknown {
				return out, unknown, first
			}
			out = append(out, messages...)
		}
		start = end
	}
	return out, false, nil
}
