package work

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/kakj-go/Judex/internal/platform/events"
)

// Withdrawal never deletes the frozen review or its decisions. Historical
// consent stays auditable, but cannot be reused for the revised arrangement.
func withdrawDraftReviews(ctx context.Context, tx pgx.Tx, project uuid.UUID, ids []uuid.UUID, applyingReview uuid.UUID, now time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	textIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		textIDs = append(textIDs, id.String())
	}
	rows, err := tx.Query(ctx, `
		UPDATE proposals p SET status='cancelled',version=version+1,updated_at=$3
		WHERE p.project_id=$1 AND p.status IN('draft','pending','stale')
		AND p.current_review_id IS DISTINCT FROM $4::uuid
		AND EXISTS (
			SELECT 1 FROM proposal_versions v
			CROSS JOIN LATERAL jsonb_path_query(v.changes_json,'$.**') c
			WHERE v.proposal_id=p.id AND v.id=p.current_review_id
			AND jsonb_typeof(c)='string' AND c#>>'{}'=ANY($2)
		) RETURNING p.id`, project, textIDs, now, applyingReview)
	if err != nil {
		return err
	}
	var affected []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		affected = append(affected, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range affected {
		if _, err = events.AppendProjectEvent(ctx, tx, project, "proposal.changed", "proposal", id.String(), nil, map[string]any{"change": "draft_review_withdrawn"}, now); err != nil {
			return err
		}
	}
	return nil
}
