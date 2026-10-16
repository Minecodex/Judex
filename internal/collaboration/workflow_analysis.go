package collaboration

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

type WorkflowNodeRef struct {
	WorkflowID uuid.UUID `json:"workflowId"`
	VersionID  uuid.UUID `json:"versionId"`
	NodeID     string    `json:"nodeId"`
	Reason     string    `json:"reason"`
}

func validateWorkflowAnalysis(ctx context.Context, tx pgx.Tx, project, run uuid.UUID, out TaskAnalysisOutput) error {
	var formal *uuid.UUID
	var formalNode *string
	var batch uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT t.workflow_id,t.node_id,r.batch_id FROM task_analyses a JOIN agent_runs r ON r.batch_id=a.batch_id AND r.project_id=a.project_id LEFT JOIN tasks t ON t.id=a.task_id AND t.project_id=a.project_id WHERE r.id=$1 AND a.project_id=$2`, run, project).Scan(&formal, &formalNode, &batch); err != nil {
		return err
	}
	scope, err := ReadAnalysisScope(ctx, tx, project, batch)
	if err != nil {
		return err
	}
	for kind, refs := range map[string][]WorkflowNodeRef{"currentNodeCandidates": out.CurrentNodeCandidates, "relatedEvidenceNodes": out.RelatedEvidenceNodes, "suggestedNextNodes": out.SuggestedNextNodes} {
		if len(refs) > 32 {
			return apierrors.Fields(kind, "too many references")
		}
		seen := map[string]bool{}
		for _, ref := range refs {
			key := ref.WorkflowID.String() + ":" + ref.VersionID.String() + ":" + ref.NodeID
			if seen[key] || strings.TrimSpace(ref.Reason) == "" || utf8.RuneCountInString(ref.Reason) > 4000 {
				return apierrors.Fields(kind, "unique references and public reason required")
			}
			seen[key] = true
			var valid bool
			err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM workflow_definitions d JOIN workflow_versions v ON v.id=d.published_version_id AND v.project_id=d.project_id CROSS JOIN LATERAL jsonb_array_elements(v.nodes_json)n WHERE d.project_id=$1 AND d.id=$2 AND v.id=$3 AND v.state='published' AND n->>'id'=$4)`, project, ref.WorkflowID, ref.VersionID, ref.NodeID).Scan(&valid)
			if err != nil {
				return err
			}
			if !valid {
				return apierrors.Fields(kind, "published node not found in project")
			}
			if kind == "currentNodeCandidates" {
				if scope.TaskID != nil {
					if formal == nil || formalNode == nil || ref.WorkflowID != *formal || ref.NodeID != *formalNode {
						return apierrors.Fields(kind, "current node must match the task's formal workflow and node; alternatives belong in suggestedNextNodes with clarification")
					}
				} else if scope.PlanID == nil && scope.IdentityID == nil {
					return apierrors.Fields(kind, "a free submission has no current work scope; retain reference paths in suggestedNextNodes")
				}
				if scope.TaskID == nil && scope.IdentityID != nil {
					if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM agent_identities i
 JOIN position_templates p ON p.project_id=i.project_id AND p.id=i.template_id AND p.status='active'
 JOIN workflow_versions v ON v.project_id=i.project_id AND v.id=$3
 CROSS JOIN LATERAL jsonb_array_elements(v.nodes_json)n
 WHERE i.project_id=$1 AND i.id=$2 AND i.kind='position' AND i.status='active'
 AND n->>'id'=$4 AND COALESCE(n->'allowedPositionIds','[]'::jsonb) ? i.template_id::text)`, project, scope.IdentityID, ref.VersionID, ref.NodeID).Scan(&valid); err != nil {
						return err
					}
					if !valid {
						return apierrors.Fields(kind, "node is not allowed for the submitted position; retain other roles' reference paths in suggestedNextNodes")
					}
				}
			}
			if kind == "currentNodeCandidates" && scope.TaskID == nil && scope.PlanID != nil {
				if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM plans p WHERE p.project_id=$1 AND p.id=$2
 AND p.status NOT IN ('accepted','cancelled') AND p.discarded_at IS NULL
 AND (p.workflow_id=$3 OR EXISTS(SELECT 1 FROM tasks t WHERE t.project_id=p.project_id AND t.plan_id=p.id
 AND t.workflow_id=$3 AND t.node_id=$4 AND t.status NOT IN ('accepted','cancelled') AND t.discarded_at IS NULL)))`, project, scope.PlanID, ref.WorkflowID, ref.NodeID).Scan(&valid); err != nil {
					return err
				}
				if !valid {
					return apierrors.Fields(kind, "node is outside the submitted plan's current work; retain historical or referenced work in relatedEvidenceNodes")
				}
			}
			if formal != nil && kind != "relatedEvidenceNodes" && ref.WorkflowID != *formal && !out.NeedsClarification {
				return apierrors.Fields("needsClarification", "candidate conflicts with formal task workflow")
			}
		}
	}
	return nil
}
