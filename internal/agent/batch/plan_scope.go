package batch

import (
	"context"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/collaboration"
	"github.com/kakj-go/Judex/internal/work"
)

func loadPlanFacts(ctx context.Context, q collaboration.Reader, project, plan uuid.UUID) ([]string, error) {
	var raw []byte
	err := q.QueryRow(ctx, `SELECT jsonb_build_object('planId',p.id,'title',p.title,'status',p.status,
 'goal',p.goal,'acceptanceCriteria',p.acceptance_criteria,'version',p.version,'workflowId',p.workflow_id,
 'latestAcceptanceId',p.latest_acceptance_id,'discardedAt',p.discarded_at,
 'ownedTasks',(SELECT COALESCE(jsonb_agg(x),'[]'::jsonb) FROM(SELECT t.id,t.title,t.status,t.version,t.workflow_id,t.node_id,t.latest_acceptance_id
 FROM tasks t WHERE t.project_id=p.project_id AND t.plan_id=p.id ORDER BY t.id LIMIT 50)x),
 'ownedTaskCount',(SELECT count(*) FROM tasks t WHERE t.project_id=p.project_id AND t.plan_id=p.id),
 'referenceTasks',(SELECT COALESCE(jsonb_agg(x),'[]'::jsonb) FROM(SELECT t.id,t.title,t.status,t.version,t.plan_id,t.workflow_id,t.node_id,t.latest_acceptance_id
 FROM plan_task_references r JOIN tasks t ON t.project_id=r.project_id AND t.id=r.task_id
 WHERE r.project_id=p.project_id AND r.plan_id=p.id ORDER BY t.id LIMIT 50)x),
 'referenceTaskCount',(SELECT count(*) FROM plan_task_references r WHERE r.project_id=p.project_id AND r.plan_id=p.id))
 FROM plans p WHERE p.project_id=$1 AND p.id=$2`, project, plan).Scan(&raw)
	if err != nil {
		return nil, err
	}
	feedback, err := work.LatestFormalFeedback(ctx, q, project, plan, "plan")
	if err != nil {
		return nil, err
	}
	return []string{"本次提交明确限定的计划（ownedTasks 是本计划工作；referenceTasks 仅作引用证据，不转移归属。各列表首50项，count更大时须用 query_work 分页补读，不宣称完整覆盖）：" + string(raw), "最近计划正式验收/退回/重开反馈：" + string(feedback)}, nil
}
