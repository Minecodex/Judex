package runner

import (
	"context"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
)

// Journal makes the execution boundary durable before contacting a model
// or running a tool. Any persistence failure stops the run immediately.
type Journal interface {
	ModelStart(context.Context, int, model.Request) error
	ModelEnd(context.Context, int, int64, bool, error) error
	ToolStart(context.Context, model.ToolCall) error
	ToolEnd(context.Context, model.ToolCall, []byte, bool) error
	Checkpoint(context.Context, []model.Message) error
}
