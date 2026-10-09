package material

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/events"
	"github.com/kakj-go/Judex/internal/platform/paging"
	"path/filepath"
	"strings"
	"time"
)

type LibraryFilter struct {
	Kind, Query, Group, Sort, ObjectType string
	ObjectID                             *uuid.UUID
}
type LibraryItem struct {
	ID               uuid.UUID     `json:"id"`
	Title            string        `json:"title"`
	Kind             string        `json:"kind"`
	Visibility       string        `json:"visibility"`
	CurrentVersionID *uuid.UUID    `json:"currentVersionId"`
	CreatedAt        time.Time     `json:"createdAt"`
	VersionID        uuid.UUID     `json:"versionId"`
	Revision         int64         `json:"revision"`
	Size             int64         `json:"size"`
	Mime             string        `json:"mime"`
	Format           string        `json:"format"`
	AuthorID         *uuid.UUID    `json:"authorId"`
	AuthorName       string        `json:"authorName"`
	UploadedAt       time.Time     `json:"uploadedAt"`
	OriginalStatus   string        `json:"originalStatus"`
	Source           string        `json:"source"`
	Purpose          string        `json:"purpose"`
	DeletedAt        *time.Time    `json:"deletedAt"`
	CanDelete        bool          `json:"canDelete"`
	PreviewStatus    string        `json:"previewStatus"`
	Associations     []Association `json:"associations"`
}
type Association struct {
	Type string    `json:"type"`
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}
type Usage struct {
	ID          uuid.UUID  `json:"id"`
	Kind        string     `json:"kind"`
	Description string     `json:"description"`
	ActorID     *uuid.UUID `json:"actorId"`
	ActorName   string     `json:"actorName"`
	CreatedAt   time.Time  `json:"createdAt"`
	Source      string     `json:"source"`
	TaskID      *uuid.UUID `json:"taskId"`
	TaskName    *string    `json:"taskName"`
	PlanID      *uuid.UUID `json:"planId"`
	PlanName    *string    `json:"planName"`
	TopicID     *uuid.UUID `json:"topicId"`
	TopicName   *string    `json:"topicName"`
}

func Format(name, mime string) string {
	if mime == "application/pdf" {
		return "pdf"
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".pdf":
		return "pdf"
	case ".docx":
		return "docx"
	case ".xlsx":
		return "xlsx"
	case ".pptx":
		return "pptx"
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		return "image"
	case ".md":
		return "markdown"
	case ".json":
		return "json"
	case ".txt", ".csv", ".log", ".yaml", ".yml":
		return "text"
	case ".zip":
		return "archive"
	}
	if strings.HasPrefix(mime, "image/") && mime != "image/svg+xml" {
		return "image"
	}
	if strings.HasPrefix(mime, "text/") && mime != "text/html" {
		return "text"
	}
	return "other"
}

func (s *Service) registerLibraryMetadata(ctx context.Context, tx pgx.Tx, project, material, version, upload, user uuid.UUID, name, mime string) error {
	if _, err := tx.Exec(ctx, `UPDATE materials SET owner_user_id=COALESCE(owner_user_id,$2),purpose=CASE WHEN purpose='' THEN (SELECT purpose FROM upload_sessions WHERE id=$3) ELSE purpose END WHERE id=$1`, material, user, upload); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE material_versions SET format=$2,upload_source=$3 WHERE id=$1`, version, Format(name, mime), audit.ContextSource(ctx))
	return err
}

const librarySelect = `SELECT m.id,m.title,m.kind,m.visibility,m.current_version_id,m.created_at,
 v.id,v.revision,v.size,v.mime,v.format,v.author_id,COALESCE(u.display_name,''),v.created_at,v.state,v.upload_source,
 COALESCE(NULLIF(btrim(m.purpose),''),(SELECT r.description FROM material_usage_records r WHERE r.version_id=v.id AND r.kind IN ('material','progress','delivery') AND NULLIF(btrim(r.description),'') IS NOT NULL ORDER BY r.created_at,r.source_id LIMIT 1),''),
 m.deleted_at,(m.deleted_at IS NULL AND (m.owner_user_id=$2 OR EXISTS(SELECT 1 FROM project_members p WHERE p.project_id=m.project_id AND p.user_id=$2 AND p.state='active' AND p.role IN ('owner','manager')))),
 CASE WHEN mp.status='ready' AND mp.kind IN ('pdf','image') AND mp.validation_revision<1 THEN 'not_requested' ELSE COALESCE(mp.status,'not_requested') END,
 COALESCE((SELECT jsonb_agg(x) FROM (
 SELECT DISTINCT 'task' AS type,t.id,t.title AS name FROM material_usage_records r JOIN tasks t ON t.id=r.task_id WHERE r.version_id=v.id
 UNION SELECT DISTINCT 'plan',p.id,p.title FROM material_usage_records r JOIN plans p ON p.id=r.plan_id WHERE r.version_id=v.id) x),'[]'::jsonb)
 /*keys*/ FROM materials m JOIN material_versions v ON v.id=m.current_version_id
 LEFT JOIN users u ON u.id=v.author_id LEFT JOIN material_previews mp ON mp.version_id=v.id`

func scanLibrary(rows interface{ Scan(...any) error }, m *LibraryItem) error {
	var links []byte
	err := rows.Scan(&m.ID, &m.Title, &m.Kind, &m.Visibility, &m.CurrentVersionID, &m.CreatedAt, &m.VersionID, &m.Revision, &m.Size, &m.Mime, &m.Format, &m.AuthorID, &m.AuthorName, &m.UploadedAt, &m.OriginalStatus, &m.Source, &m.Purpose, &m.DeletedAt, &m.CanDelete, &m.PreviewStatus, &links)
	if err != nil {
		return err
	}
	return json.Unmarshal(links, &m.Associations)
}

const libraryWhere = ` WHERE m.project_id=$1 AND m.deleted_at IS NULL AND ($3='' OR m.kind=$3)
 AND ($4='' OR m.title ILIKE '%'||$4||'%' OR COALESCE(u.display_name,'') ILIKE '%'||$4||'%' OR m.purpose ILIKE '%'||$4||'%' OR EXISTS(SELECT 1 FROM material_usage_records r WHERE r.version_id=v.id AND r.description ILIKE '%'||$4||'%'))
 AND ($5='' OR $5='documents' AND v.format IN ('pdf','docx','pptx','markdown','text') OR $5='sheets' AND v.format='xlsx' OR $5='images' AND v.format='image' OR $5='other' AND v.format IN ('archive','json','other'))
 AND ($6='' OR EXISTS(SELECT 1 FROM material_usage_records r WHERE r.version_id=v.id AND (($6='task' AND r.task_id=$7) OR ($6='plan' AND r.plan_id=$7))))`

func (s *Service) libraryArgs(ctx context.Context, user, project uuid.UUID, f LibraryFilter) ([]any, error) {
	if err := isMemberTx(ctx, s.pool, project, user); err != nil {
		return nil, err
	}
	if f.ObjectType != "" && f.ObjectType != "task" && f.ObjectType != "plan" {
		return nil, apierrors.Fields("linkedObjectType", "enum")
	}
	if (f.ObjectID == nil) != (f.ObjectType == "") {
		return nil, apierrors.Fields("linkedObjectId", "paired with linkedObjectType")
	}
	if f.Sort != "" && f.Sort != "recent" && f.Sort != "type" {
		return nil, apierrors.Fields("sort", "enum")
	}
	if f.Group != "" && f.Group != "documents" && f.Group != "sheets" && f.Group != "images" && f.Group != "other" {
		return nil, apierrors.Fields("formatGroup", "enum")
	}
	return []any{project, user, f.Kind, strings.TrimSpace(f.Query), f.Group, f.ObjectType, f.ObjectID}, nil
}

func (s *Service) LibraryTotal(ctx context.Context, user, project uuid.UUID, f LibraryFilter) (int64, error) {
	args, err := s.libraryArgs(ctx, user, project, f)
	if err != nil {
		return 0, err
	}
	var count int64
	query := `SELECT count(*) FROM (` + strings.Replace(librarySelect, "/*keys*/", "", 1) + libraryWhere + `) library`
	err = s.pool.QueryRow(ctx, query, args...).Scan(&count)
	return count, err
}

func (s *Service) Library(ctx context.Context, user, project uuid.UUID, f LibraryFilter) ([]LibraryItem, error) {
	args, err := s.libraryArgs(ctx, user, project, f)
	if err != nil {
		return nil, err
	}
	sql := librarySelect + libraryWhere + ` /*page*/`
	var rows pgx.Rows
	if f.Sort == "type" {
		rows, err = paging.QueryText(ctx, s.pool, sql, "v.format", "v.created_at", "m.id", args...)
	} else {
		rows, err = paging.Query(ctx, s.pool, sql, "v.created_at", "m.id", args...)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LibraryItem{}
	for rows.Next() {
		var v LibraryItem
		if err = scanLibrary(rows, &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Service) LibraryVersion(ctx context.Context, user, project, version uuid.UUID) (LibraryItem, error) {
	var out LibraryItem
	if err := isMemberTx(ctx, s.pool, project, user); err != nil {
		return out, err
	}
	sql := strings.Replace(librarySelect, "v.id=m.current_version_id", "v.material_id=m.id", 1)
	err := scanLibrary(s.pool.QueryRow(ctx, strings.Replace(sql, "/*keys*/", "", 1)+` WHERE m.project_id=$1 AND v.id=$3`, project, user, version), &out)
	if errors.Is(err, pgx.ErrNoRows) {
		err = apierrors.New(apierrors.NotFound, "material version not found")
	}
	return out, err
}

func (s *Service) Usages(ctx context.Context, user, project, version uuid.UUID) ([]Usage, error) {
	if _, err := s.LibraryVersion(ctx, user, project, version); err != nil {
		return nil, err
	}
	rows, err := paging.Query(ctx, s.pool, `SELECT r.source_id,r.kind,COALESCE(r.description,''),r.actor_user_id,COALESCE(u.display_name,''),r.created_at,r.source,r.task_id,t.title,r.plan_id,p.title,r.topic_id,o.title /*keys*/ FROM material_usage_records r LEFT JOIN users u ON u.id=r.actor_user_id LEFT JOIN tasks t ON t.id=r.task_id LEFT JOIN plans p ON p.id=r.plan_id LEFT JOIN topics o ON o.id=r.topic_id WHERE r.project_id=$1 AND r.version_id=$2 /*page*/`, "r.created_at", "r.source_id", project, version)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Usage{}
	for rows.Next() {
		var v Usage
		if err = rows.Scan(&v.ID, &v.Kind, &v.Description, &v.ActorID, &v.ActorName, &v.CreatedAt, &v.Source, &v.TaskID, &v.TaskName, &v.PlanID, &v.PlanName, &v.TopicID, &v.TopicName); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Service) DeleteFromLibrary(ctx context.Context, user, project, id, expected uuid.UUID) error {
	return s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, project.String()); err != nil {
			return err
		}
		if err := isMemberTx(ctx, tx, project, user); err != nil {
			return err
		}
		var owner, current *uuid.UUID
		var deleted *time.Time
		var role string
		if err := tx.QueryRow(ctx, `SELECT m.owner_user_id,m.current_version_id,m.deleted_at,p.role FROM materials m JOIN project_members p ON p.project_id=m.project_id AND p.user_id=$2 AND p.state='active' WHERE m.project_id=$1 AND m.id=$3 FOR UPDATE OF m`, project, user, id).Scan(&owner, &current, &deleted, &role); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apierrors.New(apierrors.NotFound, "material not found")
			}
			return err
		}
		if (owner == nil || *owner != user) && role != "owner" && role != "manager" {
			return apierrors.New(apierrors.Forbidden, "only uploader or project manager can delete")
		}
		if current == nil || *current != expected {
			return apierrors.New(apierrors.VersionConflict, "material version changed")
		}
		if deleted != nil {
			return nil
		}
		now := s.now()
		if _, err := tx.Exec(ctx, `UPDATE materials SET deleted_at=$2,deleted_by=$3 WHERE id=$1`, id, now, user); err != nil {
			return err
		}
		if err := audit.Append(ctx, tx, audit.Entry{ProjectID: &project, ActorType: audit.ActorUser, ActorUserID: &user, Source: audit.SourceWeb, Operation: "material.delete", ObjectType: "material", ObjectID: id.String(), OccurredAt: now}); err != nil {
			return err
		}
		_, err := events.AppendProjectEvent(ctx, tx, project, "material.changed", "material", id.String(), nil, map[string]any{"deleted": true, "versionId": expected}, now)
		return err
	})
}
