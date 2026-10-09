package work

import (
	"context"
	"github.com/google/uuid"
)

// TaskExecutionFacts projects the same executable constraints for read-only
// Agent tools as for start, report and acceptance commands. It grants no action.
type TaskExecutionFacts struct {
	ExecutionException *ExecutionException `json:"executionException"`
	Requirements       []Requirement       `json:"effectiveRequirements"`
	Blockers           []Blocker           `json:"blockers"`
}

func ReadTaskExecutionFacts(ctx context.Context, q dbQuery, project, task uuid.UUID) (TaskExecutionFacts, error) {
	out := TaskExecutionFacts{}
	var err error
	out.ExecutionException, err = activeException(ctx, q, project, task)
	if err != nil {
		return out, err
	}
	out.Requirements, out.Blockers, err = (&Service{}).RequirementsFor(ctx, q, project, task)
	if out.Blockers == nil {
		out.Blockers = []Blocker{}
	}
	return out, err
}
