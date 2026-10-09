package material

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	"github.com/kakj-go/Judex/internal/job"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/events"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type FilePreview struct {
	Status     string            `json:"status"`
	Kind       string            `json:"kind"`
	Mime       string            `json:"mime"`
	Size       int64             `json:"size"`
	Error      *string           `json:"error"`
	ContentURL *string           `json:"contentUrl"`
	Thumbnail  *PreviewThumbnail `json:"thumbnail"`
}

// A thumbnail describes how to read a miniature of the immutable preview.
// PDF.js renders page one; images and safe text/directory covers share the
// authenticated content stream. It never exposes the object-store key.
type PreviewThumbnail struct {
	Kind string `json:"kind"`
	URL  string `json:"url"`
	Page *int   `json:"page"`
}
type previewWork struct {
	Project   uuid.UUID `json:"project"`
	Version   uuid.UUID `json:"version"`
	Requester uuid.UUID `json:"requester"`
}

func (s *Service) SetConverterURL(value string) { s.converterURL = strings.TrimRight(value, "/") }
func previewKind(format string) string {
	switch format {
	case "pdf", "docx", "xlsx", "pptx":
		return "pdf"
	case "image":
		return "image"
	case "text", "markdown", "json":
		return "text"
	case "archive":
		return "archive"
	}
	return "unsupported"
}
func (s *Service) FilePreview(ctx context.Context, user, project, version uuid.UUID) (FilePreview, error) {
	item, err := s.LibraryVersion(ctx, user, project, version)
	if err != nil {
		return FilePreview{}, err
	}
	out := FilePreview{Status: "not_requested", Kind: previewKind(item.Format)}
	var validationRevision int
	err = s.pool.QueryRow(ctx, `SELECT status,kind,mime,size,error,validation_revision FROM material_previews WHERE project_id=$1 AND version_id=$2`, project, version).Scan(&out.Status, &out.Kind, &out.Mime, &out.Size, &out.Error, &validationRevision)
	if err == pgx.ErrNoRows {
		return out, nil
	}
	if out.Status == "ready" && (out.Kind == "pdf" || out.Kind == "image") && validationRevision < 1 {
		out.Status = "not_requested"
	}
	if err == nil && out.Status == "ready" {
		content := fmt.Sprintf("/api/v1/projects/%s/material-versions/%s/preview/content", project, version)
		out.ContentURL = &content
		out.Thumbnail = &PreviewThumbnail{Kind: out.Kind, URL: content}
		if out.Kind == "pdf" {
			page := 1
			out.Thumbnail.Page = &page
		}
	}
	return out, err
}
func (s *Service) PrepareFilePreview(ctx context.Context, user, project, version uuid.UUID, retry ...bool) (FilePreview, error) {
	item, err := s.LibraryVersion(ctx, user, project, version)
	if err != nil {
		return FilePreview{}, err
	}
	kind := previewKind(item.Format)
	status := "pending"
	if kind == "unsupported" {
		status = "unsupported"
	}
	err = s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, project.String()); err != nil {
			return err
		}
		var old string
		var validationRevision int
		err := tx.QueryRow(ctx, `SELECT status,validation_revision FROM material_previews WHERE version_id=$1 FOR UPDATE`, version).Scan(&old, &validationRevision)
		validated := kind != "pdf" && kind != "image" || validationRevision >= 1
		if err == nil && (old == "pending" || old == "ready" && validated && (len(retry) == 0 || !retry[0]) || old == "unsupported") {
			return nil
		}
		if err != nil && err != pgx.ErrNoRows {
			return err
		}
		now := s.now()
		if _, err = tx.Exec(ctx, `INSERT INTO material_previews(project_id,version_id,status,kind,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$5) ON CONFLICT(version_id) DO UPDATE SET status=EXCLUDED.status,error=NULL,updated_at=EXCLUDED.updated_at`, project, version, status, kind, now); err != nil {
			return err
		}
		if status == "pending" {
			var id uuid.UUID
			id, err = job.Enqueue(ctx, tx, "material.preview", previewWork{project, version, user}, nil, now, now)
			if err == nil {
				_, err = tx.Exec(ctx, `UPDATE material_previews SET job_id=$2 WHERE version_id=$1`, version, id)
			}
		}
		return err
	})
	if err != nil {
		return FilePreview{}, err
	}
	return s.FilePreview(ctx, user, project, version)
}
func (s *Service) PreviewHandler() job.Handler {
	return job.HandlerFunc{KindName: "material.preview", Attempts: 3, ExecuteFn: func(ctx context.Context, j job.Job) error {
		err := s.executeFilePreview(ctx, j)
		if err != nil {
			var w previewWork
			if json.Unmarshal(j.Payload, &w) == nil {
				cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
				defer cancel()
				_ = s.previewResult(cleanup, j, w, "failed", "", 0, "", "preview dependency unavailable; retry")
			}
		}
		return err
	}}
}
func (s *Service) executeFilePreview(ctx context.Context, j job.Job) error {
	var work previewWork
	if err := json.Unmarshal(j.Payload, &work); err != nil {
		return err
	}
	var current bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM material_previews WHERE version_id=$1 AND job_id=$2)`, work.Version, j.ID).Scan(&current); err != nil {
		return err
	}
	if !current {
		return nil
	}
	item, err := s.LibraryVersion(ctx, work.Requester, work.Project, work.Version)
	if err != nil {
		return err
	}
	body, version, err := s.FileContent(ctx, work.Requester, work.Project, item.ID, work.Version)
	if err != nil {
		return s.previewResult(ctx, j, work, "failed", "", 0, "", err.Error())
	}
	defer body.Close()
	raw, err := io.ReadAll(io.LimitReader(body, s.limits.MaxBundleBytes+1))
	if err != nil {
		return err
	}
	if int64(len(raw)) > s.limits.MaxBundleBytes {
		return s.previewResult(ctx, j, work, "failed", "", 0, "", "preview limit exceeded")
	}
	mime := version.Mime
	switch item.Format {
	case "docx", "pptx", "xlsx":
		if s.converterURL == "" {
			return s.previewResult(ctx, j, work, "failed", "", 0, "", "Office converter unavailable")
		}
		timeout, cancel := context.WithTimeout(ctx, 90*time.Second)
		defer cancel()
		req, e := http.NewRequestWithContext(timeout, "POST", s.converterURL+"/convert?name="+url.QueryEscape(item.Title), bytes.NewReader(raw))
		if e != nil {
			return e
		}
		req.Header.Set("Content-Type", "application/octet-stream")
		response, e := (&http.Client{Timeout: 95 * time.Second}).Do(req)
		if e != nil {
			return s.previewResult(ctx, j, work, "failed", "", 0, "", "Office conversion failed")
		}
		defer response.Body.Close()
		result, e := io.ReadAll(io.LimitReader(response.Body, s.limits.MaxFileBytes+1))
		if e != nil {
			return e
		}
		if response.StatusCode != 200 || !bytes.HasPrefix(result, []byte("%PDF-")) || int64(len(result)) > s.limits.MaxFileBytes {
			return s.previewResult(ctx, j, work, "failed", "", 0, "", "Office file cannot be converted")
		}
		raw = result
		mime = "application/pdf"
	case "pdf":
		mime = "application/pdf"
	case "image":
		mime = http.DetectContentType(raw[:min(len(raw), 512)])
		if !strings.HasPrefix(mime, "image/") {
			return s.previewResult(ctx, j, work, "failed", "", 0, "", "invalid image")
		}
		if err := validatePreviewImage(raw); err != nil {
			reason := "invalid image"
			if strings.Contains(err.Error(), "limit") {
				reason = "image preview pixel limit exceeded"
			}
			return s.previewResult(ctx, j, work, "failed", "", 0, "", reason)
		}
	case "archive":
		archive, e := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
		if e != nil {
			return s.previewResult(ctx, j, work, "failed", "", 0, "", "invalid ZIP")
		}
		if len(archive.File) > 10000 {
			return s.previewResult(ctx, j, work, "failed", "", 0, "", "archive directory limit exceeded")
		}
		entries := []map[string]any{}
		for _, file := range archive.File {
			if !safeRelativePath(file.Name) {
				return s.previewResult(ctx, j, work, "failed", "", 0, "", "unsafe archive path")
			}
			entries = append(entries, map[string]any{"name": file.Name, "size": file.UncompressedSize64, "directory": file.FileInfo().IsDir()})
		}
		raw, e = json.Marshal(entries)
		if e != nil {
			return e
		}
		mime = "application/json"
	case "text", "markdown", "json":
		if len(raw) > 4<<20 {
			return s.previewResult(ctx, j, work, "failed", "", 0, "", "text preview limit exceeded; download original")
		}
		if item.Format == "json" && !json.Valid(raw) {
			return s.previewResult(ctx, j, work, "failed", "", 0, "", "invalid JSON")
		}
		mime = "text/plain; charset=utf-8"
	default:
		return s.previewResult(ctx, j, work, "unsupported", "", 0, "", "unsupported file format")
	}
	if mime == "application/pdf" {
		if err := validatePreviewPDF(ctx, raw); err != nil {
			reason := "invalid PDF structure"
			if strings.Contains(err.Error(), "limit") || strings.Contains(err.Error(), "deadline") {
				reason = "PDF preview resource limit exceeded"
			}
			return s.previewResult(ctx, j, work, "failed", "", 0, "", reason)
		}
	}
	key := fmt.Sprintf("previews/%s/%s/%s-%d", work.Project, work.Version, j.ID, j.FencingToken)
	if err = s.store.Put(ctx, key, bytes.NewReader(raw), int64(len(raw)), mime); err != nil {
		return err
	}
	return s.previewResult(ctx, j, work, "ready", key, int64(len(raw)), mime, "")
}
func (s *Service) previewResult(ctx context.Context, j job.Job, w previewWork, status, key string, size int64, mime, reason string) error {
	return s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT id FROM projects WHERE id=$1 FOR UPDATE`, w.Project); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE material_previews SET status=$2,object_key=NULLIF($3,''),size=$4,mime=$5,error=NULLIF($6,''),updated_at=$7,validation_revision=1 WHERE version_id=$1 AND job_id=$8 AND EXISTS(SELECT 1 FROM background_jobs WHERE id=$8 AND fencing_token=$9 AND state='running')`, w.Version, status, key, size, mime, reason, s.now(), j.ID, j.FencingToken)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		_, err = events.AppendProjectEvent(ctx, tx, w.Project, "material.preview.changed", "material_version", w.Version.String(), nil, map[string]any{"previewStatus": status}, s.now())
		return err
	})
}
func (s *Service) PreviewContent(ctx context.Context, user, project, version uuid.UUID) (io.ReadCloser, FilePreview, error) {
	out, err := s.FilePreview(ctx, user, project, version)
	if err != nil {
		return nil, out, err
	}
	if out.Status != "ready" {
		return nil, out, apierrors.New(apierrors.RequirementUnmet, "preview is not ready")
	}
	var key string
	if err = s.pool.QueryRow(ctx, `SELECT object_key FROM material_previews WHERE project_id=$1 AND version_id=$2`, project, version).Scan(&key); err != nil {
		return nil, out, err
	}
	body, err := s.store.Get(ctx, key)
	return body, out, err
}
