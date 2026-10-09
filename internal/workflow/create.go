package workflow

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/events"
)

// All workflow creation paths share version, event and audit insertion in the
// caller's project-locked transaction.
func (s *Service) insertDefinition(ctx context.Context, tx postgres.Tx, requester, projectID uuid.UUID, body Body, presetID *string) (Definition, error) {
	now, id, versionID := s.now(), uuid.New(), uuid.New()
	nodesJSON, _ := json.Marshal(body.Nodes)
	edgesJSON, _ := json.Marshal(body.AdvisoryEdges)
	hardJSON, _ := json.Marshal(body.HardRules)
	policiesJSON, _ := json.Marshal(body.ApprovalPolicies)
	if _, err := tx.Exec(ctx, `
		INSERT INTO workflow_definitions (project_id,id,name,created_at,updated_at,preset_id)
		VALUES ($1,$2,$3,$4,$4,$5)`, projectID, id, body.Name, now, presetID); err != nil {
		return Definition{}, apierrors.New(apierrors.Internal, "definition insert failed").Wrap(err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO workflow_versions (project_id,id,definition_id,revision,state,instructions,
			nodes_json,advisory_edges_json,approval_policies_json,hard_rules_json,mermaid,author_user_id,created_at,name)
		VALUES ($1,$2,$3,1,'draft',$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		projectID, versionID, id, body.Instructions, nodesJSON, edgesJSON, policiesJSON, hardJSON,
		GenerateMermaid(body), requester, now, body.Name); err != nil {
		return Definition{}, apierrors.New(apierrors.Internal, "draft insert failed").Wrap(err)
	}
	if _, err := events.AppendProjectEvent(ctx, tx, projectID, "workflow.published", "workflow", id.String(), nil,
		map[string]any{"change": "draft_created"}, now); err != nil {
		return Definition{}, err
	}
	if err := audit.Append(ctx, tx, audit.Entry{
		ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
		Source: audit.SourceWeb, Operation: "workflow.create",
		ObjectType: "workflow", ObjectID: id.String(), OccurredAt: now,
	}); err != nil {
		return Definition{}, err
	}
	return Definition{ID: id, Name: body.Name, HasDraft: true, Version: 1, PresetID: presetID}, nil
}
