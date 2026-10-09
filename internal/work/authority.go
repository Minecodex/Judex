package work

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

// ResolveParticipant never treats a missing identity as permission to act.
// A single current assignment can be inferred; multiple assignments require
// the caller to state which responsibility the report represents.
func ResolveParticipant(ctx context.Context, tx pgx.Tx, user, project, task uuid.UUID, requested *uuid.UUID) (uuid.UUID, int64, error) {
	rows, err := tx.Query(ctx, `SELECT i.id,i.current_binding_version FROM task_participants p
 JOIN agent_identities i ON i.id=p.identity_id AND i.project_id=p.project_id AND i.status='active'
 JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version AND b.valid_until IS NULL
 JOIN project_members m ON m.project_id=i.project_id AND m.user_id=b.user_id AND m.state='active'
 WHERE p.project_id=$1 AND p.task_id=$2 AND b.user_id=$3 AND ($4::uuid IS NULL OR i.id=$4) ORDER BY i.id`, project, task, user, requested)
	if err != nil {
		return uuid.Nil, 0, err
	}
	defer rows.Close()
	var id uuid.UUID
	var version int64
	count := 0
	for rows.Next() {
		if err = rows.Scan(&id, &version); err != nil {
			return uuid.Nil, 0, err
		}
		count++
	}
	if err = rows.Err(); err != nil {
		return uuid.Nil, 0, err
	}
	if count == 0 {
		return uuid.Nil, 0, apierrors.New(apierrors.Forbidden, "current task participant required")
	}
	if count > 1 {
		return uuid.Nil, 0, apierrors.Fields("identityId", "ambiguous")
	}
	return id, version, nil
}

func RequireIdentityHolder(ctx context.Context, tx pgx.Tx, user, project uuid.UUID, identity *uuid.UUID) error {
	if identity == nil {
		return apierrors.New(apierrors.RequirementUnmet, "work has no responsible identity")
	}
	var ok bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM agent_identities i
 JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version AND b.valid_until IS NULL
 JOIN project_members m ON m.project_id=i.project_id AND m.user_id=b.user_id AND m.state='active'
 WHERE i.id=$1 AND i.project_id=$2 AND i.status='active' AND b.user_id=$3)`, *identity, project, user).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return apierrors.New(apierrors.Forbidden, "current responsible identity holder required")
	}
	return nil
}
