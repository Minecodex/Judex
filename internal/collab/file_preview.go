package collab

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"path"
	"strings"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/job"
)

func (s *Service) PreviewJob() job.Handler {
	return job.HandlerFunc{KindName: "file.preview", Attempts: 3, ExecuteFn: func(ctx context.Context, j job.Job) error {
		var payload struct {
			VersionID uuid.UUID `json:"versionId"`
		}
		if e := json.Unmarshal(j.Payload, &payload); e != nil {
			return e
		}
		e := s.preparePreview(ctx, payload.VersionID, j)
		if e != nil {
			_, update := s.Pool.Exec(context.WithoutCancel(ctx), `UPDATE file_versions SET preview_state='failed',preview_data=jsonb_build_object('error',$2::text) WHERE id=$1 AND EXISTS(SELECT 1 FROM background_jobs WHERE id=$3 AND fencing_token=$4 AND state='running')`, payload.VersionID, "Preview processing failed; retry available", j.ID, j.FencingToken)
			if update != nil {
				return update
			}
		}
		return e
	}}
}
func (s *Service) preparePreview(ctx context.Context, id uuid.UUID, lease job.Job) error {
	var key, name, format string
	var size int64
	var group uuid.UUID
	var isGroup bool
	if e := s.Pool.QueryRow(ctx, `SELECT v.object_key,v.mime,v.size,v.name,f.conversation_id,c.kind='group' FROM file_versions v JOIN files f ON f.id=v.file_id JOIN conversations c ON c.id=f.conversation_id WHERE v.id=$1`, id).Scan(&key, &format, &size, &name, &group, &isGroup); e != nil {
		return e
	}
	r, e := s.Objects.Get(ctx, key)
	if e != nil {
		return e
	}
	raw, e := io.ReadAll(io.LimitReader(r, MaxFileSize+1))
	r.Close()
	if e != nil {
		return e
	}
	if int64(len(raw)) != size {
		return fmt.Errorf("stored file size mismatch")
	}
	data, text, e := s.previewData(ctx, id, name, format, raw)
	if e != nil {
		return e
	}
	encoded, _ := json.Marshal(data)
	return s.write(ctx, func(ctx context.Context) error {
		if e = s.lockConversation(ctx, group); e != nil {
			return e
		}
		var current bool
		if e = s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM background_jobs WHERE id=$1 AND fencing_token=$2 AND state='running' AND lease_until>now())`, lease.ID, lease.FencingToken).Scan(&current); e != nil {
			return e
		}
		if !current {
			return fmt.Errorf("preview job lease lost")
		}
		if _, e = s.Pool.Exec(ctx, `UPDATE file_versions SET preview_state='ready',preview_data=$2,content_text=CASE WHEN $3<>'' THEN $3 ELSE content_text END WHERE id=$1`, id, encoded, text); e != nil {
			return e
		}
		if isGroup {
			if e = s.lockConversation(ctx, group); e != nil {
				return e
			}
			if e = s.knowledge(ctx, group, "file_preview", id.String(), map[string]any{"versionId": id, "name": name, "kind": data["kind"], "content": text}); e != nil {
				return e
			}
		}
		return s.event(ctx, group, "file.ready", map[string]any{"versionId": id})
	})
}
func safePath(value string) bool {
	return value != "" && !strings.ContainsAny(value, "\\\x00:") && !strings.HasPrefix(value, "/") && path.Clean(value) == value && value != ".." && !strings.HasPrefix(value, "../")
}
func (s *Service) expandBundle(ctx context.Context, id uuid.UUID, raw []byte) (map[string]any, error) {
	archive, e := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if e != nil {
		return nil, e
	}
	if len(archive.File) > 2000 {
		return nil, fmt.Errorf("too many archive entries")
	}
	entries := []map[string]any{}
	seen := map[string]bool{}
	var total int64
	entrypoint := ""
	for _, f := range archive.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if !f.Mode().IsRegular() || !safePath(f.Name) || seen[strings.ToLower(f.Name)] {
			return nil, fmt.Errorf("unsafe or duplicate archive path")
		}
		seen[strings.ToLower(f.Name)] = true
		r, e := f.Open()
		if e != nil {
			return nil, e
		}
		data, e := io.ReadAll(io.LimitReader(r, 200<<20-total+1))
		r.Close()
		if e != nil {
			return nil, e
		}
		total += int64(len(data))
		if total > 200<<20 {
			return nil, fmt.Errorf("expanded archive too large")
		}
		key := "v2/versions/" + id.String() + "/entries/" + f.Name
		media := mime.TypeByExtension(path.Ext(f.Name))
		if media == "" {
			media = "application/octet-stream"
		}
		if e = s.Objects.Put(ctx, key, bytes.NewReader(data), int64(len(data)), media); e != nil {
			return nil, e
		}
		hash := sha256.Sum256(data)
		if _, e = s.Pool.Exec(ctx, `INSERT INTO file_entries(version_id,path,object_key,mime,size,sha256) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(version_id,path) DO NOTHING`, id, f.Name, key, media, len(data), hex.EncodeToString(hash[:])); e != nil {
			return nil, e
		}
		entries = append(entries, map[string]any{"path": f.Name, "size": len(data), "mime": media})
		if f.Name == "index.html" {
			entrypoint = f.Name
		}
	}
	kind := "archive"
	if entrypoint != "" {
		kind = "html"
	}
	return map[string]any{"kind": kind, "entries": entries, "entrypoint": entrypoint}, nil
}
func (s *Service) Entry(ctx context.Context, conversation, version uuid.UUID, entry string) (FileContent, error) {
	out := FileContent{}
	if !safePath(entry) {
		return out, missing()
	}
	if _, e := s.Access(ctx, conversation); e != nil {
		return out, e
	}
	if e := s.fileAccess(ctx, conversation, version); e != nil {
		return out, e
	}
	var key string
	e := s.Pool.QueryRow(ctx, `SELECT object_key,mime,size FROM file_entries WHERE version_id=$1 AND path=$2`, version, entry).Scan(&key, &out.Mime, &out.Size)
	if e != nil {
		return out, missing()
	}
	out.Name = path.Base(entry)
	out.Body, e = s.Objects.Get(ctx, key)
	return out, e
}
func (s *Service) RetryPreview(ctx context.Context, conversation, version uuid.UUID) error {
	if _, e := s.Access(ctx, conversation); e != nil {
		return e
	}
	if e := s.fileAccess(ctx, conversation, version); e != nil {
		return e
	}
	payload, _ := json.Marshal(map[string]any{"versionId": version})
	tag, e := s.Pool.Exec(ctx, `INSERT INTO background_jobs(id,kind,payload,unique_key,run_after,created_at,updated_at) VALUES($1,'file.preview',$2,$3,now(),now(),now()) ON CONFLICT(kind,unique_key) WHERE unique_key IS NOT NULL DO UPDATE SET state='queued',attempt=0,run_after=now(),fencing_token=background_jobs.fencing_token+1 WHERE background_jobs.state<>'running'`, uuid.New(), payload, version.String())
	if e != nil {
		return e
	}
	if tag.RowsAffected() == 0 {
		return conflict()
	}
	_, e = s.Pool.Exec(ctx, `UPDATE file_versions SET preview_state='pending',preview_data='{}' WHERE id=$1`, version)
	return e
}
