package decision

import (
	"context"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"time"
)

func (s *Service) UpdateDraft(ctx context.Context, user, project, proposal uuid.UUID, version int64, changes []Change, reason string) (int64, error) {
	var revision int64
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, project.String()); err != nil {
			return err
		}
		if _, err := memberTx(ctx, tx, project, user); err != nil {
			return err
		}
		var current int64
		var state string
		if err := tx.QueryRow(ctx, `SELECT version,status FROM proposals WHERE id=$1 AND project_id=$2 FOR UPDATE`, proposal, project).Scan(&current, &state); err != nil {
			return apierrors.New(apierrors.NotFound, "proposal not found")
		}
		if state != "draft" {
			return apierrors.New(apierrors.InvalidTransition, "only draft may be edited")
		}
		if current != version {
			return apierrors.New(apierrors.VersionConflict, "draft changed")
		}
		if reason == "" {
			reason = "draft edit"
		}
		var err error
		revision, err = s.CreateRevision(ctx, user, project, proposal, changes, reason)
		return err
	})
	return revision, err
}

func (s *Service) GetProposal(ctx context.Context, user, project, id uuid.UUID) (map[string]any, error) {
	if _, err := memberTx(ctx, s.pool, project, user); err != nil {
		return nil, err
	}
	var kind, status string
	var version int64
	var created time.Time
	var review *uuid.UUID
	if err := s.pool.QueryRow(ctx, `SELECT kind,status,version,created_at,current_review_id FROM proposals WHERE id=$1 AND project_id=$2`, id, project).Scan(&kind, &status, &version, &created, &review); err != nil {
		return nil, apierrors.New(apierrors.NotFound, "proposal not found")
	}
	return map[string]any{"id": id, "kind": kind, "status": status, "version": version, "createdAt": created, "currentReviewId": review}, nil
}
