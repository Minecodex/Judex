package work

import (
	"context"
	"github.com/google/uuid"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/paging"
	"time"
)

type ActionFilter struct{ Kind, Category string }

func (f ActionFilter) Validate() error {
	switch f.Kind {
	case "", "submit", "approve", "receive", "accept", "revise", "reopen":
	default:
		return apierrors.Fields("kind", "enum")
	}
	if f.Category != "" && f.Category != "decision" && f.Category != "execution" {
		return apierrors.Fields("category", "enum")
	}
	return nil
}

type PlanFilter struct {
	Query, Status string
	Mine          bool
}

func (s *Service) ListPlanCards(ctx context.Context, user, project uuid.UUID, f PlanFilter) ([]Plan, error) {
	if f.Status != "" && f.Status != "draft" && f.Status != "active" && f.Status != "accepted" && f.Status != "cancelled" {
		return nil, apierrors.Fields("status", "enum")
	}
	return s.listPlans(ctx, user, project, nil, f)
}

type DeliveryFilter struct{ Type, Filter string }
type Delivery struct {
	ID           uuid.UUID  `json:"id"`
	ObjectID     uuid.UUID  `json:"objectId"`
	ObjectType   string     `json:"objectType"`
	TaskID       uuid.UUID  `json:"taskId"`
	PlanID       *uuid.UUID `json:"planId"`
	Title        string     `json:"title"`
	Status       string     `json:"status"`
	Version      int64      `json:"version"`
	Summary      string     `json:"summary"`
	Incoming     bool       `json:"incoming"`
	Outgoing     bool       `json:"outgoing"`
	SourceCount  int        `json:"sourceCount"`
	PendingCount int        `json:"pendingCount"`
	CreatedAt    time.Time  `json:"createdAt"`
}

// Delivery cards are query projections. A receipt and a task acceptance remain
// separate domain commands referencing their own frozen evidence.
func (s *Service) ListDeliveries(ctx context.Context, user, project uuid.UUID, f DeliveryFilter) ([]Delivery, error) {
	if _, e := memberTx(ctx, s.pool, project, user); e != nil {
		return nil, e
	}
	if f.Type != "" && f.Type != "all" && f.Type != "task" && f.Type != "handoff" {
		return nil, apierrors.Fields("type", "enum")
	}
	if f.Filter != "" && f.Filter != "all" && f.Filter != "receive" && f.Filter != "waiting" && f.Filter != "revise" {
		return nil, apierrors.Fields("filter", "enum")
	}
	rows, e := paging.Query(ctx, s.pool, `WITH held AS (
 SELECT i.id FROM agent_identities i JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version AND b.valid_until IS NULL WHERE i.project_id=$1 AND i.status='active' AND b.user_id=$2
 ), deliveries AS (
 SELECT t.id object_id,'task'::text object_type,t.id task_id,t.plan_id,t.title,CASE WHEN x.id IS NOT NULL THEN 'skipped' ELSE t.status END status,t.version,COALESCE(r.progress_hint,'') summary,
 x.id IS NULL AND t.reviewer_identity_id IN(SELECT id FROM held) incoming,
 EXISTS(SELECT 1 FROM task_participants tp WHERE tp.project_id=$1 AND tp.task_id=t.id AND tp.identity_id IN(SELECT id FROM held)) outgoing,
 1::bigint source_count,CASE WHEN x.id IS NULL AND t.status='delivered' THEN 1::bigint ELSE 0::bigint END pending_count,t.created_at,
 EXISTS(SELECT 1 FROM task_participants tp WHERE tp.project_id=$1 AND tp.task_id=t.id AND tp.identity_id IN(SELECT id FROM held)) AND x.id IS NULL AND t.status='delivered' waiting,
 EXISTS(SELECT 1 FROM task_participants tp WHERE tp.project_id=$1 AND tp.task_id=t.id AND tp.identity_id IN(SELECT id FROM held)) AND x.id IS NULL AND t.status='rework' revision_required
 FROM tasks t LEFT JOIN task_execution_exceptions x ON x.project_id=t.project_id AND x.task_id=t.id AND x.restored_at IS NULL LEFT JOIN work_reports r ON r.project_id=t.project_id AND r.id=t.latest_report_id
 WHERE t.project_id=$1 AND t.discarded_at IS NULL AND t.status IN('delivered','rework','accepted') AND t.latest_report_id IS NOT NULL
 UNION ALL
 SELECT h.id,'handoff',h.target_task_id,t.plan_id,h.title,
 CASE WHEN count(v.id) FILTER(WHERE v.state='pending')>0 THEN 'pending' WHEN count(v.id) FILTER(WHERE v.state IN('rejected','stale'))>0 THEN 'needs_revision' WHEN count(hs.id)=0 OR count(v.id)<count(hs.id) OR count(v.id) FILTER(WHERE v.state='draft')>0 THEN 'draft' ELSE 'accepted' END,
 h.version,COALESCE(string_agg(NULLIF(v.summary,''),' · ' ORDER BY hs.id),''),h.receiver_identity_id IN(SELECT id FROM held),
 COALESCE(bool_or(hs.sender_identity_id IN(SELECT id FROM held)),false),count(hs.id),count(v.id) FILTER(WHERE v.state='pending'),h.created_at,
 COALESCE(bool_or(hs.sender_identity_id IN(SELECT id FROM held) AND v.state='pending'),false),
 COALESCE(bool_or(hs.sender_identity_id IN(SELECT id FROM held) AND (v.state IN('draft','rejected','stale') OR v.id IS NULL)),false)
 FROM handoffs h JOIN tasks t ON t.project_id=h.project_id AND t.id=h.target_task_id
 LEFT JOIN handoff_sources hs ON hs.project_id=h.project_id AND hs.handoff_id=h.id
 LEFT JOIN source_versions v ON v.project_id=hs.project_id AND v.id=hs.current_source_version_id
 WHERE h.project_id=$1 GROUP BY h.id,t.plan_id
 ), cards AS (SELECT (md5(object_type||':'||object_id::text))::uuid id,* FROM deliveries)
 SELECT c.id,c.object_id,c.object_type,c.task_id,c.plan_id,c.title,c.status,c.version,c.summary,COALESCE(c.incoming,false),c.outgoing,c.source_count,c.pending_count,c.created_at /*keys*/
 FROM cards c WHERE ($3='' OR $3='all' OR c.object_type=$3)
 AND ($4='' OR $4='all' OR ($4='receive' AND c.object_type='handoff' AND c.incoming AND c.pending_count>0)
 OR ($4='waiting' AND c.waiting)
 OR ($4='revise' AND c.revision_required))
 /*page*/`, "c.created_at", "c.id", project, user, f.Type, f.Filter)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Delivery{}
	for rows.Next() {
		var d Delivery
		if e = rows.Scan(&d.ID, &d.ObjectID, &d.ObjectType, &d.TaskID, &d.PlanID, &d.Title, &d.Status, &d.Version, &d.Summary, &d.Incoming, &d.Outgoing, &d.SourceCount, &d.PendingCount, &d.CreatedAt); e != nil {
			return nil, e
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
