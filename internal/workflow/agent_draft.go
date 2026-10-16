package workflow

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

type agentCreationKey struct{}

// DraftForRun joins the caller's fenced effect transaction. The model cannot
// supply its actor, member privileges, project or a publication operation.
func (s *Service) DraftForRun(ctx context.Context, tx pgx.Tx, project, run uuid.UUID, lease string,
	workflowID *uuid.UUID, expectedVersion int64, body Body) (map[string]any, error) {
	if err := Validate(body); err != nil {
		return nil, err
	}
	var requester, identity uuid.UUID
	err := tx.QueryRow(ctx, `SELECT r.requested_by,i.id FROM agent_runs r
 JOIN agent_identities i ON i.id=r.identity_id AND i.project_id=r.project_id
 JOIN project_members m ON m.project_id=r.project_id AND m.user_id=r.requested_by AND m.state='active'
 JOIN users u ON u.id=m.user_id AND u.status='active'
 WHERE r.id=$1 AND r.project_id=$2 AND r.state='running' AND r.lease_token=$3
 AND i.kind='coordinator' AND i.status='active' AND m.role IN ('owner','manager')`, run, project, lease).Scan(&requester, &identity)
	if err != nil {
		return nil, apierrors.New(apierrors.Forbidden, "active manager-requested coordinator run required")
	}
	for _, node := range body.Nodes {
		for _, id := range node.AllowedPositionIds {
			var valid bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM position_templates WHERE project_id=$1 AND id=$2 AND status='active')`, project, id).Scan(&valid); err != nil {
				return nil, err
			}
			if !valid {
				return nil, apierrors.Fields("nodes[].allowedPositionIds", "position not active in project")
			}
		}
		for _, id := range node.DelegationUserIDs {
			var valid bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM project_members WHERE project_id=$1 AND user_id=$2 AND state='active')`, project, id).Scan(&valid); err != nil {
				return nil, err
			}
			if !valid {
				return nil, apierrors.Fields("nodes[].delegationUserIds", "member not active in project")
			}
		}
	}
	ctx = audit.WithAgentActor(postgres.WithTransaction(ctx, s.pool, tx), identity, run)
	var id uuid.UUID
	var headVersion int64
	deduplicated := false
	if workflowID == nil {
		raw, _ := json.Marshal(body)
		id = uuid.NewSHA1(run, raw)
		err = tx.QueryRow(ctx, `SELECT version FROM workflow_definitions WHERE project_id=$1 AND id=$2`, project, id).Scan(&headVersion)
		if err == nil {
			deduplicated = true
		} else if errors.Is(err, pgx.ErrNoRows) {
			ctx = context.WithValue(ctx, agentCreationKey{}, id)
			created, createErr := s.Create(ctx, requester, project, body)
			if createErr != nil {
				return nil, createErr
			}
			headVersion = created.Version
		} else {
			return nil, err
		}
	} else {
		if expectedVersion < 1 {
			return nil, apierrors.Fields("expectedVersion", "required for revision")
		}
		id = *workflowID
		if _, err := s.UpdateDraft(ctx, requester, project, id, expectedVersion, body); err != nil {
			return nil, err
		}
		headVersion = expectedVersion + 1
	}
	versions, err := s.ListVersions(ctx, requester, project, id, true)
	if err != nil {
		return nil, err
	}
	out := map[string]any{"workflowId": id, "version": headVersion, "source": "agent", "deduplicated": deduplicated, "published": false}
	for _, version := range versions {
		if version.State == "draft" {
			out["draftVersionId"], out["draftHash"], out["mermaid"] = version.ID, version.DraftHash, version.Mermaid
			return out, nil
		}
	}
	out["alreadyPublished"] = true
	return out, nil
}
