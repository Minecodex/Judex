// SPDX-License-Identifier: Apache-2.0

// Package batch executes discussion batches (docs/plans/v1/05 §3): claim the
// queued batch, build the coordinator context from real project facts, run
// the model through the shared runner, and commit the result as an agent
// message in the topic. Business state NEVER changes from run success.
package batch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	agentcontext "github.com/kakj-go/Judex/internal/agent/context"
	"github.com/kakj-go/Judex/internal/agent/runner"
	"github.com/kakj-go/Judex/internal/agent/tools"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
	"github.com/kakj-go/Judex/internal/job"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

// Executor runs one discussion batch end-to-end.
type Executor struct {
	Pool      *pgxpool.Pool
	Runner    *runner.Runner
	Provider  model.Provider
	ModelName string
	Clock     func() time.Time
}

func (e *Executor) now() time.Time {
	if e.Clock != nil {
		return e.Clock()
	}
	return time.Now().UTC()
}

// ExecuteBatch claims the batch (state queued→running under the project
// lock), assembles context, runs the model and commits the agent message.
// Failures mark the batch failed with the reason; they never touch formal
// business state.
func (e *Executor) ExecuteBatch(ctx context.Context, projectID, batchID uuid.UUID) error {
	// 1) Claim + gather under the project guard.
	var (
		topicID          uuid.UUID
		sourceSubmission *uuid.UUID
		sourceText       string
		maxRounds        int
		state            string
	)
	err := e.withProjectTx(ctx, projectID, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			SELECT state FROM discussion_batches WHERE id=$1 AND project_id=$2 FOR UPDATE`,
			batchID, projectID).Scan(&state); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apierrors.New(apierrors.NotFound, "batch not found")
			}
			return err
		}
		if state == "completed" {
			return errAlreadyDone
		}
		if state != "queued" && state != "running" {
			return apierrors.Newf(apierrors.InvalidTransition, "batch is %s", state)
		}
		if _, err := tx.Exec(ctx, `UPDATE discussion_batches SET state='running', updated_at=$2 WHERE id=$1`,
			batchID, e.now()); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `
			SELECT topic_id, source_submission_id, max_rounds FROM discussion_batches WHERE id=$1`,
			batchID).Scan(&topicID, &sourceSubmission, &maxRounds); err != nil {
			return err
		}
		if sourceSubmission != nil {
			_ = tx.QueryRow(ctx, `SELECT text FROM submissions WHERE id=$1`, *sourceSubmission).Scan(&sourceText)
		}
		return nil
	})
	if errors.Is(err, errAlreadyDone) {
		return nil
	}
	if err != nil {
		return err
	}

	// 2) Build the coordinator manifest from real project facts.
	facts := e.gatherFacts(ctx, projectID, topicID)
	facts.NewMaterial = sourceText
	manifest := agentcontext.Build(facts)

	// 3) Reserve one round (05 §3：第一次启动以事务预留 round ordinal).
	roundID := uuid.New()
	_ = e.withProjectTx(ctx, projectID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO discussion_rounds (project_id, id, batch_id, ordinal, state, created_at)
			VALUES ($1,$2,$3,1,'running',$4)`, projectID, roundID, batchID, e.now())
		return err
	})

	// 4) Ensure the coordinator session exists ((topic,identity) unique),
	//    then run the model outside DB transactions (01 §1).
	coordinator := e.coordinatorID(ctx, projectID)
	sessionID := e.ensureSession(ctx, projectID, topicID, coordinator)
	runID := uuid.New()
	env := e.toolEnv(projectID, topicID)
	outcome := e.Runner.Run(ctx, runner.RunRequest{
		RunID: runID, SessionID: sessionID, ProjectID: projectID,
		ModelName: e.ModelName, Manifest: manifest,
		Budget: runner.Budget{MaxRounds: maxRounds, MaxModelAttempts: 12,
			MaxTotalTokens: 120000, MaxWallClock: 5 * time.Minute},
		Env: &env,
	})

	// 5) Commit outcome: agent message on success/retryable note on failure.
	return e.commit(ctx, projectID, topicID, batchID, runID, sessionID, coordinator, outcome)
}

var errAlreadyDone = errors.New("batch already completed")

// RunOutcome mirrors the runner result for persistence.
type RunOutcome = runner.Outcome

// gatherFacts loads coordinator-visible context: recent topic messages and
// open work items (read-only; no private prompts).
func (e *Executor) gatherFacts(ctx context.Context, projectID, topicID uuid.UUID) agentcontext.Facts {
	facts := agentcontext.Facts{}
	rows, err := e.Pool.Query(ctx, `
		SELECT content FROM messages WHERE topic_id=$1 AND project_id=$2
		ORDER BY seq DESC LIMIT 12`, topicID, projectID)
	if err == nil {
		defer rows.Close()
		var recent []string
		for rows.Next() {
			var content string
			if rows.Scan(&content) == nil {
				recent = append(recent, content)
			}
		}
		// Reverse to chronological order.
		for i, j := 0, len(recent)-1; i < j; i, j = i+1, j-1 {
			recent[i], recent[j] = recent[j], recent[i]
		}
		facts.RecentMessages = recent
	}
	taskRows, err := e.Pool.Query(ctx, `
		SELECT title || '（' || status || '）' FROM tasks
		WHERE project_id=$1 AND status NOT IN ('draft','cancelled') ORDER BY updated_at DESC LIMIT 10`, projectID)
	if err == nil {
		defer taskRows.Close()
		for taskRows.Next() {
			var fact string
			if taskRows.Scan(&fact) == nil {
				facts.WorkFacts = append(facts.WorkFacts, fact)
			}
		}
	}
	return facts
}

// toolEnv wires read-only DB projections for the coordinator run (05 §6).
func (e *Executor) toolEnv(projectID, topicID uuid.UUID) tools.Env {
	return tools.Env{
		ProjectID: projectID.String(),
		ListAgents: func(ctx context.Context, pid string) ([]map[string]any, error) {
			rows, err := e.Pool.Query(ctx, `
				SELECT i.id::text, i.kind, COALESCE(u.display_name,'') AS holder,
				       COALESCE((SELECT name FROM position_templates t WHERE t.id=i.template_id),'') AS position
				FROM agent_identities i
				LEFT JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version
				LEFT JOIN users u ON u.id=b.user_id
				WHERE i.project_id=$1`, pid)
			if err != nil {
				return nil, err
			}
			defer rows.Close()
			var out []map[string]any
			for rows.Next() {
				var id, kind, holder, position string
				if rows.Scan(&id, &kind, &holder, &position) == nil {
					out = append(out, map[string]any{"identityId": id, "kind": kind,
						"holder": holder, "position": position})
				}
			}
			return out, nil
		},
		QueryWork: func(ctx context.Context, pid, objectType, cursor string, limit int) ([]map[string]any, error) {
			if limit < 1 || limit > 50 {
				limit = 20
			}
			table := map[string]string{"plan": "plans", "task": "tasks", "proposal": "proposals"}[objectType]
			if table == "" {
				return nil, fmt.Errorf("unknown objectType %q", objectType)
			}
			rows, err := e.Pool.Query(ctx, fmt.Sprintf(
				`SELECT id::text, title, status FROM %s WHERE project_id=$1 ORDER BY created_at DESC LIMIT $2`, table),
				pid, limit)
			if err != nil {
				return nil, err
			}
			defer rows.Close()
			var out []map[string]any
			for rows.Next() {
				var id, title, status string
				if rows.Scan(&id, &title, &status) == nil {
					out = append(out, map[string]any{"id": id, "title": title, "status": status,
						"objectType": objectType})
				}
			}
			return out, nil
		},
		ReadMaterial: func(ctx context.Context, pid, versionID string) (map[string]any, error) {
			var sha string
			var size int64
			var mime string
			err := e.Pool.QueryRow(ctx, `
				SELECT sha256, size, mime FROM material_versions
				WHERE id=$1 AND project_id=$2`, versionID, pid).Scan(&sha, &size, &mime)
			if err != nil {
				return nil, fmt.Errorf("材料版本不可读（沙箱内容读取依赖部署配置）")
			}
			// Sandbox content reading requires the deployment mount; metadata
			// is the honest projection without it.
			return map[string]any{"versionId": versionID, "sha256": sha, "size": size,
				"mime": mime, "note": "内容原文读取需沙箱挂载；此处仅核对元数据"}, nil
		},
		ProposeDraft: func(ctx context.Context, pid string, draft map[string]any) (string, error) {
			// Draft-only effect (05 §6)：仅创建草稿，不提交人工投票。
			id := uuid.New()
			changes, _ := json.Marshal(draft["changes"])
			tx, err := e.Pool.Begin(ctx)
			if err != nil {
				return "", err
			}
			defer tx.Rollback(ctx)
			if _, err := tx.Exec(ctx, `
				INSERT INTO proposals (project_id, id, kind, status, reason, created_at, updated_at)
				VALUES ($1,$2,'work_arrangement','draft',$3,$4,$4)`,
				pid, id, draft["reason"], e.now()); err != nil {
				return "", err
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO proposal_versions (project_id, id, proposal_id, revision, changes_json, created_at)
				VALUES ($1,$2,$3,1,$4,$5)`, pid, uuid.New(), id, changes, e.now()); err != nil {
				return "", err
			}
			if err := tx.Commit(ctx); err != nil {
				return "", err
			}
			return id.String(), nil
		},
	}
}

// commit writes the agent message + batch terminal state in one transaction.
func (e *Executor) commit(ctx context.Context, projectID, topicID, batchID, runID, sessionID, coordinator uuid.UUID, outcome runner.Outcome) error {
	return e.withProjectTx(ctx, projectID, func(tx pgx.Tx) error {
		now := e.now()
		batchState := "completed"
		content := outcome.Summary
		if outcome.State != "succeeded" {
			// Failed runs surface the reason honestly; they never fake success.
			batchState = "failed"
			content = "本次自动分析未完成（" + outcome.State + "）：" + firstLine(outcome.Summary)
			if outcome.State == "failed" && containsAny(outcome.Summary, "预算", "轮次") {
				batchState = "limit_reached"
			}
		}
		// Persist the run record for traceability (05 §10).
		if _, err := tx.Exec(ctx, `
			INSERT INTO agent_runs (project_id, id, session_id, batch_id, identity_id, state,
				manifest, budget_snapshot, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,'{}'::jsonb,$7::jsonb,$8,$8)`,
			projectID, runID, sessionID, batchID, coordinator,
			outcome.State, runBudgetJSON(outcome), now); err != nil {
			return err
		}
		var seq int64
		if err := tx.QueryRow(ctx, `
			UPDATE topics SET last_message_seq = last_message_seq + 1 WHERE id=$1 RETURNING last_message_seq`,
			topicID).Scan(&seq); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO messages (project_id, id, topic_id, seq, kind, identity_id, run_id, content, state, created_at)
			VALUES ($1,$2,$3,$4,'agent',$5,$6,$7,'committed',$8)`,
			projectID, uuid.New(), topicID, seq, coordinator,
			runID, content, now); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE discussion_batches SET state=$2, rounds_reserved=rounds_reserved+1, updated_at=$3 WHERE id=$1`,
			batchID, batchState, now); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE discussion_rounds SET state='done' WHERE batch_id=$1`, batchID); err != nil {
			return err
		}
		return nil
	})
}

// ensureSession creates or reuses the (topic, coordinator identity) session.
func (e *Executor) ensureSession(ctx context.Context, projectID, topicID, identityID uuid.UUID) uuid.UUID {
	var id uuid.UUID
	err := e.Pool.QueryRow(ctx, `
		SELECT id FROM agent_sessions WHERE topic_id=$1 AND identity_id=$2`, topicID, identityID).Scan(&id)
	if err == nil {
		return id
	}
	id = uuid.New()
	_, err = e.Pool.Exec(ctx, `
		INSERT INTO agent_sessions (project_id, id, topic_id, identity_id, created_at)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (topic_id, identity_id) DO NOTHING`,
		projectID, id, topicID, identityID, e.now())
	if err == nil {
		return id
	}
	_ = e.Pool.QueryRow(ctx, `
		SELECT id FROM agent_sessions WHERE topic_id=$1 AND identity_id=$2`, topicID, identityID).Scan(&id)
	return id
}

func (e *Executor) coordinatorID(ctx context.Context, projectID uuid.UUID) uuid.UUID {
	var id uuid.UUID
	_ = e.Pool.QueryRow(ctx,
		`SELECT id FROM agent_identities WHERE project_id=$1 AND kind='coordinator'`, projectID).Scan(&id)
	return id
}

func (e *Executor) coordinatorIDForTx(ctx context.Context, tx pgx.Tx, projectID uuid.UUID) uuid.UUID {
	var id uuid.UUID
	_ = tx.QueryRow(ctx,
		`SELECT id FROM agent_identities WHERE project_id=$1 AND kind='coordinator'`, projectID).Scan(&id)
	return id
}

func (e *Executor) withProjectTx(ctx context.Context, projectID uuid.UUID, fn func(pgx.Tx) error) error {
	tx, err := e.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `SELECT id FROM projects WHERE id=$1 FOR UPDATE`, projectID); err != nil {
		tx.Rollback(ctx)
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

func runBudgetJSON(o runner.Outcome) []byte {
	raw, _ := json.Marshal(map[string]any{
		"tokensUsed": o.TokensUsed, "usageKnown": o.UsageKnown,
		"modelCalls": o.ModelCalls, "toolCalls": o.ToolCalls,
	})
	return raw
}

func firstLine(s string) string {
	for i, r := range s {
		if r == '\n' {
			return s[:i]
		}
	}
	return s
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(sub) > 0 && len(s) >= len(sub) && contains(s, sub) {
			return true
		}
	}
	return false
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ExecuteJob adapts ExecuteBatch to the job engine payload.
func (e *Executor) ExecuteJob(ctx context.Context, j job.Job) error {
	payload := struct {
		ProjectID string `json:"projectId"`
		BatchID   string `json:"batchId"`
	}{}
	if err := json.Unmarshal(j.Payload, &payload); err != nil {
		return apierrors.New(apierrors.Validation, "bad batch payload").Wrap(err)
	}
	projectID, err1 := uuid.Parse(payload.ProjectID)
	batchID, err2 := uuid.Parse(payload.BatchID)
	if err1 != nil || err2 != nil {
		return apierrors.New(apierrors.Validation, "bad batch payload ids")
	}
	return e.ExecuteBatch(ctx, projectID, batchID)
}
