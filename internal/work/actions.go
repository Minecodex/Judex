// SPDX-License-Identifier: Apache-2.0

package work

import (
	"context"
	"time"

	"github.com/google/uuid"

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
	actions := []PendingAction{}
	rows, err := s.pool.Query(ctx, `
		SELECT p.project_id, p.id, p.current_review_id, v.deadline_at, p.created_at, p.kind
		FROM proposals p
		JOIN proposal_versions v ON v.id=p.current_review_id
		JOIN approval_slots s ON s.review_id=p.current_review_id AND s.state='pending'
		LEFT JOIN agent_identities i ON i.id=s.authority_id AND s.authority_type='identity'
		LEFT JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version
		WHERE p.status='pending'
		  AND (b.user_id=$1 OR (s.authority_type='user' AND s.authority_id=$1))
		  AND ($2::uuid IS NULL OR p.project_id=$2)
		GROUP BY p.project_id, p.id, p.current_review_id, v.deadline_at, p.created_at, p.kind
		ORDER BY v.deadline_at NULLS LAST, p.created_at
		LIMIT 100`, user, projectFilter)
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "actions(proposals) failed").Wrap(err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			a       PendingAction
			review  uuid.NullUUID
			kind    string
		)
		if err := rows.Scan(&a.ProjectID, &a.ObjectID, &review, &a.DueAt, &a.CreatedAt, &kind); err != nil {
			return nil, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		if review.Valid {
			a.ReviewID = &review.UUID
		}
		a.Kind = "approve"
		a.ObjectType = "proposal"
		a.Summary = "待审批提案（" + kind + "）"
		a.ID = uuid.New()
		actions = append(actions, a)
	}

	// Handoff receipts waiting for me (receiver with pending version).
	rows2, err := s.pool.Query(ctx, `
		SELECT h.project_id, h.id, h.created_at
		FROM handoffs h
		JOIN agent_identities i ON i.id=h.receiver_identity_id
		JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version
		JOIN handoff_sources hs ON hs.handoff_id=h.id
		JOIN source_versions v ON v.id=hs.current_source_version_id AND v.state='pending'
		WHERE b.user_id=$1 AND ($2::uuid IS NULL OR h.project_id=$2)
		GROUP BY h.project_id, h.id, h.created_at
		ORDER BY h.created_at LIMIT 100`, user, projectFilter)
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "actions(handoffs) failed").Wrap(err)
	}
	defer rows2.Close()
	for rows2.Next() {
		var a PendingAction
		if err := rows2.Scan(&a.ProjectID, &a.ObjectID, &a.CreatedAt); err != nil {
			return nil, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		a.Kind = "receive"
		a.ObjectType = "handoff"
		a.Summary = "待接收交接来源"
		a.ID = uuid.New()
		actions = append(actions, a)
	}

	// Handoff sources I should (re)send as current sender.
	rows3, err := s.pool.Query(ctx, `
		SELECT DISTINCT h.project_id, h.id, h.created_at
		FROM handoffs h
		JOIN handoff_sources hs ON hs.handoff_id=h.id
		JOIN agent_identities i ON i.id=hs.sender_identity_id
		JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version
		LEFT JOIN source_versions v ON v.id=hs.current_source_version_id
		WHERE b.user_id=$1
		  AND (v.id IS NULL OR v.state IN ('draft','rejected'))
		  AND ($2::uuid IS NULL OR h.project_id=$2)
		ORDER BY h.created_at LIMIT 100`, user, projectFilter)
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "actions(send) failed").Wrap(err)
	}
	defer rows3.Close()
	for rows3.Next() {
		var a PendingAction
		if err := rows3.Scan(&a.ProjectID, &a.ObjectID, &a.CreatedAt); err != nil {
			return nil, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		a.Kind = "submit"
		a.ObjectType = "handoff"
		a.Summary = "待发送/补交交接来源"
		a.ID = uuid.New()
		actions = append(actions, a)
	}

	// Tasks delivered awaiting my acceptance (current reviewer).
	rows4, err := s.pool.Query(ctx, `
		SELECT t.project_id, t.id, t.created_at, t.title
		FROM tasks t
		JOIN agent_identities i ON i.id=t.reviewer_identity_id
		JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version
		WHERE t.status='delivered' AND b.user_id=$1
		  AND ($2::uuid IS NULL OR t.project_id=$2)
		ORDER BY t.updated_at LIMIT 100`, user, projectFilter)
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "actions(accept) failed").Wrap(err)
	}
	defer rows4.Close()
	for rows4.Next() {
		var a PendingAction
		var title string
		if err := rows4.Scan(&a.ProjectID, &a.ObjectID, &a.CreatedAt, &title); err != nil {
			return nil, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		a.Kind = "accept"
		a.ObjectType = "task"
		a.Summary = "待验收：" + title
		a.ID = uuid.New()
		actions = append(actions, a)
	}

	// Plans ready for my overall acceptance (owner, active, all tasks accepted).
	rows5, err := s.pool.Query(ctx, `
		SELECT p.project_id, p.id, p.created_at, p.title
		FROM plans p
		JOIN agent_identities i ON i.id=p.owner_identity_id
		JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version
		WHERE p.status='active' AND b.user_id=$1
		  AND NOT EXISTS (
		    SELECT 1 FROM tasks t WHERE t.plan_id=p.id AND t.project_id=p.project_id
		      AND t.status<>'accepted' AND t.status<>'cancelled')
		  AND EXISTS (
		    SELECT 1 FROM tasks t WHERE t.plan_id=p.id AND t.project_id=p.project_id)
		  AND ($2::uuid IS NULL OR p.project_id=$2)
		ORDER BY p.created_at LIMIT 100`, user, projectFilter)
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "actions(plans) failed").Wrap(err)
	}
	defer rows5.Close()
	for rows5.Next() {
		var a PendingAction
		var title string
		if err := rows5.Scan(&a.ProjectID, &a.ObjectID, &a.CreatedAt, &title); err != nil {
			return nil, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		a.Kind = "accept"
		a.ObjectType = "plan"
		a.Summary = "任务已全部验收，待计划整体验收：" + title
		a.ID = uuid.New()
		actions = append(actions, a)
	}
	return actions, nil
}

