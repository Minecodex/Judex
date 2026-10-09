// SPDX-License-Identifier: Apache-2.0

package work

import (
	"context"
	"github.com/kakj-go/Judex/internal/platform/paging"
	"time"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/platform/auth"

	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

// PendingAction is one item of GET /me/actions (06 §4) — computed from the
// caller's CURRENT bindings (03 §9 / 04 §6), never from a cached copy.
type PendingAction struct {
	ID         uuid.UUID  `json:"id"`
	Kind       string     `json:"kind"`
	ProjectID  uuid.UUID  `json:"projectId"`
	ObjectType string     `json:"objectType"`
	ObjectID   uuid.UUID  `json:"objectId"`
	ReviewID   *uuid.UUID `json:"reviewId"`
	DueAt      *time.Time `json:"dueAt"`
	Summary    string     `json:"summary"`
	CreatedAt  time.Time  `json:"createdAt"`
}

// MyActions computes the unified todo list across proposals, handoffs,
// tasks and plans (P3-09). Read-only: no state changes.
func (s *Service) MyActions(ctx context.Context, user uuid.UUID, projectFilter *uuid.UUID, filters ...ActionFilter) ([]PendingAction, error) {
	f := ActionFilter{}
	if len(filters) > 0 {
		f = filters[0]
	}
	if err := f.Validate(); err != nil {
		return nil, err
	}
	scope := []uuid.UUID{}
	if actor := auth.FromContext(ctx); actor != nil && actor.Kind == auth.KindCLI {
		scope = actor.ProjectScope
		if len(scope) == 0 {
			return []PendingAction{}, nil
		}
	}
	rows, err := paging.Query(ctx, s.pool, myActionsSQL, "a.created_at", "a.id", user, projectFilter, scope, f.Kind, f.Category)
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "actions query failed").Wrap(err)
	}
	defer rows.Close()
	out := []PendingAction{}
	for rows.Next() {
		var a PendingAction
		if err = rows.Scan(&a.ID, &a.ProjectID, &a.ObjectID, &a.ObjectType, &a.Kind, &a.ReviewID, &a.DueAt, &a.Summary, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
