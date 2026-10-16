package workflow

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type CandidateReader interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

type Candidate struct {
	WorkflowID        uuid.UUID       `json:"workflowId"`
	VersionID         uuid.UUID       `json:"versionId"`
	Name              string          `json:"name"`
	Instructions      string          `json:"instructions"`
	Nodes             json.RawMessage `json:"nodes"`
	HardRules         json.RawMessage `json:"hardRules"`
	ApprovalPolicies  json.RawMessage `json:"approvalPolicies"`
	FormalTaskBinding bool            `json:"formalTaskBinding"`
	CandidateSources  []string        `json:"candidateSources"`
}

// PublishedCandidates separates formal assignment from plan/position/project
// references. Candidates never write or replace a task's formal binding.
func PublishedCandidates(ctx context.Context, q CandidateReader, project uuid.UUID, task *uuid.UUID, identity uuid.UUID) ([]Candidate, error) {
	return PublishedCandidatesForScope(ctx, q, project, task, nil, identity)
}

func PublishedCandidatesForScope(ctx context.Context, q CandidateReader, project uuid.UUID, task, plan *uuid.UUID, identity uuid.UUID) ([]Candidate, error) {
	rows, err := q.Query(ctx, `SELECT d.id,v.id,d.name,v.instructions,v.nodes_json,v.hard_rules_json,v.approval_policies_json,
 COALESCE(d.id=(SELECT workflow_id FROM tasks WHERE project_id=$1 AND id=$2),false) AS formal,
 COALESCE(d.id=(SELECT p.workflow_id FROM plans p WHERE p.project_id=$1 AND p.id=COALESCE($4::uuid,(SELECT plan_id FROM tasks WHERE project_id=$1 AND id=$2))),false) AS plan_source,
 EXISTS(SELECT 1 FROM position_node_bindings b JOIN agent_identities i ON i.template_id=b.template_id AND i.project_id=b.project_id
 WHERE b.project_id=$1 AND b.workflow_id=d.id AND i.status='active' AND (i.id=$3 OR i.id IN(SELECT identity_id FROM task_participants WHERE project_id=$1 AND task_id=$2)
 OR i.id=(SELECT reviewer_identity_id FROM tasks WHERE project_id=$1 AND id=$2))) AS position_source
 FROM workflow_definitions d JOIN workflow_versions v ON v.id=d.published_version_id AND v.project_id=d.project_id
 WHERE d.project_id=$1 AND v.state='published' ORDER BY formal DESC,plan_source DESC,position_source DESC,d.id`, project, task, identity, plan)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Candidate{}
	for rows.Next() {
		var id, version uuid.UUID
		var name, instructions string
		var nodes, rules, policies json.RawMessage
		var formal, plan, position bool
		if err := rows.Scan(&id, &version, &name, &instructions, &nodes, &rules, &policies, &formal, &plan, &position); err != nil {
			return nil, err
		}
		sources := []string{}
		if formal {
			sources = append(sources, "task")
		}
		if plan {
			sources = append(sources, "plan")
		}
		if position {
			sources = append(sources, "position")
		}
		if len(sources) == 0 {
			sources = append(sources, "project")
		}
		out = append(out, Candidate{WorkflowID: id, VersionID: version, Name: name, Instructions: instructions, Nodes: nodes, HardRules: rules, ApprovalPolicies: policies, FormalTaskBinding: formal, CandidateSources: sources})
	}
	return out, rows.Err()
}
