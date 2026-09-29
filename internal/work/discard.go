package work

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

func (s *Service) Discard(ctx context.Context, user, project, target uuid.UUID, kind string, version int64) error {
	table := map[string]string{"plan": "plans", "task": "tasks"}[kind]
	if table == "" {
		return apierrors.Fields("kind", "enum")
	}
	return s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, project.String()); err != nil {
			return err
		}
		if _, err := memberTx(ctx, tx, project, user); err != nil {
			return err
		}
		var author *uuid.UUID
		var state string
		var current int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT created_by,status,version FROM %s WHERE project_id=$1 AND id=$2 FOR UPDATE`, table), project, target).Scan(&author, &state, &current); err != nil {
			return apierrors.New(apierrors.NotFound, "draft not found")
		}
		if author == nil || *author != user {
			return apierrors.New(apierrors.Forbidden, "only draft author may discard")
		}
		if state != "draft" {
			return apierrors.New(apierrors.InvalidTransition, "only draft can be discarded")
		}
		if current != version {
			return apierrors.New(apierrors.VersionConflict, "draft changed")
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s SET status='cancelled',version=version+1,updated_at=now() WHERE id=$1`, table), target); err != nil {
			return err
		}
		return audit.Append(ctx, tx, audit.Entry{ProjectID: &project, ActorType: audit.ActorUser, ActorUserID: &user, Source: audit.SourceWeb, Operation: kind + ".draft.discard", ObjectType: kind, ObjectID: target.String(), OccurredAt: s.now()})
	})
}
