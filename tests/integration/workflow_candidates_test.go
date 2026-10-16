package integrationtest_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/collaboration"
	"github.com/kakj-go/Judex/internal/discussion"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/work"
	"github.com/kakj-go/Judex/internal/workflow"
)

func TestWorkflowCandidatesAndRoutingKeepFormalAssignment(t *testing.T) {
	d, projects, ids, pool := newDiscussionEnv(t)
	ctx := context.Background()
	owner, _, err := ids.Register(ctx, "Routing owner", "routing@workflow.test", "workflow-routing-123", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	project, err := projects.Create(ctx, owner.ID, projectRequest("Routing contract"))
	if err != nil {
		t.Fatal(err)
	}
	service := workflow.NewService(pool, nil)
	first, err := service.Create(ctx, owner.ID, project.ID, validBody())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Publish(ctx, owner.ID, project.ID, first.ID, first.Version, ""); err != nil {
		t.Fatal(err)
	}
	second, err := service.Create(ctx, owner.ID, project.ID, validBody())
	if err != nil {
		t.Fatal(err)
	}
	version, err := service.Publish(ctx, owner.ID, project.ID, second.ID, second.Version, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Create(ctx, owner.ID, project.ID, validBody()); err != nil {
		t.Fatal(err)
	} // draft must stay out
	node := "dev"
	workService := work.NewService(pool, nil)
	task, err := workService.CreateTaskDraft(ctx, owner.ID, project.ID, work.TaskDraft{Title: "Bound task", WorkflowID: &first.ID, NodeID: &node})
	if err != nil {
		t.Fatal(err)
	}
	read, err := workService.GetTask(ctx, owner.ID, project.ID, task.ID)
	if err != nil || len(read.WorkflowCandidates) != 2 || !read.WorkflowCandidates[0].FormalTaskBinding || read.WorkflowCandidates[0].WorkflowID != first.ID {
		t.Fatalf("candidates=%+v err=%v", read.WorkflowCandidates, err)
	}
	source, err := d.CreateSubmission(ctx, owner.ID, project.ID, discussion.Submission{ClientSubmissionID: uuid.NewString(), Purpose: "message", Source: "cli", Text: "Conflicting candidate requires clarification", TaskID: &task.ID})
	if err != nil {
		t.Fatal(err)
	}
	analysis, err := collaboration.NewService(pool).SubmissionAnalysis(ctx, owner.ID, project.ID, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	var run uuid.UUID
	if err = pool.QueryRow(ctx, `UPDATE agent_runs SET state='running',lease_token='routing-test' WHERE batch_id=$1 RETURNING id`, analysis.BatchID).Scan(&run); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	output := collaboration.TaskAnalysisOutput{Summary: "Alternative workflow needs human clarification", CurrentNodeCandidates: []collaboration.WorkflowNodeRef{{WorkflowID: second.ID, VersionID: version.ID, NodeID: "dev", Reason: "Source describes this workflow"}}}
	if _, err = collaboration.RecordTaskOutput(ctx, tx, project.ID, run, output, time.Now()); !apierrors.IsCode(err, apierrors.Validation) {
		t.Fatalf("silent conflict accepted: %v", err)
	}
	output.NeedsClarification = true
	if _, err = collaboration.RecordTaskOutput(ctx, tx, project.ID, run, output, time.Now()); !apierrors.IsCode(err, apierrors.Validation) {
		t.Fatalf("clarification allowed replacing the formal current node: %v", err)
	}
	output.SuggestedNextNodes = output.CurrentNodeCandidates
	output.CurrentNodeCandidates = []collaboration.WorkflowNodeRef{{WorkflowID: first.ID, VersionID: read.WorkflowCandidates[0].VersionID, NodeID: node, Reason: "The submitted task's immutable formal binding"}}
	if _, err = collaboration.RecordTaskOutput(ctx, tx, project.ID, run, output, time.Now()); err != nil {
		t.Fatal(err)
	}
	output.SuggestedNextNodes[0].NodeID = "not-a-node"
	if _, err = collaboration.RecordTaskOutput(ctx, tx, project.ID, run, output, time.Now()); !apierrors.IsCode(err, apierrors.Validation) {
		t.Fatalf("nonexistent node accepted: %v", err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	read, err = workService.GetTask(ctx, owner.ID, project.ID, task.ID)
	if err != nil || read.WorkflowID == nil || *read.WorkflowID != first.ID || read.Version != task.Version {
		t.Fatalf("analysis mutated formal work: %+v %v", read, err)
	}
}
