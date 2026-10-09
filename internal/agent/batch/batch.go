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
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kakj-go/Judex/internal/collaboration"

	agentcontext "github.com/kakj-go/Judex/internal/agent/context"
	"github.com/kakj-go/Judex/internal/agent/runner"
	"github.com/kakj-go/Judex/internal/agent/tools"
	"github.com/kakj-go/Judex/internal/decision"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
	"github.com/kakj-go/Judex/internal/infrastructure/opensandbox"
	"github.com/kakj-go/Judex/internal/job"
	"github.com/kakj-go/Judex/internal/material"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/work"
)

// Executor runs one discussion batch end-to-end.
type Executor struct {
	Pool         *pgxpool.Pool
	Runner       *runner.Runner
	Provider     model.Provider
	ModelName    string
	Clock        func() time.Time
	Objects      material.ObjectStore
	ResolveModel func(context.Context, uuid.UUID, uuid.UUID) (model.Provider, string, int64, int64, error)
	// SandboxFactory provisions the per-run sandbox boundary (05 §7);
	// nil means no sandbox configured and sandbox tools fail honestly.
	SandboxFactory func(runID, projectID uuid.UUID) *opensandbox.RunSandbox
}

func (e *Executor) newRunSandbox(runID, projectID uuid.UUID) *opensandbox.RunSandbox {
	if e.SandboxFactory != nil {
		return e.SandboxFactory(runID, projectID)
	}
	return opensandbox.Unavailable()
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
	var topicID uuid.UUID
	var taskID *uuid.UUID
	var sourceText string
	var maxRounds int
	if err := e.Pool.QueryRow(ctx, `SELECT COALESCE(b.topic_id,'00000000-0000-0000-0000-000000000000'::uuid),COALESCE(s.text,r.progress_hint,''),b.max_rounds,b.task_id FROM discussion_batches b LEFT JOIN submissions s ON s.id=b.source_submission_id AND s.project_id=b.project_id LEFT JOIN work_reports r ON r.id=b.source_report_id AND r.project_id=b.project_id WHERE b.id=$1 AND b.project_id=$2`, batchID, projectID).Scan(&topicID, &sourceText, &maxRounds, &taskID); err != nil {
		return err
	}
	coordinator := e.coordinatorID(ctx, projectID)
	var sessionID uuid.UUID
	var err error
	if taskID != nil && topicID == uuid.Nil {
		sessionID, err = e.ensureTaskSession(ctx, projectID, *taskID, coordinator)
	} else {
		sessionID, err = e.ensureSession(ctx, projectID, topicID, coordinator)
	}
	if err != nil {
		return err
	}
	runID := uuid.New()
	lease := uuid.NewString()
	var restored []model.Message
	startAttempt := 1
	err = e.withProjectTx(ctx, projectID, func(tx pgx.Tx) error {
		var batchState string
		var reserved int
		if err := tx.QueryRow(ctx, `SELECT state,rounds_reserved FROM discussion_batches WHERE id=$1 FOR UPDATE`, batchID).Scan(&batchState, &reserved); err != nil {
			return err
		}
		if batchState == "completed" || batchState == "cancelled" || batchState == "waiting_human" || batchState == "failed" {
			return errAlreadyDone
		}
		var oldState string
		err := tx.QueryRow(ctx, `SELECT id,state FROM agent_runs WHERE batch_id=$1 AND parent_run_id IS NULL ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, batchID).Scan(&runID, &oldState)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		var projectState string
		if err = tx.QueryRow(ctx, `SELECT status FROM projects WHERE id=$1`, projectID).Scan(&projectState); err != nil {
			return err
		}
		if projectState != "active" {
			if _, err = tx.Exec(ctx, `UPDATE agent_runs SET state='cancelled',version=version+1,updated_at=now() WHERE batch_id=$1 AND state NOT IN ('succeeded','failed','cancelled')`, batchID); err != nil {
				return err
			}
			if err = collaboration.FinishTaskAnalysis(ctx, tx, projectID, batchID, "cancelled", "项目已归档，分析已取消。", e.now()); err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `UPDATE discussion_batches SET state='cancelled',version=version+1,updated_at=now() WHERE id=$1`, batchID)
			return err
		}
		var priorTools bool
		if oldState == "queued" && reserved > 0 {
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tool_calls WHERE run_id IN(SELECT id FROM agent_runs WHERE batch_id=$1))`, batchID).Scan(&priorTools); err != nil {
				return err
			}
		}
		if oldState == "running" || oldState == "provisioning" || batchState == "running" || priorTools {
			transcript, safe, recoverErr := recoverTranscript(ctx, tx, batchID, runID)
			if recoverErr != nil {
				return recoverErr
			}
			if safe {
				restored = transcript
				if err = tx.QueryRow(ctx, `SELECT COALESCE(max(attempt),0)+1 FROM model_calls WHERE run_id=$1`, runID).Scan(&startAttempt); err != nil {
					return err
				}
				if err = claimSession(ctx, tx, sessionID, runID); err != nil {
					return err
				}
				if _, err = tx.Exec(ctx, `UPDATE discussion_batches SET state='running',version=version+1,updated_at=now() WHERE id=$1`, batchID); err != nil {
					return err
				}
				if _, err = tx.Exec(ctx, `UPDATE task_analyses SET state='running',updated_at=now() WHERE batch_id=$1`, batchID); err != nil {
					return err
				}
				_, err = tx.Exec(ctx, `UPDATE agent_runs SET state='running',lease_token=$2,version=version+1,updated_at=now() WHERE id=$1`, runID, lease)
				return err
			}
			// We cannot prove a prepared tool did not execute. Preserve evidence and
			// require reconciliation; never re-run bash after worker loss.
			if _, err = tx.Exec(ctx, `UPDATE tool_calls SET state='unknown' WHERE run_id IN(SELECT id FROM agent_runs WHERE batch_id=$1) AND state IN ('prepared','running')`, batchID); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `UPDATE agent_runs SET state='waiting_human',lease_token=$2,version=version+1,budget_snapshot=jsonb_build_object('reason','interrupted run: reconcile unknown tool results'),updated_at=now() WHERE batch_id=$1 AND state NOT IN ('succeeded','failed','cancelled')`, batchID, lease); err != nil {
				return err
			}
			if err = collaboration.FinishTaskAnalysis(ctx, tx, projectID, batchID, "waiting_human", "中断的执行需要核对，未重复执行工具。", e.now()); err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `UPDATE discussion_batches SET state='waiting_human',version=version+1,updated_at=now() WHERE id=$1`, batchID)
			return err
		}
		if reserved >= maxRounds {
			return apierrors.New(apierrors.BudgetExhausted, "discussion round limit reached")
		}
		if oldState == "" {
			runID = uuid.New()
			if _, err = tx.Exec(ctx, `INSERT INTO agent_runs(project_id,id,session_id,batch_id,identity_id,state,created_at,updated_at) VALUES($1,$2,$3,$4,$5,'queued',now(),now())`, projectID, runID, sessionID, batchID, coordinator); err != nil {
				return err
			}
		} else if oldState != "queued" {
			return errAlreadyDone
		}
		if err = tx.QueryRow(ctx, `SELECT COALESCE(max(attempt),0)+1 FROM model_calls WHERE run_id=$1`, runID).Scan(&startAttempt); err != nil {
			return err
		}
		if err = claimSession(ctx, tx, sessionID, runID); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE agent_runs SET state='running',lease_token=$2,version=version+1,updated_at=now() WHERE id=$1`, runID, lease); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE task_analyses SET state='running',updated_at=now() WHERE batch_id=$1`, batchID); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE discussion_batches SET state='running',rounds_reserved=rounds_reserved+1,version=version+1,updated_at=now() WHERE id=$1`, batchID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO discussion_rounds(project_id,id,batch_id,ordinal,state,coordinator_run_id,created_at) VALUES($1,$2,$3,$4,'running',$5,now())`, projectID, uuid.New(), batchID, reserved+1, runID)
		return err
	})
	if errors.Is(err, errAlreadyDone) {
		return nil
	}
	if err != nil {
		return err
	}
	defer e.releaseSession(sessionID, runID)
	var runState string
	if err = e.Pool.QueryRow(ctx, `SELECT state FROM agent_runs WHERE id=$1`, runID).Scan(&runState); err != nil {
		return err
	}
	if runState != "running" {
		return nil
	}
	if len(restored) > 0 {
		last := restored[len(restored)-1]
		if last.Role == "assistant" && len(last.ToolCalls) == 0 {
			return e.commit(ctx, projectID, topicID, batchID, runID, sessionID, coordinator, runner.Outcome{State: "succeeded", Summary: last.Content}, lease)
		}
	}
	facts := e.gatherFacts(ctx, projectID, topicID)
	facts.NewMaterial = sourceText
	facts.PositionPrompt = "你是项目协调者。按公开职责调用 call_agent 收集岗位意见，保留异议与未完成事项；缺少必要岗位结果时明确报告缺口。"
	facts.WorkFacts = append(facts.WorkFacts, e.materialFacts(ctx, projectID, batchID)...)
	manifest := agentcontext.Build(facts)
	rawManifest, _ := json.Marshal(manifest.SharedLayers())
	if _, err = e.Pool.Exec(ctx, `UPDATE agent_runs SET manifest=$2 WHERE id=$1 AND lease_token=$3`, runID, rawManifest, lease); err != nil {
		return err
	}
	provider, modelName, maxIn, maxOut, err := e.resolve(ctx, projectID, coordinator)
	if err != nil {
		return e.commit(ctx, projectID, topicID, batchID, runID, sessionID, coordinator, runner.Outcome{State: "failed", Summary: err.Error()}, lease)
	}
	registry := tools.New()
	tools.RegisterDefaults(registry)
	caller := &positionCaller{executor: e, project: projectID, topic: topicID, batch: batchID, parent: runID, task: taskID}
	tools.RegisterCallAgent(registry, caller)
	harness := &runner.Runner{Provider: provider, Registry: registry, Clock: e.Clock}
	env := e.toolEnv(projectID, topicID)
	runSandbox := e.newRunSandbox(runID, projectID)
	env.Sandbox = runSandbox
	defer runSandbox.Close(context.WithoutCancel(ctx))
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go e.watchCancellation(runCtx, cancel, projectID, runID, lease)
	journal := &journal{pool: e.Pool, project: projectID, run: runID, session: sessionID, batch: batchID, lease: lease}
	outcome := harness.Run(runCtx, runner.RunRequest{Lease: lease, History: restored, StartAttempt: startAttempt, RunID: runID, SessionID: sessionID, ProjectID: projectID, IdentityID: coordinator, ModelName: modelName, Manifest: manifest,
		Budget: runner.Budget{MaxRounds: maxRounds, MaxModelAttempts: runner.BatchMaxModelAttempts, MaxTotalTokens: runner.BatchMaxTotalTokens, MaxWallClock: 30 * time.Minute}, MaxInputTokens: maxIn, MaxOutputTokens: maxOut, Env: &env,
		Journal: journal, Prepare: e.prepareCall(journal, topicID, sourceText),
	})
	if outcome.State == "succeeded" && len(caller.failures) > 0 {
		outcome.State = "waiting_human"
		outcome.Summary = "岗位结果不完整：" + strings.Join(caller.failures, "；") + "\n" + outcome.Summary
	}
	return e.commit(context.WithoutCancel(ctx), projectID, topicID, batchID, runID, sessionID, coordinator, outcome, lease)
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
		WHERE project_id=$1 AND status NOT IN ('draft','cancelled') AND (id IN(SELECT object_id FROM topic_work_links WHERE topic_id=$2 AND object_type='task') OR plan_id IN(SELECT object_id FROM topic_work_links WHERE topic_id=$2 AND object_type='plan')) ORDER BY updated_at DESC LIMIT 50`, projectID, topicID)
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
	env := tools.Env{
		QueryWorkPage: e.queryWorkPage,
		ProjectID:     projectID.String(),
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
			// proposals 无 title 列（kind+reason 表意），列选择按表区分。
			query := fmt.Sprintf(
				`SELECT id::text, %s, status FROM %s WHERE project_id=$1 ORDER BY created_at DESC LIMIT $2`,
				map[string]string{"plans": "title", "tasks": "title", "proposals": "kind"}[table], table)
			rows, err := e.Pool.Query(ctx, query, pid, limit)
			if err != nil {
				return nil, err
			}
			defer rows.Close()
			var out []map[string]any
			for rows.Next() {
				var id, label, status string
				if rows.Scan(&id, &label, &status) == nil {
					key := "title"
					if objectType == "proposal" {
						key = "kind"
					}
					out = append(out, map[string]any{"id": id, key: label, "status": status,
						"objectType": objectType})
				}
			}
			return out, nil
		},
		ReadMaterial: e.readMaterial,
		ProposeDraft: func(ctx context.Context, pid string, draft map[string]any) (string, error) {
			// Draft-only effect (05 §6)：仅创建草稿，不提交人工投票。
			id := uuid.New()
			changes, _ := json.Marshal(draft["changes"])
			var typed []decision.Change
			if err := json.Unmarshal(changes, &typed); err != nil {
				return "", apierrors.Fields("changes", "invalid")
			}
			if err := decision.ValidateDraftChanges(typed); err != nil {
				return "", err
			}
			tx, err := e.beginEffect(ctx, projectID)
			if err != nil {
				return "", err
			}
			defer tx.Rollback(ctx)
			var origin *uuid.UUID
			var cutoff *int64
			if topicID != uuid.Nil {
				origin = &topicID
				authority, _ := tools.Authority(ctx)
				var seq int64
				if err = tx.QueryRow(ctx, `SELECT COALESCE(max(covered_seq),0) FROM context_checkpoints WHERE run_id=$1`, authority.Run).Scan(&seq); err != nil {
					return "", err
				}
				cutoff = &seq
			}
			typed, err = work.SnapshotPlanOrigins(ctx, tx, projectID, origin, cutoff, typed)
			if err != nil {
				return "", err
			}
			changes, _ = json.Marshal(typed)
			if _, err := tx.Exec(ctx, `
				INSERT INTO proposals (project_id, id, topic_id,kind, status, reason, created_at, updated_at)
				VALUES ($1,$2,$5,'work_arrangement','draft',$3,$4,$4)`,
				pid, id, draft["reason"], e.now(), origin); err != nil {
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
	e.contextTools(&env, projectID)
	e.activityTools(&env, projectID)
	return env
}

// commit writes the agent message + batch terminal state in one transaction.
func (e *Executor) commit(ctx context.Context, projectID, topicID, batchID, runID, sessionID, coordinator uuid.UUID, outcome runner.Outcome, lease string) error {
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
		if outcome.State == "waiting_human" {
			batchState = "waiting_human"
		}
		if outcome.State == "context_blocked" {
			outcome.State = "waiting_human"
			batchState = "waiting_human"
		}
		var current string
		var currentLease *string
		if err := tx.QueryRow(ctx, `SELECT state,lease_token FROM agent_runs WHERE id=$1 FOR UPDATE`, runID).Scan(&current, &currentLease); err != nil {
			return err
		}
		if currentLease == nil || *currentLease != lease {
			return nil
		}
		if current == "cancelled" {
			batchState = "cancelled"
			outcome.State = "cancelled"
		}
		var projectState string
		if err := tx.QueryRow(ctx, `SELECT status FROM projects WHERE id=$1`, projectID).Scan(&projectState); err != nil {
			return err
		}
		if projectState != "active" {
			outcome.State = "cancelled"
			batchState = "cancelled"
		}
		if _, err := tx.Exec(ctx, `UPDATE agent_runs SET state=$2,budget_snapshot=$3,version=version+1,updated_at=now() WHERE id=$1`, runID, outcome.State, runBudgetJSON(outcome)); err != nil {
			return err
		}
		if projectState != "active" {
			if err := collaboration.FinishTaskAnalysis(ctx, tx, projectID, batchID, "cancelled", "项目已归档，分析已取消。", now); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `UPDATE discussion_batches SET state='cancelled',version=version+1,updated_at=now() WHERE id=$1`, batchID)
			return err
		}
		if outcome.State == "succeeded" {
			if _, err := tx.Exec(ctx, `UPDATE agent_sessions SET last_consumed_seq=GREATEST(last_consumed_seq,COALESCE((SELECT max(covered_seq) FROM context_checkpoints WHERE run_id=$2),0)) WHERE id=$1`, sessionID, runID); err != nil {
				return err
			}
		}
		var activityTask *uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT task_id FROM discussion_batches WHERE id=$1 AND project_id=$2`, batchID, projectID).Scan(&activityTask); err != nil {
			return err
		}
		if activityTask != nil {
			if err := collaboration.FinishTaskAnalysis(ctx, tx, projectID, batchID, batchState, content, now); err != nil {
				return err
			}
			if topicID == uuid.Nil {
				_, err := tx.Exec(ctx, `UPDATE discussion_batches SET state=$2,version=version+1,updated_at=now() WHERE id=$1`, batchID, batchState)
				return err
			}
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
			UPDATE discussion_batches SET state=$2, version=version+1, updated_at=$3 WHERE id=$1`,
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
func (e *Executor) ensureSession(ctx context.Context, projectID, topicID, identityID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := e.Pool.QueryRow(ctx, `INSERT INTO agent_sessions(project_id,id,topic_id,identity_id,created_at) VALUES($1,$2,$3,$4,$5)
 ON CONFLICT(topic_id,identity_id) DO UPDATE SET identity_id=EXCLUDED.identity_id RETURNING id`, projectID, uuid.New(), topicID, identityID, e.now()).Scan(&id)
	return id, err
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
		"tokensUsed": o.TokensUsed, "usageKnown": o.UsageKnown, "reason": o.Summary,
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
