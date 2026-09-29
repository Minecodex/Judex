// SPDX-License-Identifier: Apache-2.0

package project

import (
	"context"
	"errors"
	"github.com/kakj-go/Judex/internal/platform/paging"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/events"
)

// Position is the API projection (06 §3).
type Position struct {
	ID             uuid.UUID     `json:"id"`
	Name           string        `json:"name"`
	Status         string        `json:"status"`
	CurrentVersion int64         `json:"currentVersion"`
	PublicSummary  string        `json:"publicSummary"`
	Prompt         string        `json:"prompt"`
	ModelID        *uuid.UUID    `json:"modelId"`
	Revision       int64         `json:"revision"`
	NodeBindings   []NodeBinding `json:"nodeBindings"`
}

// NodeBinding references a published workflow node.
type NodeBinding struct {
	WorkflowID uuid.UUID `json:"workflowId"`
	NodeID     string    `json:"nodeId"`
}

type PositionDraft struct {
	Name          string
	Prompt        string
	PublicSummary string
	ModelID       *uuid.UUID
	NodeBindings  []NodeBinding
}

// CreatePosition (manager) inserts the template plus revision 1.
func (s *Service) CreatePosition(ctx context.Context, requester, projectID uuid.UUID, draft PositionDraft) (Position, error) {
	name := strings.TrimSpace(draft.Name)
	if l := utf8.RuneCountInString(name); l < 1 || l > 120 {
		return Position{}, apierrors.Fields("name", "length")
	}
	var out Position
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, projectID.String()); err != nil {
			return err
		}
		role, err := s.MembershipForTx(ctx, tx, requester, projectID)
		if err != nil {
			return err
		}
		if role.Role != "owner" && role.Role != "manager" {
			return apierrors.New(apierrors.Forbidden, "manager or owner required")
		}
		if err := validateNodeBindings(ctx, tx, projectID, draft.NodeBindings); err != nil {
			return err
		}
		now := s.now()
		id := uuid.New()
		if _, err := tx.Exec(ctx, `
			INSERT INTO position_templates (project_id, id, name, current_version, created_at, updated_at)
			VALUES ($1,$2,$3,1,$4,$4)`, projectID, id, name, now); err != nil {
			return apierrors.New(apierrors.Internal, "template insert failed").Wrap(err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO position_versions (project_id, template_id, revision, prompt, public_summary, model_id, created_by, created_at)
			VALUES ($1,$2,1,$3,$4,$5,$6,$7)`,
			projectID, id, draft.Prompt, draft.PublicSummary, draft.ModelID, requester, now); err != nil {
			return apierrors.New(apierrors.Internal, "version insert failed").Wrap(err)
		}
		if err := writeNodeBindings(ctx, tx, projectID, id, draft.NodeBindings); err != nil {
			return err
		}
		if _, err := events.AppendProjectEvent(ctx, tx, projectID, "membership.changed", "position", id.String(), nil,
			map[string]any{"change": "created"}, now); err != nil {
			return err
		}
		out = Position{ID: id, Name: name, Status: "active", CurrentVersion: 1,
			PublicSummary: draft.PublicSummary, Prompt: draft.Prompt, ModelID: draft.ModelID,
			Revision: 1, NodeBindings: draft.NodeBindings}
		return audit.Append(ctx, tx, audit.Entry{
			ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
			Source: audit.SourceWeb, Operation: "position.create",
			ObjectType: "position", ObjectID: id.String(), OccurredAt: now,
		})
	})
	return out, err
}

// UpdatePosition produces a new revision (02 §6 职责可修改产生版本).
func (s *Service) UpdatePosition(ctx context.Context, requester, projectID, positionID uuid.UUID, expectedVersion int64, draft PositionDraft) (Position, error) {
	name := strings.TrimSpace(draft.Name)
	if l := utf8.RuneCountInString(name); l < 1 || l > 120 {
		return Position{}, apierrors.Fields("name", "length")
	}
	var out Position
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, projectID.String()); err != nil {
			return err
		}
		role, err := s.MembershipForTx(ctx, tx, requester, projectID)
		if err != nil {
			return err
		}
		if role.Role != "owner" && role.Role != "manager" {
			return apierrors.New(apierrors.Forbidden, "manager or owner required")
		}
		if err := validateNodeBindings(ctx, tx, projectID, draft.NodeBindings); err != nil {
			return err
		}
		var currentRevision int64
		if err := tx.QueryRow(ctx, `
			SELECT current_version FROM position_templates WHERE id=$1 AND project_id=$2 AND status='active' FOR UPDATE`,
			positionID, projectID).Scan(&currentRevision); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apierrors.New(apierrors.NotFound, "position not found")
			}
			return apierrors.New(apierrors.Internal, "template lookup failed").Wrap(err)
		}
		if currentRevision != expectedVersion {
			return apierrors.New(apierrors.VersionConflict, "position version conflict")
		}
		now := s.now()
		next := currentRevision + 1
		if _, err := tx.Exec(ctx, `
			UPDATE position_templates SET name=$3, current_version=$4, updated_at=$5 WHERE id=$1 AND project_id=$2`,
			positionID, projectID, name, next, now); err != nil {
			return apierrors.New(apierrors.Internal, "template update failed").Wrap(err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO position_versions (project_id, template_id, revision, prompt, public_summary, model_id, created_by, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			projectID, positionID, next, draft.Prompt, draft.PublicSummary, draft.ModelID, requester, now); err != nil {
			return apierrors.New(apierrors.Internal, "revision insert failed").Wrap(err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM position_node_bindings WHERE template_id=$1 AND project_id=$2`, positionID, projectID); err != nil {
			return apierrors.New(apierrors.Internal, "bindings clear failed").Wrap(err)
		}
		if err := writeNodeBindings(ctx, tx, projectID, positionID, draft.NodeBindings); err != nil {
			return err
		}
		if _, err := events.AppendProjectEvent(ctx, tx, projectID, "membership.changed", "position", positionID.String(), &next,
			map[string]any{"change": "revised"}, now); err != nil {
			return err
		}
		out = Position{ID: positionID, Name: name, Status: "active", CurrentVersion: next,
			PublicSummary: draft.PublicSummary, Prompt: draft.Prompt, ModelID: draft.ModelID,
			Revision: next, NodeBindings: draft.NodeBindings}
		return audit.Append(ctx, tx, audit.Entry{
			ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
			Source: audit.SourceWeb, Operation: "position.update",
			ObjectType: "position", ObjectID: positionID.String(), OccurredAt: now,
		})
	})
	return out, err
}

// ListPositions returns positions with their current revision.
func (s *Service) ListPositions(ctx context.Context, requester, projectID uuid.UUID) ([]Position, error) {
	if _, err := s.MembershipFor(ctx, requester, projectID); err != nil {
		return nil, err
	}
	rows, err := paging.Query(ctx, s.pool, `
		SELECT t.id, t.name, t.status, t.current_version, v.public_summary, v.prompt, v.model_id, v.revision
		/*keys*/ FROM position_templates t
		JOIN position_versions v ON v.template_id=t.id AND v.revision=t.current_version
		WHERE t.project_id=$1 /*page*/`, "t.created_at", "t.id", projectID)
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "positions failed").Wrap(err)
	}
	defer rows.Close()
	var out []Position
	for rows.Next() {
		var p Position
		if err := rows.Scan(&p.ID, &p.Name, &p.Status, &p.CurrentVersion, &p.PublicSummary, &p.Prompt, &p.ModelID, &p.Revision); err != nil {
			return nil, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		p.NodeBindings, _ = s.nodeBindingsFor(ctx, projectID, p.ID)
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Service) nodeBindingsFor(ctx context.Context, projectID, positionID uuid.UUID) ([]NodeBinding, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT workflow_id, node_id FROM position_node_bindings WHERE project_id=$1 AND template_id=$2`,
		projectID, positionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NodeBinding
	for rows.Next() {
		var b NodeBinding
		if err := rows.Scan(&b.WorkflowID, &b.NodeID); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func writeNodeBindings(ctx context.Context, tx pgx.Tx, projectID, positionID uuid.UUID, bindings []NodeBinding) error {
	for _, b := range bindings {
		if _, err := tx.Exec(ctx, `
			INSERT INTO position_node_bindings (project_id, template_id, workflow_id, node_id)
			VALUES ($1,$2,$3,$4)`, projectID, positionID, b.WorkflowID, b.NodeID); err != nil {
			return apierrors.New(apierrors.Internal, "binding insert failed").Wrap(err)
		}
	}
	return nil
}

// validateNodeBindings checks every binding references a PUBLISHED workflow
// version containing the node (02 §6 节点必须属于已发布 workflow).
func validateNodeBindings(ctx context.Context, tx pgx.Tx, projectID uuid.UUID, bindings []NodeBinding) error {
	for _, b := range bindings {
		var nodesJSON []byte
		err := tx.QueryRow(ctx, `
			SELECT v.nodes_json FROM workflow_definitions d
			JOIN workflow_versions v ON v.id=d.published_version_id
			WHERE d.id=$1 AND d.project_id=$2`, b.WorkflowID, projectID).Scan(&nodesJSON)
		if errors.Is(err, pgx.ErrNoRows) {
			return apierrors.Newf(apierrors.InvalidReference, "workflow %s has no published version", b.WorkflowID)
		}
		if err != nil {
			return apierrors.New(apierrors.Internal, "workflow lookup failed").Wrap(err)
		}
		var nodes []struct {
			ID string `json:"id"`
		}
		if err := jsonUnmarshal(nodesJSON, &nodes); err != nil {
			return apierrors.New(apierrors.Internal, "nodes parse failed").Wrap(err)
		}
		found := false
		for _, n := range nodes {
			if n.ID == b.NodeID {
				found = true
				break
			}
		}
		if !found {
			return apierrors.Newf(apierrors.InvalidReference, "node %s not in published workflow %s", b.NodeID, b.WorkflowID)
		}
	}
	return nil
}
