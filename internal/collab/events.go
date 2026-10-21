package collab

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/platform/auth"
)

type Event struct {
	Seq  int64           `json:"seq"`
	Kind string          `json:"kind"`
	Data json.RawMessage `json:"data"`
}

// Only the public profile identifier is broadcast, never private account data.
func (s *Service) ProfileChanged(ctx context.Context, user uuid.UUID) error {
	rows, err := s.Pool.Query(ctx, `SELECT $1::uuid AS id UNION SELECT other.user_id FROM project_members mine JOIN project_members other USING(project_id) WHERE mine.user_id=$1 AND mine.state='active' AND other.state='active' UNION SELECT other.user_id FROM conversation_members mine JOIN conversation_members other USING(conversation_id) WHERE mine.user_id=$1 AND mine.active AND other.active ORDER BY id`, user)
	if err != nil {
		return err
	}
	recipients := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		recipients = append(recipients, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range recipients {
		if err = s.userEvent(ctx, id, nil, "profile.changed", map[string]any{"userId": user}); err != nil {
			return err
		}
	}
	return nil
}

// Configuration events carry identifiers only, and never borrow a group's
// audience: project members need updates even when they have joined no groups.
func (s *Service) projectEvent(ctx context.Context, project uuid.UUID, resource string, id uuid.UUID) error {
	rows, err := s.Pool.Query(ctx, `SELECT user_id FROM project_members WHERE project_id=$1 AND state='active' ORDER BY user_id`, project)
	if err != nil {
		return err
	}
	users := []uuid.UUID{}
	for rows.Next() {
		var user uuid.UUID
		if err = rows.Scan(&user); err != nil {
			rows.Close()
			return err
		}
		users = append(users, user)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, user := range users {
		if err = s.userEventProject(ctx, user, nil, &project, "project.changed", map[string]any{"projectId": project, "resource": resource, "resourceId": id}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) event(ctx context.Context, id uuid.UUID, kind string, data any) error {
	raw, e := json.Marshal(data)
	if e != nil {
		return e
	}
	var seq int64
	if e = s.Pool.QueryRow(ctx, `UPDATE conversations SET event_seq=event_seq+1 WHERE id=$1 RETURNING event_seq`, id).Scan(&seq); e != nil {
		return e
	}
	_, e = s.Pool.Exec(ctx, `INSERT INTO conversation_events(conversation_id,seq,kind,data) VALUES($1,$2,$3,$4)`, id, seq, kind, raw)
	if e != nil {
		return e
	}
	rows, e := s.Pool.Query(ctx, `SELECT user_id FROM conversation_members WHERE conversation_id=$1 AND active ORDER BY user_id`, id)
	if e != nil {
		return e
	}
	users := []uuid.UUID{}
	for rows.Next() {
		var user uuid.UUID
		if e = rows.Scan(&user); e != nil {
			rows.Close()
			return e
		}
		users = append(users, user)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, user := range users {
		if e = s.userEvent(ctx, user, &id, "conversation.changed", map[string]any{"conversationId": id}); e != nil {
			return e
		}
	}
	return nil
}
func (s *Service) userEvent(ctx context.Context, user uuid.UUID, conversation *uuid.UUID, kind string, data any) error {
	return s.userEventProject(ctx, user, conversation, nil, kind, data)
}
func (s *Service) userEventProject(ctx context.Context, user uuid.UUID, conversation, project *uuid.UUID, kind string, data any) error {
	raw, e := json.Marshal(data)
	if e != nil {
		return e
	}
	var seq int64
	if e = s.Pool.QueryRow(ctx, `INSERT INTO user_event_counters(user_id,seq) VALUES($1,1) ON CONFLICT(user_id) DO UPDATE SET seq=user_event_counters.seq+1 RETURNING seq`, user).Scan(&seq); e != nil {
		return e
	}
	_, e = s.Pool.Exec(ctx, `INSERT INTO user_events(user_id,seq,kind,conversation_id,project_id,data) VALUES($1,$2,$3,$4,$5,$6)`, user, seq, kind, conversation, project, raw)
	return e
}
func (s *Service) notice(ctx context.Context, user uuid.UUID, conversation, project *uuid.UUID, kind string, data any, dedupe string) error {
	raw, e := json.Marshal(data)
	if e != nil {
		return e
	}
	id := uuid.New()
	tag, e := s.Pool.Exec(ctx, `INSERT INTO notifications(id,user_id,conversation_id,project_id,kind,data,dedupe_key) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(user_id,dedupe_key) DO NOTHING`, id, user, conversation, project, kind, raw, dedupe)
	if e != nil || tag.RowsAffected() == 0 {
		return e
	}
	return s.userEventProject(ctx, user, conversation, project, "notification.created", map[string]any{"id": id, "kind": kind, "data": data})
}
func (s *Service) Events(ctx context.Context, id uuid.UUID, after int64) ([]Event, error) {
	if _, e := s.Access(ctx, id); e != nil {
		return nil, e
	}
	rows, e := s.Pool.Query(ctx, `SELECT seq,kind,data FROM conversation_events WHERE conversation_id=$1 AND seq>$2 ORDER BY seq LIMIT 100`, id, after)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var item Event
		if e = rows.Scan(&item.Seq, &item.Kind, &item.Data); e != nil {
			return nil, e
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// Filter before delivery, while advancing over inaccessible records without disclosing them.
func (s *Service) PersonalEvents(ctx context.Context, after int64) ([]Event, int64, error) {
	p, e := actor(ctx)
	if e != nil {
		return nil, after, e
	}
	rows, e := s.Pool.Query(ctx, `SELECT seq,kind,conversation_id,project_id,data FROM user_events WHERE user_id=$1 AND seq>$2 ORDER BY seq LIMIT 100`, p.UserID, after)
	if e != nil {
		return nil, after, e
	}
	type candidate struct {
		event        Event
		conversation *uuid.UUID
		project      *uuid.UUID
	}
	items := []candidate{}
	for rows.Next() {
		var item candidate
		if e = rows.Scan(&item.event.Seq, &item.event.Kind, &item.conversation, &item.project, &item.event.Data); e != nil {
			rows.Close()
			return nil, after, e
		}
		items = append(items, item)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, after, e
	}
	out := []Event{}
	for _, item := range items {
		after = item.event.Seq
		if item.project != nil {
			if e = s.projectAccess(ctx, *item.project, false); e != nil {
				continue
			}
		}
		if item.conversation != nil {
			if _, e = s.Access(ctx, *item.conversation); e != nil {
				continue
			}
		}
		out = append(out, item.event)
	}
	return out, after, nil
}
func (s *Service) Notifications(ctx context.Context) ([]json.RawMessage, error) {
	p, e := actor(ctx)
	if e != nil {
		return nil, e
	}
	return s.list(ctx, `SELECT n.id,n.kind,n.data,n.conversation_id AS "conversationId",COALESCE(n.project_id,(SELECT project_id FROM groups WHERE id=n.conversation_id)) AS "projectId",n.read_at AS "readAt",n.created_at AS "createdAt" FROM notifications n
 WHERE n.user_id=$1 AND (n.conversation_id IS NULL OR EXISTS(SELECT 1 FROM conversation_members cm WHERE cm.conversation_id=n.conversation_id AND cm.user_id=$1 AND cm.active))
 AND (n.project_id IS NULL OR EXISTS(SELECT 1 FROM project_members pm WHERE pm.project_id=n.project_id AND pm.user_id=$1 AND pm.state='active')) AND ($2::boolean OR COALESCE(n.project_id,(SELECT project_id FROM groups WHERE id=n.conversation_id))=ANY($3::uuid[]) OR (n.project_id IS NULL AND NOT EXISTS(SELECT 1 FROM groups WHERE id=n.conversation_id))) ORDER BY n.created_at DESC LIMIT 100`, p.UserID, p.Kind != auth.KindCLI, nonnil(p.ProjectScope))
}
func (s *Service) ReadNotice(ctx context.Context, id uuid.UUID) error {
	p, e := actor(ctx)
	if e != nil {
		return e
	}
	_, e = s.Pool.Exec(ctx, `UPDATE notifications SET read_at=COALESCE(read_at,now()) WHERE user_id=$1 AND id=$2`, p.UserID, id)
	return e
}
