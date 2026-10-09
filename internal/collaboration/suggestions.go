package collaboration

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/events"
	"github.com/kakj-go/Judex/internal/platform/paging"
)

func mapValues(key, value string) url.Values { return url.Values{key: []string{value}} }

const suggestionSelect = `SELECT s.id,s.analysis_id,s.task_id,t.plan_id,s.title,s.reason,s.parent_topic_id,s.fork_after_seq,s.suggested_topic_id,s.state,s.version,s.result_topic_id,a.source_type,a.source_id,s.created_at
 /*keys*/ FROM discussion_suggestions s JOIN tasks t ON t.id=s.task_id JOIN task_analyses a ON a.id=s.analysis_id `

func scanSuggestion(row interface{ Scan(...any) error }) (Suggestion, error) {
	var out Suggestion
	err := row.Scan(&out.ID, &out.AnalysisID, &out.TaskID, &out.PlanID, &out.Title, &out.Reason, &out.ParentTopicID, &out.ForkAfterSeq, &out.SuggestedTopicID, &out.State, &out.Version, &out.ResultTopicID, &out.SourceRef.Type, &out.SourceRef.ID, &out.CreatedAt)
	return out, err
}
func (s *Service) ListSuggestions(ctx context.Context, user, project uuid.UUID, plan, task *uuid.UUID) ([]Suggestion, error) {
	if err := Member(ctx, s.Pool, user, project); err != nil {
		return nil, err
	}
	rows, err := paging.Query(ctx, s.Pool, suggestionSelect+`WHERE s.project_id=$1 AND ($2::uuid IS NULL OR t.plan_id=$2) AND ($3::uuid IS NULL OR t.id=$3) /*page*/`, "s.created_at", "s.id", project, nullID(plan), nullID(task))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Suggestion{}
	for rows.Next() {
		v, e := scanSuggestion(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Service) GetSuggestion(ctx context.Context, user, project, id uuid.UUID) (Suggestion, error) {
	if err := Member(ctx, s.Pool, user, project); err != nil {
		return Suggestion{}, err
	}
	out, err := scanSuggestion(s.Pool.QueryRow(ctx, strings.Replace(suggestionSelect, "/*keys*/", "", 1)+`WHERE s.project_id=$1 AND s.id=$2`, project, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return out, apierrors.New(apierrors.NotFound, "discussion suggestion not found")
	}
	return out, err
}
func (s *Service) Resolve(ctx context.Context, user, project, id uuid.UUID, in ResolveInput) (Suggestion, error) {
	var result Suggestion
	err := s.Pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, project.String()); err != nil {
			return err
		}
		if err := Member(ctx, tx, user, project); err != nil {
			return err
		}
		out, err := scanSuggestion(tx.QueryRow(ctx, strings.Replace(suggestionSelect, "/*keys*/", "", 1)+`WHERE s.project_id=$1 AND s.id=$2 FOR UPDATE OF s`, project, id))
		if err != nil {
			return apierrors.New(apierrors.NotFound, "discussion suggestion not found")
		}
		if out.State != "pending" {
			result = out
			return nil
		}
		if in.ExpectedVersion != out.Version {
			return apierrors.New(apierrors.VersionConflict, "suggestion version changed")
		}
		now := time.Now().UTC()
		topic := uuid.Nil
		links := append([]Link{}, in.Links...)
		links = append(links, Link{ObjectType: "task", ObjectID: out.TaskID})
		if out.PlanID != nil {
			links = append(links, Link{ObjectType: "plan", ObjectID: *out.PlanID})
		}
		switch in.Mode {
		case "create":
			parent := uuid.Nil
			if out.ParentTopicID != nil {
				parent = *out.ParentTopicID
			}
			title := strings.TrimSpace(in.Title)
			if title == "" {
				title = out.Title
			}
			topic, err = ForkInTx(ctx, tx.Tx, project, user, parent, ForkInput{Title: title, ForkAfterSeq: &out.ForkAfterSeq, Links: links, SourceRefs: []SourceRef{out.SourceRef}}, now)
			if err != nil {
				return err
			}
		case "main":
			if out.PlanID == nil {
				return apierrors.New(apierrors.InvalidReference, "task has no plan main discussion")
			}
			if err = tx.QueryRow(ctx, `SELECT main_topic_id FROM plans WHERE project_id=$1 AND id=$2`, project, *out.PlanID).Scan(&topic); err != nil {
				return err
			}
		case "link":
			if in.TopicID == nil {
				return apierrors.Fields("topicId", "required")
			}
			topic = *in.TopicID
			var kind string
			if err = tx.QueryRow(ctx, `SELECT kind FROM topics WHERE id=$1 AND project_id=$2`, topic, project).Scan(&kind); err != nil || kind == "handoff" {
				return apierrors.New(apierrors.InvalidReference, "target discussion not found")
			}
		case "dismiss":
		default:
			return apierrors.Fields("mode", "enum")
		}
		if in.Mode == "main" || in.Mode == "link" {
			if err = ChangeTopicLinks(ctx, tx.Tx, project, topic, nil, links, false, now); err != nil {
				return err
			}
			if err = AddSources(ctx, tx, project, topic, []SourceRef{out.SourceRef}, now); err != nil {
				return err
			}
		}
		state := "handled"
		if in.Mode == "dismiss" {
			state = "dismissed"
		}
		var target any
		if topic != uuid.Nil {
			target = topic
			out.ResultTopicID = &topic
		}
		if _, err = tx.Exec(ctx, `UPDATE discussion_suggestions SET state=$2,version=version+1,result_topic_id=$3,resolved_by=$4,resolved_at=$5 WHERE id=$1`, id, state, target, user, now); err != nil {
			return err
		}
		if _, err = events.AppendProjectEvent(ctx, tx.Tx, project, "discussion_suggestion.changed", "discussion_suggestion", id.String(), nil, map[string]any{"state": state, "topicId": target}, now); err != nil {
			return err
		}
		out.State = state
		out.Version++
		result = out
		return nil
	})
	return result, err
}
