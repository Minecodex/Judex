package discussion

import (
	"context"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/collaboration"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
)

func (s *Service) Fork(ctx context.Context, user, project, parent uuid.UUID, in collaboration.ForkInput) (Topic, error) {
	var id uuid.UUID
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, project.String()); err != nil {
			return err
		}
		if _, err := memberRoleTx(ctx, tx, project, user); err != nil {
			return err
		}
		var err error
		id, err = collaboration.ForkInTx(ctx, tx.Tx, project, user, parent, in, s.now())
		return err
	})
	if err != nil {
		return Topic{}, err
	}
	return s.GetTopic(ctx, user, project, id)
}

func (s *Service) topicReferences(ctx context.Context, project uuid.UUID, topics []Topic) error {
	indexes := map[uuid.UUID]int{}
	ids := []uuid.UUID{}
	for i := range topics {
		indexes[topics[i].ID] = i
		ids = append(ids, topics[i].ID)
		topics[i].Links = []TopicLink{}
		topics[i].SourceRefs = []collaboration.SourceRef{}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := s.pool.Query(ctx, `SELECT topic_id,object_type,object_id FROM topic_work_links WHERE project_id=$1 AND topic_id=ANY($2) ORDER BY object_type,object_id`, project, ids)
	if err != nil {
		return err
	}
	for rows.Next() {
		var topic uuid.UUID
		var link TopicLink
		if err = rows.Scan(&topic, &link.ObjectType, &link.ObjectID); err != nil {
			rows.Close()
			return err
		}
		i := indexes[topic]
		topics[i].Links = append(topics[i].Links, link)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	rows, err = s.pool.Query(ctx, `SELECT ref.topic_id,ref.source_type,ref.source_id,COALESCE(r.task_id,s.task_id)
 FROM topic_source_refs ref LEFT JOIN work_reports r ON ref.source_type='report' AND r.id=ref.source_id AND r.project_id=ref.project_id
 LEFT JOIN submissions s ON ref.source_type='submission' AND s.id=ref.source_id AND s.project_id=ref.project_id
 WHERE ref.project_id=$1 AND ref.topic_id=ANY($2) ORDER BY ref.created_at,ref.source_id`, project, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var topic uuid.UUID
		var ref collaboration.SourceRef
		if err = rows.Scan(&topic, &ref.Type, &ref.ID, &ref.TaskID); err != nil {
			return err
		}
		i := indexes[topic]
		topics[i].SourceRefs = append(topics[i].SourceRefs, ref)
	}
	return rows.Err()
}
