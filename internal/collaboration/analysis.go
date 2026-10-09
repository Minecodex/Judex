package collaboration

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/events"
	"strings"
	"time"
	"unicode/utf8"
)

type DiscussionProposal struct {
	Title           string     `json:"title"`
	Reason          string     `json:"reason"`
	ExistingTopicID *uuid.UUID `json:"existingTopicId,omitempty"`
}
type TaskAnalysisOutput struct {
	Summary       string              `json:"summary"`
	Disagreements []string            `json:"disagreements"`
	Basis         []string            `json:"basis"`
	Discussion    *DiscussionProposal `json:"discussion,omitempty"`
}

func RecordTaskOutput(ctx context.Context, tx pgx.Tx, project, run uuid.UUID, out TaskAnalysisOutput, now time.Time) (uuid.UUID, error) {
	if strings.TrimSpace(out.Summary) == "" || len(out.Summary) > 24000 {
		return uuid.Nil, apierrors.Fields("summary", "length")
	}
	for name, values := range map[string][]string{"basis": out.Basis, "disagreements": out.Disagreements} {
		if len(values) > 32 {
			return uuid.Nil, apierrors.Fields(name, "length")
		}
		for _, value := range values {
			if utf8.RuneCountInString(value) > 4000 {
				return uuid.Nil, apierrors.Fields(name, "length")
			}
		}
	}
	if out.Discussion != nil {
		if utf8.RuneCountInString(strings.TrimSpace(out.Discussion.Title)) < 1 || utf8.RuneCountInString(out.Discussion.Title) > 200 || strings.TrimSpace(out.Discussion.Reason) == "" {
			return uuid.Nil, apierrors.Fields("discussion", "title and reason required")
		}
		if out.Discussion.ExistingTopicID != nil {
			var valid bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM topics WHERE project_id=$1 AND id=$2 AND kind<>'handoff')`, project, *out.Discussion.ExistingTopicID).Scan(&valid); err != nil {
				return uuid.Nil, err
			}
			if !valid {
				return uuid.Nil, apierrors.New(apierrors.InvalidReference, "suggested discussion not found")
			}
		}
	}
	var id uuid.UUID
	if out.Basis == nil {
		out.Basis = []string{}
	}
	if out.Disagreements == nil {
		out.Disagreements = []string{}
	}
	output, _ := json.Marshal(out)
	raw, _ := json.Marshal(out.Discussion)
	if out.Discussion == nil {
		raw = []byte("{}")
	}
	disagreements, _ := json.Marshal(out.Disagreements)
	if out.Disagreements == nil {
		disagreements = []byte("[]")
	}
	basis, _ := json.Marshal(out.Basis)
	if out.Basis == nil {
		basis = []byte("[]")
	}
	err := tx.QueryRow(ctx, `UPDATE task_analyses a SET summary=$3,disagreements=$4,discussion_json=$5,updated_at=$6,basis=$7,output_json=$8
 FROM agent_runs r JOIN projects p ON p.id=r.project_id JOIN agent_identities i ON i.id=r.identity_id AND i.kind='coordinator'
 WHERE r.id=$2 AND r.project_id=$1 AND a.batch_id=r.batch_id AND r.parent_run_id IS NULL
 AND r.state='running' AND p.status='active' RETURNING a.id`, project, run, out.Summary, disagreements, raw, now, basis, output).Scan(&id)
	if err == pgx.ErrNoRows {
		return id, apierrors.New(apierrors.Forbidden, "task analysis requires an active coordinator run")
	}
	return id, err
}

func FinishTaskAnalysis(ctx context.Context, tx pgx.Tx, project, batch uuid.UUID, state, summary string, now time.Time) error {
	var id, task uuid.UUID
	var raw []byte
	var parent *uuid.UUID
	var point int64
	if state == "completed" {
		var recorded []byte
		err := tx.QueryRow(ctx, `SELECT output_json FROM task_analyses WHERE project_id=$1 AND batch_id=$2`, project, batch).Scan(&recorded)
		if err == pgx.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		var output TaskAnalysisOutput
		if err = json.Unmarshal(recorded, &output); err != nil {
			return err
		}
		if strings.TrimSpace(output.Summary) == "" {
			state = "failed"
			summary = "模型未保存结构化公开分析，原记录已保留；可重试分析。"
		}
	}
	err := tx.QueryRow(ctx, `UPDATE task_analyses SET state=$3,summary=CASE WHEN $3='completed' THEN output_json->>'summary' ELSE $4 END,
 basis=CASE WHEN $3='completed' THEN COALESCE(output_json->'basis','[]'::jsonb) ELSE basis END,
 disagreements=CASE WHEN $3='completed' THEN COALESCE(output_json->'disagreements','[]'::jsonb) ELSE disagreements END,
 discussion_json=CASE WHEN $3='completed' THEN COALESCE(output_json->'discussion','{}'::jsonb) ELSE discussion_json END,
 error_code=CASE WHEN $3='completed' THEN NULL ELSE $3 END,updated_at=$5
 WHERE project_id=$1 AND batch_id=$2 RETURNING id,task_id,discussion_json,parent_topic_id,fork_after_seq`, project, batch, state, summary, now).Scan(&id, &task, &raw, &parent, &point)
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if state == "completed" {
		var proposal DiscussionProposal
		if err = json.Unmarshal(raw, &proposal); err != nil {
			return err
		}
		if proposal.Title != "" {
			if _, err = tx.Exec(ctx, `INSERT INTO discussion_suggestions(project_id,id,analysis_id,task_id,title,reason,parent_topic_id,fork_after_seq,suggested_topic_id,created_at)
   VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT(analysis_id) DO NOTHING`, project, uuid.New(), id, task, proposal.Title, proposal.Reason, nullID(parent), point, nullID(proposal.ExistingTopicID), now); err != nil {
				return err
			}
			if _, err = events.AppendProjectEvent(ctx, tx, project, "discussion_suggestion.changed", "task", task.String(), nil, map[string]any{"analysisId": id}, now); err != nil {
				return err
			}
		}
	}
	_, err = events.AppendProjectEvent(ctx, tx, project, "analysis.changed", "task", task.String(), nil, map[string]any{"analysisId": id, "state": state}, now)
	return err
}
