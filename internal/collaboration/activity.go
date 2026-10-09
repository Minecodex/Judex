package collaboration

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/paging"
)

// A formal report with a backing submission contributes exactly one activity.
const activityCTE = `WITH activities AS (
 SELECT r.id,r.project_id,r.task_id,r.report_kind AS kind,'report'::text AS source_type,
 r.source,COALESCE(r.progress_hint,'') AS text,r.created_by AS actor_user_id,r.identity_id,r.created_at,
 ARRAY(SELECT material_version_id::text FROM work_report_materials WHERE report_id=r.id ORDER BY material_version_id) AS material_ids,
 (SELECT topic_id FROM submissions WHERE id=r.submission_id) AS original_topic
 FROM work_reports r WHERE r.project_id=$1 AND r.task_id=$2
 UNION ALL
 SELECT s.id,s.project_id,s.task_id,CASE WHEN s.discussion_intent='question' THEN 'question' WHEN s.discussion_intent='reply' THEN 'reply' ELSE 'message' END,
 'submission',s.source,s.text,s.actor_user_id,s.identity_id,s.created_at,
 ARRAY(SELECT material_version_id::text FROM submission_materials WHERE submission_id=s.id ORDER BY material_version_id),s.topic_id
 FROM submissions s WHERE s.project_id=$1 AND s.task_id=$2 AND s.status='ready'
 AND NOT EXISTS(SELECT 1 FROM work_reports r WHERE r.submission_id=s.id)
	 UNION ALL
	 SELECT w.id,w.project_id,w.object_id,'decision','work_change',w.source,w.text,w.actor_user_id,NULL::uuid,w.created_at,ARRAY[]::text[],NULL::uuid FROM work_change_records w WHERE w.project_id=$1 AND w.object_type='task' AND w.object_id=$2
	 ) `

func (s *Service) ListActivity(ctx context.Context, user, project, task uuid.UUID) ([]Activity, error) {
	if err := Member(ctx, s.Pool, user, project); err != nil {
		return nil, err
	}
	var exists bool
	if err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tasks WHERE project_id=$1 AND id=$2)`, project, task).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, apierrors.New(apierrors.NotFound, "task not found")
	}
	rows, err := paging.Query(ctx, s.Pool, activityCTE+`SELECT a.id,a.task_id,a.kind,a.source_type,a.source,a.text,a.actor_user_id,COALESCE(u.display_name,''),a.identity_id,a.created_at,a.material_ids,
 ARRAY(SELECT DISTINCT x.id::text FROM (
 SELECT topic_id AS id FROM topic_source_refs WHERE project_id=$1 AND source_type=a.source_type AND source_id=a.id
 UNION SELECT a.original_topic WHERE a.original_topic IS NOT NULL)x),
 (SELECT row_to_json(x) FROM (SELECT id,task_id AS "taskId",batch_id AS "batchId",source_type AS "sourceType",source_id AS "sourceId",state,summary,disagreements,error_code AS "errorCode",updated_at AS "updatedAt"
 ,basis FROM task_analyses ta WHERE ta.project_id=a.project_id AND ta.source_type=a.source_type AND ta.source_id=a.id)x),
 COALESCE((SELECT jsonb_agg(jsonb_build_object('versionId',v.id,'name',m.title) ORDER BY v.id)
 FROM material_versions v JOIN materials m ON m.id=v.material_id WHERE v.project_id=$1 AND v.id::text=ANY(a.material_ids)),'[]'::jsonb)
 /*keys*/ FROM activities a LEFT JOIN users u ON u.id=a.actor_user_id WHERE true /*page*/`, "a.created_at", "a.id", project, task)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Activity{}
	for rows.Next() {
		var a Activity
		var analysis, materials []byte
		if err = rows.Scan(&a.ID, &a.TaskID, &a.Kind, &a.SourceType, &a.Source, &a.Text, &a.ActorUserID, &a.ActorName, &a.IdentityID, &a.CreatedAt, &a.MaterialVersionIDs, &a.TopicIDs, &analysis, &materials); err != nil {
			return nil, err
		}
		if len(analysis) > 0 {
			a.Analysis = &Analysis{}
			if err = json.Unmarshal(analysis, a.Analysis); err != nil {
				return nil, err
			}
		}
		if err = json.Unmarshal(materials, &a.Materials); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

type TaskSummary struct {
	TaskID         uuid.UUID `json:"taskId"`
	ActivityCount  int       `json:"activityCount"`
	LatestActivity *Activity `json:"latestActivity"`
}
type PlanSummary struct {
	PlanID      uuid.UUID     `json:"planId"`
	MainTopicID *uuid.UUID    `json:"mainTopicId"`
	Tasks       []TaskSummary `json:"tasks"`
	Suggestions []Suggestion  `json:"suggestions"`
}

func (s *Service) PlanSummary(ctx context.Context, user, project, plan uuid.UUID) (PlanSummary, error) {
	out := PlanSummary{PlanID: plan, Tasks: []TaskSummary{}, Suggestions: []Suggestion{}}
	if err := Member(ctx, s.Pool, user, project); err != nil {
		return out, err
	}
	if err := s.Pool.QueryRow(ctx, `SELECT main_topic_id FROM plans WHERE project_id=$1 AND id=$2`, project, plan).Scan(&out.MainTopicID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return out, apierrors.New(apierrors.NotFound, "plan not found")
		}
		return out, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT id FROM tasks WHERE project_id=$1 AND (plan_id=$2 OR id IN(SELECT task_id FROM plan_task_references WHERE plan_id=$2)) ORDER BY created_at,id`, project, plan)
	if err != nil {
		return out, err
	}
	ids := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return out, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	// One page per visible task, never an unbounded copy of task histories.
	for _, id := range ids {
		task := TaskSummary{TaskID: id}
		if err = s.Pool.QueryRow(ctx, activityCTE+`SELECT count(*) FROM activities`, project, id).Scan(&task.ActivityCount); err != nil {
			return out, err
		}
		firstCtx, _ := paging.Parse(ctx, "plan-summary:"+plan.String()+":"+id.String(), mapValues("limit", "1"))
		items, e := s.ListActivity(firstCtx, user, project, id)
		if e != nil {
			return out, e
		}
		if len(items) > 0 {
			task.LatestActivity = &items[0]
		}
		out.Tasks = append(out.Tasks, task)
	}
	out.Suggestions, err = s.ListSuggestions(ctx, user, project, &plan, nil)
	return out, err
}
