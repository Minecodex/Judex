package work

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func requirementEvidence(ctx context.Context, q dbQuery, project uuid.UUID, requirements []Requirement) ([]map[string]any, error) {
	out := []map[string]any{}
	for _, r := range requirements {
		proof := map[string]any{"kind": r.Kind, "targetId": r.TargetID}
		var state string
		var version int64
		var reference *uuid.UUID
		var hash string
		var err error
		switch r.Kind {
		case "task_acceptance":
			err = q.QueryRow(ctx, `SELECT status,version,latest_acceptance_id FROM tasks WHERE project_id=$1 AND id=$2`, project, r.TargetID).Scan(&state, &version, &reference)
		case "handoff_receipt":
			err = q.QueryRow(ctx, `SELECT COALESCE(v.state,'draft'),COALESCE(v.revision,0),s.current_source_version_id FROM handoff_sources s LEFT JOIN source_versions v ON v.id=s.current_source_version_id WHERE s.project_id=$1 AND s.id=$2`, project, r.TargetID).Scan(&state, &version, &reference)
		case "material_ready":
			err = q.QueryRow(ctx, `SELECT state,revision,sha256 FROM material_versions WHERE project_id=$1 AND id=$2`, project, r.TargetID).Scan(&state, &version, &hash)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			state = "missing"
		} else if err != nil {
			return nil, err
		}
		proof["state"] = state
		proof["version"] = version
		proof["reference"] = reference
		proof["sha256"] = hash
		out = append(out, proof)
	}
	return out, nil
}
