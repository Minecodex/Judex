package runner

import (
	"context"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
)

// Journal makes the execution boundary durable before contacting a model
// or running a tool. Any persistence failure stops the run immediately.
type Journal interface {
	ModelStart(context.Context, int, model.Request) error
	ModelEnd(context.Context, int, model.UsageSnapshot, bool, error) error
	ToolStart(context.Context, model.ToolCall) error
	ToolEnd(context.Context, model.ToolCall, []byte, bool) error
	Checkpoint(context.Context, []model.Message) error
}

// BudgetReader includes other positions and recovered calls in the same batch.
// ModelStart still makes the authoritative, atomic reservation.
type BudgetReader interface {
	RemainingBudget(context.Context) (tokens int64, attempts int, err error)
}
