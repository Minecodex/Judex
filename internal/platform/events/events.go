// SPDX-License-Identifier: Apache-2.0

// Package events implements the project event sequence and outbox from
// docs/plans/v1/01 §5 and 04 §5: the project seq is allocated under the
// caller's project row lock inside the SAME business transaction, appended to
// project_events (immutable, (project_id, seq) unique), and mirrored into
// outbox_events purely to wake deliverers — never as the source of truth.
package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Refs are minimal object references carried in the event payload envelope.
type Refs map[string]any

// ProjectEvent is one durable, ordered business fact.
type ProjectEvent struct {
	ProjectID   uuid.UUID
	EventID     uuid.UUID
	Seq         int64
	Type        string
	ObjectType  string
	ObjectID    string
	Version     *int64
	Payload     map[string]any
	OccurredAt  time.Time
}

// AppendProjectEvent allocates the next seq for the project (requires the
// caller to already hold the project row FOR UPDATE — see postgres.Tx
// LockProjectForUpdate) and inserts both project_events and an outbox wake
// row in the same transaction.
func AppendProjectEvent(ctx context.Context, tx pgx.Tx, projectID uuid.UUID, eventType, objectType, objectID string, version *int64, payload map[string]any, now time.Time) (ProjectEvent, error) {
	var seq int64
	if err := tx.QueryRow(ctx,
		`UPDATE projects SET event_seq = event_seq + 1, updated_at = $2 WHERE id = $1 RETURNING event_seq`,
		projectID, now).Scan(&seq); err != nil {
		return ProjectEvent{}, err
	}
	eventID := uuid.New()
	if payload == nil {
		payload = map[string]any{}
	}
	occurred := now.UTC()
	if _, err := tx.Exec(ctx, `
		INSERT INTO project_events (project_id, seq, event_id, type, object_type, object_id, version, payload, occurred_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		projectID, seq, eventID, eventType, objectType, objectID, version, payload, occurred); err != nil {
		return ProjectEvent{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO outbox_events (id, project_id, project_seq, kind, payload, created_at)
		VALUES ($1,$2,$3,'project.event',$4,$5)`,
		uuid.New(), projectID, seq, payload, occurred); err != nil {
		return ProjectEvent{}, err
	}
	return ProjectEvent{
		ProjectID: projectID, EventID: eventID, Seq: seq,
		Type: eventType, ObjectType: objectType, ObjectID: objectID,
		Version: version, Payload: payload, OccurredAt: occurred,
	}, nil
}

// OutboxRow is one pending wake-up entry.
type OutboxRow struct {
	ID        uuid.UUID
	ProjectID uuid.UUID
	Seq       *int64
	Kind      string
	Payload   json.RawMessage
}

// ClaimPendingOutbox marks up to limit rows delivered and returns them.
// Delivery is best-effort wake-up only; SSE consumers always replay from
// project_events by seq (04 §5).
func ClaimPendingOutbox(ctx context.Context, tx pgx.Tx, now time.Time, limit int) ([]OutboxRow, error) {
	rows, err := tx.Query(ctx, `
		UPDATE outbox_events SET delivered_at = $1
		WHERE id IN (
			SELECT id FROM outbox_events WHERE delivered_at IS NULL
			ORDER BY created_at LIMIT $2 FOR UPDATE SKIP LOCKED
		)
		RETURNING id, project_id, project_seq, kind, payload`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OutboxRow
	for rows.Next() {
		var r OutboxRow
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.Seq, &r.Kind, &r.Payload); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
