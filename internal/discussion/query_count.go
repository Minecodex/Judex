package discussion

import (
	"context"
	"github.com/google/uuid"
	"strings"
)

const topicListSQL = `
		SELECT t.id,t.title,t.kind,t.context_type,t.context_id,t.last_message_seq,t.created_at,t.parent_topic_id,t.fork_after_seq,t.links_version
 /*keys*/ FROM topics t WHERE t.project_id=$1 AND t.kind<>'handoff'
 AND ($2::uuid IS NULL OR EXISTS(SELECT 1 FROM topic_work_links l LEFT JOIN tasks w ON l.object_type='task' AND w.project_id=l.project_id AND w.id=l.object_id WHERE l.project_id=t.project_id AND l.topic_id=t.id AND ((l.object_type='plan' AND l.object_id=$2) OR w.plan_id=$2)))
 AND ($3::uuid IS NULL OR EXISTS(SELECT 1 FROM topic_work_links l WHERE l.project_id=t.project_id AND l.topic_id=t.id AND l.object_type='task' AND l.object_id=$3))
 AND ($4='' OR strpos(lower(t.title),lower($4))>0) AND ($5='' OR t.kind=$5)
 /*page*/`

func (s *Service) CountTopics(ctx context.Context, user, project uuid.UUID, f TopicFilter) (n int, err error) {
	if _, err = memberRoleTx(ctx, s.pool, project, user); err != nil {
		return
	}
	sql := strings.ReplaceAll(strings.ReplaceAll(topicListSQL, "/*keys*/", ""), "/*page*/", "")
	err = s.pool.QueryRow(ctx, "SELECT count(*) FROM ("+sql+") counted", project, f.PlanID, f.TaskID, strings.TrimSpace(f.Query), f.Kind).Scan(&n)
	return
}
