package batch

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/collaboration"
)

// A human extension must not lose already returned public position opinions.
// Only the coordinator receives them. Private prompts are never selected;
// bindings, position/preference revisions and published versions must match.
func loadPublicOpinions(ctx context.Context, q collaboration.Reader, project, batch uuid.UUID, versions map[string]string) (string, error) {
	versionJSON, err := json.Marshal(versions)
	if err != nil {
		return "", err
	}
	var raw []byte
	err = q.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(x ORDER BY x."identityId"),'[]'::jsonb) FROM (
 SELECT DISTINCT ON(r.identity_id) r.identity_id AS "identityId",r.id AS "runId",r.parent_run_id AS "parentRunId",
 r.binding_version AS "bindingVersion",r.created_at AS "createdAt",
 left(COALESCE(r.budget_snapshot->>'reason',''),6000) AS "publicOpinion",
 char_length(COALESCE(r.budget_snapshot->>'reason',''))>6000 AS truncated
 FROM agent_runs r JOIN agent_identities i ON i.id=r.identity_id AND i.project_id=r.project_id
 JOIN position_templates t ON t.id=i.template_id AND t.project_id=i.project_id AND t.status='active'
 JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version AND b.valid_until IS NULL
 JOIN users u ON u.id=b.user_id AND u.status='active'
 JOIN project_members m ON m.project_id=i.project_id AND m.user_id=b.user_id AND m.state='active'
 LEFT JOIN personal_position_preferences pref ON pref.project_id=i.project_id AND pref.position_id=i.template_id AND pref.user_id=b.user_id
 WHERE r.project_id=$1 AND r.batch_id=$2 AND r.parent_run_id IS NOT NULL AND r.state='succeeded'
 AND i.kind='position' AND i.status='active' AND i.current_binding_version=r.binding_version
 AND (r.manifest->>'positionVersion')::bigint=t.current_version
 AND (r.manifest->>'preferenceRevision')::bigint=COALESCE(pref.revision,0)
 AND r.manifest->'workflowVersions'=$3::jsonb
 ORDER BY r.identity_id,r.created_at DESC,r.id DESC)x`, project, batch, versionJSON).Scan(&raw)
	if err != nil {
		return "", err
	}
	return "本批次已返回的公开岗位意见（仅协调者可用；来源run/binding/模板与偏好revision/流程版本匹配。保留各自读取范围、截断及未覆盖项，仍须对照当前正式事实；失败/私有回声结果不在此列，不替代人工审阅或验收）：" + string(raw), nil
}
