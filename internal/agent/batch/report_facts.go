package batch

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/collaboration"
)

// Work reports are public work evidence, separate from knowledge candidates.
// Show bounded text plus immutable version IDs so the model can read sources
// with read_material instead of inventing a sandbox path or declaring absence.
func loadReportFacts(ctx context.Context, q collaboration.Reader, project, task uuid.UUID) (map[string]any, error) {
	var raw []byte
	err := q.QueryRow(ctx, `SELECT jsonb_build_object('taskId',t.id,'latestReportId',t.latest_report_id,
 'latestAcceptanceId',t.latest_acceptance_id,
 'recentReports',(SELECT COALESCE(jsonb_agg(x),'[]'::jsonb) FROM(SELECT r.id,r.report_kind AS kind,r.identity_id AS "identityId",
 r.binding_version AS "bindingVersion",r.created_at AS "createdAt",left(COALESCE(r.progress_hint,''),2000) AS text,
 char_length(COALESCE(r.progress_hint,''))>2000 AS "textTruncated",
 (SELECT COALESCE(jsonb_agg(jsonb_build_object('versionId',v.id,'materialId',v.material_id,'sha256',v.sha256,'mime',v.mime,'state',v.state) ORDER BY v.id),'[]'::jsonb)
 FROM work_report_materials m JOIN material_versions v ON v.project_id=m.project_id AND v.id=m.material_version_id WHERE m.project_id=r.project_id AND m.report_id=r.id) AS "materialVersions"
 FROM work_reports r WHERE r.project_id=t.project_id AND r.task_id=t.id ORDER BY r.created_at DESC,r.id DESC LIMIT 4)x),
 'reportCount',(SELECT count(*) FROM work_reports r WHERE r.project_id=t.project_id AND r.task_id=t.id))
 FROM tasks t WHERE t.project_id=$1 AND t.id=$2`, project, task).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	err = json.Unmarshal(raw, &result)
	return result, err
}
