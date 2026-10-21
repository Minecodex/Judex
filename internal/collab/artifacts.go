package collab

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

func (s *Service) PublishArtifact(ctx context.Context, p Publication, callID, name string, data []byte, file *uuid.UUID, expected int64) (uuid.UUID, error) {
	id := uuid.Nil
	if len(data) > 8<<20 || name == "" || path.Base(name) != name || strings.ContainsAny(name, "\\\x00") {
		return id, apierrors.Fields("name", "invalid artifact")
	}
	err := s.write(ctx, func(ctx context.Context) error {
		if e := s.lockConversation(ctx, p.GroupID); e != nil {
			return e
		}
		pub := &publication{RoleID: p.RoleID, RunID: p.RunID, DiscussionID: p.DiscussionID, Fence: p.Fence, Path: p.Path, DeliveryID: p.DeliveryID}
		if e := s.checkPublication(ctx, pub); e != nil {
			return e
		}
		fid := uuid.New()
		revision := 1
		if file != nil {
			fid = *file
			var current int64
			if e := s.Pool.QueryRow(ctx, `SELECT latest_revision+1,version FROM files WHERE id=$1 AND conversation_id=$2 FOR UPDATE`, fid, p.GroupID).Scan(&revision, &current); e != nil {
				return missing()
			}
			if current != expected {
				return conflict()
			}
		} else {
			if _, e := s.Pool.Exec(ctx, `INSERT INTO files(id,conversation_id,name,created_by) VALUES($1,$2,$3,$4)`, fid, p.GroupID, name, p.UserID); e != nil {
				return e
			}
		}
		id = uuid.NewSHA1(p.RunID, []byte(callID))
		key := "v2/versions/" + id.String() + "/original"
		media := detectMime(name, data)
		sum := sha256.Sum256(data)
		hash := hex.EncodeToString(sum[:])
		text := ""
		if utf8.Valid(data) && (strings.HasPrefix(media, "text/") || media == "application/json") {
			text = string(data)
		}
		if e := s.Objects.Put(ctx, key, bytes.NewReader(data), int64(len(data)), media); e != nil {
			return e
		}
		if _, e := s.Pool.Exec(ctx, `INSERT INTO file_versions(id,file_id,revision,object_key,sha256,size,mime,content_text,author_user_id,name) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, id, fid, revision, key, hash, len(data), media, text, p.UserID, name); e != nil {
			return e
		}
		if _, e := s.Pool.Exec(ctx, `UPDATE files SET latest_revision=$2,removed=false,version=version+1 WHERE id=$1`, fid, revision); e != nil {
			return e
		}
		if _, e := s.appendMessage(ctx, p.GroupID, p.UserID, "agent", MessageInput{FileVersionIDs: []uuid.UUID{id}}, pub); e != nil {
			return e
		}
		if e := s.knowledge(ctx, p.GroupID, "file", id.String(), map[string]any{"versionId": id, "fileId": fid, "revision": revision, "name": name, "mime": media, "content": text, "authorUserId": p.UserID, "roleAgentId": p.RoleID}); e != nil {
			return e
		}
		payload, _ := json.Marshal(map[string]any{"versionId": id})
		_, e := s.Pool.Exec(ctx, `INSERT INTO background_jobs(id,kind,payload,unique_key,run_after,created_at,updated_at) VALUES($1,'file.preview',$2,$3,now(),now(),now())`, uuid.New(), payload, id.String())
		return e
	})
	return id, err
}
