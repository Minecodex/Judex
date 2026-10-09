package work

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/google/uuid"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

// taskReview uses one canonical manifest for display and submission. It
// covers every current contribution, material, condition and reviewer binding.
func (s *Service) taskReview(ctx context.Context, q dbQuery, project, task uuid.UUID) (AcceptanceReview, error) {
	out := AcceptanceReview{ReviewID: uuid.NewString(), TargetType: "task", TargetID: task, Reports: []map[string]any{}}
	var agreement int64
	var reviewer *uuid.UUID
	var binding int64
	err := q.QueryRow(ctx, `SELECT version,agreement_version,reviewer_identity_id FROM tasks WHERE project_id=$1 AND id=$2`, project, task).Scan(&out.TargetVersion, &agreement, &reviewer)
	if err != nil {
		return out, apierrors.New(apierrors.NotFound, "task not found")
	}
	if reviewer != nil {
		if err = q.QueryRow(ctx, `SELECT current_binding_version FROM agent_identities WHERE project_id=$1 AND id=$2`, project, reviewer).Scan(&binding); err != nil {
			return out, err
		}
	}
	rows, err := q.Query(ctx, `SELECT DISTINCT ON(identity_id) id,identity_id,report_kind,COALESCE(progress_hint,''),created_at FROM work_reports
 WHERE project_id=$1 AND task_id=$2 AND agreement_version=$3 ORDER BY identity_id,task_version DESC,id DESC`, project, task, agreement)
	if err != nil {
		return out, err
	}
	var reports []map[string]any
	for rows.Next() {
		var id, identity uuid.UUID
		var kind, text string
		var at any
		if err = rows.Scan(&id, &identity, &kind, &text, &at); err != nil {
			rows.Close()
			return out, err
		}
		reports = append(reports, map[string]any{"reportId": id.String(), "identityId": identity.String(), "kind": kind, "text": text, "createdAt": at})
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return out, err
	}
	for _, r := range reports {
		materials := []string{}
		rows, err = q.Query(ctx, `SELECT material_version_id::text FROM work_report_materials WHERE report_id=$1 ORDER BY material_version_id`, r["reportId"])
		if err != nil {
			return out, err
		}
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return out, err
			}
			materials = append(materials, id)
		}
		rows.Close()
		r["materialVersionIds"] = materials
		out.Reports = append(out.Reports, r)
	}
	requirements, blockers, err := s.RequirementsFor(ctx, q, project, task)
	if err != nil {
		return out, err
	}
	out.Blockers = blockers
	proofs, err := requirementEvidence(ctx, q, project, requirements)
	if err != nil {
		return out, err
	}
	manifest := map[string]any{"schemaVersion": 1, "taskId": task, "version": out.TargetVersion, "agreementVersion": agreement, "reviewer": reviewer, "bindingVersion": binding, "reports": out.Reports, "requirements": requirements, "blockers": blockers, "requirementEvidence": proofs}
	raw, err := json.Marshal(manifest)
	if err != nil {
		return out, err
	}
	hash := sha256.Sum256(raw)
	out.ReviewHash = hex.EncodeToString(hash[:])
	out.Manifest = raw
	return out, nil
}
