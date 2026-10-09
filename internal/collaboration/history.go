// SPDX-License-Identifier: Apache-2.0
// Package collaboration owns shared conversation history and task activity
// projections. It does not grant permission to change formal work or approvals.
package collaboration

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/events"
)

type Reader interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}
type Writer interface {
	Reader
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}
type Link struct {
	ObjectType string    `json:"objectType"`
	ObjectID   uuid.UUID `json:"objectId"`
}
type SourceRef struct {
	Type   string     `json:"type"`
	ID     uuid.UUID  `json:"id"`
	TaskID *uuid.UUID `json:"taskId,omitempty"`
}
type ForkInput struct {
	Title        string      `json:"title"`
	ForkAfterSeq *int64      `json:"forkAfterSeq"`
	Links        []Link      `json:"links"`
	SourceRefs   []SourceRef `json:"sourceRefs"`
}
type Origin struct {
	TopicID  uuid.UUID
	AfterSeq *int64
}
type Message struct {
	ID                 uuid.UUID     `json:"id"`
	TopicID            uuid.UUID     `json:"topicId"`
	OriginTopicID      uuid.UUID     `json:"originTopicId"`
	Inherited          bool          `json:"inherited"`
	Seq                int64         `json:"seq"`
	Kind               string        `json:"kind"`
	AuthorUserID       *uuid.UUID    `json:"authorUserId"`
	AuthorName         *string       `json:"authorDisplayName"`
	IdentityID         *uuid.UUID    `json:"identityId"`
	Source             *string       `json:"source"`
	SubmissionID       *uuid.UUID    `json:"submissionId"`
	TaskID             *uuid.UUID    `json:"taskId"`
	RunID              *uuid.UUID    `json:"runId"`
	Content            string        `json:"content"`
	State              string        `json:"state"`
	MaterialVersionIDs []string      `json:"materialVersionIds"`
	Materials          []MaterialRef `json:"materials"`
	CreatedAt          time.Time     `json:"createdAt"`
}

// Each ancestor contributes only its own suffix up to its child's frozen cutoff.
// Child seq starts at fork_after_seq+1, so before/after cursors remain monotonic.
const HistoryCTE = `WITH RECURSIVE lineage AS (
 SELECT id,parent_topic_id,fork_after_seq,$4::bigint AS ceiling,ARRAY[id] AS path
 FROM topics WHERE id=$2 AND project_id=$1
 UNION ALL
 SELECT p.id,p.parent_topic_id,p.fork_after_seq,LEAST(l.ceiling,l.fork_after_seq),l.path||p.id
 FROM lineage l JOIN topics p ON p.id=l.parent_topic_id AND p.project_id=$1
 WHERE NOT p.id=ANY(l.path)
), visible AS (
 SELECT m.* FROM messages m JOIN lineage l ON l.id=m.topic_id
 WHERE m.project_id=$1 AND m.seq>l.fork_after_seq AND m.seq<=l.ceiling
) `

func ReadHistory(ctx context.Context, q Reader, project, topic uuid.UUID, before, after, covered int64, limit int) ([]Message, error) {
	if covered < 0 {
		covered = math.MaxInt64
	}
	if limit < 1 || limit > 101 {
		limit = 50
	}
	var exists bool
	if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM topics WHERE project_id=$1 AND id=$2)`, project, topic).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, apierrors.New(apierrors.NotFound, "topic not found")
	}
	direction := "DESC"
	if after != 0 {
		direction = "ASC"
	}
	rows, err := q.Query(ctx, HistoryCTE+`SELECT m.id,m.topic_id,m.seq,m.kind,m.author_user_id,COALESCE(u.display_name,''),
 m.identity_id,m.submission_id,m.run_id,m.content,m.state,m.created_at,s.source,COALESCE(s.task_id,b.task_id),
 ARRAY(SELECT material_version_id::text FROM submission_materials WHERE submission_id=m.submission_id ORDER BY material_version_id),
 COALESCE((SELECT jsonb_agg(jsonb_build_object('versionId',v.id,'name',a.title) ORDER BY v.id) FROM submission_materials sm JOIN material_versions v ON v.id=sm.material_version_id JOIN materials a ON a.id=v.material_id WHERE sm.submission_id=m.submission_id),'[]'::jsonb)
 FROM visible m LEFT JOIN users u ON u.id=m.author_user_id LEFT JOIN submissions s ON s.id=m.submission_id
 LEFT JOIN agent_runs r ON r.id=m.run_id LEFT JOIN discussion_batches b ON b.id=r.batch_id
 WHERE ($3::bigint=0 OR m.seq<$3) AND ($5::bigint<=0 OR m.seq>$5)
 ORDER BY m.seq `+direction+` LIMIT $6`, project, topic, before, covered, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		var author string
		var materials []byte
		if err = rows.Scan(&m.ID, &m.OriginTopicID, &m.Seq, &m.Kind, &m.AuthorUserID, &author, &m.IdentityID, &m.SubmissionID, &m.RunID, &m.Content, &m.State, &m.CreatedAt, &m.Source, &m.TaskID, &m.MaterialVersionIDs, &materials); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(materials, &m.Materials); err != nil {
			return nil, err
		}
		m.TopicID = topic
		m.Inherited = m.OriginTopicID != topic
		if m.AuthorUserID != nil {
			m.AuthorName = &author
		}
		out = append(out, m)
	}
	if after == 0 {
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
	}
	return out, rows.Err()
}

func ValidateFork(ctx context.Context, q Reader, project, parent uuid.UUID, point *int64) (int64, error) {
	var last int64
	var kind string
	if err := q.QueryRow(ctx, `SELECT last_message_seq,kind FROM topics WHERE project_id=$1 AND id=$2`, project, parent).Scan(&last, &kind); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, apierrors.New(apierrors.InvalidReference, "parent topic not found")
		}
		return 0, err
	}
	if kind == "handoff" {
		return 0, apierrors.New(apierrors.InvalidReference, "handoff conversations retain their business object")
	}
	cut := last
	if point != nil {
		cut = *point
	}
	if cut < 0 || cut > last {
		return 0, apierrors.Fields("forkAfterSeq", "range")
	}
	if cut > 0 {
		history, err := ReadHistory(ctx, q, project, parent, cut+1, cut-1, cut, 1)
		if err != nil {
			return 0, err
		}
		if len(history) != 1 || history[0].Seq != cut || history[0].State != "committed" {
			return 0, apierrors.New(apierrors.InvalidReference, "fork point must be a committed visible message")
		}
	}
	return cut, nil
}

func ValidateLinks(ctx context.Context, q Reader, project uuid.UUID, links []Link) error {
	for _, link := range links {
		var exists bool
		switch link.ObjectType {
		case "plan":
			if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM plans WHERE project_id=$1 AND id=$2)`, project, link.ObjectID).Scan(&exists); err != nil {
				return err
			}
		case "task":
			if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tasks WHERE project_id=$1 AND id=$2)`, project, link.ObjectID).Scan(&exists); err != nil {
				return err
			}
		default:
			return apierrors.Fields("links.objectType", "enum")
		}
		if !exists {
			return apierrors.New(apierrors.InvalidReference, "linked work belongs to another project or does not exist")
		}
	}
	return nil
}

func AddSources(ctx context.Context, q Writer, project, topic uuid.UUID, refs []SourceRef, now time.Time) error {
	for _, ref := range refs {
		var exists bool
		switch ref.Type {
		case "report":
			if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM work_reports WHERE project_id=$1 AND id=$2)`, project, ref.ID).Scan(&exists); err != nil {
				return err
			}
		case "submission":
			if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM submissions WHERE project_id=$1 AND id=$2)`, project, ref.ID).Scan(&exists); err != nil {
				return err
			}
		case "message":
			if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM messages WHERE project_id=$1 AND id=$2)`, project, ref.ID).Scan(&exists); err != nil {
				return err
			}
		default:
			return apierrors.Fields("sourceRefs.type", "enum")
		}
		if !exists {
			return apierrors.New(apierrors.InvalidReference, "source record not found in project")
		}
		if _, err := q.Exec(ctx, `INSERT INTO topic_source_refs(project_id,topic_id,source_type,source_id,created_at) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, project, topic, ref.Type, ref.ID, now); err != nil {
			return err
		}
	}
	return nil
}

func ForkInTx(ctx context.Context, q pgx.Tx, project, user, parent uuid.UUID, in ForkInput, now time.Time) (uuid.UUID, error) {
	title := strings.TrimSpace(in.Title)
	if utf8.RuneCountInString(title) < 1 || utf8.RuneCountInString(title) > 200 {
		return uuid.Nil, apierrors.Fields("title", "length")
	}
	var parentID any
	cut := int64(0)
	if parent != uuid.Nil {
		var err error
		cut, err = ValidateFork(ctx, q, project, parent, in.ForkAfterSeq)
		if err != nil {
			return uuid.Nil, err
		}
		parentID = parent
	}
	if err := ValidateLinks(ctx, q, project, in.Links); err != nil {
		return uuid.Nil, err
	}
	id := uuid.New()
	var actor any
	if user != uuid.Nil {
		actor = user
	}
	if _, err := q.Exec(ctx, `INSERT INTO topics(project_id,id,title,kind,parent_topic_id,fork_after_seq,last_message_seq,created_by,created_at) VALUES($1,$2,$3,'discussion',$4,$5,$5,$6,$7)`, project, id, title, parentID, cut, actor, now); err != nil {
		return uuid.Nil, err
	}
	for _, link := range in.Links {
		if _, err := q.Exec(ctx, `INSERT INTO topic_work_links(project_id,topic_id,object_type,object_id,created_at) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, project, id, link.ObjectType, link.ObjectID, now); err != nil {
			return uuid.Nil, err
		}
	}
	if err := AddSources(ctx, q, project, id, in.SourceRefs, now); err != nil {
		return uuid.Nil, err
	}
	if _, err := events.AppendProjectEvent(ctx, q, project, "topic.forked", "topic", id.String(), nil, map[string]any{"parentTopicId": parentID, "forkAfterSeq": cut}, now); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func EnsurePlanTopic(ctx context.Context, q pgx.Tx, project, plan, user uuid.UUID, title string, parent uuid.UUID, point *int64, now time.Time) (uuid.UUID, error) {
	var current *uuid.UUID
	if err := q.QueryRow(ctx, `SELECT main_topic_id FROM plans WHERE project_id=$1 AND id=$2 FOR UPDATE`, project, plan).Scan(&current); err != nil {
		return uuid.Nil, err
	}
	if current != nil {
		return *current, nil
	}
	id, err := ForkInTx(ctx, q, project, user, parent, ForkInput{Title: title, ForkAfterSeq: point, Links: []Link{{ObjectType: "plan", ObjectID: plan}}}, now)
	if err != nil {
		return uuid.Nil, err
	}
	if _, err = q.Exec(ctx, `UPDATE topics SET context_type='plan',context_id=$2 WHERE id=$1`, id, plan); err != nil {
		return uuid.Nil, err
	}
	_, err = q.Exec(ctx, `UPDATE plans SET main_topic_id=$2 WHERE id=$1`, plan, id)
	return id, err
}
