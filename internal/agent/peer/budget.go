package peer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
)

// Reserve the full possible cost before network I/O. An interrupted request
// retains its reservation across process restarts instead of getting free quota.
func (e *Executor) reserveCall(ctx context.Context, r *run, id uuid.UUID, snapshot int64, name, hash string, tokens int64) error {
	return e.Service.Pool.Transact(ctx, func(ctx context.Context, _ postgres.Tx) error {
		ok, err := e.live(ctx, r)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("run lease lost before model reservation")
		}
		tag, err := e.Service.Pool.Exec(ctx, `UPDATE role_runs SET model_calls=model_calls+1,tokens=tokens+$3,budget_model_calls=budget_model_calls+1,budget_tokens=budget_tokens+$3,snapshot_version=$4 WHERE id=$1 AND fence=$2 AND budget_model_calls<30 AND budget_tokens+$3<=300000 AND budget_started_at+interval '30 minutes'>clock_timestamp()`, r.Publication.RunID, r.Publication.Fence, tokens, snapshot)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return fmt.Errorf("technical call/token budget exhausted before model request")
		}
		_, err = e.Service.Pool.Exec(ctx, `INSERT INTO role_model_calls(id,run_id,snapshot_version,model_id,input_hash,state,reserved_tokens,budget_epoch) SELECT $1,id,$3,$4,$5,'started',$6,budget_epoch FROM role_runs WHERE id=$2`, id, r.Publication.RunID, snapshot, name, hash, tokens)
		return err
	})
}

func (e *Executor) chargeCall(ctx context.Context, r *run, id uuid.UUID, usage model.UsageSnapshot, fallback int64) error {
	charged := fallback
	if usage.Known() {
		charged = usage.Total()
	}
	raw, _ := json.Marshal(usage)
	var delta int64
	err := e.Service.Pool.Transact(ctx, func(ctx context.Context, _ postgres.Tx) error {
		var reserved, epoch int64
		if err := e.Service.Pool.QueryRow(ctx, `UPDATE role_model_calls SET state='completed',usage=$2,charged_tokens=$3 WHERE id=$1 AND state='started' RETURNING reserved_tokens,budget_epoch`, id, raw, charged).Scan(&reserved, &epoch); err != nil {
			return err
		}
		delta = charged - reserved
		_, err := e.Service.Pool.Exec(ctx, `UPDATE role_runs SET tokens=tokens+$2,budget_tokens=budget_tokens+CASE WHEN budget_epoch=$3 THEN $2 ELSE 0 END WHERE id=$1`, r.Publication.RunID, delta, epoch)
		return err
	})
	if err == nil {
		r.Tokens += delta
	}
	return err
}
