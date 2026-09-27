// SPDX-License-Identifier: Apache-2.0

// Package handoff implements source-level handoffs per docs/plans/v1/03 §6:
// a handoff bundles multiple sources; each sender confirms their own source
// version; the receiver accepts/rejects each source separately; rejection
// of an unaccepted source marks the related task rework, while a source
// whose task is already accepted can only raise a reopen request; new
// revisions keep old decisions and require the sender to re-confirm.
package handoff

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/events"
)

// Handoff is the aggregate projection (06 §5).
type Handoff struct {
	ID                 uuid.UUID `json:"id"`
	Title              string    `json:"title"`
	TargetTaskID       uuid.UUID `json:"targetTaskId"`
	ReceiverIdentityID uuid.UUID `json:"receiverIdentityId"`
	Kind               string    `json:"kind"`
	State              string    `json:"state"`
	Version            int64     `json:"version"`
	Sources            []Source  `json:"sources"`
}

// Source is one source seat inside a handoff.
type Source struct {
	ID               uuid.UUID  `json:"id"`
	SourceTaskID     uuid.UUID  `json:"sourceTaskId"`
	SenderIdentityID uuid.UUID  `json:"senderIdentityId"`
	CurrentVersionID *uuid.UUID `json:"currentVersionId"`
	CurrentVersion   *int64     `json:"currentVersion"`
	State            string     `json:"state"`
	Reason           *string    `json:"reason"`
}

type Service struct {
	pool *postgres.Pool
	now  func() time.Time
}

func NewService(pool *postgres.Pool, now func() time.Time) *Service {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{pool: pool, now: now}
}

func memberTx(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, projectID, requester uuid.UUID) error {
	var role string
	err := q.QueryRow(ctx, `
		SELECT role FROM project_members WHERE project_id=$1 AND user_id=$2 AND state='active'`,
		projectID, requester).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return apierrors.New(apierrors.NotFound, "project not found")
	}
	return err
}

// Create drafts a handoff with its sources (03 §6 可跨任务交接).
func (s *Service) Create(ctx context.Context, requester, projectID uuid.UUID, title string, targetTaskID, receiverIdentityID uuid.UUID, kind string, sourceSpecs []struct {
	SourceTaskID     uuid.UUID
	SenderIdentityID uuid.UUID
}) (Handoff, error) {
	if kind != "dependency" && kind != "stage" {
		return Handoff{}, apierrors.Fields("kind", "enum")
	}
	if len(sourceSpecs) == 0 {
		return Handoff{}, apierrors.Fields("sources", "required")
	}
	var out Handoff
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockProjectForUpdate(ctx, projectID.String()); err != nil {
			return err
		}
		if err := memberTx(ctx, tx, projectID, requester); err != nil {
			return err
		}
		for _, spec := range sourceSpecs {
			var srcProject uuid.UUID
			if err := tx.QueryRow(ctx, `SELECT project_id FROM tasks WHERE id=$1`, spec.SourceTaskID).
				Scan(&srcProject); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return apierrors.New(apierrors.InvalidReference, "source task not found")
				}
				return apierrors.New(apierrors.Internal, "source lookup failed").Wrap(err)
			}
			if srcProject != projectID {
				return apierrors.New(apierrors.InvalidReference, "source task in another project")
			}
		}
		now := s.now()
		id := uuid.New()
		if _, err := tx.Exec(ctx, `
			INSERT INTO handoffs (project_id, id, target_task_id, receiver_identity_id, kind, title, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$7)`,
			projectID, id, targetTaskID, receiverIdentityID, kind, title, now); err != nil {
			return apierrors.New(apierrors.Internal, "handoff insert failed").Wrap(err)
		}
		out = Handoff{ID: id, Title: title, TargetTaskID: targetTaskID,
			ReceiverIdentityID: receiverIdentityID, Kind: kind, State: "waiting", Version: 1}
		for _, spec := range sourceSpecs {
			sourceID := uuid.New()
			if _, err := tx.Exec(ctx, `
				INSERT INTO handoff_sources (project_id, id, handoff_id, source_task_id, sender_identity_id)
				VALUES ($1,$2,$3,$4,$5)`,
				projectID, sourceID, id, spec.SourceTaskID, spec.SenderIdentityID); err != nil {
				return apierrors.New(apierrors.Internal, "source insert failed").Wrap(err)
			}
			out.Sources = append(out.Sources, Source{
				ID: sourceID, SourceTaskID: spec.SourceTaskID,
				SenderIdentityID: spec.SenderIdentityID, State: "draft",
			})
		}
		return audit.Append(ctx, tx, audit.Entry{
			ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
			Source: audit.SourceWeb, Operation: "handoff.create",
			ObjectType: "handoff", ObjectID: id.String(), OccurredAt: now,
		})
	})
	return out, err
}

// SendSource freezes a source version (sender identity holder only) and
// moves it to pending (03 §6 来源发送).
func (s *Service) SendSource(ctx context.Context, requester, projectID, handoffID, sourceID uuid.UUID, summary string) (Source, error) {
	if strings.TrimSpace(summary) == "" {
		return Source{}, apierrors.Fields("summary", "required")
	}
	var out Source
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockProjectForUpdate(ctx, projectID.String()); err != nil {
			return err
		}
		if err := memberTx(ctx, tx, projectID, requester); err != nil {
			return err
		}
		var (
			senderIdentity uuid.UUID
			sourceTask     uuid.UUID
			currentVersion uuid.NullUUID
			currentState   string
		)
		if err := tx.QueryRow(ctx, `
			SELECT hs.sender_identity_id, hs.source_task_id, hs.current_source_version_id,
			       COALESCE((SELECT v.state FROM source_versions v WHERE v.id=hs.current_source_version_id),'draft')
			FROM handoff_sources hs WHERE hs.id=$1 AND hs.handoff_id=$2 AND hs.project_id=$3 FOR UPDATE`,
			sourceID, handoffID, projectID).
			Scan(&senderIdentity, &sourceTask, &currentVersion, &currentState); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apierrors.New(apierrors.NotFound, "source not found")
			}
			return apierrors.New(apierrors.Internal, "source lookup failed").Wrap(err)
		}
		// Only the CURRENT binding holder of the sender identity sends.
		var holder *uuid.UUID
		if err := tx.QueryRow(ctx, `
			SELECT b.user_id FROM agent_identities i
			LEFT JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version
			WHERE i.id=$1`, senderIdentity).Scan(&holder); err != nil {
			return apierrors.New(apierrors.Internal, "binding lookup failed").Wrap(err)
		}
		if holder == nil || *holder != requester {
			return apierrors.New(apierrors.Forbidden, "只有该来源当前发送人可以发送")
		}
		if currentState == "pending" || currentState == "accepted" {
			return apierrors.New(apierrors.InvalidTransition, "source version is "+currentState)
		}
		var nextRev int64
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(max(revision),0)+1 FROM source_versions WHERE source_id=$1`, sourceID).
			Scan(&nextRev); err != nil {
			return apierrors.New(apierrors.Internal, "revision lookup failed").Wrap(err)
		}
		now := s.now()
		versionID := uuid.New()
		// Pull the latest report of the source task as evidence when present.
		var reportID uuid.NullUUID
		_ = tx.QueryRow(ctx, `SELECT id FROM work_reports WHERE task_id=$1 ORDER BY created_at DESC LIMIT 1`, sourceTask).Scan(&reportID)
		if _, err := tx.Exec(ctx, `
			INSERT INTO source_versions (project_id, id, source_id, revision, report_id, summary, state, sent_by, sent_at, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,'pending',$7,$8,$9)`,
			projectID, versionID, sourceID, nextRev, reportID, summary, requester, now, now); err != nil {
			return apierrors.New(apierrors.Internal, "version insert failed").Wrap(err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE handoff_sources SET current_source_version_id=$2 WHERE id=$1`, sourceID, versionID); err != nil {
			return apierrors.New(apierrors.Internal, "source update failed").Wrap(err)
		}
		if _, err := events.AppendProjectEvent(ctx, tx, projectID, "handoff.changed", "handoff", handoffID.String(), nil,
			map[string]any{"change": "source_sent", "sourceId": sourceID, "versionId": versionID}, now); err != nil {
			return err
		}
		if err := audit.Append(ctx, tx, audit.Entry{
			ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
			IdentityID: &senderIdentity, Source: audit.SourceWeb,
			Operation: "handoff.source.send", ObjectType: "handoff", ObjectID: handoffID.String(),
			OccurredAt: now,
		}); err != nil {
			return err
		}
		rev := nextRev
		out = Source{ID: sourceID, SourceTaskID: sourceTask, SenderIdentityID: senderIdentity,
			CurrentVersionID: &versionID, CurrentVersion: &rev, State: "pending"}
		return nil
	})
	return out, err
}

// DecideSource is the receiver's per-source decision (03 §6): accept records
// the receipt without implicitly accepting the source task; reject requires
// a reason and, for a not-yet-accepted source task, marks it rework — an
// already accepted source task only yields a reopen request instead.
func (s *Service) DecideSource(ctx context.Context, requester, projectID, handoffID, sourceID uuid.UUID, accept bool, reason string) (Source, error) {
	if !accept && strings.TrimSpace(reason) == "" {
		return Source{}, apierrors.Fields("reason", "required")
	}
	var out Source
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockProjectForUpdate(ctx, projectID.String()); err != nil {
			return err
		}
		if err := memberTx(ctx, tx, projectID, requester); err != nil {
			return err
		}
		var receiverIdentity uuid.UUID
		if err := tx.QueryRow(ctx, `
			SELECT receiver_identity_id FROM handoffs WHERE id=$1 AND project_id=$2 FOR UPDATE`,
			handoffID, projectID).Scan(&receiverIdentity); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apierrors.New(apierrors.NotFound, "handoff not found")
			}
			return apierrors.New(apierrors.Internal, "handoff lookup failed").Wrap(err)
		}
		var holder *uuid.UUID
		if err := tx.QueryRow(ctx, `
			SELECT b.user_id FROM agent_identities i
			LEFT JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version
			WHERE i.id=$1`, receiverIdentity).Scan(&holder); err != nil {
			return apierrors.New(apierrors.Internal, "binding lookup failed").Wrap(err)
		}
		if holder == nil || *holder != requester {
			return apierrors.New(apierrors.Forbidden, "只有当前接收人可以决定")
		}
		var (
			sourceTask     uuid.UUID
			senderIdentity uuid.UUID
			versionID      uuid.NullUUID
			versionState   string
		)
		if err := tx.QueryRow(ctx, `
			SELECT hs.source_task_id, hs.sender_identity_id, hs.current_source_version_id,
			       COALESCE((SELECT v.state FROM source_versions v WHERE v.id=hs.current_source_version_id),'none')
			FROM handoff_sources hs WHERE hs.id=$1 AND hs.handoff_id=$2 FOR UPDATE`,
			sourceID, handoffID).Scan(&sourceTask, &senderIdentity, &versionID, &versionState); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apierrors.New(apierrors.NotFound, "source not found")
			}
			return apierrors.New(apierrors.Internal, "source lookup failed").Wrap(err)
		}
		if !versionID.Valid || versionState != "pending" {
			return apierrors.New(apierrors.InvalidTransition, "source version is "+versionState)
		}
		now := s.now()
		decision := "accept"
		nextState := "accepted"
		if !accept {
			decision = "reject"
			nextState = "rejected"
			// Unaccepted source task -> rework; accepted task -> reopen request only.
			var taskStatus string
			if err := tx.QueryRow(ctx, `SELECT status FROM tasks WHERE id=$1`, sourceTask).Scan(&taskStatus); err != nil {
				return apierrors.New(apierrors.Internal, "task lookup failed").Wrap(err)
			}
			if taskStatus != "accepted" {
				if _, err := tx.Exec(ctx, `
					UPDATE tasks SET status='rework', version=version+1, updated_at=$2 WHERE id=$1`,
					sourceTask, now); err != nil {
					return apierrors.New(apierrors.Internal, "rework failed").Wrap(err)
				}
			}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE source_versions SET state=$2 WHERE id=$1`, versionID.UUID, nextState); err != nil {
			return apierrors.New(apierrors.Internal, "version update failed").Wrap(err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO source_decisions (id, source_version_id, actor_user_id, decision, reason, created_at)
			VALUES ($1,$2,$3,$4,$5,$6)`,
			uuid.New(), versionID.UUID, requester, decision, nullable(reason), now); err != nil {
			return apierrors.New(apierrors.Internal, "decision insert failed").Wrap(err)
		}
		// Recompute aggregate state (03 §6 整包状态计算).
		aggState, err := s.aggregateState(ctx, tx, handoffID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE handoffs SET version=version+1, updated_at=$2 WHERE id=$1`, handoffID, now); err != nil {
			return apierrors.New(apierrors.Internal, "handoff update failed").Wrap(err)
		}
		if _, err := events.AppendProjectEvent(ctx, tx, projectID, "handoff.changed", "handoff", handoffID.String(), nil,
			map[string]any{"change": "source_" + decision, "sourceId": sourceID, "aggregate": aggState}, now); err != nil {
			return err
		}
		if err := audit.Append(ctx, tx, audit.Entry{
			ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
			IdentityID: &receiverIdentity, Source: audit.SourceWeb,
			Operation: "handoff.source." + decision, ObjectType: "handoff", ObjectID: handoffID.String(),
			Reason: strPtrOrNil(reason), OccurredAt: now,
		}); err != nil {
			return err
		}
		out = Source{ID: sourceID, SourceTaskID: sourceTask, SenderIdentityID: senderIdentity,
			CurrentVersionID: &versionID.UUID, State: nextState}
		return nil
	})
	return out, err
}

func (s *Service) aggregateState(ctx context.Context, tx pgx.Tx, handoffID uuid.UUID) (string, error) {
	rows, err := tx.Query(ctx, `
		SELECT COALESCE((SELECT v.state FROM source_versions v WHERE v.id=hs.current_source_version_id),'draft')
		FROM handoff_sources hs WHERE hs.handoff_id=$1`, handoffID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	pendingOrDraft, rejected, accepted := 0, 0, 0
	for rows.Next() {
		var state string
		if err := rows.Scan(&state); err != nil {
			return "", err
		}
		switch state {
		case "accepted":
			accepted++
		case "rejected":
			rejected++
		default:
			pendingOrDraft++
		}
	}
	switch {
	case pendingOrDraft > 0:
		return "waiting", nil
	case rejected > 0:
		return "needs_revision", nil
	default:
		return "accepted", nil
	}
}

// List returns the project's handoffs with sources and versions.
func (s *Service) List(ctx context.Context, requester, projectID uuid.UUID) ([]Handoff, error) {
	if err := memberTx(ctx, s.pool, projectID, requester); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, title, target_task_id, receiver_identity_id, kind, version FROM handoffs
		WHERE project_id=$1 ORDER BY created_at DESC LIMIT 100`, projectID)
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "handoffs failed").Wrap(err)
	}
	defer rows.Close()
	var out []Handoff
	for rows.Next() {
		var h Handoff
		if err := rows.Scan(&h.ID, &h.Title, &h.TargetTaskID, &h.ReceiverIdentityID, &h.Kind, &h.Version); err != nil {
			return nil, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		srcRows, err := s.pool.Query(ctx, `
			SELECT hs.id, hs.source_task_id, hs.sender_identity_id, hs.current_source_version_id,
			       COALESCE((SELECT v.revision FROM source_versions v WHERE v.id=hs.current_source_version_id),0),
			       COALESCE((SELECT v.state FROM source_versions v WHERE v.id=hs.current_source_version_id),'draft')
			FROM handoff_sources hs WHERE hs.handoff_id=$1`, out[i].ID)
		if err != nil {
			return nil, err
		}
		for srcRows.Next() {
			var src Source
			var versionID uuid.NullUUID
			var rev int64
			if err := srcRows.Scan(&src.ID, &src.SourceTaskID, &src.SenderIdentityID, &versionID, &rev, &src.State); err != nil {
				srcRows.Close()
				return nil, err
			}
			if versionID.Valid {
				vid := versionID.UUID
				src.CurrentVersionID = &vid
				if rev > 0 {
					src.CurrentVersion = &rev
				}
			}
			out[i].Sources = append(out[i].Sources, src)
		}
		srcRows.Close()
		// Aggregate from loaded sources.
		pending, rejected := 0, 0
		for _, src := range out[i].Sources {
			switch src.State {
			case "accepted":
			case "rejected":
				rejected++
			default:
				pending++
			}
		}
		switch {
		case pending > 0:
			out[i].State = "waiting"
		case rejected > 0:
			out[i].State = "needs_revision"
		default:
			out[i].State = "accepted"
		}
	}
	return out, nil
}

func nullable(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func strPtrOrNil(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}
