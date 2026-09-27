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
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/events"
)

// Topic is the API projection (06 §4).
type Topic struct {
	ID             uuid.UUID   `json:"id"`
	Title          string      `json:"title"`
	Kind           string      `json:"kind"`
	ContextType    *string     `json:"contextType"`
	ContextID      *uuid.UUID  `json:"contextId"`
	LastMessageSeq int64       `json:"lastMessageSeq"`
	Links          []TopicLink `json:"links"`
	CreatedAt      time.Time   `json:"createdAt"`
}

type TopicLink struct {
	ObjectType string    `json:"objectType"`
	ObjectID   uuid.UUID `json:"objectId"`
}

// Message is one committed chat record (kind/author derived server-side).
type Message struct {
	ID                 uuid.UUID  `json:"id"`
	TopicID            uuid.UUID  `json:"topicId"`
	Seq                int64      `json:"seq"`
	Kind               string     `json:"kind"`
	AuthorUserID       *uuid.UUID `json:"authorUserId"`
	AuthorName         *string    `json:"authorDisplayName"`
	IdentityID         *uuid.UUID `json:"identityId"`
	Source             *string    `json:"source"`
	SubmissionID       *uuid.UUID `json:"submissionId"`
	RunID              *uuid.UUID `json:"runId"`
	Content            string     `json:"content"`
	State              string     `json:"state"`
	MaterialVersionIDs []string   `json:"materialVersionIds"`
	CreatedAt          time.Time  `json:"createdAt"`
}

// Submission is the unified entry record.
type Submission struct {
	ID                 uuid.UUID  `json:"id"`
	ClientSubmissionID string     `json:"clientSubmissionId"`
	Purpose            string     `json:"purpose"`
	Source             string     `json:"source"`
	Text               string     `json:"text"`
	TopicID            *uuid.UUID `json:"topicId"`
	TaskID             *uuid.UUID `json:"taskId"`
	IdentityID         *uuid.UUID `json:"identityId"`
	ActorUserID        *uuid.UUID `json:"actorUserId"`
	Status             string     `json:"status"`
	MaterialVersionIDs []string   `json:"materialVersionIds"`
	MessageID          *uuid.UUID `json:"messageId"`
	CreatedAt          time.Time  `json:"createdAt"`
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
		if err := tx.LockProjectForUpdate(ctx, projectID.String()); err != nil {
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
		out = Topic{ID: id, Title: title, Kind: "discussion", Links: links, CreatedAt: now}
		return nil
	})
	return out, err
}

// ListTopics returns discussion topics ordered by recent activity.
func (s *Service) ListTopics(ctx context.Context, requester, projectID uuid.UUID) ([]Topic, error) {
	if _, err := memberRoleTx(ctx, s.pool, projectID, requester); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, title, kind, context_type, context_id, last_message_seq, created_at
		FROM topics WHERE project_id=$1 AND kind<>'handoff'
		ORDER BY created_at DESC LIMIT 100`, projectID)
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "topics failed").Wrap(err)
	}
	defer rows.Close()
	var out []Topic
	for rows.Next() {
		var t Topic
		if err := rows.Scan(&t.ID, &t.Title, &t.Kind, &t.ContextType, &t.ContextID, &t.LastMessageSeq, &t.CreatedAt); err != nil {
			return nil, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// LinkTopic attaches plan/task references (06 §4).
func (s *Service) LinkTopic(ctx context.Context, requester, projectID, topicID uuid.UUID, links []TopicLink) error {
	for _, link := range links {
		if link.ObjectType != "plan" && link.ObjectType != "task" {
			return apierrors.Fields("targetRefs[].objectType", "enum")
		}
	}
	return s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockProjectForUpdate(ctx, projectID.String()); err != nil {
			return err
		}
		if _, err := memberRoleTx(ctx, tx, projectID, requester); err != nil {
			return err
		}
		for _, link := range links {
			if _, err := tx.Exec(ctx, `
				INSERT INTO topic_work_links (project_id, topic_id, object_type, object_id, created_at)
				VALUES ($1,$2,$3,$4,$5)
				ON CONFLICT (topic_id, object_type, object_id) DO NOTHING`,
				projectID, topicID, link.ObjectType, link.ObjectID, s.now()); err != nil {
				return apierrors.New(apierrors.Internal, "link failed").Wrap(err)
			}
		}
		return nil
	})
}

// ListMessages pages messages by seq (beforeSeq anchor for history fill).
func (s *Service) ListMessages(ctx context.Context, requester, projectID, topicID uuid.UUID, beforeSeq int64, limit int) ([]Message, error) {
	if _, err := memberRoleTx(ctx, s.pool, projectID, requester); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT m.id, m.topic_id, m.seq, m.kind, m.author_user_id,
		       COALESCE(u.display_name, ''), m.identity_id, m.submission_id, m.run_id,
		       m.content, m.state, m.created_at
		FROM messages m LEFT JOIN users u ON u.id = m.author_user_id
		WHERE m.project_id=$1 AND m.topic_id=$2 AND ($3=0 OR m.seq<$3)
		ORDER BY m.seq DESC LIMIT $4`, projectID, topicID, beforeSeq, limit)
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "messages failed").Wrap(err)
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var (
			m           Message
			authorName  string
			authorNull  uuid.NullUUID
			identityN   uuid.NullUUID
			submissionN uuid.NullUUID
			runN        uuid.NullUUID
		)
		if err := rows.Scan(&m.ID, &m.TopicID, &m.Seq, &m.Kind, &authorNull, &authorName,
			&identityN, &submissionN, &runN, &m.Content, &m.State, &m.CreatedAt); err != nil {
			return nil, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		if authorNull.Valid {
			id := authorNull.UUID
			m.AuthorUserID = &id
			m.AuthorName = &authorName
		}
		if identityN.Valid {
			id := identityN.UUID
			m.IdentityID = &id
		}
		if submissionN.Valid {
			id := submissionN.UUID
			m.SubmissionID = &id
		}
		if runN.Valid {
			id := runN.UUID
			m.RunID = &id
		}
		m.MaterialVersionIDs = []string{}
		out = append(out, m)
	}
	// Reverse to ascending order.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, rows.Err()
}

// CreateSubmission is the unified POST /projects/{p}/submissions entry
// (04 §1). All material versions must be ready in-project; the submission
// and its committed message land in one transaction keyed by
// (project, actor, clientSubmissionId) so retries return the original.
func (s *Service) CreateSubmission(ctx context.Context, requester, projectID uuid.UUID, sub Submission) (Submission, error) {
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
	if l := utf8.RuneCountInString(sub.Text); l < 1 || l > 65536 {
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
		if err := tx.LockProjectForUpdate(ctx, projectID.String()); err != nil {
			return err
		}
		role, err := memberRoleTx(ctx, tx, projectID, requester)
		if err != nil {
			return err
		}
		_ = role
		// Replay: identical clientSubmissionId returns the stored result.
		var existing Submission
		scanErr := tx.QueryRow(ctx, `
			SELECT id, client_submission_id, purpose, source, text, topic_id, task_id, identity_id,
			       actor_user_id, status, created_at
			FROM submissions
			WHERE project_id=$1 AND actor_user_id=$2 AND client_submission_id=$3`,
			projectID, requester, sub.ClientSubmissionID).
			Scan(&existing.ID, &existing.ClientSubmissionID, &existing.Purpose, &existing.Source,
				&existing.Text, &existing.TopicID, &existing.TaskID, &existing.IdentityID,
				&existing.ActorUserID, &existing.Status, &existing.CreatedAt)
		if scanErr == nil {
			// Attach message ref if present.
			var messageID uuid.NullUUID
			_ = tx.QueryRow(ctx, `SELECT id FROM messages WHERE submission_id=$1 LIMIT 1`, existing.ID).Scan(&messageID)
			if messageID.Valid {
				id := messageID.UUID
				existing.MessageID = &id
			}
			existing.MaterialVersionIDs, _ = s.submissionMaterials(ctx, tx, existing.ID)
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
				WHERE v.id=$1`, versionID).Scan(&state, &materialProject); err != nil {
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
	now := s.now()
	id := uuid.New()
	payload := map[string]any{"text": sub.Text, "purpose": sub.Purpose, "materials": sub.MaterialVersionIDs}
	rawPayload, _ := json.Marshal(payload)
	sum := sha256.Sum256(rawPayload)
	if _, err := tx.Exec(ctx, `
		INSERT INTO submissions (project_id, id, actor_user_id, identity_id, source, client_submission_id,
			text, topic_id, task_id, purpose, status, payload_hash, expected_task_version, code_refs_json, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'ready',$11,$12,'[]',$13)`,
		projectID, id, requester, nullableUUID(sub.IdentityID), sub.Source, sub.ClientSubmissionID,
		sub.Text, nullableUUID(sub.TopicID), nullableUUID(sub.TaskID), sub.Purpose,
		hex.EncodeToString(sum[:]), 0, now); err != nil {
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
