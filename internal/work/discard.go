package work

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

func (s *Service) Discard(ctx context.Context, user, project, target uuid.UUID, kind string, version int64, reviewHash ...string) error {
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
		access, err := workAccess(ctx, tx, project, user, target, kind)
		if err != nil {
			return err
		}
		if !access.Capabilities.DiscardDraft {
			return apierrors.New(apierrors.Forbidden, "draft discard requires author, plan owner or manager")
		}
		if state != "draft" {
			return apierrors.New(apierrors.InvalidTransition, "only draft can be discarded")
		}
		if current != version {
			return apierrors.New(apierrors.VersionConflict, "draft changed")
		}
		if len(reviewHash) > 0 {
			review, e := s.discardReview(ctx, tx, tx, user, project, target, kind)
			if e != nil {
				return e
			}
			if reviewHash[0] != review.ReviewHash {
				return apierrors.New(apierrors.ReviewStale, "draft discard impact changed")
			}
		}
		ids := []uuid.UUID{target}
		var plan *uuid.UUID
		if kind == "plan" {
			plan = &target
			ids = []uuid.UUID{}
			rows, e := tx.Query(ctx, `SELECT id FROM tasks WHERE project_id=$1 AND plan_id=$2 AND discarded_at IS NULL ORDER BY id FOR UPDATE`, project, target)
			if e != nil {
				return e
			}
			for rows.Next() {
				var id uuid.UUID
				if e = rows.Scan(&id); e != nil {
					rows.Close()
					return e
				}
				ids = append(ids, id)
			}
			e = rows.Err()
			rows.Close()
			if e != nil {
				return e
			}
		}
		blockers, e := s.draftReferences(ctx, tx, project, ids, plan)
		if e != nil {
			return e
		}
		if len(blockers) > 0 {
			return apierrors.New(apierrors.DependencyChanged, "draft has effective references").WithDetails(map[string]any{"blockers": blockers})
		}
		if kind == "plan" {
			if _, e = tx.Exec(ctx, `UPDATE tasks SET status='cancelled',discarded_at=$3,version=version+1,updated_at=$3 WHERE project_id=$1 AND id=ANY($2)`, project, ids, s.now()); e != nil {
				return e
			}
			for _, id := range ids {
				if e = s.recordWorkChange(ctx, tx, project, user, id, "task", "draft.discarded", "所属草稿计划已丢弃，原记录保留", nil); e != nil {
					return e
				}
			}
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s SET status='cancelled',discarded_at=$2,version=version+1,updated_at=$2 WHERE id=$1`, table), target, s.now()); err != nil {
			return err
		}
		if e = s.withdrawDraftReviews(ctx, tx, project, append(ids, target)); e != nil {
			return e
		}
		if e = s.recordWorkChange(ctx, tx, project, user, target, kind, "draft.discarded", "未生效草稿已丢弃，原讨论和资料保留", map[string]any{"taskIds": ids}); e != nil {
			return e
		}
		return audit.Append(ctx, tx, audit.Entry{ProjectID: &project, ActorType: audit.ActorUser, ActorUserID: &user, Source: audit.SourceWeb, Operation: kind + ".draft.discard", ObjectType: kind, ObjectID: target.String(), OccurredAt: s.now()})
	})
}
