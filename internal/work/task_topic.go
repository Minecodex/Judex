package work

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/events"
)

// EnsureTaskMainTopic is an explicit member action, never called by reports or
// background analysis. The project lock and unique FK make retries and separate
// browser requests converge on one persisted conversation.
func (s *Service) EnsureTaskMainTopic(ctx context.Context, user, project, task uuid.UUID) (id uuid.UUID, created bool, err error) {
	err = s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if e := tx.LockActiveProject(ctx, project.String()); e != nil {
			return e
		}
		if _, e := memberTx(ctx, tx, project, user); e != nil {
			return e
		}
		var existing *uuid.UUID
		var title string
		if e := tx.QueryRow(ctx, `SELECT main_topic_id,title FROM tasks WHERE project_id=$1 AND id=$2 FOR UPDATE`, project, task).Scan(&existing, &title); e != nil {
			if errors.Is(e, pgx.ErrNoRows) {
				return apierrors.New(apierrors.NotFound, "task not found")
			}
			return e
		}
		if existing != nil {
			id = *existing
			return nil
		}
		id = uuid.New()
		now := s.now()
		if _, e := tx.Exec(ctx, `INSERT INTO topics(project_id,id,title,kind,context_type,context_id,created_by,created_at) VALUES($1,$2,$3,'discussion','task',$4,$5,$6)`, project, id, title, task, user, now); e != nil {
			return e
		}
		if _, e := tx.Exec(ctx, `INSERT INTO topic_work_links(project_id,topic_id,object_type,object_id,created_at) VALUES($1,$2,'task',$3,$4)`, project, id, task, now); e != nil {
			return e
		}
		if _, e := tx.Exec(ctx, `UPDATE tasks SET main_topic_id=$3 WHERE project_id=$1 AND id=$2`, project, task, id); e != nil {
			return e
		}
		_, e := events.AppendProjectEvent(ctx, tx, project, "topic.links.changed", "topic", id.String(), nil, map[string]any{"taskId": task, "mainTopicId": id}, now)
		created = e == nil
		return e
	})
	return
}
