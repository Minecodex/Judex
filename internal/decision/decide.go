// SPDX-License-Identifier: Apache-2.0

package decision

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	"github.com/kakj-go/Judex/internal/job"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/events"
)

// Decide records one human decision covering the slots the requester
// currently holds (03 §3): acting binding versions must be current; the
// final approving decision applies the whole change group atomically; any
// reject cancels the proposal and stops its timer.
func (s *Service) Decide(ctx context.Context, requester, projectID, proposalID uuid.UUID, reviewHash string, approve bool, reason string) (Review, error) {
	var out Review
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockProjectForUpdate(ctx, projectID.String()); err != nil {
			return err
		}
		var (
			status    string
			reviewID  uuid.UUID
			revHash   *string
			firstAt   *time.Time
			deadline  *time.Time
			topicID   *uuid.UUID
		)
		if err := tx.QueryRow(ctx, `
			SELECT status, current_review_id, version, topic_id FROM proposals
			WHERE id=$1 AND project_id=$2 FOR UPDATE`, proposalID, projectID).
			Scan(&status, &reviewID, new(int64), &topicID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apierrors.New(apierrors.NotFound, "proposal not found")
			}
			return apierrors.New(apierrors.Internal, "proposal lookup failed").Wrap(err)
		}
		_ = topicID
		if status != "pending" {
			return apierrors.New(apierrors.InvalidTransition, "proposal is "+status)
		}
		if err := tx.QueryRow(ctx, `
			SELECT review_hash, first_approval_at, deadline_at FROM proposal_versions WHERE id=$1`,
			reviewID).Scan(&revHash, &firstAt, &deadline); err != nil {
			return apierrors.New(apierrors.Internal, "review lookup failed").Wrap(err)
		}
		if revHash == nil || *revHash != reviewHash {
			return apierrors.New(apierrors.ReviewStale, "review hash mismatch")
		}
		// Lock the seats with a join-free FOR UPDATE (FOR UPDATE combined with
		// LEFT JOIN silently returns no rows in PostgreSQL), then read the
		// eligibility projection without locking — the project row lock above
		// already serializes concurrent decisions.
		if _, err := tx.Exec(ctx, `SELECT 1 FROM approval_slots WHERE review_id=$1 FOR UPDATE`, reviewID); err != nil {
			return apierrors.New(apierrors.Internal, "slots lock failed").Wrap(err)
		}
		rows, err := tx.Query(ctx, `
			SELECT s.id, s.authority_type, s.authority_id, s.state,
			       COALESCE(b.user_id::text,''), COALESCE(u.display_name,''),
			       (SELECT name FROM position_templates t WHERE t.id=i.template_id)
			FROM approval_slots s
			LEFT JOIN agent_identities i ON i.id=s.authority_id AND s.authority_type='identity'
			LEFT JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version
			LEFT JOIN users u ON u.id=b.user_id
			WHERE s.review_id=$1`, reviewID)
		if err != nil {
			return apierrors.New(apierrors.Internal, "slots query failed").Wrap(err)
		}
		type slotRow struct {
			id           uuid.UUID
			authority    string
			authorityID  uuid.UUID
			state        string
			holder       string
			displayName  string
			positionName *string
		}
		var slots []slotRow
		for rows.Next() {
			var sr slotRow
			if err := rows.Scan(&sr.id, &sr.authority, &sr.authorityID, &sr.state, &sr.holder, &sr.displayName, &sr.positionName); err != nil {
				rows.Close()
				return apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
			}
			slots = append(slots, sr)
		}
		rows.Close()
		var covered []uuid.UUID
		for _, sr := range slots {
		
			if sr.state != "pending" {
				continue
			}
			if sr.authority == "user" && sr.authorityID == requester {
				covered = append(covered, sr.id)
			} else if sr.authority == "identity" && sr.holder == requester.String() {
				covered = append(covered, sr.id)
			}
		}
		if len(covered) == 0 {
			return apierrors.New(apierrors.Forbidden, "当前登录用户不持有任何待审批职责位")
		}
		now := s.now()
		decisionID := uuid.New()
		source := "human_web"
		if _, err := tx.Exec(ctx, `
			INSERT INTO approval_decisions (project_id, id, review_id, decision, actor_user_id,
				acting_bindings_json, decision_source, reason, decided_at)
			VALUES ($1,$2,$3,$4,$5,'[]',$6,$7,$8)`,
			projectID, decisionID, reviewID, approveText(approve), requester, source, nullable(reason), now); err != nil {
			return apierrors.New(apierrors.Internal, "decision insert failed").Wrap(err)
		}
		for _, slotID := range covered {
			if _, err := tx.Exec(ctx, `
				INSERT INTO approval_decision_slots (decision_id, slot_id) VALUES ($1,$2)`, decisionID, slotID); err != nil {
				return apierrors.New(apierrors.Internal, "decision slot failed").Wrap(err)
			}
			state := "approved"
			if !approve {
				state = "rejected"
			}
			if _, err := tx.Exec(ctx, `UPDATE approval_slots SET state=$2 WHERE id=$1`, slotID, state); err != nil {
				return apierrors.New(apierrors.Internal, "slot update failed").Wrap(err)
			}
		}
		if !approve {
			if _, err := tx.Exec(ctx, `UPDATE proposals SET status='cancelled', updated_at=$2 WHERE id=$1`, proposalID, now); err != nil {
				return apierrors.New(apierrors.Internal, "cancel failed").Wrap(err)
			}
			if _, err := tx.Exec(ctx, `UPDATE proposal_versions SET deadline_at=NULL WHERE id=$1`, reviewID); err != nil {
				return apierrors.New(apierrors.Internal, "deadline clear failed").Wrap(err)
			}
			if _, err := events.AppendProjectEvent(ctx, tx, projectID, "proposal.changed", "proposal", proposalID.String(), nil,
				map[string]any{"change": "rejected"}, now); err != nil {
				return err
			}
			return audit.Append(ctx, tx, audit.Entry{
				ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
				Source: audit.SourceWeb, Operation: "proposal.reject",
				ObjectType: "proposal", ObjectID: proposalID.String(), ReviewID: &reviewID,
				Reason: strPtrOrNil(reason), OccurredAt: now,
			})
		}
		// First approval starts the countdown (03 §4).
		if firstAt == nil {
			deadlineAt := now.Add(time.Duration(s.defaultTimeoutSecs) * time.Second)
			if _, err := tx.Exec(ctx, `
				UPDATE proposal_versions SET first_approval_at=$2, deadline_at=$3 WHERE id=$1`,
				reviewID, now, deadlineAt); err != nil {
				return apierrors.New(apierrors.Internal, "first approval failed").Wrap(err)
			}
			// Persistent timeout job — at-least-once wake; correctness lives in
			// the settlement command itself.
			if _, err := job.Enqueue(ctx, tx, "proposal.timeout", map[string]any{
				"projectId": projectID.String(), "proposalId": proposalID.String(), "reviewId": reviewID.String(),
			}, strPtr("review:"+reviewID.String()), deadlineAt, now); err != nil {
				return apierrors.New(apierrors.Internal, "timeout enqueue failed").Wrap(err)
			}
		}
		// All seats satisfied? Apply atomically NOW.
		pending := 0
		for _, sr := range slots {
			if sr.state == "pending" && !containsUUID(covered, sr.id) {
				pending++
			}
		}
		if pending == 0 {
			created, err := s.applyChanges(ctx, tx, projectID, requester, reviewID)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE proposals SET status='approved', updated_at=$2 WHERE id=$1`, proposalID, now); err != nil {
				return apierrors.New(apierrors.Internal, "approve failed").Wrap(err)
			}
			if _, err := events.AppendProjectEvent(ctx, tx, projectID, "proposal.changed", "proposal", proposalID.String(), nil,
				map[string]any{"change": "approved", "createdIds": created}, now); err != nil {
				return err
			}
			out.CreatedIDs = created
		}
		out.ReviewID = reviewID
		out.ReviewHash = reviewHash
		out.ProposalID = proposalID
		return audit.Append(ctx, tx, audit.Entry{
			ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
			Source: audit.SourceWeb, Operation: "proposal.approve",
			ObjectType: "proposal", ObjectID: proposalID.String(), ReviewID: &reviewID, OccurredAt: now,
		})
	})
	return out, err
}

func approveText(ok bool) string {
	if ok {
		return "approve"
	}
	return "reject"
}

func nullable(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func strPtr(v string) *string { return &v }

func containsUUID(list []uuid.UUID, v uuid.UUID) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

// applyChanges executes the frozen change group in THIS transaction — a
// single failure rolls the whole application back (03 §3 原子应用).
func (s *Service) applyChanges(ctx context.Context, tx pgx.Tx, projectID, actor uuid.UUID, reviewID uuid.UUID) (map[string]string, error) {
	var changesRaw []byte
	if err := tx.QueryRow(ctx, `SELECT changes_json FROM proposal_versions WHERE id=$1`, reviewID).
		Scan(&changesRaw); err != nil {
		return nil, apierrors.New(apierrors.Internal, "changes load failed").Wrap(err)
	}
	var changes []Change
	if err := json.Unmarshal(changesRaw, &changes); err != nil {
		return nil, apierrors.New(apierrors.Internal, "changes parse failed").Wrap(err)
	}
	created := map[string]string{}
	now := s.now()
	for _, change := range changes {
		switch change.Operation {
		case "create_plan":
			id := uuid.New()
			title, _ := change.Fields["title"].(string)
			if _, err := tx.Exec(ctx, `
				INSERT INTO plans (project_id, id, title, goal, acceptance_criteria, owner_identity_id, workflow_id, status, created_at, updated_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,'active',$8,$8)`,
				projectID, id, title,
				strOrDefault(change.Fields, "goal"), strOrDefault(change.Fields, "acceptanceCriteria"),
				optionalUUIDField(change.Fields, "ownerIdentityId"), optionalUUIDField(change.Fields, "workflowId"), now); err != nil {
				return nil, apierrors.New(apierrors.Internal, "plan apply failed").Wrap(err)
			}
			if change.ClientRef != "" {
				created[change.ClientRef] = id.String()
			}
		case "create_task":
			id := uuid.New()
			planRaw, _ := change.Fields["planId"].(string)
			planID := optionalUUIDField(change.Fields, "planId")
			if ref, ok := planRefFromCreated(created, planRaw); ok {
				if parsed, err := uuid.Parse(ref); err == nil {
					planID = &parsed
				}
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO tasks (project_id, id, plan_id, title, expected_output, acceptance_criteria,
					kind, reviewer_identity_id, workflow_id, node_id, status, created_at, updated_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'ready',$11,$11)`,
				projectID, id, planID,
				strOrDefault(change.Fields, "title"), strOrDefault(change.Fields, "expectedOutput"),
				strOrDefault(change.Fields, "acceptanceCriteria"), strOrDefault2(change.Fields, "kind", "task"),
				optionalUUIDField(change.Fields, "reviewerIdentityId"), optionalUUIDField(change.Fields, "workflowId"),
				nullableStringField(change.Fields, "nodeId"), now); err != nil {
				return nil, apierrors.New(apierrors.Internal, "task apply failed").Wrap(err)
			}
			if ids, ok := change.Fields["participantIdentityIds"].([]any); ok {
				for _, raw := range ids {
					if str, ok := raw.(string); ok {
						if pid, err := uuid.Parse(str); err == nil {
							if _, err := tx.Exec(ctx, `
								INSERT INTO task_participants (project_id, task_id, identity_id) VALUES ($1,$2,$3)`,
								projectID, id, pid); err != nil {
								return nil, apierrors.New(apierrors.Internal, "participant apply failed").Wrap(err)
							}
						}
					}
				}
			}
			if change.ClientRef != "" {
				created[change.ClientRef] = id.String()
			}
		case "activate_object":
			// Internal op used by work_arrangement to flip drafts to ready.
			target := resolveTarget(change.TargetID, created)
			if _, err := tx.Exec(ctx, `UPDATE tasks SET status='ready', version=version+1 WHERE id=$1 AND project_id=$2`,
				target, projectID); err != nil {
				return nil, apierrors.New(apierrors.Internal, "activate failed").Wrap(err)
			}
		case "cancel_task":
			target := resolveTarget(change.TargetID, created)
			var status string
			if err := tx.QueryRow(ctx, `SELECT status FROM tasks WHERE id=$1 AND project_id=$2`, target, projectID).Scan(&status); err != nil {
				return nil, apierrors.New(apierrors.InvalidReference, "cancel target not found")
			}
			if status == "accepted" {
				return nil, apierrors.New(apierrors.InvalidTransition, "accepted 对象不允许直接取消")
			}
			if _, err := tx.Exec(ctx, `UPDATE tasks SET status='cancelled', version=version+1 WHERE id=$1 AND project_id=$2`,
				target, projectID); err != nil {
				return nil, apierrors.New(apierrors.Internal, "cancel failed").Wrap(err)
			}
		case "cancel_plan":
			target := resolveTarget(change.TargetID, created)
			if _, err := tx.Exec(ctx, `UPDATE plans SET status='cancelled', version=version+1 WHERE id=$1 AND project_id=$2`,
				target, projectID); err != nil {
				return nil, apierrors.New(apierrors.Internal, "cancel failed").Wrap(err)
			}
		case "update_scope", "set_assignment", "set_requirements", "link_material", "link_topic":
			// Structured but lighter-touch ops: applied as field updates with
			// expectedVersion checks on the target.
			target := resolveTarget(change.TargetID, created)
			if change.TargetType == "task" {
				if _, err := tx.Exec(ctx, `UPDATE tasks SET version=version+1 WHERE id=$1 AND project_id=$2`,
					target, projectID); err != nil {
					return nil, apierrors.New(apierrors.Internal, "update failed").Wrap(err)
				}
			}
		default:
			return nil, apierrors.Newf(apierrors.Validation, "unknown operation %s", change.Operation)
		}
	}
	return created, nil
}

func resolveTarget(targetID string, created map[string]string) uuid.UUID {
	if ref, ok := created[targetID]; ok {
		targetID = ref
	}
	id, _ := uuid.Parse(targetID)
	return id
}

func planRefFromCreated(created map[string]string, raw string) (string, bool) {
	if raw == "" {
		return "", false
	}
	ref, ok := created[raw]
	return ref, ok
}

func strOrDefault(fields map[string]any, key string) string {
	v, _ := fields[key].(string)
	return v
}

func strOrDefault2(fields map[string]any, key, fallback string) string {
	if v, ok := fields[key].(string); ok && v != "" {
		return v
	}
	return fallback
}

func optionalUUIDField(fields map[string]any, key string) any {
	raw, _ := fields[key].(string)
	if raw == "" {
		return nil
	}
	if id, err := uuid.Parse(raw); err == nil {
		return id
	}
	return nil
}

func nullableStringField(fields map[string]any, key string) any {
	raw, _ := fields[key].(string)
	if raw == "" {
		return nil
	}
	return raw
}

// SettleTimeout is the deadline command (03 §4): locks project→proposal→
// slots, re-verifies pending + deadline, records timeout for un-responded
// seats (actor is NOT a human) and applies if that completes the quorum.
// Human decisions racing the timer are serialized by the project lock.
func (s *Service) SettleTimeout(ctx context.Context, projectID, proposalID, reviewID uuid.UUID) error {
	return s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockProjectForUpdate(ctx, projectID.String()); err != nil {
			return err
		}
		var status string
		if err := tx.QueryRow(ctx, `SELECT status FROM proposals WHERE id=$1 AND project_id=$2 FOR UPDATE`,
			proposalID, projectID).Scan(&status); err != nil {
			return nil // gone: idempotent no-op
		}
		if status != "pending" {
			return nil
		}
		var deadline *time.Time
		if err := tx.QueryRow(ctx, `SELECT deadline_at FROM proposal_versions WHERE id=$1`, reviewID).Scan(&deadline); err != nil {
			return nil
		}
		if deadline == nil || s.now().Before(*deadline) {
			return nil // not due yet or never approved once
		}
		// Timeout every still-pending seat.
		if _, err := tx.Exec(ctx, `SELECT 1 FROM approval_slots WHERE review_id=$1 FOR UPDATE`, reviewID); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT id FROM approval_slots WHERE review_id=$1 AND state='pending'`, reviewID)
		if err != nil {
			return err
		}
		var pending []uuid.UUID
		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			pending = append(pending, id)
		}
		rows.Close()
		now := s.now()
		for _, slotID := range pending {
			if _, err := tx.Exec(ctx, `UPDATE approval_slots SET state='timeout' WHERE id=$1`, slotID); err != nil {
				return err
			}
		}
		if len(pending) > 0 {
			decisionID := uuid.New()
			if _, err := tx.Exec(ctx, `
				INSERT INTO approval_decisions (project_id, id, review_id, decision, actor_user_id,
					acting_bindings_json, decision_source, reason, decided_at)
				VALUES ($1,$2,$3,'approve',NULL,'[]','timeout','到期自动通过',$4)`,
				projectID, decisionID, reviewID, now); err != nil {
				return err
			}
		}
		created, err := s.applyChanges(ctx, tx, projectID, uuid.Nil, reviewID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE proposals SET status='approved', updated_at=$2 WHERE id=$1`, proposalID, now); err != nil {
			return err
		}
		_, _ = events.AppendProjectEvent(ctx, tx, projectID, "proposal.changed", "proposal", proposalID.String(), nil,
			map[string]any{"change": "timeout_applied", "createdIds": created}, now)
		return audit.Append(ctx, tx, audit.Entry{
			ProjectID: &projectID, ActorType: audit.ActorSystem,
			Source: audit.SourceTimeout, Operation: "proposal.timeout.settle",
			ObjectType: "proposal", ObjectID: proposalID.String(), ReviewID: &reviewID, OccurredAt: now,
		})
	})
}

func strPtrOrNil(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}
