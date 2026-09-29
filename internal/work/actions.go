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
func (s *Service) MyActions(ctx context.Context, user uuid.UUID, projectFilter *uuid.UUID) ([]PendingAction, error) {
	scope := []uuid.UUID{}
	if actor := auth.FromContext(ctx); actor != nil && actor.Kind == auth.KindCLI {
		scope = actor.ProjectScope
		if len(scope) == 0 {
			return []PendingAction{}, nil
		}
	}
	rows, err := paging.Query(ctx, s.pool, `
 WITH held AS (
 SELECT i.id,i.project_id FROM agent_identities i JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version AND b.valid_until IS NULL WHERE b.user_id=$1 AND i.status='active'
 ), actions AS (
 SELECT DISTINCT p.project_id,p.id object_id,'proposal'::text object_type,'approve'::text kind,p.current_review_id review_id,v.deadline_at due_at,'待审批提案（'||p.kind||'）' summary,p.created_at FROM proposals p JOIN proposal_versions v ON v.id=p.current_review_id JOIN approval_slots s ON s.review_id=p.current_review_id WHERE p.status='pending' AND s.state='pending' AND ((s.authority_type='user' AND s.authority_id=$1) OR (s.authority_type='identity' AND s.authority_id IN(SELECT id FROM held)))
 UNION ALL
 SELECT DISTINCT h.project_id,h.id,'handoff','receive',NULL::uuid,NULL::timestamptz,'待接收交接来源',h.created_at FROM handoffs h JOIN handoff_sources hs ON hs.handoff_id=h.id JOIN source_versions v ON v.id=hs.current_source_version_id WHERE v.state='pending' AND h.receiver_identity_id IN(SELECT id FROM held)
 UNION ALL
 SELECT DISTINCT h.project_id,h.id,'handoff','submit',NULL::uuid,NULL::timestamptz,'待发送/补交交接来源',h.created_at FROM handoffs h JOIN handoff_sources hs ON hs.handoff_id=h.id LEFT JOIN source_versions v ON v.id=hs.current_source_version_id WHERE (v.id IS NULL OR v.state IN ('draft','rejected')) AND hs.sender_identity_id IN(SELECT id FROM held)
 UNION ALL
 SELECT t.project_id,t.id,'task','accept',NULL::uuid,NULL::timestamptz,'待验收：'||t.title,t.created_at FROM tasks t WHERE t.status='delivered' AND t.reviewer_identity_id IN(SELECT id FROM held)
 UNION ALL
 SELECT p.project_id,p.id,'plan','accept',NULL::uuid,NULL::timestamptz,'任务已全部验收，待计划整体验收：'||p.title,p.created_at FROM plans p WHERE p.status='active' AND p.owner_identity_id IN(SELECT id FROM held) AND EXISTS(SELECT 1 FROM tasks t WHERE t.plan_id=p.id OR EXISTS(SELECT 1 FROM plan_task_references r WHERE r.plan_id=p.id AND r.task_id=t.id)) AND NOT EXISTS(SELECT 1 FROM tasks t WHERE (t.plan_id=p.id OR EXISTS(SELECT 1 FROM plan_task_references r WHERE r.plan_id=p.id AND r.task_id=t.id)) AND t.status NOT IN ('accepted','cancelled'))
 ), scoped AS (
 SELECT (md5(a.object_type||':'||a.object_id::text||':'||a.kind))::uuid id,a.* FROM actions a JOIN project_members m ON m.project_id=a.project_id AND m.user_id=$1 AND m.state='active' JOIN projects p ON p.id=a.project_id AND p.status='active' WHERE ($2::uuid IS NULL OR a.project_id=$2) AND (cardinality($3::uuid[])=0 OR a.project_id=ANY($3))
 ) SELECT a.id,a.project_id,a.object_id,a.object_type,a.kind,a.review_id,a.due_at,a.summary,a.created_at /*keys*/ FROM scoped a WHERE true /*page*/`, "a.created_at", "a.id", user, projectFilter, scope)
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
