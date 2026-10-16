package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

// PublishedContent identifies one immutable public definition, independently
// of which version the aggregate currently publishes.
type PublishedContent struct {
	WorkflowID  uuid.UUID `json:"workflowId"`
	VersionID   uuid.UUID `json:"versionId"`
	Revision    int64     `json:"revision"`
	IsCurrent   bool      `json:"isCurrent"`
	Body        Body      `json:"body"`
	BodySHA256  string    `json:"bodySha256"`
	PublishedAt time.Time `json:"publishedAt"`
}

func (s *Service) ReadPublished(ctx context.Context, user, project, definition, version uuid.UUID) (PublishedContent, error) {
	if _, err := s.membership(ctx, s.pool, user, project); err != nil {
		return PublishedContent{}, err
	}
	return readPublished(ctx, s.pool, project, definition, version)
}

// ReadPublishedForRun checks live authority before exposing public content.
// No actor, project, lease or publication permission comes from model input.
func (s *Service) ReadPublishedForRun(ctx context.Context, project, run uuid.UUID, lease string, definition, version uuid.UUID) (PublishedContent, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return PublishedContent{}, err
	}
	defer tx.Rollback(ctx)
	var valid bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM agent_runs r
 JOIN projects p ON p.id=r.project_id AND p.status='active'
 JOIN agent_identities i ON i.id=r.identity_id AND i.project_id=r.project_id AND i.status='active'
 JOIN project_members m ON m.project_id=r.project_id AND m.user_id=r.requested_by AND m.state='active'
 JOIN users u ON u.id=m.user_id AND u.status='active'
 WHERE r.id=$1 AND r.project_id=$2 AND r.state='running' AND r.lease_token=$3
 AND (i.kind='coordinator' OR (i.current_binding_version=r.binding_version AND EXISTS(
 SELECT 1 FROM identity_bindings b JOIN users h ON h.id=b.user_id AND h.status='active'
 JOIN project_members hm ON hm.project_id=i.project_id AND hm.user_id=b.user_id AND hm.state='active'
 JOIN position_templates t ON t.id=i.template_id AND t.project_id=i.project_id AND t.status='active'
 WHERE b.identity_id=i.id AND b.binding_version=r.binding_version AND b.valid_until IS NULL))))`, run, project, lease).Scan(&valid)
	if err != nil {
		return PublishedContent{}, err
	}
	if !valid {
		return PublishedContent{}, apierrors.New(apierrors.Forbidden, "active run authority required")
	}
	return readPublished(ctx, tx, project, definition, version)
}

func readPublished(ctx context.Context, q querier, project, definition, version uuid.UUID) (PublishedContent, error) {
	var out PublishedContent
	var raw []byte
	err := q.QueryRow(ctx, `SELECT v.definition_id,v.id,v.revision,d.published_version_id=v.id,v.published_at,
 jsonb_build_object('name',v.name,'instructions',v.instructions,'nodes',v.nodes_json,
 'advisoryEdges',v.advisory_edges_json,'hardRules',v.hard_rules_json,'approvalPolicies',v.approval_policies_json)
 FROM workflow_versions v JOIN workflow_definitions d ON d.id=v.definition_id AND d.project_id=v.project_id
 WHERE v.project_id=$1 AND v.definition_id=$2 AND v.id=$3 AND v.state='published'`, project, definition, version).Scan(
		&out.WorkflowID, &out.VersionID, &out.Revision, &out.IsCurrent, &out.PublishedAt, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, apierrors.New(apierrors.NotFound, "published workflow version not found in project")
	}
	if err != nil {
		return out, err
	}
	if err = json.Unmarshal(raw, &out.Body); err != nil {
		return out, err
	}
	canonical, err := json.Marshal(out.Body)
	if err != nil {
		return out, err
	}
	hash := sha256.Sum256(canonical)
	out.BodySHA256 = hex.EncodeToString(hash[:])
	return out, nil
}
