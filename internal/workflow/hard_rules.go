package workflow

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

// Workflow gates reference existing objects in their project. Future work is
// described in node responsibilities and gets concrete task requirements later.
func validateHardRuleReferences(ctx context.Context, q querier, project uuid.UUID, rules []HardRule) error {
	for i, rule := range rules {
		table := map[string]string{"task_acceptance": "tasks", "handoff_receipt": "handoff_sources", "material_ready": "material_versions"}[rule.Kind]
		if table == "" {
			return apierrors.Fields("hardRules.kind", "whitelist")
		}
		target, err := uuid.Parse(rule.TargetID)
		if err != nil || target == uuid.Nil {
			return apierrors.Fields(fmt.Sprintf("hardRules[%d].targetId", i), "non-zero UUID required")
		}
		var exists bool
		if err := q.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM "+table+" WHERE project_id=$1 AND id=$2)", project, target).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return apierrors.Fields(fmt.Sprintf("hardRules[%d].targetId", i), "target not in project")
		}
	}
	return nil
}
