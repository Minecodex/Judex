// SPDX-License-Identifier: Apache-2.0

// Package project implements project lifecycle per docs/plans/v1/02 §4:
// atomic creation (project + owner membership + main venue + coordinator
// identity + first event), member-visible listing, archive/restore. Joining
// is invitation-only; nothing here grants global privileges.
package project

import (
	"context"
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

// Project is the API projection (06 §3).
type Project struct {
	ID                    uuid.UUID  `json:"id"`
	Title                 string     `json:"title"`
	Description           string     `json:"description"`
	Kind                  string     `json:"kind"`
	Status                string     `json:"status"`
	CreatorUserID         uuid.UUID  `json:"-"`
	OwnerUserID           uuid.UUID  `json:"-"`
	MaxDiscussionRounds   int        `json:"maxDiscussionRounds"`
	ApprovalTimeoutSeconds int       `json:"approvalTimeoutSeconds"`
	DefaultModelID        *uuid.UUID `json:"defaultModelId"`
	Version               int64      `json:"version"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
	// ViewerRole is filled per requester: owner/manager/member or nil.
	ViewerRole *string `json:"role"`
}

// CreateRequest is the create command payload.
type CreateRequest struct {
	Title                 string
	Description           string
	Kind                  string
	MaxDiscussionRounds   int
	ApprovalTimeoutSeconds int
}

// Service owns project persistence.
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

func validateCreate(req CreateRequest) error {
	req.Title = strings.TrimSpace(req.Title)
	if l := utf8.RuneCountInString(req.Title); l < 1 || l > 200 {
		return apierrors.Fields("title", "length")
	}
	if req.Kind == "" {
		req.Kind = "software"
	}
	if req.Kind != "software" && req.Kind != "design" {
		return apierrors.Fields("kind", "enum")
	}
	if req.MaxDiscussionRounds == 0 {
		req.MaxDiscussionRounds = 3
	}
	if req.MaxDiscussionRounds < 1 || req.MaxDiscussionRounds > 100 {
		return apierrors.Fields("maxDiscussionRounds", "range")
	}
	if req.ApprovalTimeoutSeconds == 0 {
		req.ApprovalTimeoutSeconds = 86400
	}
	if req.ApprovalTimeoutSeconds < 60 {
		return apierrors.Fields("approvalTimeoutSeconds", "range")
	}
	return nil
}

// Create performs the atomic creation of 02 §4: one transaction writing the
// project, the owner membership, the main venue topic, the coordinator
// identity (unbound), the first project event and the audit row. The creator
// does NOT receive any position binding — positions arrive via invitations.
func (s *Service) Create(ctx context.Context, creator uuid.UUID, req CreateRequest) (Project, error) {
	if err := validateCreate(req); err != nil {
		return Project{}, err
	}
	if req.Kind == "" {
		req.Kind = "software"
	}
	if req.MaxDiscussionRounds == 0 {
		req.MaxDiscussionRounds = 3
	}
	if req.ApprovalTimeoutSeconds == 0 {
		req.ApprovalTimeoutSeconds = 86400
	}
	title := strings.TrimSpace(req.Title)

	var created Project
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockUserForShare(ctx, creator.String()); err != nil {
			return err
		}
		now := s.now()
		id := uuid.New()
		if err := tx.QueryRow(ctx, `
			INSERT INTO projects (id, title, description, kind, creator_user_id, owner_user_id,
			                      max_discussion_rounds, approval_timeout_seconds, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$5,$6,$7,$8,$8)
			RETURNING id, title, description, kind, status, max_discussion_rounds, approval_timeout_seconds, version, created_at, updated_at`,
			id, title, req.Description, req.Kind, creator, req.MaxDiscussionRounds, req.ApprovalTimeoutSeconds, now).
			Scan(&created.ID, &created.Title, &created.Description, &created.Kind, &created.Status,
				&created.MaxDiscussionRounds, &created.ApprovalTimeoutSeconds,
				&created.Version, &created.CreatedAt, &created.UpdatedAt); err != nil {
			return apierrors.New(apierrors.Internal, "project insert failed").Wrap(err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO project_members (project_id, user_id, role, state, joined_at)
			VALUES ($1,$2,'owner','active',$3)`, id, creator, now); err != nil {
			return apierrors.New(apierrors.Internal, "owner membership failed").Wrap(err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO topics (project_id, id, title, kind, created_by, created_at)
			VALUES ($1,$2,$3,'project_room',$4,$5)`,
			id, uuid.New(), title, creator, now); err != nil {
			return apierrors.New(apierrors.Internal, "main venue failed").Wrap(err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO agent_identities (project_id, id, kind, status, created_at)
			VALUES ($1,$2,'coordinator','active',$3)`, id, uuid.New(), now); err != nil {
			return apierrors.New(apierrors.Internal, "coordinator identity failed").Wrap(err)
		}
		if _, err := events.AppendProjectEvent(ctx, tx, id, "membership.changed", "project", id.String(), nil,
			map[string]any{"change": "created"}, now); err != nil {
			return apierrors.New(apierrors.Internal, "event append failed").Wrap(err)
		}
		return audit.Append(ctx, tx, audit.Entry{
			ProjectID:   &id,
			ActorType:   audit.ActorUser,
			ActorUserID: &creator,
			Source:      audit.SourceWeb,
			Operation:   "project.create",
			ObjectType:  "project", ObjectID: id.String(),
			OccurredAt: now,
		})
	})
	if err != nil {
		return Project{}, err
	}
	created.CreatorUserID = creator
	created.OwnerUserID = creator
	owner := "owner"
	created.ViewerRole = &owner
	return created, nil
}

// Membership is the viewer's standing in one project.
type Membership struct {
	Role      string
	State     string
	ProjectID uuid.UUID
	UserID    uuid.UUID
}

// MembershipFor returns the ACTIVE membership or NOT_FOUND (project
// existence is hidden from non-members per 02 §4/E07).
func (s *Service) MembershipFor(ctx context.Context, requester, projectID uuid.UUID) (Membership, error) {
	var m Membership
	err := s.pool.QueryRow(ctx, `
		SELECT project_id, user_id, role, state FROM project_members
		WHERE project_id=$1 AND user_id=$2 AND state='active'`, projectID, requester).
		Scan(&m.ProjectID, &m.UserID, &m.Role, &m.State)
	if errors.Is(err, pgx.ErrNoRows) {
		// Distinguish "project missing" and "not a member" identically.
		return Membership{}, apierrors.New(apierrors.NotFound, "project not found")
	}
	if err != nil {
		return Membership{}, apierrors.New(apierrors.Internal, "membership lookup failed").Wrap(err)
	}
	return m, nil
}

// ListForUser returns projects where the user is an active member, newest
// first (cursor pagination wired at the handler layer).
func (s *Service) ListForUser(ctx context.Context, user uuid.UUID, limit int, afterCreatedAt *time.Time, afterID *uuid.UUID) ([]Project, bool, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT p.id, p.title, p.description, p.kind, p.status, p.version,
		       p.max_discussion_rounds, p.approval_timeout_seconds, p.default_model_id,
		       p.created_at, p.updated_at, m.role
		FROM project_members m JOIN projects p ON p.id = m.project_id
		WHERE m.user_id=$1 AND m.state='active'
		  AND ($2::timestamptz IS NULL OR (p.created_at, p.id) < ($2, $3::uuid))
		ORDER BY p.created_at DESC, p.id DESC
		LIMIT $4`, user, afterCreatedAt, afterID, limit+1)
	if err != nil {
		return nil, false, apierrors.New(apierrors.Internal, "project list failed").Wrap(err)
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		p := Project{}
		if err := rows.Scan(&p.ID, &p.Title, &p.Description, &p.Kind, &p.Status, &p.Version,
			&p.MaxDiscussionRounds, &p.ApprovalTimeoutSeconds, &p.DefaultModelID,
			&p.CreatedAt, &p.UpdatedAt, &p.ViewerRole); err != nil {
			return nil, false, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, false, apierrors.New(apierrors.Internal, "rows failed").Wrap(err)
	}
	more := len(out) > limit
	if more {
		out = out[:limit]
	}
	return out, more, nil
}

// Get returns the project if the requester is an active member.
func (s *Service) Get(ctx context.Context, requester, projectID uuid.UUID) (Project, error) {
	if _, err := s.MembershipFor(ctx, requester, projectID); err != nil {
		return Project{}, err
	}
	var p Project
	err := s.pool.QueryRow(ctx, `
		SELECT id, title, description, kind, status, version,
		       max_discussion_rounds, approval_timeout_seconds, default_model_id,
		       created_at, updated_at
		FROM projects WHERE id=$1`, projectID).
		Scan(&p.ID, &p.Title, &p.Description, &p.Kind, &p.Status, &p.Version,
			&p.MaxDiscussionRounds, &p.ApprovalTimeoutSeconds, &p.DefaultModelID,
			&p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Project{}, apierrors.New(apierrors.NotFound, "project not found")
	}
	if err != nil {
		return Project{}, apierrors.New(apierrors.Internal, "project get failed").Wrap(err)
	}
	role := "member"
	if m, err := s.MembershipFor(ctx, requester, projectID); err == nil {
		role = m.Role
	}
	p.ViewerRole = &role
	return p, nil
}

// Archive (owner only) freezes writes: status flips, reason audited, history
// retained; restore reverses it (02 §4 归档不是清空数据).
func (s *Service) Archive(ctx context.Context, requester, projectID uuid.UUID, expectedVersion int64, reason string, archive bool) (Project, error) {
	if strings.TrimSpace(reason) == "" {
		return Project{}, apierrors.Fields("reason", "required")
	}
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockProjectForUpdate(ctx, projectID.String()); err != nil {
			return err
		}
		m, err := s.MembershipForTx(ctx, tx, requester, projectID)
		if err != nil {
			return err
		}
		if m.Role != "owner" {
			return apierrors.New(apierrors.Forbidden, "only the owner can archive or restore")
		}
		wantStatus := "archived"
		if !archive {
			wantStatus = "active"
		}
		tag, err := tx.Exec(ctx, `
			UPDATE projects SET status=$3, archived_at=CASE WHEN $3='archived' THEN $4::timestamptz ELSE NULL END,
			       archived_reason=CASE WHEN $3='archived' THEN $5 ELSE NULL END,
			       version=version+1, updated_at=$4::timestamptz
			WHERE id=$1 AND version=$2 AND status<>$3`,
			projectID, expectedVersion, wantStatus, s.now(), reason)
		if err != nil {
			return apierrors.New(apierrors.Internal, "archive update failed").Wrap(err)
		}
		if tag.RowsAffected() == 0 {
			return apierrors.New(apierrors.VersionConflict, "project version conflict or already in state")
		}
		op := "project.archive"
		if !archive {
			op = "project.restore"
		}
		if _, err := events.AppendProjectEvent(ctx, tx, projectID, "membership.changed", "project", projectID.String(), nil,
			map[string]any{"change": wantStatus}, s.now()); err != nil {
			return err
		}
		return audit.Append(ctx, tx, audit.Entry{
			ProjectID:   &projectID,
			ActorType:   audit.ActorUser,
			ActorUserID: &requester,
			Source:      audit.SourceWeb,
			Operation:   op,
			ObjectType:  "project", ObjectID: projectID.String(),
			Reason:     &reason,
			OccurredAt: s.now(),
		})
	})
	if err != nil {
		return Project{}, err
	}
	return s.Get(ctx, requester, projectID)
}

// MembershipForTx is the transactional variant used inside commands.
func (s *Service) MembershipForTx(ctx context.Context, tx pgx.Tx, requester, projectID uuid.UUID) (Membership, error) {
	var m Membership
	err := tx.QueryRow(ctx, `
		SELECT project_id, user_id, role, state FROM project_members
		WHERE project_id=$1 AND user_id=$2 AND state='active'`, projectID, requester).
		Scan(&m.ProjectID, &m.UserID, &m.Role, &m.State)
	if errors.Is(err, pgx.ErrNoRows) {
		return Membership{}, apierrors.New(apierrors.NotFound, "project not found")
	}
	if err != nil {
		return Membership{}, apierrors.New(apierrors.Internal, "membership lookup failed").Wrap(err)
	}
	return m, nil
}

// BootstrapCursor returns the current event seq for SSE bootstrap (04 §5).
func (s *Service) BootstrapCursor(ctx context.Context, projectID uuid.UUID, out *int64) error {
	return s.pool.QueryRow(ctx,
		`SELECT event_seq FROM projects WHERE id=$1`, projectID).Scan(out)
}
