package batch

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	agentcontext "github.com/kakj-go/Judex/internal/agent/context"
	"github.com/kakj-go/Judex/internal/agent/tools"
	"github.com/kakj-go/Judex/internal/collaboration"
	"github.com/kakj-go/Judex/internal/work"
)

func (e *Executor) ensureTaskSession(ctx context.Context, project, task, identity uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := e.Pool.QueryRow(ctx, `INSERT INTO agent_sessions(project_id,id,task_id,identity_id,created_at)VALUES($1,$2,$3,$4,$5)
 ON CONFLICT(task_id,identity_id) WHERE task_id IS NOT NULL DO UPDATE SET identity_id=EXCLUDED.identity_id RETURNING id`, project, uuid.New(), task, identity, e.now()).Scan(&id)
	return id, err
}

func (e *Executor) ensureProjectSession(ctx context.Context, project, identity uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := e.Pool.QueryRow(ctx, `INSERT INTO agent_sessions(project_id,id,identity_id,created_at) VALUES($1,$2,$3,$4)
 ON CONFLICT(project_id,identity_id) WHERE topic_id IS NULL AND task_id IS NULL DO UPDATE SET identity_id=EXCLUDED.identity_id RETURNING id`, project, uuid.New(), identity, e.now()).Scan(&id)
	return id, err
}
func (e *Executor) activityTools(env *tools.Env, project uuid.UUID) {
	env.RecordTaskAnalysis = func(ctx context.Context, rawRun string, args map[string]any) (map[string]any, error) {
		run, err := uuid.Parse(rawRun)
		if err != nil {
			return nil, err
		}
		raw, err := json.Marshal(args)
		if err != nil {
			return nil, err
		}
		var out collaboration.TaskAnalysisOutput
		if err = json.Unmarshal(raw, &out); err != nil {
			return nil, err
		}
		tx, err := e.beginEffect(ctx, project)
		if err != nil {
			return nil, err
		}
		defer tx.Rollback(ctx)
		id, err := collaboration.RecordTaskOutput(ctx, tx, project, run, out, e.now())
		if err != nil {
			return nil, err
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, err
		}
		return map[string]any{"analysisId": id, "state": "draft", "discussionCreated": false}, nil
	}
}

func loadTaskFacts(ctx context.Context, q collaboration.Reader, project, task uuid.UUID) ([]string, error) {
	var raw []byte
	err := q.QueryRow(ctx, `SELECT jsonb_build_object('taskId',t.id,'title',t.title,'status',t.status,'expectedOutput',t.expected_output,
 'acceptanceCriteria',t.acceptance_criteria,'version',t.version,'agreementVersion',t.agreement_version,'workflowId',t.workflow_id,'nodeId',t.node_id,
 'discardedAt',t.discarded_at,'executionException',(SELECT jsonb_build_object('id',x.id,'previousStatus',x.previous_status,'reason',x.reason,'actorUserId',x.actor_user_id,'createdAt',x.created_at,'waivers',x.waivers_json)FROM task_execution_exceptions x WHERE x.project_id=t.project_id AND x.task_id=t.id AND x.restored_at IS NULL),
 'participants',(SELECT COALESCE(jsonb_agg(jsonb_build_object('identityId',p.identity_id,'responsibility',p.responsibility)),'[]') FROM task_participants p WHERE p.task_id=t.id),
 'requirements',(SELECT COALESCE(jsonb_agg(to_jsonb(r)),'[]') FROM task_requirements r WHERE r.task_id=t.id),
 'plan',(SELECT jsonb_build_object('id',p.id,'title',p.title,'goal',p.goal,'acceptanceCriteria',p.acceptance_criteria,'status',p.status,'version',p.version) FROM plans p WHERE p.id=t.plan_id))
 FROM tasks t WHERE t.id=$2 AND t.project_id=$1`, project, task).Scan(&raw)
	if err != nil {
		return nil, err
	}
	out := []string{"当前正式工作：" + string(raw)}
	feedback, err := work.LatestFormalFeedback(ctx, q, project, task, "task")
	if err != nil {
		return nil, err
	}
	out = append(out, "最近正式验收/退回/重开反馈（保留决定来源；legacy_audit 的旧退回没有完整快照，不补造）："+string(feedback))
	execution, err := work.ReadTaskExecutionFacts(ctx, q, project, task)
	if err != nil {
		return nil, err
	}
	runtimeJSON, err := json.Marshal(execution)
	if err != nil {
		return nil, err
	}
	out = append(out, "当前执行条件与运行例外（只用于分析，正式决定由有权人确认）："+string(runtimeJSON))
	reports, err := loadReportFacts(ctx, q, project, task)
	if err != nil {
		return nil, err
	}
	reportJSON, err := json.Marshal(reports)
	if err != nil {
		return nil, err
	}
	out = append(out, "当前任务的公开报告来源（最近4份，每份正文首2000字符，截断明确标记；固定材料可用read_material按需读，不将知识候选查询无结果当作没有报告，不将metadata当作原件已读）："+string(reportJSON))
	rows, err := q.Query(ctx, `SELECT d.id,d.title,d.status FROM tasks d WHERE d.project_id=$1 AND d.id IN(SELECT target_id FROM task_requirements WHERE task_id=$2 AND kind='task_acceptance') ORDER BY d.id`, project, task)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var title, status string
		if err = rows.Scan(&id, &title, &status); err != nil {
			return nil, err
		}
		out = append(out, fmt.Sprintf("相关前置任务 %s %s %s", id, title, status))
	}
	return out, rows.Err()
}

func (e *Executor) prepareScope(ctx context.Context, q collaboration.Reader, project, topic, batch uuid.UUID, facts *agentcontext.Facts) (*uuid.UUID, error) {
	scope, err := collaboration.ReadAnalysisScope(ctx, q, project, batch)
	if err != nil {
		return nil, err
	}
	task := scope.TaskID
	if task != nil {
		rows, err := loadTaskFacts(ctx, q, project, *task)
		if err != nil {
			return nil, err
		}
		facts.WorkFacts = rows
		if facts.IdentityID == nil {
			facts.PositionPrompt += "\n本次是任务上报分析，不是代替成员重做研发或完整测试。只围绕当前任务、必要依赖和已发布流程核对，不混入其他任务聊天。先核对报告、固定版本和关键来源，再按需向 list_agent 中可委派的身份提出具体问题；已有充分来源时不要逐行重读整个源码包或反复重跑成员测试。材料不足或未覆盖时如实保留缺口，不为追求全量覆盖耗尽预算。取得必要依据后及时调用 record_task_analysis 保存公开摘要、异议和未完成项；分析完成不代表成果通过验收。只有需要持续协商时才给 discussion 建议，人选择承载位置。"
		} else {
			facts.PositionPrompt += "\n你在本任务只提供本岗位意见给协调者，不负责协调者的 record_task_analysis，也不代替成员制作或重跑完整交付。依据具体提问、固定版本和必要来源给出有依据的结论、异议与未覆盖项；来源不足就明确缺口，不反复读取整包以冒充全面核对。"
		}
	}
	if task == nil && scope.PlanID != nil {
		facts.WorkFacts, err = loadPlanFacts(ctx, q, project, *scope.PlanID)
		if err != nil {
			return nil, err
		}
		facts.PositionPrompt += "\n本次提交以 planId 明确限定计划。只围绕原请求和该计划事实分析，其他计划工作只能作为标明来源的背景证据；计划引用不转移任务归属。已接受或取消计划没有当前执行节点，currentNodeCandidates 为空，历史放 relatedEvidenceNodes。查询或流程参考请求不代表建稿、新工作或岗位征询授权；只有原请求要求制作草稿时才建稿。公开摘要列实际来源、异议和未覆盖项，不返回模板占位符。"
	} else if task == nil && topic == uuid.Nil && facts.IdentityID == nil {
		facts.PositionPrompt += "\n本次是无任务、无讨论的项目范围提交分析。原提交已保存；仅按原请求形成分析、澄清或所要求的草稿，不因项目有目标就创建未请求的计划。完成前调用 record_task_analysis 保存可查询的公开摘要、来源依据及实际草稿 ID；创建草稿不表示已经发布或批准。"
	}
	if topic != uuid.Nil {
		after := int64(-1)
		for {
			messages, err := collaboration.ReadHistory(ctx, q, project, topic, 0, after, facts.CoveredSeq, 100)
			if err != nil {
				return nil, err
			}
			for _, m := range messages {
				if m.State == "committed" && (task == nil || m.TaskID == nil || *m.TaskID == *task) {
					facts.RecentMessages = append(facts.RecentMessages, fmt.Sprintf("message:%s:%d [%s] %s", m.OriginTopicID, m.Seq, m.Kind, m.Content))
				}
				after = m.Seq
			}
			if len(messages) < 100 {
				break
			}
		}
	}
	return task, nil
}

// Shared set for discovery and delegation; $1/$2 always name project/task.
const taskAllowedIdentitiesSQL = `
 SELECT p.identity_id FROM task_participants p WHERE p.project_id=$1 AND p.task_id=$2
 UNION SELECT t.reviewer_identity_id FROM tasks t WHERE t.project_id=$1 AND t.id=$2
 UNION SELECT i.id FROM tasks t JOIN workflow_definitions d ON d.id=t.workflow_id
 JOIN workflow_versions v ON v.id=d.published_version_id JOIN agent_identities i ON i.project_id=t.project_id
 CROSS JOIN LATERAL jsonb_array_elements(v.nodes_json)n
 WHERE t.project_id=$1 AND t.id=$2 AND n->>'id'=t.node_id AND (n->'allowedPositionIds') ? i.template_id::text`

func taskAllowsIdentity(ctx context.Context, q collaboration.Reader, project, task, identity uuid.UUID) (bool, error) {
	var allowed bool
	err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM (`+taskAllowedIdentitiesSQL+`) allowed WHERE identity_id=$3)`, project, task, identity).Scan(&allowed)
	return allowed, err
}

func scopedObjects(ctx context.Context, q collaboration.Reader, project, topic uuid.UUID, task *uuid.UUID, scopedPlan ...*uuid.UUID) ([]string, error) {
	var rows pgx.Rows
	var err error
	if task != nil {
		rows, err = q.Query(ctx, `SELECT DISTINCT id::text FROM(
 SELECT id FROM tasks WHERE project_id=$1 AND id=$2
 UNION SELECT plan_id FROM tasks WHERE project_id=$1 AND id=$2 AND plan_id IS NOT NULL
 UNION SELECT target_id FROM task_requirements WHERE project_id=$1 AND task_id=$2 AND target_id IS NOT NULL)x`, project, *task)
	} else if len(scopedPlan) > 0 && scopedPlan[0] != nil {
		rows, err = q.Query(ctx, `SELECT DISTINCT id::text FROM(
 SELECT id FROM plans WHERE project_id=$1 AND id=$2
 UNION SELECT id FROM tasks WHERE project_id=$1 AND plan_id=$2
 UNION SELECT task_id FROM plan_task_references WHERE project_id=$1 AND plan_id=$2)x`, project, scopedPlan[0])
	} else {
		rows, err = q.Query(ctx, `SELECT DISTINCT id::text FROM(
 SELECT object_id AS id FROM topic_work_links WHERE project_id=$1 AND topic_id=$2
 UNION SELECT t.plan_id FROM tasks t JOIN topic_work_links l ON l.project_id=t.project_id AND l.object_type='task' AND l.object_id=t.id WHERE l.project_id=$1 AND l.topic_id=$2 AND t.plan_id IS NOT NULL
 UNION SELECT t.id FROM tasks t WHERE t.project_id=$1 AND t.plan_id IN(SELECT object_id FROM topic_work_links WHERE project_id=$1 AND topic_id=$2 AND object_type='plan')
 UNION SELECT r.task_id FROM plan_task_references r WHERE r.project_id=$1 AND r.plan_id IN(SELECT object_id FROM topic_work_links WHERE project_id=$1 AND topic_id=$2 AND object_type='plan'))x`, project, topic)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
