package collab

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

const PartSize int64 = 8 << 20
const MaxFileSize int64 = 100 << 20

type UploadInput struct {
	Name            string     `json:"name"`
	Mime            string     `json:"mime"`
	Size            int64      `json:"size"`
	SHA256          string     `json:"sha256"`
	FileID          *uuid.UUID `json:"fileId,omitempty"`
	ExpectedVersion *int64     `json:"expectedVersion,omitempty"`
}
type Upload struct {
	ID, ConversationID, UserID uuid.UUID
	FileID                     *uuid.UUID
	Name, Mime, SHA256, State  string
	Size                       int64
	ExpectedVersion            *int64
	ResultVersionID            *uuid.UUID
}

func (s *Service) fileAccess(ctx context.Context, conversation, version uuid.UUID) error {
	var allowed bool
	e := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM file_versions v JOIN files f ON f.id=v.file_id WHERE v.id=$2 AND
 (f.conversation_id=$1 OR EXISTS(SELECT 1 FROM file_grants g WHERE g.conversation_id=$1 AND g.version_id=v.id) OR EXISTS(SELECT 1 FROM message_files mf JOIN messages m ON m.id=mf.message_id WHERE mf.version_id=v.id AND m.conversation_id=$1)))`, conversation, version).Scan(&allowed)
	if e != nil {
		return e
	}
	if !allowed {
		return missing()
	}
	return nil
}
func (s *Service) CreateUpload(ctx context.Context, conversation uuid.UUID, in UploadInput) (json.RawMessage, error) {
	if _, e := s.Access(ctx, conversation); e != nil {
		return nil, e
	}
	if s.Objects == nil {
		return nil, apierrors.New(apierrors.DependencyDown, "object storage unavailable")
	}
	if e := validText(in.Name, "name", 255, true); e != nil {
		return nil, e
	}
	if path.Base(in.Name) != in.Name || strings.ContainsAny(in.Name, "\\\x00") {
		return nil, apierrors.Fields("name", "file name only")
	}
	if in.Size < 0 || in.Size > MaxFileSize {
		return nil, apierrors.Fields("size", "range")
	}
	sum, e := hex.DecodeString(in.SHA256)
	if e != nil || len(sum) != 32 {
		return nil, apierrors.Fields("sha256", "sha256 hex required")
	}
	if in.FileID != nil {
		var current int64
		e = s.Pool.QueryRow(ctx, `SELECT version FROM files WHERE id=$1 AND conversation_id=$2 FOR UPDATE`, in.FileID, conversation).Scan(&current)
		if e != nil {
			return nil, missing()
		}
		if in.ExpectedVersion == nil || current != *in.ExpectedVersion {
			return nil, conflict()
		}
	}
	if in.Mime == "" {
		in.Mime = "application/octet-stream"
	}
	p, _ := actor(ctx)
	id := uuid.New()
	_, e = s.Pool.Exec(ctx, `INSERT INTO uploads(id,conversation_id,user_id,file_id,name,mime,size,sha256,expected_version) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, conversation, p.UserID, in.FileID, in.Name, in.Mime, in.Size, strings.ToLower(in.SHA256), in.ExpectedVersion)
	if e != nil {
		return nil, e
	}
	return json.Marshal(map[string]any{"id": id, "partSize": PartSize, "partCount": max(1, (in.Size+PartSize-1)/PartSize)})
}
func (s *Service) upload(ctx context.Context, conversation, id uuid.UUID) (Upload, error) {
	out := Upload{}
	if _, e := s.Access(ctx, conversation); e != nil {
		return out, e
	}
	p, _ := actor(ctx)
	e := s.Pool.QueryRow(ctx, `SELECT id,conversation_id,user_id,file_id,name,mime,size,sha256,state,expected_version,result_version_id FROM uploads WHERE id=$1 AND conversation_id=$2 AND user_id=$3 AND expires_at>now()`, id, conversation, p.UserID).Scan(&out.ID, &out.ConversationID, &out.UserID, &out.FileID, &out.Name, &out.Mime, &out.Size, &out.SHA256, &out.State, &out.ExpectedVersion, &out.ResultVersionID)
	if errors.Is(e, pgx.ErrNoRows) {
		return out, missing()
	}
	return out, e
}
func (s *Service) UploadPart(ctx context.Context, conversation, id uuid.UUID, part int, body io.Reader) (json.RawMessage, error) {
	u, e := s.upload(ctx, conversation, id)
	if e != nil {
		return nil, e
	}
	if u.State != "pending" {
		return nil, apierrors.New(apierrors.InvalidTransition, "upload is not pending")
	}
	count := max(int64(1), (u.Size+PartSize-1)/PartSize)
	if part < 1 || int64(part) > count {
		return nil, apierrors.Fields("part", "range")
	}
	data, e := io.ReadAll(io.LimitReader(body, PartSize+1))
	if e != nil {
		return nil, e
	}
	expected := PartSize
	if int64(part) == count {
		expected = u.Size - (int64(part)-1)*PartSize
	}
	if int64(len(data)) != expected {
		return nil, apierrors.Fields("body", "part size mismatch")
	}
	hash := sha256.Sum256(data)
	sum := hex.EncodeToString(hash[:])
	key := fmt.Sprintf("v2/uploads/%s/%d/%s", id, part, sum)
	if e = s.Objects.Put(ctx, key, bytes.NewReader(data), int64(len(data)), "application/octet-stream"); e != nil {
		return nil, e
	}
	tag, e := s.Pool.Exec(ctx, `INSERT INTO upload_parts(upload_id,part,object_key,sha256,size) VALUES($1,$2,$3,$4,$5) ON CONFLICT(upload_id,part) DO UPDATE SET object_key=EXCLUDED.object_key WHERE upload_parts.sha256=EXCLUDED.sha256 AND upload_parts.size=EXCLUDED.size`, id, part, key, sum, len(data))
	if e != nil {
		return nil, e
	}
	if tag.RowsAffected() == 0 {
		return nil, apierrors.New(apierrors.IdempotencyConflict, "part already contains different bytes")
	}
	return json.Marshal(map[string]any{"part": part, "sha256": sum, "size": len(data)})
}
func (s *Service) CompleteUpload(ctx context.Context, conversation, id uuid.UUID, announce ...bool) (json.RawMessage, error) {
	var result uuid.UUID
	err := s.write(ctx, func(ctx context.Context) error {
		u, e := s.upload(ctx, conversation, id)
		if e != nil {
			return e
		}
		if u.ResultVersionID != nil {
			result = *u.ResultVersionID
			return nil
		}
		if e = s.lockConversation(ctx, conversation); e != nil {
			return e
		}
		// Re-read after the lock: concurrent completions return the same immutable version.
		u, e = s.upload(ctx, conversation, id)
		if e != nil {
			return e
		}
		if u.ResultVersionID != nil {
			result = *u.ResultVersionID
			return nil
		}
		if u.State != "pending" {
			return apierrors.New(apierrors.InvalidTransition, "upload cancelled")
		}
		rows, e := s.Pool.Query(ctx, `SELECT part,object_key FROM upload_parts WHERE upload_id=$1 ORDER BY part`, id)
		if e != nil {
			return e
		}
		keys := []string{}
		for rows.Next() {
			var n int
			var key string
			if e = rows.Scan(&n, &key); e != nil {
				rows.Close()
				return e
			}
			if n != len(keys)+1 {
				rows.Close()
				return apierrors.Fields("parts", "missing")
			}
			keys = append(keys, key)
		}
		rows.Close()
		if int64(len(keys)) != max(int64(1), (u.Size+PartSize-1)/PartSize) {
			return apierrors.Fields("parts", "missing")
		}
		var data bytes.Buffer
		hash := sha256.New()
		for _, key := range keys {
			r, e := s.Objects.Get(ctx, key)
			if e != nil {
				return e
			}
			_, e = io.Copy(io.MultiWriter(&data, hash), io.LimitReader(r, u.Size+1-int64(data.Len())))
			r.Close()
			if e != nil {
				return e
			}
		}
		if int64(data.Len()) != u.Size || hex.EncodeToString(hash.Sum(nil)) != u.SHA256 {
			return apierrors.Fields("sha256", "file checksum mismatch")
		}
		p, _ := actor(ctx)
		fileID := uuid.New()
		revision := 1
		if u.FileID != nil {
			fileID = *u.FileID
			var version int64
			if e = s.Pool.QueryRow(ctx, `SELECT latest_revision+1,version FROM files WHERE id=$1 AND conversation_id=$2 FOR UPDATE`, fileID, conversation).Scan(&revision, &version); e != nil {
				return missing()
			}
			if u.ExpectedVersion == nil || version != *u.ExpectedVersion {
				return conflict()
			}
		} else {
			if _, e = s.Pool.Exec(ctx, `INSERT INTO files(id,conversation_id,name,created_by) VALUES($1,$2,$3,$4)`, fileID, conversation, u.Name, p.UserID); e != nil {
				return e
			}
		}
		result = uuid.New()
		mimeType := detectMime(u.Name, data.Bytes())
		key := "v2/versions/" + result.String() + "/original"
		if e = s.Objects.Put(ctx, key, bytes.NewReader(data.Bytes()), u.Size, mimeType); e != nil {
			return e
		}
		text := ""
		if utf8.Valid(data.Bytes()) && (strings.HasPrefix(mimeType, "text/") || mimeType == "application/json") {
			text = data.String()
		}
		if _, e = s.Pool.Exec(ctx, `INSERT INTO file_versions(id,file_id,revision,object_key,sha256,size,mime,content_text,author_user_id,name) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, result, fileID, revision, key, u.SHA256, u.Size, mimeType, text, p.UserID, u.Name); e != nil {
			return e
		}
		if _, e = s.Pool.Exec(ctx, `UPDATE files SET latest_revision=$2,removed=false,version=version+1 WHERE id=$1`, fileID, revision); e != nil {
			return e
		}
		if _, e = s.Pool.Exec(ctx, `UPDATE uploads SET state='completed',result_version_id=$2 WHERE id=$1`, id, result); e != nil {
			return e
		}
		source := "web"
		if p.Kind == "cli" {
			source = "cli"
		}
		if len(announce) == 0 || announce[0] {
			if _, e = s.appendMessage(ctx, conversation, p.UserID, source, MessageInput{FileVersionIDs: []uuid.UUID{result}}, nil); e != nil {
				return e
			}
		}
		var group bool
		if e = s.Pool.QueryRow(ctx, `SELECT kind='group' FROM conversations WHERE id=$1`, conversation).Scan(&group); e != nil {
			return e
		}
		if group {
			if e = s.knowledge(ctx, conversation, "file", result.String(), map[string]any{"versionId": result, "fileId": fileID, "name": u.Name, "revision": revision, "mime": mimeType, "content": text, "authorUserId": p.UserID}); e != nil {
				return e
			}
		}
		payload, _ := json.Marshal(map[string]any{"versionId": result})
		_, e = s.Pool.Exec(ctx, `INSERT INTO background_jobs(id,kind,payload,unique_key,run_after,created_at,updated_at) VALUES($1,'file.preview',$2,$3,now(),now(),now())`, uuid.New(), payload, result.String())
		return e
	})
	if err != nil {
		return nil, err
	}
	return s.FileVersion(ctx, conversation, result)
}
func (s *Service) CancelUpload(ctx context.Context, conversation, id uuid.UUID) error {
	if _, e := s.upload(ctx, conversation, id); e != nil {
		return e
	}
	_, e := s.Pool.Exec(ctx, `UPDATE uploads SET state='cancelled' WHERE id=$1 AND state='pending'`, id)
	return e
}

const fileColumns = `v.id,v.file_id AS "fileId",v.revision,v.sha256,v.size,v.mime,v.created_at AS "createdAt",v.author_user_id AS "authorUserId",
 (SELECT display_name FROM users WHERE id=v.author_user_id) AS "authorName",
 (SELECT CASE WHEN avatar_png IS NULL THEN 0 ELSE avatar_version END FROM users WHERE id=v.author_user_id) AS "authorAvatarVersion",
 v.name,CASE WHEN f.conversation_id=$1 THEN f.version ELSE COALESCE((SELECT fg.version FROM file_grants fg WHERE fg.conversation_id=$1 AND fg.version_id=v.id),1) END AS version,CASE WHEN f.conversation_id=$1 THEN f.removed ELSE false END AS removed,f.conversation_id=$1 AS owned,v.preview_state AS "previewState",v.preview_data-'objectKey' AS "previewData"`

func (s *Service) Files(ctx context.Context, conversation uuid.UUID) ([]json.RawMessage, error) {
	if _, e := s.Access(ctx, conversation); e != nil {
		return nil, e
	}
	return s.list(ctx, `SELECT DISTINCT ON(f.id) `+fileColumns+` FROM files f JOIN file_versions v ON v.file_id=f.id WHERE
 (f.conversation_id=$1 AND NOT f.removed) OR (f.conversation_id<>$1 AND EXISTS(SELECT 1 FROM file_grants g WHERE g.conversation_id=$1 AND g.version_id=v.id AND g.listed))
 ORDER BY f.id,v.revision DESC`, conversation)
}
func (s *Service) FileVersion(ctx context.Context, conversation, version uuid.UUID) (json.RawMessage, error) {
	if _, e := s.Access(ctx, conversation); e != nil {
		return nil, e
	}
	if e := s.fileAccess(ctx, conversation, version); e != nil {
		return nil, e
	}
	return s.one(ctx, `SELECT `+fileColumns+` FROM file_versions v JOIN files f ON f.id=v.file_id WHERE v.id=$2`, conversation, version)
}
func (s *Service) FileVersions(ctx context.Context, conversation, file uuid.UUID) ([]json.RawMessage, error) {
	if _, e := s.Access(ctx, conversation); e != nil {
		return nil, e
	}
	return s.list(ctx, `SELECT `+fileColumns+` FROM file_versions v JOIN files f ON f.id=v.file_id WHERE f.id=$2 AND (f.conversation_id=$1 OR EXISTS(SELECT 1 FROM file_grants g WHERE g.conversation_id=$1 AND g.version_id=v.id) OR EXISTS(SELECT 1 FROM message_files mf JOIN messages m ON m.id=mf.message_id WHERE m.conversation_id=$1 AND mf.version_id=v.id)) ORDER BY v.revision DESC`, conversation, file)
}

type FileContent struct {
	Body       io.ReadCloser
	Mime, Name string
	Size       int64
}

func (s *Service) Content(ctx context.Context, conversation, version uuid.UUID, preview bool) (FileContent, error) {
	out := FileContent{}
	if _, e := s.Access(ctx, conversation); e != nil {
		return out, e
	}
	if e := s.fileAccess(ctx, conversation, version); e != nil {
		return out, e
	}
	var key string
	var data map[string]any
	e := s.Pool.QueryRow(ctx, `SELECT v.object_key,v.mime,v.size,v.name,v.preview_data FROM file_versions v JOIN files f ON f.id=v.file_id WHERE v.id=$1`, version).Scan(&key, &out.Mime, &out.Size, &out.Name, &data)
	if e != nil {
		return out, e
	}
	if preview {
		if previewMime, ok := data["mime"].(string); ok {
			out.Mime = previewMime
		}
		if k, ok := data["objectKey"].(string); ok {
			key = k
			out.Mime = "application/pdf"
			if previewMime, ok := data["mime"].(string); ok {
				out.Mime = previewMime
			}
			if n, ok := data["size"].(float64); ok {
				out.Size = int64(n)
			}
		}
	}
	out.Body, e = s.Objects.Get(ctx, key)
	return out, e
}
func (s *Service) RemoveFile(ctx context.Context, conversation, file uuid.UUID, version int64) error {
	return s.write(ctx, func(ctx context.Context) error {
		if _, e := s.Access(ctx, conversation); e != nil {
			return e
		}
		if e := s.lockConversation(ctx, conversation); e != nil {
			return e
		}
		var owned bool
		if e := s.Pool.QueryRow(ctx, `SELECT conversation_id=$1 FROM files WHERE id=$2`, conversation, file).Scan(&owned); e != nil {
			return missing()
		}
		if owned {
			tag, e := s.Pool.Exec(ctx, `UPDATE files SET removed=true,version=version+1 WHERE id=$1 AND conversation_id=$2 AND version=$3`, file, conversation, version)
			if e != nil {
				return e
			}
			if tag.RowsAffected() == 0 {
				return conflict()
			}
		} else {
			var current int64
			if e := s.Pool.QueryRow(ctx, `SELECT g.version FROM file_grants g JOIN file_versions v ON v.id=g.version_id WHERE g.conversation_id=$1 AND v.file_id=$2 AND g.listed ORDER BY v.revision DESC LIMIT 1`, conversation, file).Scan(&current); e != nil {
				return missing()
			}
			if current != version {
				return conflict()
			}
			if _, e := s.Pool.Exec(ctx, `UPDATE file_grants g SET listed=false,version=g.version+1 FROM file_versions v WHERE g.version_id=v.id AND g.conversation_id=$1 AND v.file_id=$2`, conversation, file); e != nil {
				return e
			}
		}
		return s.event(ctx, conversation, "file.removed", map[string]any{"fileId": file})
	})
}

func detectMime(name string, data []byte) string {
	detected := http.DetectContentType(data[:min(len(data), 512)])
	ext := strings.ToLower(path.Ext(name))
	if bytes.HasPrefix(data, []byte("%PDF-")) {
		return "application/pdf"
	}
	if bytes.HasPrefix(data, []byte("PK")) {
		if ext == ".docx" || ext == ".xlsx" || ext == ".pptx" {
			return mime.TypeByExtension(ext)
		}
		return "application/zip"
	}
	if strings.HasPrefix(detected, "text/plain") {
		if ext == ".json" {
			return "application/json"
		}
		if ext == ".md" {
			return "text/markdown; charset=utf-8"
		}
	}
	return detected
}
