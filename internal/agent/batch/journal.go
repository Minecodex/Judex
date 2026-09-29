package batch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kakj-go/Judex/internal/agent/runner"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

type journal struct {
	pool                         *pgxpool.Pool
	project, run, session, batch uuid.UUID
	lease                        string
	reserved                     int64
	coveredSeq                   int64
	contextMetadata              map[string]any
}

func (j *journal) transaction(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := j.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	if _, err = tx.Exec(ctx, `SELECT 1 FROM discussion_batches WHERE id=$1 FOR UPDATE`, j.batch); err != nil {
		return err
	}
	var state string
	var token *string
	if err = tx.QueryRow(ctx, `SELECT r.state,r.lease_token FROM agent_runs r JOIN projects p ON p.id=r.project_id JOIN agent_identities i ON i.id=r.identity_id AND i.project_id=r.project_id
 WHERE r.id=$1 AND r.project_id=$2 AND p.status='active' AND i.status='active'
 AND (i.kind='coordinator' OR (i.current_binding_version=r.binding_version AND EXISTS(SELECT 1 FROM identity_bindings b JOIN users u ON u.id=b.user_id JOIN project_members m ON m.user_id=b.user_id AND m.project_id=i.project_id WHERE b.identity_id=i.id AND b.binding_version=i.current_binding_version AND b.valid_until IS NULL AND u.status='active' AND m.state='active')))
 FOR UPDATE OF r`, j.run, j.project).Scan(&state, &token); err != nil {
		return err
	}
	if state != "running" || token == nil || *token != j.lease {
		return apierrors.New(apierrors.InvalidTransition, "run cancelled or lease replaced")
	}
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (j *journal) event(ctx context.Context, tx pgx.Tx, kind string) error {
	_, err := tx.Exec(ctx, `INSERT INTO run_events(project_id,id,run_id,seq,type,payload,created_at)
 SELECT $1,$2,$3,COALESCE(max(seq),0)+1,$4,'{}',now() FROM run_events WHERE run_id=$3`, j.project, uuid.New(), j.run, kind)
	return err
}
func (j *journal) ModelStart(ctx context.Context, attempt int, request model.Request) error {
	raw, _ := json.Marshal(request)
	sum := sha256.Sum256(raw)
	reserve := request.MaxOutputTokens + runner.RequestBytes(request.Messages, request.Tools)
	err := j.transaction(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE discussion_batches SET model_attempts=model_attempts+1,reserved_tokens=reserved_tokens+$2
   WHERE id=$1 AND state='running' AND model_attempts<$3 AND used_tokens+reserved_tokens+$2<=$4`, j.batch, reserve, runner.BatchMaxModelAttempts, runner.BatchMaxTotalTokens)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return apierrors.New(apierrors.BudgetExhausted, "batch model/token budget exhausted")
		}
		if _, err = tx.Exec(ctx, `INSERT INTO model_calls(project_id,id,run_id,attempt,request_manifest_hash,state,started_at,reserved_tokens) VALUES($1,$2,$3,$4,$5,'running',now(),$6)`, j.project, uuid.New(), j.run, attempt, hex.EncodeToString(sum[:]), reserve); err != nil {
			return err
		}
		return j.event(ctx, tx, "model.started")
	})
	if err == nil {
		j.reserved = reserve
	}
	return err
}
func (j *journal) ModelEnd(ctx context.Context, attempt int, used int64, known bool, callErr error) error {
	return j.transaction(ctx, func(tx pgx.Tx) error {
		state := "succeeded"
		if callErr != nil {
			state = "failed"
		}
		if !known {
			used = j.reserved
		}
		usage, _ := json.Marshal(map[string]any{"totalTokens": used, "conservative": !known})
		if _, err := tx.Exec(ctx, `UPDATE model_calls SET state=$3,usage_json=$4,usage_known=$5,ended_at=now() WHERE run_id=$1 AND attempt=$2 AND state='running'`, j.run, attempt, state, usage, known); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE discussion_batches SET reserved_tokens=GREATEST(0,reserved_tokens-$2),used_tokens=used_tokens+$3 WHERE id=$1`, j.batch, j.reserved, used); err != nil {
			return err
		}
		return j.event(ctx, tx, "model."+state)
	})
}
func (j *journal) ToolStart(ctx context.Context, call model.ToolCall) error {
	sum := sha256.Sum256([]byte(call.Name + "\n" + call.Arguments))
	return j.transaction(ctx, func(tx pgx.Tx) error {
		var existing string
		err := tx.QueryRow(ctx, `SELECT state FROM tool_calls WHERE run_id=$1 AND tool_call_id=$2`, j.run, call.ID).Scan(&existing)
		if err == nil {
			return apierrors.New(apierrors.InvalidTransition, "tool call already recorded; refusing replay")
		}
		if err != pgx.ErrNoRows {
			return err
		}
		effect := "read"
		switch call.Name {
		case "read", "write", "edit", "bash":
			effect = "sandbox"
		case "propose_changes", "record_analysis", "publish":
			effect = "draft"
		}
		if _, err = tx.Exec(ctx, `INSERT INTO tool_calls(project_id,id,run_id,tool_call_id,name,args_hash,effect_class,state,started_at) VALUES($1,$2,$3,$4,$5,$6,$7,'prepared',now())`, j.project, uuid.New(), j.run, call.ID, call.Name, hex.EncodeToString(sum[:]), effect); err != nil {
			return err
		}
		return j.event(ctx, tx, "tool.prepared")
	})
}
func (j *journal) ToolEnd(ctx context.Context, call model.ToolCall, result []byte, unknown bool) error {
	return j.transaction(ctx, func(tx pgx.Tx) error {
		state := "succeeded"
		var value map[string]any
		_ = json.Unmarshal(result, &value)
		if value["error"] != nil {
			state = "failed"
		}
		if unknown {
			state = "unknown"
		}
		if len(result) > 1<<20 {
			return fmt.Errorf("tool result exceeds durable inline limit")
		}
		if _, err := tx.Exec(ctx, `UPDATE tool_calls SET state=$3,result_json=$4,ended_at=now() WHERE run_id=$1 AND tool_call_id=$2 AND state='prepared'`, j.run, call.ID, state, result); err != nil {
			return err
		}
		return j.event(ctx, tx, "tool."+state)
	})
}
func (j *journal) Checkpoint(ctx context.Context, messages []model.Message) error {
	// Never persist private system prompts in a shared checkpoint projection.
	safe := []model.Message{}
	for _, message := range messages {
		if message.Role != "system" {
			safe = append(safe, message)
		}
	}
	raw, err := json.Marshal(map[string]any{"messages": safe, "metadata": j.contextMetadata})
	if err != nil {
		return err
	}
	return j.transaction(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO context_checkpoints(project_id,id,session_id,run_id,covered_seq,manifest,summary,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,now())`, j.project, uuid.New(), j.session, j.run, j.coveredSeq, raw, fmt.Sprintf("source messages through seq %d; original messages and tool results retained", j.coveredSeq))
		return err
	})
}
