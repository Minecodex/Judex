package decision

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/events"
)

// A savepoint rolls back the whole proposed group while allowing the stale
// review itself to be recorded. HTTP returns the conflict after that commit.
func (s *Service) applyReviewed(ctx context.Context, tx pgx.Tx, project, actor, review uuid.UUID) (map[string]string, error) {
	savepoint, err := tx.Begin(ctx)
	if err != nil {
		return nil, err
	}
	created, err := s.applyChanges(ctx, savepoint, project, actor, review)
	if err == nil {
		if err = savepoint.Commit(ctx); err != nil {
			return nil, err
		}
		raw, _ := json.Marshal(created)
		if _, err = tx.Exec(ctx, `UPDATE proposal_versions SET created_ids_json=$2 WHERE id=$1`, review, raw); err != nil {
			return nil, err
		}
		return created, nil
	}
	if rollbackErr := savepoint.Rollback(ctx); rollbackErr != nil {
		return nil, rollbackErr
	}
	code := apierrors.From(err).Code
	switch code {
	case apierrors.ReviewStale, apierrors.VersionConflict, apierrors.InvalidReference, apierrors.RequirementUnmet, apierrors.DependencyChanged, apierrors.InvalidTransition, apierrors.Validation:
	default:
		return nil, err
	}
	var proposal uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT proposal_id FROM proposal_versions WHERE id=$1`, review).Scan(&proposal); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE proposals SET status='stale',version=version+1,updated_at=now() WHERE id=$1`, proposal); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE proposal_versions SET deadline_at=NULL WHERE id=$1`, review); err != nil {
		return nil, err
	}
	if _, err = events.AppendProjectEvent(ctx, tx, project, "proposal.changed", "proposal", proposal.String(), nil, map[string]any{"change": "stale", "cause": string(code)}, s.now()); err != nil {
		return nil, err
	}
	return nil, apierrors.New(apierrors.ReviewStale, "review dependencies changed; revise and collect new decisions").WithDetails(map[string]any{"proposalId": proposal, "cause": code}).WithCommittedResult()
}
