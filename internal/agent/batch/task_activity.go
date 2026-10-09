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
	execution, err := work.ReadTaskExecutionFacts(ctx, q, project, task)
	if err != nil {
		return nil, err
	}
	runtimeJSON, err := json.Marshal(execution)
	if err != nil {
		return nil, err
	}
	out = append(out, "当前执行条件与运行例外（只用于分析，正式决定由有权人确认）："+string(runtimeJSON))
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
	var task *uuid.UUID
	if err := q.QueryRow(ctx, `SELECT task_id FROM discussion_batches WHERE id=$1 AND project_id=$2`, batch, project).Scan(&task); err != nil {
		return nil, err
	}
	if task != nil {
		rows, err := loadTaskFacts(ctx, q, project, *task)
		if err != nil {
			return nil, err
		}
		facts.WorkFacts = rows
		if facts.IdentityID == nil {
			facts.PositionPrompt += "\n本次是一个任务上报的分析批次。只围绕当前任务、必要依赖和已发布流程核对，不把其他任务的聊天混入。按需调用已分配身份。结束前调用 record_task_analysis 保存结构化公开摘要与异议；只有需要持续协商时才提供 discussion 建议。讨论建议必须由人选择承载位置。"
		}
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

func taskAllowsIdentity(ctx context.Context, q collaboration.Reader, project, task, identity uuid.UUID) (bool, error) {
	var allowed bool
	err := q.QueryRow(ctx, `SELECT EXISTS(
 SELECT 1 FROM task_participants p WHERE p.project_id=$1 AND p.task_id=$2 AND p.identity_id=$3
 UNION ALL SELECT 1 FROM tasks t WHERE t.project_id=$1 AND t.id=$2 AND t.reviewer_identity_id=$3
 UNION ALL SELECT 1 FROM tasks t JOIN workflow_definitions d ON d.id=t.workflow_id
 JOIN workflow_versions v ON v.id=d.published_version_id JOIN agent_identities i ON i.id=$3 AND i.project_id=t.project_id
 CROSS JOIN LATERAL jsonb_array_elements(v.nodes_json)n
 WHERE t.project_id=$1 AND t.id=$2 AND n->>'id'=t.node_id AND (n->'allowedPositionIds') ? i.template_id::text
 )`, project, task, identity).Scan(&allowed)
	return allowed, err
}

func scopedObjects(ctx context.Context, q collaboration.Reader, project, topic uuid.UUID, task *uuid.UUID) ([]string, error) {
	var rows pgx.Rows
	var err error
	if task != nil {
		rows, err = q.Query(ctx, `SELECT DISTINCT id::text FROM(
 SELECT id FROM tasks WHERE project_id=$1 AND id=$2
 UNION SELECT plan_id FROM tasks WHERE project_id=$1 AND id=$2 AND plan_id IS NOT NULL
 UNION SELECT target_id FROM task_requirements WHERE project_id=$1 AND task_id=$2 AND target_id IS NOT NULL)x`, project, *task)
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
