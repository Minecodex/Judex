// SPDX-License-Identifier: Apache-2.0

package work

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

// Repository is the metadata projection (06 §3) — no Git credentials.
type Repository struct {
	ID            uuid.UUID  `json:"id"`
	DisplayName   string     `json:"displayName"`
	URL           string     `json:"url"`
	Provider      string     `json:"provider"`
	DefaultBranch string     `json:"defaultBranch"`
	Version       int64      `json:"version"`
	ArchivedAt    *time.Time `json:"archivedAt"`
}

func managerOnly(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, projectID, requester uuid.UUID) error {
	var role string
	err := q.QueryRow(ctx, `
		SELECT role FROM project_members WHERE project_id=$1 AND user_id=$2 AND state='active'`,
		projectID, requester).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return apierrors.New(apierrors.NotFound, "project not found")
	}
	if err != nil {
		return err
	}
	if role != "owner" && role != "manager" {
		return apierrors.New(apierrors.Forbidden, "manager or owner required")
	}
	return nil
}

// ListRepositories returns project repository metadata (member read).
func (s *Service) ListRepositories(ctx context.Context, requester, projectID uuid.UUID) ([]Repository, error) {
	if err := memberRead(ctx, s.pool, projectID, requester); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, display_name, url, provider, default_branch, version, archived_at
		FROM repository_links WHERE project_id=$1 AND archived_at IS NULL
		ORDER BY created_at`, projectID)
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "repos failed").Wrap(err)
	}
	defer rows.Close()
	var out []Repository
	for rows.Next() {
		var r Repository
		if err := rows.Scan(&r.ID, &r.DisplayName, &r.URL, &r.Provider, &r.DefaultBranch, &r.Version, &r.ArchivedAt); err != nil {
			return nil, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func memberRead(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, projectID, requester uuid.UUID) error {
	var role string
	err := q.QueryRow(ctx, `
		SELECT role FROM project_members WHERE project_id=$1 AND user_id=$2 AND state='active'`,
		projectID, requester).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return apierrors.New(apierrors.NotFound, "project not found")
	}
	return err
}

// CreateRepository registers metadata only (manager write; no push creds).
func (s *Service) CreateRepository(ctx context.Context, requester, projectID uuid.UUID, displayName, rawURL, provider, defaultBranch string) (Repository, error) {
	if strings.TrimSpace(displayName) == "" || strings.TrimSpace(rawURL) == "" {
		return Repository{}, apierrors.Fields("displayName/url", "required")
	}
	if defaultBranch == "" {
		defaultBranch = "main"
	}
	if provider == "" {
		provider = "git"
	}
	var out Repository
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockProjectForUpdate(ctx, projectID.String()); err != nil {
			return err
		}
		if err := managerOnly(ctx, tx, projectID, requester); err != nil {
			return err
		}
		now := s.now()
		id := uuid.New()
		if _, err := tx.Exec(ctx, `
			INSERT INTO repository_links (project_id, id, display_name, url, provider, default_branch, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$7)`,
			projectID, id, displayName, rawURL, provider, defaultBranch, now); err != nil {
			return apierrors.New(apierrors.Internal, "repo insert failed").Wrap(err)
		}
		if err := audit.Append(ctx, tx, audit.Entry{
			ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
			Source: audit.SourceWeb, Operation: "repository.create",
			ObjectType: "repository", ObjectID: id.String(), OccurredAt: now,
		}); err != nil {
			return err
		}
		out = Repository{ID: id, DisplayName: displayName, URL: rawURL, Provider: provider,
			DefaultBranch: defaultBranch, Version: 1}
		return nil
	})
	return out, err
}

// UpdateRepository edits metadata with version checks (manager).
func (s *Service) UpdateRepository(ctx context.Context, requester, projectID, repoID uuid.UUID, expectedVersion int64, displayName, rawURL, provider, defaultBranch string) (Repository, error) {
	var out Repository
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockProjectForUpdate(ctx, projectID.String()); err != nil {
			return err
		}
		if err := managerOnly(ctx, tx, projectID, requester); err != nil {
			return err
		}
		var current Repository
		if err := tx.QueryRow(ctx, `
			SELECT id, display_name, url, provider, default_branch, version, archived_at
			FROM repository_links WHERE id=$1 AND project_id=$2 FOR UPDATE`, repoID, projectID).
			Scan(&current.ID, &current.DisplayName, &current.URL, &current.Provider,
				&current.DefaultBranch, &current.Version, &current.ArchivedAt); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apierrors.New(apierrors.NotFound, "repository not found")
			}
			return apierrors.New(apierrors.Internal, "repo lookup failed").Wrap(err)
		}
		if current.Version != expectedVersion {
			return apierrors.New(apierrors.VersionConflict, "repository version conflict")
		}
		if displayName != "" {
			current.DisplayName = displayName
		}
		if rawURL != "" {
			current.URL = rawURL
		}
		if provider != "" {
			current.Provider = provider
		}
		if defaultBranch != "" {
			current.DefaultBranch = defaultBranch
		}
		if _, err := tx.Exec(ctx, `
			UPDATE repository_links SET display_name=$3, url=$4, provider=$5, default_branch=$6,
				version=version+1, updated_at=$7 WHERE id=$1 AND project_id=$2`,
			repoID, projectID, current.DisplayName, current.URL, current.Provider,
			current.DefaultBranch, s.now()); err != nil {
			return apierrors.New(apierrors.Internal, "repo update failed").Wrap(err)
		}
		current.Version++
		out = current
		return nil
	})
	return out, err
}

// AuditEntry is the visible audit projection (no private prompts).
type AuditEntry struct {
	ID          uuid.UUID  `json:"id"`
	ActorType   string     `json:"actorType"`
	ActorUserID *uuid.UUID `json:"actorUserId"`
	Source      string     `json:"source"`
	Operation   string     `json:"operation"`
	ObjectType  string     `json:"objectType"`
	ObjectID    string     `json:"objectId"`
	Reason      *string    `json:"reason"`
	OccurredAt  time.Time  `json:"occurredAt"`
}

// ListAudit pages the project's visible audit trail (member read).
func (s *Service) ListAudit(ctx context.Context, requester, projectID uuid.UUID) ([]AuditEntry, error) {
	if err := memberRead(ctx, s.pool, projectID, requester); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, actor_type, actor_user_id, source, operation, object_type, object_id, reason, occurred_at
		FROM audit_events WHERE project_id=$1
		ORDER BY occurred_at DESC LIMIT 200`, projectID)
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "audit failed").Wrap(err)
	}
	defer rows.Close()
	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.ActorType, &e.ActorUserID, &e.Source, &e.Operation,
			&e.ObjectType, &e.ObjectID, &e.Reason, &e.OccurredAt); err != nil {
			return nil, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
