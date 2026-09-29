package work

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/google/uuid"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

func (s *Service) planReview(ctx context.Context, q dbQuery, project, plan uuid.UUID) (AcceptanceReview, error) {
	out := AcceptanceReview{ReviewID: uuid.NewString(), TargetType: "plan", TargetID: plan, TaskAcceptanceIDs: []string{}, Blockers: []Blocker{}}
	var title, goal, criteria, state string
	var owner *uuid.UUID
	var binding int64
	err := q.QueryRow(ctx, `SELECT p.version,p.title,p.goal,p.acceptance_criteria,p.status,p.owner_identity_id,COALESCE(i.current_binding_version,0)
 FROM plans p LEFT JOIN agent_identities i ON i.id=p.owner_identity_id WHERE p.project_id=$1 AND p.id=$2`, project, plan).Scan(&out.TargetVersion, &title, &goal, &criteria, &state, &owner, &binding)
	if err != nil {
		return out, apierrors.New(apierrors.NotFound, "plan not found")
	}
	rows, err := q.Query(ctx, `SELECT t.id,t.status,t.latest_acceptance_id,t.title FROM tasks t WHERE t.project_id=$1 AND
 (t.plan_id=$2 OR EXISTS(SELECT 1 FROM plan_task_references r WHERE r.project_id=$1 AND r.plan_id=$2 AND r.task_id=t.id)) ORDER BY t.id`, project, plan)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	tasks := []map[string]any{}
	for rows.Next() {
		var id uuid.UUID
		var state, title string
		var acceptance *uuid.UUID
		if err = rows.Scan(&id, &state, &acceptance, &title); err != nil {
			return out, err
		}
		tasks = append(tasks, map[string]any{"taskId": id, "title": title, "status": state, "acceptanceId": acceptance})
		if state == "accepted" && acceptance != nil {
			out.TaskAcceptanceIDs = append(out.TaskAcceptanceIDs, acceptance.String())
		} else if state != "cancelled" {
			out.Blockers = append(out.Blockers, Blocker{Phase: "accept", ObjectType: "task", ObjectID: id.String(), Reason: "任务尚未有效验收"})
		}
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	raw, err := json.Marshal(map[string]any{"schemaVersion": 1, "planId": plan, "version": out.TargetVersion, "title": title, "goal": goal, "criteria": criteria, "state": state, "owner": owner, "ownerBinding": binding, "tasks": tasks})
	if err != nil {
		return out, err
	}
	hash := sha256.Sum256(raw)
	out.ReviewHash = hex.EncodeToString(hash[:])
	out.Manifest = raw
	return out, nil
}
