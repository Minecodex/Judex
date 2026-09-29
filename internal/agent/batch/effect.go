package batch

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/kakj-go/Judex/internal/agent/tools"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

// Business draft effects recheck fencing in their own transaction, after taking
// the project lock. A superseded worker cannot commit after a new lease starts.
func (e *Executor) beginEffect(ctx context.Context, project uuid.UUID) (pgx.Tx, error) {
	authority, ok := tools.Authority(ctx)
	if !ok || authority.Project != project.String() || authority.Lease == "" {
		return nil, apierrors.New(apierrors.Forbidden, "runtime authority required")
	}
	tx, err := e.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (pgx.Tx, error) { tx.Rollback(context.WithoutCancel(ctx)); return nil, err }
	var active string
	if err = tx.QueryRow(ctx, `SELECT status FROM projects WHERE id=$1 FOR UPDATE`, project).Scan(&active); err != nil || active != "active" {
		return fail(apierrors.New(apierrors.Forbidden, "active project required"))
	}
	var batch uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT batch_id FROM agent_runs WHERE id=$1 AND project_id=$2`, authority.Run, project).Scan(&batch); err != nil {
		return fail(err)
	}
	if _, err = tx.Exec(ctx, `SELECT 1 FROM discussion_batches WHERE id=$1 FOR UPDATE`, batch); err != nil {
		return fail(err)
	}
	var valid bool
	err = tx.QueryRow(ctx, `SELECT r.state='running' AND r.lease_token=$3 AND i.status='active' AND (i.kind='coordinator' OR (i.current_binding_version=r.binding_version AND EXISTS(SELECT 1 FROM identity_bindings b JOIN project_members m ON m.user_id=b.user_id AND m.project_id=i.project_id JOIN users u ON u.id=b.user_id WHERE b.identity_id=i.id AND b.binding_version=i.current_binding_version AND b.valid_until IS NULL AND m.state='active' AND u.status='active'))) FROM agent_runs r JOIN agent_identities i ON i.id=r.identity_id WHERE r.id=$1 AND r.project_id=$2 FOR UPDATE OF r`, authority.Run, project, authority.Lease).Scan(&valid)
	if err != nil || !valid {
		return fail(apierrors.New(apierrors.Forbidden, "run lease or binding changed"))
	}
	return tx, nil
}
