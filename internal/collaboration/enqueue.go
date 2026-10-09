package collaboration

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/kakj-go/Judex/internal/job"
	"github.com/kakj-go/Judex/internal/platform/events"
	"time"
)

// EnqueueAnalysis is called inside the source transaction. A report and its
// backing submission share the report's canonical source identity.
func EnqueueAnalysis(ctx context.Context, tx pgx.Tx, project, task, user uuid.UUID, sourceType string, sourceID uuid.UUID, replyTopic *uuid.UUID, now time.Time) (uuid.UUID, error) {
	var existing uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT id FROM task_analyses WHERE project_id=$1 AND source_type=$2 AND source_id=$3`, project, sourceType, sourceID).Scan(&existing); err == nil {
		return existing, nil
	} else if err != pgx.ErrNoRows {
		return uuid.Nil, err
	}
	var rounds int
	var coordinator uuid.UUID
	var session uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT max_discussion_rounds FROM projects WHERE id=$1`, project).Scan(&rounds); err != nil {
		return uuid.Nil, err
	}
	if err := tx.QueryRow(ctx, `SELECT id FROM agent_identities WHERE project_id=$1 AND kind='coordinator' AND status='active'`, project).Scan(&coordinator); err != nil {
		return uuid.Nil, err
	}
	if replyTopic == nil {
		if err := tx.QueryRow(ctx, `INSERT INTO agent_sessions(project_id,id,task_id,identity_id,created_at)
 VALUES($1,$2,$3,$4,$5) ON CONFLICT(task_id,identity_id) WHERE task_id IS NOT NULL
 DO UPDATE SET identity_id=EXCLUDED.identity_id RETURNING id`, project, uuid.New(), task, coordinator, now).Scan(&session); err != nil {
			return uuid.Nil, err
		}
	}
	var submission, report any
	if sourceType == "report" {
		report = sourceID
		if replyTopic == nil {
			if err := tx.QueryRow(ctx, `SELECT s.topic_id FROM work_reports r LEFT JOIN submissions s ON s.id=r.submission_id WHERE r.id=$1 AND r.project_id=$2`, sourceID, project).Scan(&replyTopic); err != nil {
				return uuid.Nil, err
			}
		}
	} else {
		submission = sourceID
	}
	if replyTopic != nil {
		if err := tx.QueryRow(ctx, `INSERT INTO agent_sessions(project_id,id,topic_id,identity_id,created_at) VALUES($1,$2,$3,$4,$5) ON CONFLICT(topic_id,identity_id) DO UPDATE SET identity_id=EXCLUDED.identity_id RETURNING id`, project, uuid.New(), *replyTopic, coordinator, now).Scan(&session); err != nil {
			return uuid.Nil, err
		}
	}
	batch := uuid.New()
	if _, err := tx.Exec(ctx, `INSERT INTO discussion_batches(project_id,id,topic_id,task_id,source_submission_id,source_report_id,policy_snapshot,max_rounds,state,trigger_kind,created_at,updated_at)
 VALUES($1,$2,$3,$4,$5,$6,jsonb_build_object('maxRounds',$7::int),$7,'queued','task_activity',$8,$8)`, project, batch, nullID(replyTopic), task, submission, report, rounds, now); err != nil {
		return uuid.Nil, err
	}
	id := uuid.New()
	if _, err := tx.Exec(ctx, `INSERT INTO task_analyses(project_id,id,task_id,batch_id,source_type,source_id,state,parent_topic_id,fork_after_seq,created_at,updated_at)
 SELECT $1,$2,$3,$4,$5,$6,'queued',COALESCE($8::uuid,p.main_topic_id),COALESCE(tp.last_message_seq,0),$7,$7
 FROM tasks t LEFT JOIN plans p ON p.id=t.plan_id AND p.project_id=t.project_id
 LEFT JOIN topics tp ON tp.id=COALESCE($8::uuid,p.main_topic_id) WHERE t.id=$3 AND t.project_id=$1`, project, id, task, batch, sourceType, sourceID, now, nullID(replyTopic)); err != nil {
		return uuid.Nil, err
	}
	run := uuid.New()
	if _, err := tx.Exec(ctx, `INSERT INTO agent_runs(project_id,id,session_id,batch_id,identity_id,state,requested_by,created_at,updated_at) VALUES($1,$2,$3,$4,$5,'queued',$6,$7,$7)`, project, run, session, batch, coordinator, user, now); err != nil {
		return uuid.Nil, err
	}
	unique := "batch:" + batch.String()
	if _, err := job.Enqueue(ctx, tx, "discussion.batch", map[string]string{"projectId": project.String(), "batchId": batch.String()}, &unique, now, now); err != nil {
		return uuid.Nil, err
	}
	_, err := events.AppendProjectEvent(ctx, tx, project, "task.activity.changed", "task", task.String(), nil, map[string]any{"analysisId": id, "sourceType": sourceType, "sourceId": sourceID}, now)
	return id, err
}
