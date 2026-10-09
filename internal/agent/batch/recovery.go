package batch

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
)

// Restore only completed server-tool transcripts. Unknown effects, unfinished
// children and lost sandbox state require reconciliation instead of replay.
func recoverTranscript(ctx context.Context, tx pgx.Tx, batch, run uuid.UUID) ([]model.Message, bool, error) {
	var unsafe bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tool_calls WHERE run_id IN(SELECT id FROM agent_runs WHERE batch_id=$1) AND (state IN ('prepared','running','unknown') OR name IN ('bash','read','write','edit'))) OR EXISTS(SELECT 1 FROM agent_runs WHERE batch_id=$1 AND parent_run_id IS NOT NULL AND state NOT IN ('succeeded','failed','cancelled'))`, batch).Scan(&unsafe); err != nil {
		return nil, false, err
	}
	if unsafe {
		return nil, false, nil
	}
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT manifest FROM context_checkpoints WHERE run_id=$1 ORDER BY created_at DESC,id DESC LIMIT 1`, run).Scan(&raw); err != nil {
		if err == pgx.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}
	var checkpoint struct {
		Messages []model.Message `json:"messages"`
	}
	if err := json.Unmarshal(raw, &checkpoint); err != nil {
		return nil, false, err
	}
	history := []model.Message{}
	for _, m := range checkpoint.Messages {
		if m.Role == "assistant" || m.Role == "tool" {
			history = append(history, m)
		}
	}
	if len(history) == 0 {
		return nil, false, nil
	}
	// A crash may occur after durable tool results but before the second checkpoint.
	responded := map[string]bool{}
	for _, m := range history {
		if m.Role == "tool" {
			responded[m.ToolCallID] = true
		}
	}
	for _, m := range append([]model.Message{}, history...) {
		for _, call := range m.ToolCalls {
			if responded[call.ID] {
				continue
			}
			var raw []byte
			var state string
			if err := tx.QueryRow(ctx, `SELECT state,result_json FROM tool_calls WHERE run_id=$1 AND tool_call_id=$2`, run, call.ID).Scan(&state, &raw); err != nil || raw == nil {
				return nil, false, nil
			}
			if state != "succeeded" && state != "failed" {
				return nil, false, nil
			}
			history = append(history, model.Message{Role: "tool", ToolCallID: call.ID, Content: string(raw)})
		}
	}
	// The previous model request's usage is unknown; charge its reservation once.
	if _, err := tx.Exec(ctx, `UPDATE discussion_batches SET used_tokens=used_tokens+COALESCE((SELECT sum(reserved_tokens) FROM model_calls WHERE run_id=$2 AND state='running'),0),reserved_tokens=GREATEST(0,reserved_tokens-COALESCE((SELECT sum(reserved_tokens) FROM model_calls WHERE run_id=$2 AND state='running'),0)) WHERE id=$1`, batch, run); err != nil {
		return nil, false, err
	}
	if _, err := tx.Exec(ctx, `UPDATE model_calls SET state='failed',usage_known=false,usage_json=jsonb_build_object('totalTokens',reserved_tokens,'conservative',true),error_code='WORKER_INTERRUPTED',ended_at=now() WHERE run_id=$1 AND state='running'`, run); err != nil {
		return nil, false, err
	}
	return history, true, nil
}
