package integrationtest_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/agent/batch"
	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/collaboration"
	"github.com/kakj-go/Judex/internal/discussion"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

func TestRunCancellationConvergesAnalysis(t *testing.T) {
	for _, state := range []string{"queued", "running", "waiting_human"} {
		t.Run(state, func(t *testing.T) {
			discussions, projects, identities, pool := newDiscussionEnv(t)
			ctx := audit.WithSource(context.Background(), audit.SourceCLI)
			user, _, err := identities.Register(ctx, "Cancellation", state+"@cancel.test", "cancel-password-123", "127.0.0.1")
			if err != nil {
				t.Fatal(err)
			}
			project, err := projects.Create(ctx, user.ID, projectRequest("Cancellation"))
			if err != nil {
				t.Fatal(err)
			}
			source, err := discussions.CreateSubmission(ctx, user.ID, project.ID, discussion.Submission{ClientSubmissionID: uuid.NewString(), Purpose: "message", Source: "cli", Text: "Cancellation fixture"})
			if err != nil {
				t.Fatal(err)
			}
			analyses := collaboration.NewService(pool)
			original, err := analyses.SubmissionAnalysis(ctx, user.ID, project.ID, source.ID)
			if err != nil || original.BatchID == nil {
				t.Fatalf("original analysis: %+v %v", original, err)
			}
			provider := &taskAnalysisProvider{}
			executor := batch.Executor{Pool: pool.Pool, Provider: provider, ModelName: "controlled-cancellation"}
			if err = executor.ExecuteBatch(ctx, project.ID, *original.BatchID); err != nil {
				t.Fatal(err)
			}
			var runID uuid.UUID
			if err = pool.QueryRow(ctx, `SELECT id FROM agent_runs WHERE batch_id=$1 AND parent_run_id IS NULL`, original.BatchID).Scan(&runID); err != nil {
				t.Fatal(err)
			}
			// Recreate each interrupted/waiting checkpoint in this disposable PG
			// fixture. Business acceptance is never seeded by this contract test.
			for _, table := range []string{"agent_runs", "discussion_batches", "task_analyses"} {
				if _, err = pool.Exec(ctx, "UPDATE "+table+" SET state=$2 WHERE "+map[string]string{"agent_runs": "id", "discussion_batches": "id", "task_analyses": "batch_id"}[table]+"=$1", map[string]uuid.UUID{"agent_runs": runID, "discussion_batches": *original.BatchID, "task_analyses": *original.BatchID}[table], state); err != nil {
					t.Fatal(err)
				}
			}
			service := batch.Service{Pool: pool}
			before, err := service.Get(ctx, user.ID, project.ID, runID)
			if err != nil {
				t.Fatal(err)
			}
			callsBefore := provider.calls.Load()
			if callsBefore == 0 {
				t.Fatal("cancellation fixture did not execute its model")
			}
			var attempts, tokens int
			if err = pool.QueryRow(ctx, `SELECT model_attempts,used_tokens FROM discussion_batches WHERE id=$1`, original.BatchID).Scan(&attempts, &tokens); err != nil {
				t.Fatal(err)
			}
			if err = service.Cancel(ctx, user.ID, project.ID, runID, before.Version, "原件已在完整交付报告中，停止重复分析"); err != nil {
				t.Fatal(err)
			}
			after, err := service.Get(ctx, user.ID, project.ID, runID)
			if err != nil || after.State != "cancelled" || after.Version != before.Version+1 {
				t.Fatalf("run cancellation: %+v %v", after, err)
			}
			result, err := analyses.SubmissionAnalysis(ctx, user.ID, project.ID, source.ID)
			if err != nil || result.State != "cancelled" || result.ID != original.ID || result.BatchID == nil || *result.BatchID != *original.BatchID || !strings.Contains(result.Summary, "停止重复分析") {
				t.Fatalf("analysis did not converge: %+v %v", result, err)
			}
			// Old deployments could persist cancelled batch/run with queued
			// analysis. Repeating the current-version cancellation repairs only
			// that projection, without another run version or model invocation.
			if _, err = pool.Exec(ctx, `UPDATE task_analyses SET state='queued' WHERE id=$1`, original.ID); err != nil {
				t.Fatal(err)
			}
			if err = service.Cancel(ctx, user.ID, project.ID, runID, after.Version, "核对原取消状态，停止重复分析"); err != nil {
				t.Fatal(err)
			}
			repaired, err := analyses.SubmissionAnalysis(ctx, user.ID, project.ID, source.ID)
			if err != nil || repaired.State != "cancelled" {
				t.Fatalf("legacy cancellation projection: %+v %v", repaired, err)
			}
			current, err := service.Get(ctx, user.ID, project.ID, runID)
			if err != nil || current.Version != after.Version {
				t.Fatalf("idempotent cancellation changed version: %+v %v", current, err)
			}
			if err = service.Cancel(ctx, user.ID, project.ID, runID, before.Version, "stale cancellation"); !apierrors.IsCode(err, apierrors.VersionConflict) {
				t.Fatalf("stale cancellation: %v", err)
			}
			var cancellationAudits, repairAudits int
			if err = pool.QueryRow(ctx, `SELECT count(*) FILTER(WHERE operation='agent.run.cancel'),count(*) FILTER(WHERE operation='agent.run.cancel_reconcile') FROM audit_events WHERE project_id=$1 AND actor_user_id=$2 AND source='cli'`, project.ID, user.ID).Scan(&cancellationAudits, &repairAudits); err != nil || cancellationAudits != 1 || repairAudits != 1 {
				t.Fatalf("cancellation/repair actor channel audit: %d/%d %v", cancellationAudits, repairAudits, err)
			}
			var currentAttempts, currentTokens, sources, tasks int
			if err = pool.QueryRow(ctx, `SELECT model_attempts,used_tokens,(SELECT count(*) FROM submissions WHERE project_id=$2),(SELECT count(*) FROM tasks WHERE project_id=$2) FROM discussion_batches WHERE id=$1`, original.BatchID, project.ID).Scan(&currentAttempts, &currentTokens, &sources, &tasks); err != nil || currentAttempts != attempts || currentTokens != tokens || sources != 1 || tasks != 0 || provider.calls.Load() != callsBefore {
				t.Fatalf("cancellation changed evidence/budget: %d %d %d %d calls=%d err=%v", currentAttempts, currentTokens, sources, tasks, provider.calls.Load(), err)
			}
		})
	}
}

func TestLateChildCancellationPreservesCompletedAnalysis(t *testing.T) {
	discussions, projects, identities, pool := newDiscussionEnv(t)
	ctx := context.Background()
	user, _, err := identities.Register(ctx, "Late child", "late-child@cancel.test", "cancel-password-123", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	project, err := projects.Create(ctx, user.ID, projectRequest("Late child"))
	if err != nil {
		t.Fatal(err)
	}
	source, err := discussions.CreateSubmission(ctx, user.ID, project.ID, discussion.Submission{ClientSubmissionID: uuid.NewString(), Purpose: "message", Source: "cli", Text: "Late child fixture"})
	if err != nil {
		t.Fatal(err)
	}
	analyses := collaboration.NewService(pool)
	analysis, err := analyses.SubmissionAnalysis(ctx, user.ID, project.ID, source.ID)
	if err != nil || analysis.BatchID == nil {
		t.Fatalf("analysis: %+v %v", analysis, err)
	}
	provider := &taskAnalysisProvider{}
	executor := batch.Executor{Pool: pool.Pool, Provider: provider, ModelName: "controlled-late-child"}
	if err = executor.ExecuteBatch(ctx, project.ID, *analysis.BatchID); err != nil {
		t.Fatal(err)
	}
	finished, err := analyses.SubmissionAnalysis(ctx, user.ID, project.ID, source.ID)
	if err != nil || finished.State != "completed" {
		t.Fatalf("finished: %+v %v", finished, err)
	}
	child := uuid.New()
	if _, err = pool.Exec(ctx, `INSERT INTO agent_runs(project_id,id,session_id,batch_id,identity_id,parent_run_id,state,requested_by,created_at,updated_at) SELECT project_id,$2,session_id,batch_id,identity_id,id,'running',$3,now(),now() FROM agent_runs WHERE batch_id=$1 AND parent_run_id IS NULL`, analysis.BatchID, child, user.ID); err != nil {
		t.Fatal(err)
	}
	service := batch.Service{Pool: pool}
	run, err := service.Get(ctx, user.ID, project.ID, child)
	if err != nil {
		t.Fatal(err)
	}
	if err = service.Cancel(ctx, user.ID, project.ID, child, run.Version, "停止迟到的子运行，保留已完成分析"); err != nil {
		t.Fatal(err)
	}
	current, err := analyses.SubmissionAnalysis(ctx, user.ID, project.ID, source.ID)
	if err != nil || current.State != "completed" || current.Summary != finished.Summary || string(current.Output) != string(finished.Output) {
		t.Fatalf("late child rewrote analysis: %+v %v", current, err)
	}
	var batchState string
	if err = pool.QueryRow(ctx, `SELECT state FROM discussion_batches WHERE id=$1`, analysis.BatchID).Scan(&batchState); err != nil || batchState != "completed" {
		t.Fatalf("late child rewrote batch: %s %v", batchState, err)
	}
}
