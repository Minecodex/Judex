package handoff

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func freezeEvidence(ctx context.Context, tx pgx.Tx, project, task uuid.UUID) ([]byte, error) {
	var evidence []byte
	err := tx.QueryRow(ctx, `SELECT jsonb_build_object('reports',COALESCE(jsonb_agg(jsonb_build_object(
  'reportId',r.id,'identityId',r.identity_id,'text',COALESCE(r.progress_hint,''),'kind',r.report_kind,
  'materialVersionIds',ARRAY(SELECT material_version_id FROM work_report_materials WHERE report_id=r.id ORDER BY material_version_id),
  'codeRefs',r.code_refs_json,'environmentRefs',r.environment_refs_json) ORDER BY r.identity_id),'[]'::jsonb))
  FROM (SELECT DISTINCT ON(identity_id) * FROM work_reports
   WHERE project_id=$1 AND task_id=$2 AND agreement_version=(SELECT agreement_version FROM tasks WHERE id=$2 AND project_id=$1)
   ORDER BY identity_id,task_version DESC,id DESC) r`, project, task).Scan(&evidence)
	return evidence, err
}
