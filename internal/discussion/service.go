// SPDX-License-Identifier: Apache-2.0

// Package discussion implements topics, messages and the unified submission
// entry per docs/plans/v1/04 §1: text + ready material versions finalize in
// ONE transaction (submission + committed message + event); author kind and
// channel are always derived server-side; the same clientSubmissionId never
// creates a second copy (unique constraint + replay path).
package discussion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/kakj-go/Judex/internal/platform/paging"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/collaboration"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/events"
	"github.com/kakj-go/Judex/internal/work"
)

// Topic is the API projection (06 §4).
type Topic struct {
	LinksVersion   int64                     `json:"linksVersion"`
	ParentTopicID  *uuid.UUID                `json:"parentTopicId"`
	ForkAfterSeq   int64                     `json:"forkAfterSeq"`
	SourceRefs     []collaboration.SourceRef `json:"sourceRefs"`
	ID             uuid.UUID                 `json:"id"`
	Title          string                    `json:"title"`
	Kind           string                    `json:"kind"`
	ContextType    *string                   `json:"contextType"`
	ContextID      *uuid.UUID                `json:"contextId"`
	LastMessageSeq int64                     `json:"lastMessageSeq"`
	Links          []TopicLink               `json:"links"`
	CreatedAt      time.Time                 `json:"createdAt"`
}

type TopicLink = collaboration.Link

// Message is one committed chat record (kind/author derived server-side).
type Message = collaboration.Message

// Submission is the unified entry record.
type Submission struct {
	PlanID              *uuid.UUID `json:"planId"`
	DiscussionIntent    string     `json:"discussionIntent"`
	ExpectedTaskVersion int64      `json:"expectedTaskVersion,omitempty"`
	ReportID            *uuid.UUID `json:"reportId,omitempty"`
	ID                  uuid.UUID  `json:"id"`
	ClientSubmissionID  string     `json:"clientSubmissionId"`
	Purpose             string     `json:"purpose"`
	Source              string     `json:"source"`
	Text                string     `json:"text"`
	TopicID             *uuid.UUID `json:"topicId"`
	TaskID              *uuid.UUID `json:"taskId"`
	IdentityID          *uuid.UUID `json:"identityId"`
	ActorUserID         *uuid.UUID `json:"actorUserId"`
	Status              string     `json:"status"`
	MaterialVersionIDs  []string   `json:"materialVersionIds"`
	MessageID           *uuid.UUID `json:"messageId"`
	CreatedAt           time.Time  `json:"createdAt"`
}

type Service struct {
	pool *postgres.Pool
	now  func() time.Time
}

func NewService(pool *postgres.Pool, now func() time.Time) *Service {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{pool: pool, now: now}
}

func memberRoleTx(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, projectID, requester uuid.UUID) (string, error) {
	var role string
	err := q.QueryRow(ctx, `
		SELECT role FROM project_members WHERE project_id=$1 AND user_id=$2 AND state='active'`,
		projectID, requester).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apierrors.New(apierrors.NotFound, "project not found")
	}
	return role, err
}

// CreateTopic is the explicit human action (AI proposals arrive via P3).
func (s *Service) CreateTopic(ctx context.Context, requester, projectID uuid.UUID, title, initialMessage string, links []TopicLink) (Topic, error) {
	title = strings.TrimSpace(title)
	if l := utf8.RuneCountInString(title); l < 1 || l > 200 {
		return Topic{}, apierrors.Fields("title", "length")
	}
	if len(initialMessage) > 65536 {
		return Topic{}, apierrors.Fields("initialMessage", "length")
	}
	for _, link := range links {
		if link.ObjectType != "plan" && link.ObjectType != "task" {
			return Topic{}, apierrors.Fields("links[].objectType", "enum")
		}
	}
	var out Topic
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, projectID.String()); err != nil {
			return err
		}
		if _, err := memberRoleTx(ctx, tx, projectID, requester); err != nil {
			return err
		}

		now := s.now()
		id := uuid.New()
		if _, err := tx.Exec(ctx, `
			INSERT INTO topics (project_id, id, title, kind, created_by, created_at)
			VALUES ($1,$2,$3,'discussion',$4,$5)`, projectID, id, title, requester, now); err != nil {
			return apierrors.New(apierrors.Internal, "topic insert failed").Wrap(err)
		}
		if err := collaboration.ValidateLinks(ctx, tx, projectID, links); err != nil {
			return err
		}
		for _, link := range links {
			if _, err := tx.Exec(ctx, `
				INSERT INTO topic_work_links (project_id, topic_id, object_type, object_id, created_at)
				VALUES ($1,$2,$3,$4,$5)`, projectID, id, link.ObjectType, link.ObjectID, now); err != nil {
				return apierrors.New(apierrors.Internal, "link insert failed").Wrap(err)
			}
		}
		if initialMessage != "" {
			sub, err := s.finalizeSubmissionTx(ctx, tx, requester, projectID, Submission{
				ClientSubmissionID: uuid.NewString(),
				Purpose:            "message",
				Source:             "web",
				Text:               initialMessage,
				TopicID:            &id,
			}, nil)
			if err != nil {
				return err
			}
			_ = sub
		}
		if _, err := events.AppendProjectEvent(ctx, tx, projectID, "message.committed", "topic", id.String(), nil,
			map[string]any{"change": "topic_created"}, now); err != nil {
			return err
		}
		out = Topic{ID: id, Title: title, Kind: "discussion", Links: links, CreatedAt: now, LinksVersion: 1}
		return nil
	})
	return out, err
}

// ListTopics returns discussion topics ordered by recent activity.
func (s *Service) ListTopics(ctx context.Context, requester, projectID uuid.UUID, filters ...TopicFilter) ([]Topic, error) {
	f := TopicFilter{}
	if len(filters) > 0 {
		f = filters[0]
	}
	if f.PlanID != nil && f.TaskID != nil {
		return nil, apierrors.Fields("scope", "choose planId or taskId")
	}
	if f.Kind != "" && f.Kind != "discussion" && f.Kind != "project_room" {
		return nil, apierrors.Fields("kind", "enum")
	}
	if _, err := memberRoleTx(ctx, s.pool, projectID, requester); err != nil {
		return nil, err
	}
	rows, err := paging.Query(ctx, s.pool, topicListSQL, "t.created_at", "t.id", projectID, f.PlanID, f.TaskID, strings.TrimSpace(f.Query), f.Kind)
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "topics failed").Wrap(err)
	}
	defer rows.Close()
	var out []Topic
	for rows.Next() {
		var t Topic
		if err := rows.Scan(&t.ID, &t.Title, &t.Kind, &t.ContextType, &t.ContextID, &t.LastMessageSeq, &t.CreatedAt, &t.ParentTopicID, &t.ForkAfterSeq, &t.LinksVersion); err != nil {
			return nil, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		out = append(out, t)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if err = s.topicReferences(ctx, projectID, out); err != nil {
		return nil, err
	}
	return out, nil
}

// LinkTopic attaches plan/task references (06 §4).
func (s *Service) LinkTopic(ctx context.Context, requester, projectID, topicID uuid.UUID, links []TopicLink) error {
	return s.changeLinks(ctx, requester, projectID, topicID, nil, links, false)
}

// ListMessages pages messages by seq (beforeSeq anchor for history fill).
func (s *Service) ListMessages(ctx context.Context, requester, projectID, topicID uuid.UUID, beforeSeq int64, limit int, after ...int64) ([]Message, error) {
	if _, err := memberRoleTx(ctx, s.pool, projectID, requester); err != nil {
		return nil, err
	}
	afterSeq := int64(0)
	if len(after) > 0 {
		afterSeq = after[0]
	}
	return collaboration.ReadHistory(ctx, s.pool, projectID, topicID, beforeSeq, afterSeq, -1, limit)
}

// CreateSubmission is the unified POST /projects/{p}/submissions entry
// (04 §1). All material versions must be ready in-project; the submission
// and its committed message land in one transaction keyed by
// (project, actor, clientSubmissionId) so retries return the original.
func (s *Service) CreateSubmission(ctx context.Context, requester, projectID uuid.UUID, sub Submission) (Submission, error) {
	if sub.DiscussionIntent == "" {
		sub.DiscussionIntent = "auto"
	}
	if sub.DiscussionIntent != "auto" && sub.DiscussionIntent != "question" && sub.DiscussionIntent != "reply" {
		return Submission{}, apierrors.Fields("discussionIntent", "enum")
	}
	if sub.DiscussionIntent == "reply" && (sub.TopicID == nil || sub.TaskID == nil) {
		return Submission{}, apierrors.Fields("topicId", "required for reply")
	}
	if sub.DiscussionIntent != "auto" && sub.Purpose != "message" {
		return Submission{}, apierrors.Fields("purpose", "question and reply use message")
	}
	if sub.ClientSubmissionID == "" {
		return Submission{}, apierrors.Fields("clientSubmissionId", "required")
	}
	if _, err := uuid.Parse(sub.ClientSubmissionID); err != nil {
		return Submission{}, apierrors.Fields("clientSubmissionId", "invalid")
	}
	switch sub.Purpose {
	case "message", "progress", "delivery", "material":
	default:
		return Submission{}, apierrors.Fields("purpose", "enum")
	}
	if l := utf8.RuneCountInString(sub.Text); (l < 1 && len(sub.MaterialVersionIDs) == 0) || l > 65536 {
		return Submission{}, apierrors.Fields("text", "length")
	}
	if sub.TopicID != nil {
		var kind string
		if err := s.pool.QueryRow(ctx, `SELECT kind FROM topics WHERE id=$1 AND project_id=$2`,
			*sub.TopicID, projectID).Scan(&kind); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return Submission{}, apierrors.New(apierrors.InvalidReference, "topic not found")
			}
			return Submission{}, apierrors.New(apierrors.Internal, "topic lookup failed").Wrap(err)
		}
		if kind == "handoff" {
			return Submission{}, apierrors.New(apierrors.InvalidReference, "handoff topics accept handoff messages only")
		}
	}
	var out Submission
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, projectID.String()); err != nil {
			return err
		}
		role, err := memberRoleTx(ctx, tx, projectID, requester)
		if err != nil {
			return err
		}
		_ = role
		if sub.PlanID != nil {
			var valid bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM plans WHERE id=$1 AND project_id=$2)`, sub.PlanID, projectID).Scan(&valid); err != nil {
				return err
			}
			if !valid {
				return apierrors.New(apierrors.InvalidReference, "plan not found in project")
			}
		}
		if sub.TaskID != nil {
			var plan *uuid.UUID
			if err := tx.QueryRow(ctx, `SELECT plan_id FROM tasks WHERE id=$1 AND project_id=$2`, sub.TaskID, projectID).Scan(&plan); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return apierrors.New(apierrors.InvalidReference, "task not found in project")
				}
				return err
			}
			if sub.PlanID != nil && (plan == nil || *plan != *sub.PlanID) {
				return apierrors.Fields("planId", "does not match task")
			}
			sub.PlanID = plan
		}
		if sub.TaskID != nil {
			var valid bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tasks WHERE project_id=$1 AND id=$2)`, projectID, *sub.TaskID).Scan(&valid); err != nil {
				return err
			}
			if !valid {
				return apierrors.New(apierrors.InvalidReference, "task not found in project")
			}
		}
		// Replay: identical clientSubmissionId returns the stored result.
		var existing Submission
		scanErr := tx.QueryRow(ctx, `
			SELECT id, client_submission_id, purpose, source, text, topic_id, task_id, identity_id,
			       actor_user_id, status, created_at,expected_task_version,discussion_intent,plan_id
			FROM submissions
			WHERE project_id=$1 AND actor_user_id=$2 AND client_submission_id=$3`,
			projectID, requester, sub.ClientSubmissionID).
			Scan(&existing.ID, &existing.ClientSubmissionID, &existing.Purpose, &existing.Source,
				&existing.Text, &existing.TopicID, &existing.TaskID, &existing.IdentityID,
				&existing.ActorUserID, &existing.Status, &existing.CreatedAt, &existing.ExpectedTaskVersion, &existing.DiscussionIntent, &existing.PlanID)
		if scanErr == nil {
			// Attach message ref if present.
			var messageID uuid.NullUUID
			_ = tx.QueryRow(ctx, `SELECT id FROM messages WHERE submission_id=$1 LIMIT 1`, existing.ID).Scan(&messageID)
			if messageID.Valid {
				id := messageID.UUID
				existing.MessageID = &id
			}
			existing.MaterialVersionIDs, _ = s.submissionMaterials(ctx, tx, existing.ID)
			if submissionHash(existing) != submissionHash(sub) {
				return apierrors.New(apierrors.IdempotencyConflict, "clientSubmissionId reused with different content")
			}
			var report *uuid.UUID
			if sub.TaskID != nil {
				if err := tx.QueryRow(ctx, `SELECT id FROM work_reports WHERE submission_id=$1 AND task_id=$2`, existing.ID, sub.TaskID).Scan(&report); err == nil {
					existing.ReportID = report
				}
			}
			out = existing
			return nil
		}
		if !errors.Is(scanErr, pgx.ErrNoRows) {
			return apierrors.New(apierrors.Internal, "submission lookup failed").Wrap(scanErr)
		}
		sub.ActorUserID = &requester
		created, err := s.finalizeSubmissionTx(ctx, tx, requester, projectID, sub, func(versionID uuid.UUID) error {
			var state string
			var materialProject uuid.UUID
			if err := tx.QueryRow(ctx, `
				SELECT v.state, m.project_id FROM material_versions v
				JOIN materials m ON m.id=v.material_id
				WHERE v.id=$1 AND m.deleted_at IS NULL`, versionID).Scan(&state, &materialProject); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return apierrors.New(apierrors.InvalidReference, "material version not found")
				}
				return apierrors.New(apierrors.Internal, "material lookup failed").Wrap(err)
			}
			if state != "ready" {
				return apierrors.Newf(apierrors.RequirementUnmet, "material version %s is %s, not ready", versionID, state)
			}
			if materialProject != projectID {
				return apierrors.New(apierrors.InvalidReference, "material belongs to another project")
			}
			return nil
		})
		if err != nil {
			return err
		}
		if sub.Purpose == "progress" || sub.Purpose == "delivery" {
			if sub.TaskID == nil {
				return apierrors.Fields("taskId", "required")
			}
			report, _, err := work.NewService(s.pool, s.now).ReportInTx(ctx, tx, requester, projectID, work.ReportInput{SubmissionID: &created.ID, TaskID: *sub.TaskID, IdentityID: sub.IdentityID, Kind: sub.Purpose, Text: sub.Text, MaterialVersionIDs: sub.MaterialVersionIDs, ExpectedTaskVersion: sub.ExpectedTaskVersion})
			if err != nil {
				return err
			}
			created.ReportID = &report
		}
		if sub.TaskID != nil && created.ReportID == nil {
			if _, err := collaboration.EnqueueAnalysis(ctx, tx.Tx, projectID, *sub.TaskID, requester, "submission", created.ID, sub.TopicID, s.now()); err != nil {
				return err
			}
		}
		out = created
		return nil
	})
	return out, err
}

func (s *Service) submissionMaterials(ctx context.Context, q interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}, submissionID uuid.UUID) ([]string, error) {
	rows, err := q.Query(ctx, `
		SELECT material_version_id FROM submission_materials WHERE submission_id=$1`, submissionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// finalizeSubmissionTx writes submission + message + links atomically and
// bumps topic.last_message_seq under the project lock.
func (s *Service) finalizeSubmissionTx(ctx context.Context, tx pgx.Tx, requester, projectID uuid.UUID, sub Submission, materialCheck func(uuid.UUID) error) (Submission, error) {
	if sub.DiscussionIntent == "" {
		sub.DiscussionIntent = "auto"
	}
	now := s.now()
	id := uuid.New()
	hash := submissionHash(sub)
	if _, err := tx.Exec(ctx, `
		INSERT INTO submissions (project_id, id, actor_user_id, identity_id, source, client_submission_id,
			text, topic_id, task_id, purpose, status, payload_hash, expected_task_version, code_refs_json, created_at,discussion_intent,plan_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'ready',$11,$12,'[]',$13,$14,$15)`,
		projectID, id, requester, nullableUUID(sub.IdentityID), sub.Source, sub.ClientSubmissionID,
		sub.Text, nullableUUID(sub.TopicID), nullableUUID(sub.TaskID), sub.Purpose,
		hash, sub.ExpectedTaskVersion, now, sub.DiscussionIntent, sub.PlanID); err != nil {
		var pgErr interface{ SQLState() string }
		if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
			return Submission{}, apierrors.New(apierrors.IdempotencyConflict, "duplicate clientSubmissionId")
		}
		return Submission{}, apierrors.New(apierrors.Internal, "submission insert failed").Wrap(err)
	}
	for _, versionID := range sub.MaterialVersionIDs {
		vid, err := uuid.Parse(versionID)
		if err != nil {
			return Submission{}, apierrors.Fields("materialVersionIds", "invalid")
		}
		if materialCheck != nil {
			if err := materialCheck(vid); err != nil {
				return Submission{}, err
			}
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO submission_materials (project_id, submission_id, material_version_id)
			VALUES ($1,$2,$3)`, projectID, id, vid); err != nil {
			return Submission{}, apierrors.New(apierrors.Internal, "submission material failed").Wrap(err)
		}
		if _, err := events.AppendProjectEvent(ctx, tx, projectID, "material.changed", "material_version", vid.String(), nil, map[string]any{"versionId": vid, "submissionId": id}, now); err != nil {
			return Submission{}, err
		}
	}
	messageID := uuid.Nil
	if sub.TopicID != nil && sub.Purpose != "material" {
		var seq int64
		if err := tx.QueryRow(ctx, `
			UPDATE topics SET last_message_seq = last_message_seq + 1 WHERE id=$1 RETURNING last_message_seq`,
			*sub.TopicID).Scan(&seq); err != nil {
			return Submission{}, apierrors.New(apierrors.Internal, "seq failed").Wrap(err)
		}
		messageID = uuid.New()
		if _, err := tx.Exec(ctx, `
			INSERT INTO messages (project_id, id, topic_id, seq, kind, author_user_id, identity_id,
				submission_id, run_id, content, state, created_at)
			VALUES ($1,$2,$3,$4,'human',$5,$6,$7,NULL,$8,'committed',$9)`,
			projectID, messageID, *sub.TopicID, seq, requester, nullableUUID(sub.IdentityID),
			id, sub.Text, now); err != nil {
			return Submission{}, apierrors.New(apierrors.Internal, "message insert failed").Wrap(err)
		}
		if _, err := events.AppendProjectEvent(ctx, tx, projectID, "message.committed", "topic", sub.TopicID.String(), &seq,
			map[string]any{"messageId": messageID, "seq": seq}, now); err != nil {
			return Submission{}, err
		}
	}
	if err := audit.Append(ctx, tx, audit.Entry{
		ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
		Source: audit.SourceWeb, Operation: "submission.create",
		ObjectType: "submission", ObjectID: id.String(), OccurredAt: now,
	}); err != nil {
		return Submission{}, err
	}
	sub.ID = id
	sub.Status = "ready"
	sub.CreatedAt = now
	if messageID != uuid.Nil {
		mid := messageID
		sub.MessageID = &mid
	}
	return sub, nil
}

func nullableUUID(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return *id
}

func submissionHash(sub Submission) string {
	payload := map[string]any{"plan": sub.PlanID, "text": sub.Text, "purpose": sub.Purpose, "materials": sub.MaterialVersionIDs, "topic": sub.TopicID, "task": sub.TaskID, "identity": sub.IdentityID, "expectedVersion": sub.ExpectedTaskVersion, "discussionIntent": sub.DiscussionIntent}
	if len(sub.MaterialVersionIDs) == 0 {
		payload["materials"] = []string{}
	}
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (s *Service) GetTopic(ctx context.Context, user, project, topic uuid.UUID) (Topic, error) {
	if _, err := memberRoleTx(ctx, s.pool, project, user); err != nil {
		return Topic{}, err
	}
	var out Topic
	err := s.pool.QueryRow(ctx, `SELECT id,title,kind,context_type,context_id,last_message_seq,created_at,parent_topic_id,fork_after_seq,links_version FROM topics WHERE id=$1 AND project_id=$2`, topic, project).Scan(&out.ID, &out.Title, &out.Kind, &out.ContextType, &out.ContextID, &out.LastMessageSeq, &out.CreatedAt, &out.ParentTopicID, &out.ForkAfterSeq, &out.LinksVersion)
	if err != nil {
		return out, apierrors.New(apierrors.NotFound, "topic not found")
	}
	items := []Topic{out}
	if err = s.topicReferences(ctx, project, items); err != nil {
		return out, err
	}
	return items[0], nil
}
