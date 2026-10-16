package integrationtest_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/agent/batch"
	"github.com/kakj-go/Judex/internal/collaboration"
	"github.com/kakj-go/Judex/internal/discussion"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

// Controlled contract test: real PostgreSQL, deterministic provider. This is
// separate from the real-host GLM/Kimi scene evidence.
func TestProjectSubmissionAnalysisLifecycle(t *testing.T) {
	d, projects, ids, pool := newDiscussionEnv(t)
	ctx := context.Background()
	user, _, err := ids.Register(ctx, "Project analyst", "project@analysis.test", "analysis-project-123", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	project, err := projects.Create(ctx, user.ID, projectRequest("Empty project"))
	if err != nil {
		t.Fatal(err)
	}
	var initialTopics int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM topics WHERE project_id=$1`, project.ID).Scan(&initialTopics); err != nil {
		t.Fatal(err)
	}
	input := discussion.Submission{ClientSubmissionID: uuid.NewString(), Purpose: "message", Source: "cli", Text: "EMPTY_PROJECT create a workflow draft, keep formal work for human approval"}
	source, err := d.CreateSubmission(ctx, user.ID, project.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	service := collaboration.NewService(pool)
	queued, err := service.SubmissionAnalysis(ctx, user.ID, project.ID, source.ID)
	if err != nil || queued.TaskID != nil || queued.ScopeKind != "project" || queued.BatchID == nil {
		t.Fatalf("queued: %+v %v", queued, err)
	}
	repeated, err := d.CreateSubmission(ctx, user.ID, project.ID, input)
	if err != nil || repeated.ID != source.ID {
		t.Fatalf("duplicate source %+v %v", repeated, err)
	}
	provider := &taskAnalysisProvider{}
	executor := batch.Executor{Pool: pool.Pool, Provider: provider, ModelName: "controlled-project-test"}
	if err = executor.ExecuteBatch(ctx, project.ID, *queued.BatchID); err != nil {
		t.Fatal(err)
	}
	finished, err := service.SubmissionAnalysis(ctx, user.ID, project.ID, source.ID)
	if err != nil || finished.State != "completed" || finished.ScopeKind != "project" || !strings.Contains(string(finished.Output), "summary") {
		t.Fatalf("finished: %+v %v", finished, err)
	}
	for _, prompt := range provider.prompts {
		if !strings.Contains(prompt, "EMPTY_PROJECT") {
			t.Fatal("source missing from model context")
		}
	}
	var analyses, sessions, tasks, topics int
	err = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM task_analyses WHERE project_id=$1),(SELECT count(*) FROM agent_sessions WHERE project_id=$1),(SELECT count(*) FROM tasks WHERE project_id=$1),(SELECT count(*) FROM topics WHERE project_id=$1)`, project.ID).Scan(&analyses, &sessions, &tasks, &topics)
	if err != nil || analyses != 1 || sessions != 1 || tasks != 0 || topics != initialTopics {
		t.Fatalf("unexpected objects analyses=%d sessions=%d tasks=%d topics=%d err=%v", analyses, sessions, tasks, topics, err)
	}
	other, err := projects.Create(ctx, user.ID, projectRequest("Other project"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.SubmissionAnalysis(ctx, user.ID, other.ID, source.ID); !apierrors.IsCode(err, apierrors.NotFound) {
		t.Fatalf("cross-project analysis: %v", err)
	}
}
