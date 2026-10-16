package integrationtest_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/collaboration"
	"github.com/kakj-go/Judex/internal/discussion"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/workflow"
)

func TestWorkflowAgentDraftAtomicityAndAuthority(t *testing.T) {
	d, projects, ids, pool := newDiscussionEnv(t)
	ctx := context.Background()
	owner, _, err := ids.Register(ctx, "Workflow owner", "agent@workflow.test", "agent-workflow-123", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	project, err := projects.Create(ctx, owner.ID, projectRequest("Model draft contract"))
	if err != nil {
		t.Fatal(err)
	}
	source, err := d.CreateSubmission(ctx, owner.ID, project.ID, discussion.Submission{ClientSubmissionID: uuid.NewString(), Purpose: "message", Source: "cli", Text: "create a workflow"})
	if err != nil {
		t.Fatal(err)
	}
	analysis, err := collaboration.NewService(pool).SubmissionAnalysis(ctx, owner.ID, project.ID, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	var run uuid.UUID
	if err = pool.QueryRow(ctx, `UPDATE agent_runs SET state='running',lease_token='draft-contract' WHERE batch_id=$1 RETURNING id`, analysis.BatchID).Scan(&run); err != nil {
		t.Fatal(err)
	}
	service := workflow.NewService(pool, nil)
	tx, err := pool.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = service.DraftForRun(ctx, tx, project.ID, run, "stale-worker", nil, 0, validBody()); !apierrors.IsCode(err, apierrors.Forbidden) {
		t.Fatalf("stale lease: %v", err)
	}
	missing := uuid.New()
	if _, err = service.DraftForRun(ctx, tx, project.ID, run, "draft-contract", &missing, 1, validBody()); !apierrors.IsCode(err, apierrors.NotFound) {
		t.Fatalf("invented revision ID must be not found, not a database error: %v", err)
	}
	// A bad target must not poison the shared effect transaction or create a
	// partial draft. The next genuine creation must still work exactly once.
	first, err := service.DraftForRun(ctx, tx, project.ID, run, "draft-contract", nil, 0, validBody())
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := service.DraftForRun(ctx, tx, project.ID, run, "draft-contract", nil, 0, validBody())
	if err != nil || repeated["workflowId"] != first["workflowId"] || repeated["deduplicated"] != true {
		t.Fatalf("duplicate: %+v %v", repeated, err)
	}
	id := first["workflowId"].(uuid.UUID)
	var definitions, audits int
	err = tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM workflow_definitions WHERE project_id=$1),(SELECT count(*) FROM audit_events WHERE project_id=$1 AND actor_type='agent' AND source='agent')`, project.ID).Scan(&definitions, &audits)
	if err != nil || definitions != 1 || audits != 1 {
		t.Fatalf("draft/audit counts %d %d %v", definitions, audits, err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM workflow_definitions WHERE id=$1`, id).Scan(&count); err != nil || count != 0 {
		t.Fatalf("draft escaped effect transaction %d %v", count, err)
	}
	tx, err = pool.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	first, err = service.DraftForRun(ctx, tx, project.ID, run, "draft-contract", nil, 0, validBody())
	if err != nil {
		t.Fatal(err)
	}
	changed := validBody()
	changed.Instructions = "Require evidence before review"
	revision, err := service.DraftForRun(ctx, tx, project.ID, run, "draft-contract", &id, first["version"].(int64), changed)
	if err != nil || revision["draftHash"] == first["draftHash"] {
		t.Fatalf("revision %+v %v", revision, err)
	}
	if _, err = service.DraftForRun(ctx, tx, project.ID, run, "draft-contract", &id, first["version"].(int64), changed); !apierrors.IsCode(err, apierrors.VersionConflict) {
		t.Fatalf("stale revision: %v", err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	definitionsList, err := service.List(ctx, owner.ID, project.ID)
	if err != nil || len(definitionsList) != 1 {
		t.Fatalf("definitions %+v %v", definitionsList, err)
	}
	definition := definitionsList[0]
	if err != nil || definition.PublishedVersionID != nil || !definition.HasDraft {
		t.Fatalf("model published: %+v %v", definition, err)
	}
	if _, err = service.Publish(ctx, owner.ID, project.ID, id, definition.Version, revision["draftHash"].(string)); err != nil {
		t.Fatal(err)
	}
}
