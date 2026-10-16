package integrationtest_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/agent/batch"
	"github.com/kakj-go/Judex/internal/collaboration"
	"github.com/kakj-go/Judex/internal/discussion"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/work"
	"github.com/kakj-go/Judex/internal/workflow"
)

func TestPlanOnlyAnalysisRejectsUnrelatedAndCompletedCurrentNodes(t *testing.T) {
	d, projects, ids, pool := newDiscussionEnv(t)
	ctx := context.Background()
	owner, _, err := ids.Register(ctx, "Plan scope", "plan-scope@test.local", "plan-scope-password", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	p, err := projects.Create(ctx, owner.ID, projectRequest("Scoped routing"))
	if err != nil {
		t.Fatal(err)
	}
	wf := workflow.NewService(pool, nil)
	first, err := wf.Create(ctx, owner.ID, p.ID, validBody())
	if err != nil {
		t.Fatal(err)
	}
	firstVersion, err := wf.Publish(ctx, owner.ID, p.ID, first.ID, first.Version, "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := wf.Create(ctx, owner.ID, p.ID, validBody())
	if err != nil {
		t.Fatal(err)
	}
	secondVersion, err := wf.Publish(ctx, owner.ID, p.ID, second.ID, second.Version, "")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := work.NewService(pool, nil).CreatePlanDraft(ctx, owner.ID, p.ID, "Only this plan", "Goal", "Criteria", nil, &first.ID)
	if err != nil {
		t.Fatal(err)
	}
	source, err := d.CreateSubmission(ctx, owner.ID, p.ID, discussion.Submission{ClientSubmissionID: uuid.NewString(), Purpose: "message", DiscussionIntent: "question", Source: "cli", Text: "Only this plan; do not draft work", PlanID: &plan.ID})
	if err != nil {
		t.Fatal(err)
	}
	a, err := collaboration.NewService(pool).SubmissionAnalysis(ctx, owner.ID, p.ID, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	var run uuid.UUID
	if err = pool.QueryRow(ctx, `UPDATE agent_runs SET state='running',lease_token='scope-test' WHERE batch_id=$1 RETURNING id`, a.BatchID).Scan(&run); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	ref := collaboration.WorkflowNodeRef{WorkflowID: second.ID, VersionID: secondVersion.ID, NodeID: "dev", Reason: "Another plan has this current node"}
	out := collaboration.TaskAnalysisOutput{Summary: "This source is scoped to one plan.", NeedsClarification: true, CurrentNodeCandidates: []collaboration.WorkflowNodeRef{ref}}
	if _, err = collaboration.RecordTaskOutput(ctx, tx, p.ID, run, out, time.Now()); !apierrors.IsCode(err, apierrors.Validation) {
		t.Fatalf("clarification bypassed plan scope: %v", err)
	}
	out.CurrentNodeCandidates[0] = collaboration.WorkflowNodeRef{WorkflowID: first.ID, VersionID: firstVersion.ID, NodeID: "dev", Reason: "This plan's workflow"}
	if _, err = collaboration.RecordTaskOutput(ctx, tx, p.ID, run, out, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE plans SET status='accepted' WHERE id=$1`, plan.ID); err != nil {
		t.Fatal(err)
	} // controlled historical fixture
	if _, err = collaboration.RecordTaskOutput(ctx, tx, p.ID, run, out, time.Now()); !apierrors.IsCode(err, apierrors.Validation) {
		t.Fatalf("completed plan has a current node: %v", err)
	}
	out.CurrentNodeCandidates = nil
	out.RelatedEvidenceNodes = []collaboration.WorkflowNodeRef{ref}
	if _, err = collaboration.RecordTaskOutput(ctx, tx, p.ID, run, out, time.Now()); err != nil {
		t.Fatalf("historical evidence was incorrectly blocked: %v", err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestPlanSubmissionContextAndPagingPreserveOwnedVersusReferencedWork(t *testing.T) {
	d, projects, ids, pool := newDiscussionEnv(t)
	ctx := context.Background()
	owner, _, err := ids.Register(ctx, "Scope context", "scope-context@test.local", "scope-context-password", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	p, err := projects.Create(ctx, owner.ID, projectRequest("Context scope"))
	if err != nil {
		t.Fatal(err)
	}
	w := work.NewService(pool, nil)
	plan, err := w.CreatePlanDraft(ctx, owner.ID, p.ID, "REQUESTED_PLAN", "requested goal", "criteria", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	other, err := w.CreatePlanDraft(ctx, owner.ID, p.ID, "UNRELATED_PLAN", "unrelated goal", "criteria", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	owned, err := w.CreateTaskDraft(ctx, owner.ID, p.ID, work.TaskDraft{Title: "OWNED_TASK", PlanID: &plan.ID})
	if err != nil {
		t.Fatal(err)
	}
	reference, err := w.CreateTaskDraft(ctx, owner.ID, p.ID, work.TaskDraft{Title: "REFERENCE_TASK", PlanID: &other.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.CreateTaskDraft(ctx, owner.ID, p.ID, work.TaskDraft{Title: "UNRELATED_TASK", PlanID: &other.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO plan_task_references(project_id,plan_id,task_id) VALUES($1,$2,$3)`, p.ID, plan.ID, reference.ID); err != nil {
		t.Fatal(err)
	}
	source, err := d.CreateSubmission(ctx, owner.ID, p.ID, discussion.Submission{ClientSubmissionID: uuid.NewString(), Purpose: "message", DiscussionIntent: "question", Source: "cli", Text: "Read only the supplied plan", PlanID: &plan.ID})
	if err != nil {
		t.Fatal(err)
	}
	a, err := collaboration.NewService(pool).SubmissionAnalysis(ctx, owner.ID, p.ID, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	checked := false
	provider := contextProvider(func(_ context.Context, req model.Request) (<-chan model.Event, error) {
		out := make(chan model.Event, 4)
		if len(req.Messages) == 2 {
			contextText := req.Messages[1].Content
			if !strings.Contains(contextText, "REQUESTED_PLAN") || !strings.Contains(contextText, owned.ID.String()) || strings.Contains(contextText, "UNRELATED_TASK") {
				t.Fatal("explicit scope lost in fresh model context")
			}
			out <- model.Event{Type: "toolCallReady", ToolCallID: "query", ToolName: "query_work", ArgsJSON: `{"objectType":"task","limit":20}`}
		} else {
			var result struct {
				Data struct {
					Items []map[string]any `json:"items"`
				} `json:"data"`
			}
			last := req.Messages[len(req.Messages)-1]
			if err := json.Unmarshal([]byte(last.Content), &result); err != nil {
				t.Fatal(err)
			}
			if len(result.Data.Items) != 2 {
				t.Fatalf("plan query leaked or hid work: %+v", result)
			}
			roles := map[string]string{}
			for _, item := range result.Data.Items {
				roles[item["title"].(string)] = item["scopeRole"].(string)
			}
			if roles["OWNED_TASK"] != "owned" || roles["REFERENCE_TASK"] != "reference" {
				t.Fatalf("reference reclassified as owned: %+v", roles)
			}
			checked = true
			out <- model.Event{Type: "toolCallReady", ToolCallID: "record", ToolName: "record_task_analysis", ArgsJSON: `{"summary":"已按明确计划核对本计划与引用工作，引用不转移任务归属。","basis":["controlled plan source"]}`}
		}
		out <- model.Event{Type: "usage", Usage: model.UsageSnapshot{InputTokens: model.TokenCount(100), OutputTokens: model.TokenCount(30)}}
		out <- model.Event{Type: "finish"}
		close(out)
		return out, nil
	})
	e := batch.Executor{Pool: pool.Pool, Provider: provider, ModelName: "controlled-plan-scope"}
	if err = e.ExecuteBatch(ctx, p.ID, *a.BatchID); err != nil {
		t.Fatal(err)
	}
	if !checked {
		t.Fatal("scoped tool result never reached the model")
	}
	finished, err := collaboration.NewService(pool).SubmissionAnalysis(ctx, owner.ID, p.ID, source.ID)
	if err != nil || finished.State != "completed" {
		t.Fatalf("analysis failed: %+v %v", finished, err)
	}
}

func TestPlaceholderIsNotPersistedOrRestoredAsCompletedAnalysis(t *testing.T) {
	d, projects, ids, pool := newDiscussionEnv(t)
	ctx := context.Background()
	owner, _, err := ids.Register(ctx, "Summary guard", "summary-guard@test.local", "summary-guard-password", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	p, err := projects.Create(ctx, owner.ID, projectRequest("Guard"))
	if err != nil {
		t.Fatal(err)
	}
	source, err := d.CreateSubmission(ctx, owner.ID, p.ID, discussion.Submission{ClientSubmissionID: uuid.NewString(), Purpose: "message", Source: "cli", Text: "Summarize actual source"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := collaboration.NewService(pool).SubmissionAnalysis(ctx, owner.ID, p.ID, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	var run uuid.UUID
	if err = pool.QueryRow(ctx, `UPDATE agent_runs SET state='running',lease_token='summary-test' WHERE batch_id=$1 RETURNING id`, a.BatchID).Scan(&run); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = collaboration.RecordTaskOutput(ctx, tx, p.ID, run, collaboration.TaskAnalysisOutput{Summary: "__SLOT_B__"}, time.Now()); !apierrors.IsCode(err, apierrors.Validation) {
		t.Fatalf("placeholder persisted: %v", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE task_analyses SET output_json='{"summary":"__SLOT_B__"}'::jsonb WHERE id=$1`, a.ID); err != nil {
		t.Fatal(err)
	} // interrupted legacy record
	if err = collaboration.FinishTaskAnalysis(ctx, tx, p.ID, *a.BatchID, "completed", "transport complete", time.Now()); err != nil {
		t.Fatal(err)
	}
	var state string
	if err = tx.QueryRow(ctx, `SELECT state FROM task_analyses WHERE id=$1`, a.ID).Scan(&state); err != nil || state != "failed" {
		t.Fatalf("legacy placeholder called completed: %s %v", state, err)
	}
}
