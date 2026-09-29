// SPDX-License-Identifier: Apache-2.0

// Package audit writes immutable audit events inside business transactions.
// Audit rows must never contain passwords, full tokens, personal prompts or
// whole file contents (docs/plans/v1/01 §3 M001).
package audit

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ActorKind mirrors audit_events.actor_type.
type ActorKind string

const (
	ActorUser     ActorKind = "user"
	ActorOperator ActorKind = "operator"
	ActorAgent    ActorKind = "agent"
	ActorSystem   ActorKind = "system"
)

// Source mirrors audit_events.source (decision channel).
type Source string

type sourceContextKey struct{}

func WithSource(ctx context.Context, source Source) context.Context {
	return context.WithValue(ctx, sourceContextKey{}, source)
}
func ContextSource(ctx context.Context) Source {
	if s, ok := ctx.Value(sourceContextKey{}).(Source); ok {
		return s
	}
	return SourceWeb
}

const (
	SourceWeb      Source = "web"
	SourceCLI      Source = "cli"
	SourceDelegate Source = "delegate"
	SourceTimeout  Source = "timeout"
	SourceWorker   Source = "worker"
	SourceOperator Source = "operator"
	SourceAgent    Source = "agent"
)

// Entry is one audit fact; ObjectID is a string so non-uuid refs fit.
type Entry struct {
	ProjectID      *uuid.UUID
	ActorType      ActorKind
	ActorUserID    *uuid.UUID
	IdentityID     *uuid.UUID
	BindingVersion *int64
	Source         Source
	Operation      string
	ObjectType     string
	ObjectID       string
	ReviewID       *uuid.UUID
	RequestID      string
	Reason         *string
	OccurredAt     time.Time
}

// Append inserts the entry in the caller's transaction. IDs are generated
// here so the row exists even if the caller later rolls back other effects —
// the transaction guarantees all-or-nothing with the business change.
func Append(ctx context.Context, tx pgx.Tx, e Entry) error {
	if e.Source == SourceWeb {
		e.Source = ContextSource(ctx)
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO audit_events
			(id, project_id, actor_type, actor_user_id, identity_id, binding_version,
			 source, operation, object_type, object_id, review_id, request_id, reason, occurred_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		uuid.NewString(), e.ProjectID, string(e.ActorType), e.ActorUserID, e.IdentityID,
		e.BindingVersion, string(e.Source), e.Operation, e.ObjectType, e.ObjectID,
		e.ReviewID, e.RequestID, e.Reason, e.OccurredAt)
	return err
}
