package project

import (
	"context"
	"github.com/google/uuid"
)

// Keep invitation choices identical in create, manager and recipient reads.
// Hydrate a whole page in one query rather than querying once per invitation.
func (s *Service) invitationPositions(ctx context.Context, invitations []Invitation) error {
	ids := make([]uuid.UUID, 0, len(invitations))
	indexes := map[uuid.UUID]int{}
	for i := range invitations {
		invitations[i].Positions = []PositionLite{}
		ids = append(ids, invitations[i].ID)
		indexes[invitations[i].ID] = i
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := s.pool.Query(ctx, `SELECT ip.invitation_id,p.id,p.name FROM invitation_positions ip JOIN position_templates p ON p.project_id=ip.project_id AND p.id=ip.position_template_id WHERE ip.invitation_id=ANY($1::uuid[]) ORDER BY p.name,p.id`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var invitation uuid.UUID
		var position PositionLite
		if err = rows.Scan(&invitation, &position.ID, &position.Name); err != nil {
			return err
		}
		i := indexes[invitation]
		invitations[i].Positions = append(invitations[i].Positions, position)
	}
	return rows.Err()
}
