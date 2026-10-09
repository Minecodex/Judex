package work

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/google/uuid"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

func requirementFingerprint(r Requirement) string {
	phase := r.Phase
	if r.originalPhase != "" {
		phase = r.originalPhase
	}
	raw, _ := json.Marshal([]any{r.SourceTaskID, r.ID, r.Kind, r.TargetID, r.MaterialVersionID, phase, r.Hard, r.Label})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func rawRequirements(ctx context.Context, q dbQuery, project, task uuid.UUID) ([]Requirement, error) {
	out := []Requirement{}
	rows, err := q.Query(ctx, `SELECT id,phase,kind,target_id,material_version_id,hard,label FROM task_requirements WHERE project_id=$1 AND task_id=$2 ORDER BY id`, project, task)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var r Requirement
		if err = rows.Scan(&r.ID, &r.Phase, &r.Kind, &r.TargetID, &r.MaterialVersionID, &r.Hard, &r.Label); err != nil {
			rows.Close()
			return nil, err
		}
		r.SourceTaskID = task
		out = append(out, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	workflow, err := TaskWorkflowRequirements(ctx, q, project, task)
	if err != nil {
		return nil, err
	}
	for i := range workflow {
		workflow[i].SourceTaskID = task
	}
	return append(out, workflow...), nil
}
func (s *Service) effectiveRequirements(ctx context.Context, q dbQuery, project, task uuid.UUID) ([]Requirement, error) {
	requirements, err := rawRequirements(ctx, q, project, task)
	if err != nil {
		return nil, err
	}
	out := []Requirement{}
	var expand func(Requirement, map[uuid.UUID]bool) error
	expand = func(r Requirement, seen map[uuid.UUID]bool) error {
		if r.Kind != "task_acceptance" || !r.Hard {
			out = append(out, r)
			return nil
		}
		active, err := activeException(ctx, q, project, r.TargetID)
		if err != nil {
			return err
		}
		waived := false
		if active != nil {
			for _, w := range active.Waivers {
				if w.TaskID == r.SourceTaskID && w.RequirementID == r.ID && w.Fingerprint == requirementFingerprint(r) {
					waived = true
					break
				}
			}
		}
		r.Waived = waived
		out = append(out, r)
		if !waived {
			return nil
		}
		if seen[r.TargetID] {
			return apierrors.New(apierrors.DependencyChanged, "execution exception inheritance contains a cycle")
		}
		nextSeen := map[uuid.UUID]bool{}
		for id, v := range seen {
			nextSeen[id] = v
		}
		nextSeen[r.TargetID] = true
		upstream, err := rawRequirements(ctx, q, project, r.TargetID)
		if err != nil {
			return err
		}
		for _, next := range upstream {
			if !next.Hard {
				continue
			}
			via := r.TargetID
			next.InheritedFrom = &via
			next.originalPhase = next.Phase
			next.Phase = r.Phase
			if err = expand(next, nextSeen); err != nil {
				return err
			}
		}
		return nil
	}
	for _, r := range requirements {
		if err = expand(r, map[uuid.UUID]bool{task: true}); err != nil {
			return nil, err
		}
	}
	return out, nil
}
