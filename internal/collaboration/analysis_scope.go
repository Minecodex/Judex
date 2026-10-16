package collaboration

import (
	"context"

	"github.com/google/uuid"
)

// AnalysisScope preserves the explicit object on the canonical submission.
// A plan-scoped question is not a request to select work from another plan.
// Referenced tasks remain evidence, not members of the plan's current work.
type AnalysisScope struct {
	TaskID     *uuid.UUID `json:"taskId"`
	PlanID     *uuid.UUID `json:"planId"`
	IdentityID *uuid.UUID `json:"identityId"`
}

func ReadAnalysisScope(ctx context.Context, q Reader, project, batch uuid.UUID) (AnalysisScope, error) {
	var scope AnalysisScope
	err := q.QueryRow(ctx, `SELECT b.task_id,CASE WHEN b.task_id IS NOT NULL THEN t.plan_id ELSE s.plan_id END,s.identity_id
 FROM discussion_batches b LEFT JOIN tasks t ON t.id=b.task_id AND t.project_id=b.project_id
 LEFT JOIN submissions s ON s.id=b.source_submission_id AND s.project_id=b.project_id
 WHERE b.project_id=$1 AND b.id=$2`, project, batch).Scan(&scope.TaskID, &scope.PlanID, &scope.IdentityID)
	return scope, err
}
