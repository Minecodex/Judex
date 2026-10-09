package work

import (
	"context"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/platform/auth"
	"strings"
)

const planCardsSQL = `
		SELECT p.id, p.title, p.goal, p.acceptance_criteria, p.status, p.owner_identity_id,
		       p.workflow_id, p.latest_acceptance_id, p.version, p.created_at,p.main_topic_id,
		       count(t.id), count(t.id) FILTER (WHERE t.status='accepted'),
		       count(t.id) FILTER (WHERE t.status NOT IN ('accepted','cancelled')),
		       count(t.id) FILTER (WHERE t.status='cancelled'),
 count(t.id) FILTER(WHERE t.status NOT IN('accepted','cancelled') AND NOT EXISTS(SELECT 1 FROM task_execution_exceptions x WHERE x.project_id=t.project_id AND x.task_id=t.id AND x.restored_at IS NULL) AND EXISTS(SELECT 1 FROM task_participants tp JOIN agent_identities i ON i.id=tp.identity_id JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version AND b.valid_until IS NULL WHERE tp.task_id=t.id AND b.user_id=$6 AND i.status='active')),
 COALESCE((SELECT u.display_name FROM agent_identities i JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version AND b.valid_until IS NULL JOIN users u ON u.id=b.user_id WHERE i.id=p.owner_identity_id),''),p.updated_at,ARRAY(SELECT r.task_id FROM plan_task_references r WHERE r.project_id=p.project_id AND r.plan_id=p.id ORDER BY r.task_id) /*keys*/
		FROM plans p LEFT JOIN tasks t ON t.project_id=p.project_id AND t.discarded_at IS NULL AND (t.plan_id=p.id OR EXISTS(SELECT 1 FROM plan_task_references r WHERE r.plan_id=p.id AND r.task_id=t.id))
		WHERE p.project_id=$1 AND (p.discarded_at IS NULL OR $2::uuid IS NOT NULL) AND ($2::uuid IS NULL OR p.id=$2)
 AND ($3='' OR strpos(lower(p.title),lower($3))>0) AND ($4='' OR p.status=$4)
 AND (NOT $5 OR EXISTS(SELECT 1 FROM agent_identities i JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version AND b.valid_until IS NULL WHERE i.project_id=p.project_id AND i.status='active' AND b.user_id=$6 AND (i.id=p.owner_identity_id OR EXISTS(SELECT 1 FROM task_participants tp JOIN tasks w ON w.id=tp.task_id AND w.project_id=tp.project_id WHERE tp.identity_id=i.id AND w.plan_id=p.id))))
 /*page*/ GROUP BY p.id`
const myActionsSQL = `
 WITH held AS (
 SELECT i.id,i.project_id FROM agent_identities i JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version AND b.valid_until IS NULL WHERE b.user_id=$1 AND i.status='active'
 ), actions AS (
 SELECT DISTINCT p.project_id,p.id object_id,'proposal'::text object_type,'approve'::text kind,p.current_review_id review_id,v.deadline_at due_at,'待审批提案（'||p.kind||'）' summary,p.created_at FROM proposals p JOIN proposal_versions v ON v.id=p.current_review_id JOIN approval_slots s ON s.review_id=p.current_review_id WHERE p.status='pending' AND s.state='pending' AND ((s.authority_type='user' AND s.authority_id=$1) OR (s.authority_type='identity' AND s.authority_id IN(SELECT id FROM held)))
 UNION ALL
 SELECT DISTINCT h.project_id,h.id,'handoff','receive',NULL::uuid,NULL::timestamptz,'待接收交接来源',h.created_at FROM handoffs h JOIN handoff_sources hs ON hs.handoff_id=h.id JOIN source_versions v ON v.id=hs.current_source_version_id WHERE v.state='pending' AND h.receiver_identity_id IN(SELECT id FROM held)
 UNION ALL
 SELECT DISTINCT h.project_id,h.id,'handoff','submit',NULL::uuid,NULL::timestamptz,'待发送/补交交接来源',h.created_at FROM handoffs h JOIN handoff_sources hs ON hs.handoff_id=h.id LEFT JOIN source_versions v ON v.id=hs.current_source_version_id WHERE (v.id IS NULL OR v.state IN ('draft','rejected')) AND hs.sender_identity_id IN(SELECT id FROM held)
 UNION ALL
 SELECT t.project_id,t.id,'task','accept',NULL::uuid,NULL::timestamptz,'待验收：'||t.title,t.created_at FROM tasks t WHERE t.status='delivered' AND t.discarded_at IS NULL AND NOT EXISTS(SELECT 1 FROM task_execution_exceptions x WHERE x.project_id=t.project_id AND x.task_id=t.id AND x.restored_at IS NULL) AND t.reviewer_identity_id IN(SELECT id FROM held)
 UNION ALL
 SELECT p.project_id,p.id,'plan','accept',NULL::uuid,NULL::timestamptz,'必需任务已满足，待计划整体核对：'||p.title,p.created_at FROM plans p WHERE p.status='active' AND p.owner_identity_id IN(SELECT id FROM held) AND EXISTS(SELECT 1 FROM tasks t WHERE t.plan_id=p.id OR EXISTS(SELECT 1 FROM plan_task_references r WHERE r.plan_id=p.id AND r.task_id=t.id)) AND NOT EXISTS(SELECT 1 FROM tasks t WHERE (t.plan_id=p.id OR EXISTS(SELECT 1 FROM plan_task_references r WHERE r.plan_id=p.id AND r.task_id=t.id)) AND t.discarded_at IS NULL AND t.status NOT IN ('draft','accepted','cancelled') AND (t.plan_id IS DISTINCT FROM p.id OR NOT EXISTS(SELECT 1 FROM task_execution_exceptions x WHERE x.project_id=t.project_id AND x.task_id=t.id AND x.restored_at IS NULL)))
 ), scoped AS (
 SELECT (md5(a.object_type||':'||a.object_id::text||':'||a.kind))::uuid id,a.* FROM actions a JOIN project_members m ON m.project_id=a.project_id AND m.user_id=$1 AND m.state='active' JOIN projects p ON p.id=a.project_id AND p.status='active' WHERE ($2::uuid IS NULL OR a.project_id=$2) AND (cardinality($3::uuid[])=0 OR a.project_id=ANY($3))
 ) SELECT a.id,a.project_id,a.object_id,a.object_type,a.kind,a.review_id,a.due_at,a.summary,a.created_at /*keys*/ FROM scoped a WHERE ($4='' OR a.kind=$4) AND ($5='' OR ($5='decision' AND a.kind IN ('approve','receive','accept')) OR ($5='execution' AND a.kind IN ('submit','revise','reopen'))) /*page*/`

func (s *Service) projectionCount(ctx context.Context, sql string, args ...any) (n int, err error) {
	sql = strings.ReplaceAll(strings.ReplaceAll(sql, "/*keys*/", ""), "/*page*/", "")
	err = s.pool.QueryRow(ctx, "SELECT count(*) FROM ("+sql+") counted", args...).Scan(&n)
	return
}
func (s *Service) CountPlanCards(ctx context.Context, user, project uuid.UUID, f PlanFilter) (int, error) {
	if _, e := memberTx(ctx, s.pool, project, user); e != nil {
		return 0, e
	}
	return s.projectionCount(ctx, planCardsSQL, project, (*uuid.UUID)(nil), strings.TrimSpace(f.Query), f.Status, f.Mine, user)
}
func (s *Service) CountMyActions(ctx context.Context, user uuid.UUID, project *uuid.UUID, f ActionFilter) (int, error) {
	if e := f.Validate(); e != nil {
		return 0, e
	}
	scope := []uuid.UUID{}
	if actor := auth.FromContext(ctx); actor != nil && actor.Kind == auth.KindCLI {
		scope = actor.ProjectScope
		if len(scope) == 0 {
			return 0, nil
		}
	}
	return s.projectionCount(ctx, myActionsSQL, user, project, scope, f.Kind, f.Category)
}
