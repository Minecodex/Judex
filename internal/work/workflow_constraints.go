package work

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/google/uuid"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/workflow"
	"sort"
)

// WorkflowConstraint freezes only the rules relevant to a review. Names and
// instructions may change without invalidating an unrelated human decision.
type WorkflowConstraint struct {
	AllowedPositionIDs []uuid.UUID   `json:"allowedPositionIds"`
	WorkflowID         uuid.UUID     `json:"workflowId"`
	VersionID          uuid.UUID     `json:"versionId"`
	NodeID             string        `json:"nodeId"`
	Hash               string        `json:"hash"`
	IdentityIDs        []uuid.UUID   `json:"identityIds"`
	DelegationUserIDs  []uuid.UUID   `json:"delegationUserIds"`
	Requirements       []Requirement `json:"requirements"`
}

func LoadWorkflowConstraint(ctx context.Context, q dbQuery, project, id uuid.UUID, nodeID string) (WorkflowConstraint, error) {
	out := WorkflowConstraint{WorkflowID: id, NodeID: nodeID, IdentityIDs: []uuid.UUID{}, DelegationUserIDs: []uuid.UUID{}, Requirements: []Requirement{}}
	var nodesRaw, rulesRaw, policiesRaw []byte
	if err := q.QueryRow(ctx, `SELECT v.id,v.nodes_json,v.hard_rules_json,v.approval_policies_json FROM workflow_definitions d JOIN workflow_versions v ON v.id=d.published_version_id AND v.project_id=d.project_id WHERE d.project_id=$1 AND d.id=$2 AND v.state='published'`, project, id).Scan(&out.VersionID, &nodesRaw, &rulesRaw, &policiesRaw); err != nil {
		return out, apierrors.New(apierrors.InvalidReference, "published workflow in project required")
	}
	var nodes []workflow.Node
	var rules []workflow.HardRule
	var policies map[string]string
	if err := json.Unmarshal(nodesRaw, &nodes); err != nil {
		return out, err
	}
	if err := json.Unmarshal(rulesRaw, &rules); err != nil {
		return out, err
	}
	if err := json.Unmarshal(policiesRaw, &policies); err != nil {
		return out, err
	}
	selected := []map[string]any{}
	found := nodeID == ""
	identitySet := map[uuid.UUID]bool{}
	delegates := map[uuid.UUID]bool{}
	for _, node := range nodes {
		if nodeID != "" && node.ID != nodeID {
			continue
		}
		found = true
		policy := node.DefaultApprovalPolicy
		if v, ok := policies[node.ID]; ok {
			policy = v
		}
		positions := append([]string{}, node.AllowedPositionIds...)
		for _, raw := range positions {
			parsed, err := uuid.Parse(raw)
			if err != nil {
				return out, apierrors.Fields("allowedPositionIds", "uuid")
			}
			out.AllowedPositionIDs = append(out.AllowedPositionIDs, parsed)
		}
		sort.Strings(positions)
		granted := append([]string{}, node.DelegationUserIDs...)
		sort.Strings(granted)
		selected = append(selected, map[string]any{"id": node.ID, "positions": positions, "approval": policy, "delegationUserIds": granted})
		for _, raw := range granted {
			user, err := uuid.Parse(raw)
			if err != nil {
				return out, apierrors.Fields("delegationUserIds", "uuid")
			}
			delegates[user] = true
		}
		if policy != "all" {
			continue
		}
		for _, position := range positions {
			rows, err := q.Query(ctx, `SELECT i.id FROM agent_identities i JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version AND b.valid_until IS NULL JOIN project_members m ON m.project_id=i.project_id AND m.user_id=b.user_id AND m.state='active' WHERE i.project_id=$1 AND i.template_id=$2 AND i.status='active' ORDER BY i.id`, project, position)
			if err != nil {
				return out, err
			}
			count := 0
			for rows.Next() {
				var identity uuid.UUID
				if err = rows.Scan(&identity); err != nil {
					rows.Close()
					return out, err
				}
				identitySet[identity] = true
				count++
			}
			rows.Close()
			if err = rows.Err(); err != nil {
				return out, err
			}
			if count == 0 {
				return out, apierrors.New(apierrors.RequirementUnmet, "workflow approval position has no active holder")
			}
		}
	}
	if !found {
		return out, apierrors.New(apierrors.InvalidReference, "workflow node not found")
	}
	for _, rule := range rules {
		if rule.NodeID != "" && nodeID != "" && rule.NodeID != nodeID {
			continue
		}
		target, err := uuid.Parse(rule.TargetID)
		if err != nil && rule.TargetID != "" {
			return out, apierrors.New(apierrors.RequirementUnmet, "workflow hard rule requires a concrete project target")
		}
		out.Requirements = append(out.Requirements, Requirement{ID: uuid.NewSHA1(id, []byte(rule.NodeID+rule.Kind+rule.Phase+rule.TargetID)), Phase: rule.Phase, Kind: rule.Kind, TargetID: target, Hard: true, Label: "workflow:" + id.String() + ":" + rule.NodeID})
	}
	for id := range identitySet {
		out.IdentityIDs = append(out.IdentityIDs, id)
	}
	sort.Slice(out.IdentityIDs, func(i, j int) bool { return out.IdentityIDs[i].String() < out.IdentityIDs[j].String() })
	for id := range delegates {
		out.DelegationUserIDs = append(out.DelegationUserIDs, id)
	}
	sort.Slice(out.DelegationUserIDs, func(i, j int) bool { return out.DelegationUserIDs[i].String() < out.DelegationUserIDs[j].String() })
	sort.Slice(selected, func(i, j int) bool { return selected[i]["id"].(string) < selected[j]["id"].(string) })
	sort.Slice(out.Requirements, func(i, j int) bool { return out.Requirements[i].ID.String() < out.Requirements[j].ID.String() })
	raw, _ := json.Marshal(map[string]any{"nodes": selected, "rules": out.Requirements, "identities": out.IdentityIDs})
	hash := sha256.Sum256(raw)
	out.Hash = hex.EncodeToString(hash[:])
	return out, nil
}
func ChangeWorkflowConstraints(ctx context.Context, q dbQuery, project uuid.UUID, changes []Change) ([]WorkflowConstraint, error) {
	out := []WorkflowConstraint{}
	seen := map[string]bool{}
	for _, change := range changes {
		raw, _ := change.Fields["workflowId"].(string)
		node, _ := change.Fields["nodeId"].(string)
		if change.TargetID != "" {
			if target, err := uuid.Parse(change.TargetID); err == nil {
				var workflowID *uuid.UUID
				var storedNode *string
				if change.TargetType == "task" {
					err = q.QueryRow(ctx, `SELECT COALESCE(t.workflow_id,p.workflow_id),t.node_id FROM tasks t LEFT JOIN plans p ON p.id=t.plan_id WHERE t.project_id=$1 AND t.id=$2`, project, target).Scan(&workflowID, &storedNode)
				} else {
					err = q.QueryRow(ctx, `SELECT workflow_id,NULL::text FROM plans WHERE project_id=$1 AND id=$2`, project, target).Scan(&workflowID, &storedNode)
				}
				if err != nil {
					return nil, apierrors.New(apierrors.InvalidReference, "change target not in project")
				}
				if raw == "" && workflowID != nil {
					raw = workflowID.String()
				}
				if node == "" && storedNode != nil {
					node = *storedNode
				}
			}
		}
		if raw == "" {
			if plan, ok := change.Fields["planId"].(string); ok {
				if id, err := uuid.Parse(plan); err == nil {
					var wid *uuid.UUID
					if err = q.QueryRow(ctx, `SELECT workflow_id FROM plans WHERE id=$1 AND project_id=$2`, id, project).Scan(&wid); err != nil {
						return nil, apierrors.New(apierrors.InvalidReference, "owning plan not found")
					}
					if wid != nil {
						raw = wid.String()
					}
				} else {
					for _, other := range changes {
						if other.ClientRef == plan {
							raw, _ = other.Fields["workflowId"].(string)
						}
					}
				}
			}
		}
		if raw == "" {
			if node != "" {
				return nil, apierrors.Fields("nodeId", "workflow required")
			}
			continue
		}
		id, err := uuid.Parse(raw)
		if err != nil {
			return nil, apierrors.Fields("workflowId", "uuid")
		}
		key := raw + ":" + node
		if seen[key] {
			continue
		}
		seen[key] = true
		constraint, err := LoadWorkflowConstraint(ctx, q, project, id, node)
		if err != nil {
			return nil, err
		}
		out = append(out, constraint)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].WorkflowID.String()+out[i].NodeID < out[j].WorkflowID.String()+out[j].NodeID
	})
	return out, nil
}
func TaskWorkflowRequirements(ctx context.Context, q dbQuery, project, task uuid.UUID) ([]Requirement, error) {
	var id *uuid.UUID
	var node *string
	if err := q.QueryRow(ctx, `SELECT COALESCE(t.workflow_id,p.workflow_id),t.node_id FROM tasks t LEFT JOIN plans p ON p.id=t.plan_id WHERE t.id=$1 AND t.project_id=$2`, task, project).Scan(&id, &node); err != nil {
		return nil, err
	}
	if id == nil {
		return nil, nil
	}
	nodeID := ""
	if node != nil {
		nodeID = *node
	}
	constraint, err := LoadWorkflowConstraint(ctx, q, project, *id, nodeID)
	if err != nil {
		return nil, err
	}
	out := []Requirement{}
	for _, r := range constraint.Requirements {
		if r.TargetID != uuid.Nil {
			out = append(out, r)
			continue
		}
		rows, err := q.Query(ctx, `SELECT target_id,material_version_id FROM task_requirements WHERE project_id=$1 AND task_id=$2 AND kind=$3 AND hard=true AND (phase=$4 OR phase='both')`, project, task, r.Kind, r.Phase)
		if err != nil {
			return nil, err
		}
		found := false
		for rows.Next() {
			bound := r
			if err = rows.Scan(&bound.TargetID, &bound.MaterialVersionID); err != nil {
				rows.Close()
				return nil, err
			}
			out = append(out, bound)
			found = true
		}
		rows.Close()
		if err = rows.Err(); err != nil {
			return nil, err
		}
		if !found {
			out = append(out, r)
		}
	}
	return out, nil
}

func validateWorkflowParticipants(ctx context.Context, q dbQuery, project uuid.UUID, refs []WorkflowConstraint, identities []uuid.UUID) error {
	for _, constraint := range refs {
		if constraint.NodeID == "" || len(constraint.AllowedPositionIDs) == 0 {
			continue
		}
		for _, identity := range identities {
			var allowed bool
			if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM agent_identities WHERE id=$1 AND project_id=$2 AND template_id=ANY($3::uuid[]))`, identity, project, constraint.AllowedPositionIDs).Scan(&allowed); err != nil {
				return err
			}
			if !allowed {
				return apierrors.New(apierrors.InvalidReference, "participant position is outside the bound workflow node")
			}
		}
	}
	return nil
}
