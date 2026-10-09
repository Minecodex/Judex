package work

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/google/uuid"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"sort"
)

type ExceptionImpact struct {
	TaskID         uuid.UUID     `json:"taskId"`
	PlanID         *uuid.UUID    `json:"planId"`
	Title          string        `json:"title"`
	Status         string        `json:"status"`
	Version        int64         `json:"version"`
	AlreadyStarted bool          `json:"alreadyStarted"`
	Requirements   []Requirement `json:"requirements"`
}
type ExceptionReview struct {
	TaskID             uuid.UUID           `json:"taskId"`
	TargetVersion      int64               `json:"targetVersion"`
	Operation          string              `json:"operation"`
	ReviewHash         string              `json:"reviewHash"`
	Title              string              `json:"title"`
	Current            *ExecutionException `json:"current"`
	AffectedTasks      []ExceptionImpact   `json:"affectedTasks"`
	ReferencingPlanIDs []uuid.UUID         `json:"referencingPlanIds"`
}

func (s *Service) ExecutionExceptionReview(ctx context.Context, user, project, task uuid.UUID, operation string) (ExceptionReview, error) {
	return s.exceptionReview(ctx, poolAsQuery{s.pool}, user, project, task, operation)
}
func (s *Service) exceptionReview(ctx context.Context, q dbQuery, user, project, task uuid.UUID, operation string) (ExceptionReview, error) {
	out := ExceptionReview{TaskID: task, Operation: operation, AffectedTasks: []ExceptionImpact{}, ReferencingPlanIDs: []uuid.UUID{}}
	access, err := workAccess(ctx, q, project, user, task, "task")
	if err != nil {
		return out, err
	}
	if operation != "skip" && operation != "restore" {
		return out, apierrors.Fields("operation", "enum")
	}
	role, err := memberTx(ctx, q, project, user)
	if err != nil {
		return out, err
	}
	if role != "owner" && role != "manager" {
		return out, apierrors.New(apierrors.Forbidden, "execution exception requires project manager")
	}
	if operation == "skip" && !access.Capabilities.Skip || operation == "restore" && !access.Capabilities.Restore {
		return out, apierrors.New(apierrors.InvalidTransition, "task cannot apply this execution exception")
	}
	var status string
	if err = q.QueryRow(ctx, `SELECT title,status,version FROM tasks WHERE project_id=$1 AND id=$2`, project, task).Scan(&out.Title, &status, &out.TargetVersion); err != nil {
		return out, err
	}
	out.Current, err = activeException(ctx, q, project, task)
	if err != nil {
		return out, err
	}
	rows, err := q.Query(ctx, `SELECT id,plan_id,title,status,version FROM tasks WHERE project_id=$1 AND discarded_at IS NULL AND status NOT IN('draft','cancelled') ORDER BY id`, project)
	if err != nil {
		return out, err
	}
	all := []ExceptionImpact{}
	for rows.Next() {
		v := ExceptionImpact{Requirements: []Requirement{}}
		if err = rows.Scan(&v.TaskID, &v.PlanID, &v.Title, &v.Status, &v.Version); err != nil {
			rows.Close()
			return out, err
		}
		v.AlreadyStarted = v.Status == "working" || v.Status == "delivered" || v.Status == "accepted" || v.Status == "rework"
		all = append(all, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	requirements := map[uuid.UUID][]Requirement{}
	constraintManifest := map[string]any{}
	for _, v := range all {
		requirements[v.TaskID], err = rawRequirements(ctx, q, project, v.TaskID)
		if err != nil {
			return out, err
		}
		refs, e := ChangeWorkflowConstraints(ctx, q, project, []Change{{TargetType: "task", TargetID: v.TaskID.String()}})
		if e != nil {
			return out, e
		}
		constraintManifest[v.TaskID.String()] = refs
	}
	affected := map[uuid.UUID]bool{task: true}
	changed := true
	for changed {
		changed = false
		for _, v := range all {
			if affected[v.TaskID] {
				continue
			}
			for _, r := range requirements[v.TaskID] {
				if r.Kind == "task_acceptance" && affected[r.TargetID] {
					affected[v.TaskID] = true
					changed = true
					break
				}
			}
		}
	}
	for _, v := range all {
		if v.TaskID == task || !affected[v.TaskID] {
			continue
		}
		for _, r := range requirements[v.TaskID] {
			if r.Kind == "task_acceptance" && r.TargetID == task {
				r.Fingerprint = requirementFingerprint(r)
				v.Requirements = append(v.Requirements, r)
			}
		}
		out.AffectedTasks = append(out.AffectedTasks, v)
	}
	rows, err = q.Query(ctx, `SELECT plan_id FROM plan_task_references r JOIN plans p ON p.project_id=r.project_id AND p.id=r.plan_id WHERE r.project_id=$1 AND r.task_id=$2 AND p.discarded_at IS NULL ORDER BY plan_id`, project, task)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return out, err
		}
		out.ReferencingPlanIDs = append(out.ReferencingPlanIDs, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	raw, err := json.Marshal(map[string]any{"review": out, "status": status, "requirements": requirements, "constraints": constraintManifest, "role": role})
	if err != nil {
		return out, err
	}
	sum := sha256.Sum256(raw)
	out.ReviewHash = hex.EncodeToString(sum[:])
	return out, nil
}

type ExceptionCommand struct {
	ExpectedVersion    int64             `json:"expectedVersion"`
	ReviewHash         string            `json:"reviewHash"`
	Reason             string            `json:"reason"`
	Waivers            []ExceptionWaiver `json:"waivers"`
	AcknowledgeStarted bool              `json:"acknowledgeStarted"`
}

func validateWaivers(review ExceptionReview, selected []ExceptionWaiver) ([]ExceptionWaiver, error) {
	valid := map[string]bool{}
	for _, v := range review.AffectedTasks {
		for _, r := range v.Requirements {
			if r.Hard && r.Kind == "task_acceptance" {
				valid[v.TaskID.String()+r.ID.String()+r.Fingerprint] = true
			}
		}
	}
	out := []ExceptionWaiver{}
	seen := map[string]bool{}
	for _, w := range selected {
		key := w.TaskID.String() + w.RequirementID.String() + w.Fingerprint
		if !valid[key] || seen[key] {
			return nil, apierrors.Fields("waivers", "invalid, duplicate or stale requirement")
		}
		seen[key] = true
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].TaskID.String()+out[i].RequirementID.String() < out[j].TaskID.String()+out[j].RequirementID.String()
	})
	return out, nil
}
