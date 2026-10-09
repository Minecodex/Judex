package work

import (
	"context"
	"github.com/google/uuid"
)

// Populate the visible page in one query. Route cards get real responsibilities
// without a browser request for every task detail.
func (s *Service) taskParticipants(ctx context.Context, project uuid.UUID, tasks []Task) error {
	ids := []uuid.UUID{}
	indexes := map[uuid.UUID]int{}
	for i := range tasks {
		ids = append(ids, tasks[i].ID)
		indexes[tasks[i].ID] = i
		tasks[i].Participants = []Participant{}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, e := s.pool.Query(ctx, `SELECT p.task_id,p.identity_id,COALESCE(u.display_name,''),p.responsibility
 FROM task_participants p JOIN agent_identities i ON i.project_id=p.project_id AND i.id=p.identity_id
 LEFT JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version AND b.valid_until IS NULL
 LEFT JOIN users u ON u.id=b.user_id WHERE p.project_id=$1 AND p.task_id=ANY($2) ORDER BY p.task_id,p.identity_id`, project, ids)
	if e != nil {
		return e
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var p Participant
		if e = rows.Scan(&id, &p.IdentityID, &p.DisplayName, &p.Responsibility); e != nil {
			return e
		}
		i := indexes[id]
		tasks[i].Participants = append(tasks[i].Participants, p)
	}
	return rows.Err()
}
