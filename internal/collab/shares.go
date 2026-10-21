package collab

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"io"
)

type ShareInput struct {
	TargetGroupID  uuid.UUID   `json:"targetGroupId"`
	MessageID      *uuid.UUID  `json:"messageId,omitempty"`
	FileVersionIDs []uuid.UUID `json:"fileVersionIds"`
	Summary        string      `json:"summary"`
}

func (s *Service) Share(ctx context.Context, source uuid.UUID, in ShareInput) (json.RawMessage, error) {
	if e := s.requireHuman(ctx); e != nil {
		return nil, e
	}
	var result uuid.UUID
	e := s.write(ctx, func(ctx context.Context) error {
		a, e := s.Access(ctx, source)
		if e != nil {
			return e
		}
		b, e := s.Access(ctx, in.TargetGroupID)
		if e != nil {
			return e
		}
		if a.Kind != "group" || b.Kind != "group" {
			return apierrors.Fields("targetGroupId", "sharing requires joined groups")
		}
		p, _ := actor(ctx)
		body := in.Summary
		var origin json.RawMessage
		if in.MessageID != nil {
			var name, content string
			if e = s.Pool.QueryRow(ctx, `SELECT u.display_name,m.body FROM messages m JOIN users u ON u.id=m.author_user_id WHERE m.id=$1 AND m.conversation_id=$2`, in.MessageID, source).Scan(&name, &content); e != nil {
				return missing()
			}
			body = name + ":\n" + content + "\n" + body
			origin, e = s.one(ctx, `SELECT m.id AS "messageId",m.author_user_id AS "authorUserId",u.display_name AS "authorName",m.source,m.created_at AS "createdAt",pos.name AS "positionName" FROM messages m JOIN users u ON u.id=m.author_user_id LEFT JOIN role_agents r ON r.id=m.role_agent_id LEFT JOIN positions pos ON pos.id=r.position_id WHERE m.id=$1 AND m.conversation_id=$2`, in.MessageID, source)
			if e != nil {
				return e
			}
		}
		if body == "" && len(in.FileVersionIDs) == 0 {
			return apierrors.Fields("summary", "content required")
		}
		// Lock targets in a stable order when two groups share with each other concurrently.
		ids := []uuid.UUID{source, in.TargetGroupID}
		if ids[0].String() > ids[1].String() {
			ids[0], ids[1] = ids[1], ids[0]
		}
		for _, id := range ids {
			if e = s.lockConversation(ctx, id); e != nil {
				return e
			}
		}
		for _, v := range in.FileVersionIDs {
			if e = s.fileAccess(ctx, source, v); e != nil {
				return e
			}
			if _, e = s.Pool.Exec(ctx, `INSERT INTO file_grants(conversation_id,version_id,granted_by) VALUES($1,$2,$3) ON CONFLICT(conversation_id,version_id) DO UPDATE SET listed=true,version=file_grants.version+1`, in.TargetGroupID, v, p.UserID); e != nil {
				return e
			}
			var content json.RawMessage
			if content, e = s.one(ctx, `SELECT v.id AS "versionId",v.name,v.revision,v.mime,v.content_text AS content,v.author_user_id AS "authorUserId" FROM file_versions v JOIN files f ON f.id=v.file_id WHERE v.id=$1`, v); e != nil {
				return e
			}
			if e = s.knowledge(ctx, in.TargetGroupID, "file", v.String(), content); e != nil {
				return e
			}
		}
		result, e = s.appendMessage(ctx, in.TargetGroupID, p.UserID, "share", MessageInput{Body: body, FileVersionIDs: in.FileVersionIDs, origin: origin}, nil)
		if e != nil {
			return e
		}
		_, e = s.Pool.Exec(ctx, `INSERT INTO shares(id,source_message_id,target_conversation_id,message_id,shared_by) VALUES($1,$2,$3,$4,$5)`, uuid.New(), in.MessageID, in.TargetGroupID, result, p.UserID)
		return e
	})
	if e != nil {
		return nil, e
	}
	return s.Message(ctx, in.TargetGroupID, result)
}
func (s *Service) SaveCopy(ctx context.Context, conversation, version uuid.UUID, name string) (json.RawMessage, error) {
	content, e := s.Content(ctx, conversation, version, false)
	if e != nil {
		return nil, e
	}
	defer content.Body.Close()
	var sum string
	if e = s.Pool.QueryRow(ctx, `SELECT sha256 FROM file_versions WHERE id=$1`, version).Scan(&sum); e != nil {
		return nil, e
	}
	if name == "" {
		name = content.Name
	}
	raw, e := s.CreateUpload(ctx, conversation, UploadInput{Name: name, Mime: content.Mime, Size: content.Size, SHA256: sum})
	if e != nil {
		return nil, e
	}
	var upload struct {
		ID uuid.UUID `json:"id"`
	}
	if e = json.Unmarshal(raw, &upload); e != nil {
		return nil, e
	}
	for part := 1; int64(part-1)*PartSize < max(int64(1), content.Size); part++ {
		if _, e = s.UploadPart(ctx, conversation, upload.ID, part, io.LimitReader(content.Body, PartSize)); e != nil {
			return nil, e
		}
	}
	return s.CompleteUpload(ctx, conversation, upload.ID)
}
