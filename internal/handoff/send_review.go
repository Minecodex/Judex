package handoff

import (
	"context"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

func (s *Service) SendReviewed(ctx context.Context, user, project, handoff, source, report uuid.UUID, nextVersion int64, summary string) (Source, error) {
	var out Source
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, project.String()); err != nil {
			return err
		}
		if err := memberTx(ctx, tx, project, user); err != nil {
			return err
		}
		var expected int64
		if err := tx.QueryRow(ctx, `SELECT COALESCE(max(revision),0)+1 FROM source_versions WHERE project_id=$1 AND source_id=$2`, project, source).Scan(&expected); err != nil {
			return err
		}
		if nextVersion != expected {
			return apierrors.New(apierrors.ReviewStale, "source revision changed")
		}
		var err error
		out, err = s.SendSource(ctx, user, project, handoff, source, summary, report)
		return err
	})
	return out, err
}
