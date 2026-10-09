// SPDX-License-Identifier: Apache-2.0

// Package workflow implements structured process definitions per
// docs/plans/v1/02 §7: nodes/advisoryEdges/hardRules/approvalPolicies are
// the authority (Mermaid is GENERATED, never parsed into permissions);
// published versions are immutable; publishing validates node ids, edge
// references and hard-rule cycles.
package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/kakj-go/Judex/internal/platform/paging"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/events"
)

// Node is one process node (structured authority).
type Node struct {
	Kind                  string   `json:"kind,omitempty"`
	Phase                 string   `json:"phase,omitempty"`
	DelegationUserIDs     []string `json:"delegationUserIds,omitempty"`
	ID                    string   `json:"id"`
	Name                  string   `json:"name"`
	Responsibility        string   `json:"responsibility"`
	AllowedPositionIds    []string `json:"allowedPositionIds"`
	DefaultApprovalPolicy string   `json:"defaultApprovalPolicy"`
}

// Edge is an advisory ordering between nodes (not a permission).
type Edge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Kind  string `json:"kind,omitempty"`
	Label string `json:"label,omitempty"`
}

// HardRule is a whitelisted structured precondition (02 §7).
type HardRule struct {
	NodeID   string `json:"nodeId,omitempty"`
	Kind     string `json:"kind"`
	Phase    string `json:"phase"`
	TargetID string `json:"targetId,omitempty"`
}

// Body is the draft/published definition payload.
type Body struct {
	Name             string            `json:"name"`
	Instructions     string            `json:"instructions"`
	Nodes            []Node            `json:"nodes"`
	AdvisoryEdges    []Edge            `json:"advisoryEdges"`
	HardRules        []HardRule        `json:"hardRules"`
	ApprovalPolicies map[string]string `json:"approvalPolicies"`
}

// Definition is the workflow aggregate head.
type Definition struct {
	PresetID           *string    `json:"presetId,omitempty"`
	ID                 uuid.UUID  `json:"id"`
	Name               string     `json:"name"`
	PublishedVersionID *uuid.UUID `json:"publishedVersionId"`
	HasDraft           bool       `json:"hasDraft"`
	Version            int64      `json:"version"`
}

// Version is one immutable revision (draft or published).
type Version struct {
	DraftHash   string     `json:"draftHash,omitempty"`
	ID          uuid.UUID  `json:"id"`
	Revision    int64      `json:"revision"`
	State       string     `json:"state"`
	Body        *Body      `json:"body,omitempty"`
	Mermaid     string     `json:"mermaid,omitempty"`
	PublishedAt *time.Time `json:"publishedAt"`
}

type Service struct {
	pool *postgres.Pool
	now  func() time.Time
}

func NewService(pool *postgres.Pool, now func() time.Time) *Service {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{pool: pool, now: now}
}

// Validate enforces the structural rules; violations are VALIDATION_ERROR
// with field details (B02).
func Validate(body Body) error {
	if n := utf8.RuneCountInString(strings.TrimSpace(body.Name)); n < 1 || n > 200 {
		return apierrors.Fields("name", "length")
	}
	if len(body.Nodes) == 0 {
		return apierrors.Fields("nodes", "required")
	}
	seen := map[string]bool{}
	for _, node := range body.Nodes {
		if node.ID == "" || strings.ContainsAny(node.ID, " \t\r\n/") {
			return apierrors.Fields("nodes[].id", "format")
		}
		if seen[node.ID] {
			return apierrors.Fields("nodes[].id", "duplicate")
		}
		seen[node.ID] = true
		if node.DefaultApprovalPolicy != "all" && node.DefaultApprovalPolicy != "none" {
			return apierrors.Fields("nodes[].defaultApprovalPolicy", "enum")
		}
		for _, id := range append(append([]string{}, node.AllowedPositionIds...), node.DelegationUserIDs...) {
			if _, err := uuid.Parse(id); err != nil {
				return apierrors.Fields("nodes[].allowedPositionIds", "invalid")
			}
		}
	}
	for _, edge := range body.AdvisoryEdges {
		if !seen[edge.From] || !seen[edge.To] {
			return apierrors.Fields("advisoryEdges", "unknown-node")
		}
		if edge.From == edge.To {
			return apierrors.Fields("advisoryEdges", "self")
		}
	}
	if err := validateGraphAnnotations(body); err != nil {
		return err
	}
	if hasCycle(body.Nodes, forwardEdges(body.AdvisoryEdges)) {
		return apierrors.Fields("advisoryEdges", "cycle")
	}
	for i, rule := range body.HardRules {
		if rule.NodeID != "" && !seen[rule.NodeID] {
			return apierrors.Fields("hardRules.nodeId", "unknown-node")
		}
		if rule.TargetID != "" {
			if _, err := uuid.Parse(rule.TargetID); err != nil {
				return apierrors.Fields("hardRules.targetId", "uuid")
			}
		}
		switch rule.Kind {
		case "task_acceptance", "handoff_receipt", "material_ready":
		default:
			return apierrors.Fields(fmt.Sprintf("hardRules[%d].kind", i), "whitelist")
		}
		switch rule.Phase {
		case "start", "accept", "both":
		default:
			return apierrors.Fields(fmt.Sprintf("hardRules[%d].phase", i), "enum")
		}
	}
	for nodeID, policy := range body.ApprovalPolicies {
		if !seen[nodeID] {
			return apierrors.Fields("approvalPolicies", "unknown-node")
		}
		if policy != "all" && policy != "none" {
			return apierrors.Fields("approvalPolicies", "enum")
		}
	}
	return nil
}

func hasCycle(nodes []Node, edges []Edge) bool {
	adj := map[string][]string{}
	for _, e := range edges {
		adj[e.From] = append(adj[e.From], e.To)
	}
	state := map[string]int{} // 0 unvisited, 1 in-stack, 2 done
	var visit func(string) bool
	visit = func(n string) bool {
		state[n] = 1
		for _, next := range adj[n] {
			switch state[next] {
			case 1:
				return true
			case 0:
				if visit(next) {
					return true
				}
			}
		}
		state[n] = 2
		return false
	}
	for _, n := range nodes {
		if state[n.ID] == 0 && visit(n.ID) {
			return true
		}
	}
	return false
}

// ConstraintHash fingerprints the permission-relevant parts for review
// impact analysis (02 §7 服务端比较规范化约束 hash)。
func ConstraintHash(body Body) string {
	nodes := append([]Node(nil), body.Nodes...)
	for i := range nodes {
		// Display grouping and decision shape do not grant business authority.
		nodes[i].Kind = ""
		nodes[i].Phase = ""
	}
	relevant := struct {
		Nodes            []Node            `json:"nodes"`
		HardRules        []HardRule        `json:"hardRules"`
		ApprovalPolicies map[string]string `json:"approvalPolicies"`
	}{nodes, body.HardRules, body.ApprovalPolicies}
	raw, _ := json.Marshal(canonicalize(relevant))
	return fmt.Sprintf("%x", sha256Sum(raw))
}

// Create inserts a new definition with its first draft revision.
func (s *Service) Create(ctx context.Context, requester, projectID uuid.UUID, body Body) (Definition, error) {
	if err := Validate(body); err != nil {
		return Definition{}, err
	}
	var out Definition
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, projectID.String()); err != nil {
			return err
		}
		if _, err := s.membership(ctx, tx, requester, projectID); err != nil {
			return err
		}
		var err error
		out, err = s.insertDefinition(ctx, tx, requester, projectID, body, nil)
		return err
	})
	return out, err
}

// querier abstracts pool/tx so membership checks work in and out of tx.
type querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

// membership checks the caller is an active project member (manager rules
// enforced by callers where 02 §7 requires write rights).
func (s *Service) membership(ctx context.Context, q querier, requester, projectID uuid.UUID) (string, error) {
	var role string
	err := q.QueryRow(ctx, `
		SELECT role FROM project_members WHERE project_id=$1 AND user_id=$2 AND state='active'`,
		projectID, requester).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apierrors.New(apierrors.NotFound, "project not found")
	}
	if err != nil {
		return "", apierrors.New(apierrors.Internal, "membership lookup failed").Wrap(err)
	}
	return role, nil
}

func requireManager(role string) error {
	if role != "owner" && role != "manager" {
		return apierrors.New(apierrors.Forbidden, "manager or owner required")
	}
	return nil
}

// UpdateDraft replaces the draft body (manager; AI may only create drafts).
func (s *Service) UpdateDraft(ctx context.Context, requester, projectID, workflowID uuid.UUID, expectedVersion int64, body Body) (Version, error) {
	if err := Validate(body); err != nil {
		return Version{}, err
	}
	var out Version
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, projectID.String()); err != nil {
			return err
		}
		role, err := s.membership(ctx, tx, requester, projectID)
		if err != nil {
			return err
		}
		if err := requireManager(role); err != nil {
			return err
		}
		var draftID uuid.UUID
		var revision int64
		err = tx.QueryRow(ctx, `
			SELECT id, revision FROM workflow_versions
			WHERE definition_id=$1 AND project_id=$2 AND state='draft' FOR UPDATE`,
			workflowID, projectID).Scan(&draftID, &revision)
		if errors.Is(err, pgx.ErrNoRows) {
			// Create a fresh draft on top of the published version.
			var maxRev int64
			if err := tx.QueryRow(ctx, `
				SELECT COALESCE(max(revision),0) FROM workflow_versions WHERE definition_id=$1 AND project_id=$2`,
				workflowID, projectID).Scan(&maxRev); err != nil {
				return apierrors.New(apierrors.Internal, "revision lookup failed").Wrap(err)
			}
			draftID = uuid.New()
			revision = maxRev + 1
			nodesJSON, _ := json.Marshal(body.Nodes)
			edgesJSON, _ := json.Marshal(body.AdvisoryEdges)
			hardJSON, _ := json.Marshal(body.HardRules)
			policiesJSON, _ := json.Marshal(body.ApprovalPolicies)
			if _, err := tx.Exec(ctx, `
				INSERT INTO workflow_versions (project_id, id, definition_id, revision, state, instructions,
					nodes_json, advisory_edges_json, approval_policies_json, hard_rules_json, mermaid, author_user_id, created_at,name)
				VALUES ($1,$2,$3,$4,'draft',$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
				projectID, draftID, workflowID, revision, body.Instructions, nodesJSON, edgesJSON, policiesJSON, hardJSON,
				GenerateMermaid(body), requester, s.now(), body.Name); err != nil {
				return apierrors.New(apierrors.Internal, "draft create failed").Wrap(err)
			}
		} else if err != nil {
			return apierrors.New(apierrors.Internal, "draft lookup failed").Wrap(err)
		} else {
			nodesJSON, _ := json.Marshal(body.Nodes)
			edgesJSON, _ := json.Marshal(body.AdvisoryEdges)
			hardJSON, _ := json.Marshal(body.HardRules)
			policiesJSON, _ := json.Marshal(body.ApprovalPolicies)
			if _, err := tx.Exec(ctx, `
				UPDATE workflow_versions SET instructions=$3, nodes_json=$4, advisory_edges_json=$5,
					approval_policies_json=$6, hard_rules_json=$7, mermaid=$8,name=$9
				WHERE id=$1 AND project_id=$2`,
				draftID, projectID, body.Instructions, nodesJSON, edgesJSON, policiesJSON, hardJSON, GenerateMermaid(body), body.Name); err != nil {
				return apierrors.New(apierrors.Internal, "draft update failed").Wrap(err)
			}
		}
		tag, err := tx.Exec(ctx, `
			UPDATE workflow_definitions SET name=$3, version=version+1, updated_at=$4
			WHERE id=$1 AND project_id=$2 AND version=$5`,
			workflowID, projectID, body.Name, s.now(), expectedVersion)
		if err != nil || tag.RowsAffected() == 0 {
			return apierrors.New(apierrors.VersionConflict, "workflow version conflict")
		}
		out = Version{ID: draftID, Revision: revision, State: "draft", Body: &body, Mermaid: GenerateMermaid(body)}
		if _, err := events.AppendProjectEvent(ctx, tx, projectID, "workflow.published", "workflow", workflowID.String(), &revision,
			map[string]any{"change": "draft_updated"}, s.now()); err != nil {
			return err
		}
		return audit.Append(ctx, tx, audit.Entry{ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
			Source: audit.SourceWeb, Operation: "workflow.draft.update", ObjectType: "workflow", ObjectID: workflowID.String(), OccurredAt: s.now()})
	})
	return out, err
}

// Publish freezes the current draft as an immutable published version
// (manager human action; AI cannot call this — 02 §7).
func (s *Service) Publish(ctx context.Context, requester, projectID, workflowID uuid.UUID, expectedVersion int64, draftHash string) (Version, error) {
	var out Version
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, projectID.String()); err != nil {
			return err
		}
		role, err := s.membership(ctx, tx, requester, projectID)
		if err != nil {
			return err
		}
		if err := requireManager(role); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `
			UPDATE workflow_definitions SET version=version+1, updated_at=$3 WHERE id=$1 AND project_id=$2 AND version=$4`,
			workflowID, projectID, s.now(), expectedVersion)
		if err != nil || tag.RowsAffected() == 0 {
			return apierrors.New(apierrors.VersionConflict, "workflow version conflict")
		}
		var (
			versionID          uuid.UUID
			revision           int64
			nodesJSON          []byte
			edgesJSON          []byte
			hardJSON           []byte
			policyJSON         []byte
			mermaid            string
			name, instructions string
		)
		err = tx.QueryRow(ctx, `
			SELECT id, revision, nodes_json, advisory_edges_json, hard_rules_json, approval_policies_json, mermaid,name,instructions
			FROM workflow_versions WHERE definition_id=$1 AND project_id=$2 AND state='draft' FOR UPDATE`,
			workflowID, projectID).Scan(&versionID, &revision, &nodesJSON, &edgesJSON, &hardJSON, &policyJSON, &mermaid, &name, &instructions)
		if errors.Is(err, pgx.ErrNoRows) {
			return apierrors.New(apierrors.NotFound, "no draft to publish")
		}
		if err != nil {
			return apierrors.New(apierrors.Internal, "draft lookup failed").Wrap(err)
		}
		if draftHash != "" && draftHash != draftReviewHash(name, instructions, nodesJSON, edgesJSON, hardJSON, policyJSON) {
			return apierrors.New(apierrors.ReviewStale, "draft changed since review")
		}
		if err = validateDelegations(ctx, tx, projectID, workflowID, role, nodesJSON); err != nil {
			return err
		}
		publishedAt := s.now()
		if _, err := tx.Exec(ctx, `
			UPDATE workflow_versions SET state='published', published_at=$3 WHERE id=$1 AND project_id=$2`,
			versionID, projectID, publishedAt); err != nil {
			return apierrors.New(apierrors.Internal, "publish failed").Wrap(err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE workflow_definitions SET published_version_id=$2 WHERE id=$1`, workflowID, versionID); err != nil {
			return apierrors.New(apierrors.Internal, "head update failed").Wrap(err)
		}
		if _, err := events.AppendProjectEvent(ctx, tx, projectID, "workflow.published", "workflow", workflowID.String(), &revision,
			map[string]any{"revision": revision}, publishedAt); err != nil {
			return err
		}
		out = Version{ID: versionID, Revision: revision, State: "published", Mermaid: mermaid, PublishedAt: &publishedAt}
		return audit.Append(ctx, tx, audit.Entry{
			ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
			Source: audit.SourceWeb, Operation: "workflow.publish",
			ObjectType: "workflow", ObjectID: workflowID.String(), OccurredAt: publishedAt,
		})
	})
	return out, err
}

// List returns the project's workflow definitions.
func (s *Service) List(ctx context.Context, requester, projectID uuid.UUID) ([]Definition, error) {
	if _, err := s.membership(ctx, s.pool, requester, projectID); err != nil {
		return nil, err
	}
	rows, err := paging.Query(ctx, s.pool, `
		SELECT d.id, d.name, d.published_version_id,
		       EXISTS(SELECT 1 FROM workflow_versions v WHERE v.definition_id=d.id AND v.state='draft'),
		       d.version, d.preset_id
		/*keys*/ FROM workflow_definitions d WHERE d.project_id=$1 /*page*/`, "d.created_at", "d.id", projectID)
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "list failed").Wrap(err)
	}
	defer rows.Close()
	var out []Definition
	for rows.Next() {
		var d Definition
		if err := rows.Scan(&d.ID, &d.Name, &d.PublishedVersionID, &d.HasDraft, &d.Version, &d.PresetID); err != nil {
			return nil, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ListVersions returns revisions (published bodies included, draft last).
func (s *Service) ListVersions(ctx context.Context, requester, projectID, workflowID uuid.UUID, includeDraft bool) ([]Version, error) {
	if _, err := s.membership(ctx, s.pool, requester, projectID); err != nil {
		return nil, err
	}
	rows, err := paging.Query(ctx, s.pool, `
		SELECT id, revision, state, instructions, nodes_json, advisory_edges_json, hard_rules_json,
		       approval_policies_json, mermaid, published_at,name
		/*keys*/ FROM workflow_versions WHERE definition_id=$1 AND project_id=$2
		  AND (state='published' OR $3)
		/*page*/`, "created_at", "id", workflowID, projectID, includeDraft)
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "versions failed").Wrap(err)
	}
	defer rows.Close()
	var out []Version
	for rows.Next() {
		var (
			v          Version
			instr      string
			name       string
			nodesJSON  []byte
			edgesJSON  []byte
			hardJSON   []byte
			policyJSON []byte
		)
		if err := rows.Scan(&v.ID, &v.Revision, &v.State, &instr, &nodesJSON, &edgesJSON, &hardJSON, &policyJSON, &v.Mermaid, &v.PublishedAt, &name); err != nil {
			return nil, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		body := &Body{Name: name, Instructions: instr}
		_ = json.Unmarshal(nodesJSON, &body.Nodes)
		_ = json.Unmarshal(edgesJSON, &body.AdvisoryEdges)
		_ = json.Unmarshal(hardJSON, &body.HardRules)
		_ = json.Unmarshal(policyJSON, &body.ApprovalPolicies)
		v.Body = body
		v.DraftHash = draftReviewHash(name, instr, nodesJSON, edgesJSON, hardJSON, policyJSON)
		out = append(out, v)
	}
	return out, rows.Err()
}

func hashBytes(parts ...[]byte) string {
	var b []byte
	for _, p := range parts {
		b = append(b, p...)
	}
	return fmt.Sprintf("%x", sha256Sum(b))
}

func sha256Sum(b []byte) []byte {
	return __sha256(b)
}

func canonicalize(v any) any {
	switch value := v.(type) {
	case []Node:
		sorted := append([]Node(nil), value...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
		return sorted
	case struct {
		Nodes            []Node            `json:"nodes"`
		HardRules        []HardRule        `json:"hardRules"`
		ApprovalPolicies map[string]string `json:"approvalPolicies"`
	}:
		value.Nodes = canonicalize(value.Nodes).([]Node)
		return value
	default:
		return v
	}
}

func draftReviewHash(name, instructions string, nodes, edges, rules, policies []byte) string {
	raw, _ := json.Marshal(map[string]any{"schemaVersion": 1, "name": name, "instructions": instructions, "nodes": json.RawMessage(nodes), "advisoryEdges": json.RawMessage(edges), "hardRules": json.RawMessage(rules), "approvalPolicies": json.RawMessage(policies)})
	return fmt.Sprintf("%x", sha256Sum(raw))
}
