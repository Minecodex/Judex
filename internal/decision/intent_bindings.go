package decision

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Service) intentBindings(ctx context.Context, tx pgx.Tx, user, project uuid.UUID, operation string, object uuid.UUID) (map[string]int64, error) {
	// Only responsibilities involved in this command are frozen. Replacing an
	// unrelated role must not invalidate an otherwise unchanged confirmation.
	query := `SELECT i.id::text,i.current_binding_version FROM agent_identities i
 JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version AND b.valid_until IS NULL
 WHERE i.project_id=$1 AND b.user_id=$2 AND (
 ($3 IN ('task.acceptance','task.reopen') AND i.id=(SELECT reviewer_identity_id FROM tasks WHERE project_id=$1 AND id=$4)) OR
 ($3 IN ('plan.acceptance','plan.reopen') AND i.id=(SELECT owner_identity_id FROM plans WHERE project_id=$1 AND id=$4)) OR
 ($3 IN ('proposal.decision','proposal.submit') AND i.id IN(SELECT a.authority_id FROM approval_slots a JOIN proposals p ON p.current_review_id=a.review_id WHERE p.project_id=$1 AND p.id=$4 AND a.authority_type='identity' AND a.state='pending')) OR
 ($3='handoff.send' AND i.id=(SELECT sender_identity_id FROM handoff_sources WHERE project_id=$1 AND id=$4)) OR
 ($3='handoff.decision' AND i.id=(SELECT h.receiver_identity_id FROM handoffs h JOIN handoff_sources s ON s.handoff_id=h.id WHERE s.project_id=$1 AND s.id=$4)))`
	rows, err := tx.Query(ctx, query, project, user, operation, object)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]int64{}
	for rows.Next() {
		var id string
		var version int64
		if err = rows.Scan(&id, &version); err != nil {
			return nil, err
		}
		result[id] = version
	}
	return result, rows.Err()
}
