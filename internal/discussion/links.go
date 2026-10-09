package discussion

import (
	"context"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/collaboration"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

type TopicFilter struct {
	PlanID, TaskID *uuid.UUID
	Query, Kind    string
}

func (s *Service) ReplaceTopicLinks(ctx context.Context, user, project, topic uuid.UUID, expected int64, links []TopicLink) error {
	if expected < 1 {
		return apierrors.Fields("expectedLinksVersion", "positive")
	}
	return s.changeLinks(ctx, user, project, topic, &expected, links, true)
}
func (s *Service) changeLinks(ctx context.Context, user, project, topic uuid.UUID, expected *int64, links []TopicLink, replace bool) error {
	return s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if e := tx.LockActiveProject(ctx, project.String()); e != nil {
			return e
		}
		if _, e := memberRoleTx(ctx, tx, project, user); e != nil {
			return e
		}
		return collaboration.ChangeTopicLinks(ctx, tx.Tx, project, topic, expected, links, replace, s.now())
	})
}
