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

func TestPublishedWorkflowReadScopesAndHistoricalBody(t *testing.T) {
	d, projects, ids, pool := newDiscussionEnv(t)
	ctx := context.Background()
	owner, _, err := ids.Register(ctx, "Workflow reader", "published-read@test.local", "published-read-password", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	p, err := projects.Create(ctx, owner.ID, projectRequest("Fixed version read"))
	if err != nil {
		t.Fatal(err)
	}
	other, err := projects.Create(ctx, owner.ID, projectRequest("Other project"))
	if err != nil {
		t.Fatal(err)
	}
	svc := workflow.NewService(pool, nil)
	def, err := svc.Create(ctx, owner.ID, p.ID, validBody())
	if err != nil {
		t.Fatal(err)
	}
	drafts, err := svc.ListVersions(ctx, owner.ID, p.ID, def.ID, true)
	if err != nil || len(drafts) != 1 {
		t.Fatalf("drafts: %v %v", drafts, err)
	}
	if _, err = svc.ReadPublished(ctx, owner.ID, p.ID, def.ID, drafts[0].ID); !apierrors.IsCode(err, apierrors.NotFound) {
		t.Fatalf("draft exposed: %v", err)
	}
	first, err := svc.Publish(ctx, owner.ID, p.ID, def.ID, def.Version, "")
	if err != nil {
		t.Fatal(err)
	}
	old, err := svc.ReadPublished(ctx, owner.ID, p.ID, def.ID, first.ID)
	if err != nil || !old.IsCurrent || old.Body.Instructions != validBody().Instructions || len(old.BodySHA256) != 64 {
		t.Fatalf("fixed body: %+v %v", old, err)
	}
	for _, scope := range []struct{ user, project, definition, version uuid.UUID }{
		{uuid.New(), p.ID, def.ID, first.ID}, {owner.ID, other.ID, def.ID, first.ID}, {owner.ID, p.ID, uuid.New(), first.ID}, {owner.ID, p.ID, def.ID, uuid.New()},
	} {
		if _, err = svc.ReadPublished(ctx, scope.user, scope.project, scope.definition, scope.version); !apierrors.IsCode(err, apierrors.NotFound) {
			t.Fatalf("scope leaked: %v", err)
		}
	}
	listed, err := svc.List(ctx, owner.ID, p.ID)
	if err != nil || len(listed) != 1 {
		t.Fatal(err)
	}
	changed := validBody()
	changed.Instructions = "New published instructions, preserve old evidence"
	_, err = svc.UpdateDraft(ctx, owner.ID, p.ID, def.ID, listed[0].Version, changed)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Publish(ctx, owner.ID, p.ID, def.ID, listed[0].Version+1, "")
	if err != nil {
		t.Fatal(err)
	}
	oldAgain, err := svc.ReadPublished(ctx, owner.ID, p.ID, def.ID, first.ID)
	if err != nil || oldAgain.IsCurrent || oldAgain.BodySHA256 != old.BodySHA256 || oldAgain.Body.Instructions != old.Body.Instructions {
		t.Fatalf("history changed: %+v %v", oldAgain, err)
	}
	current, err := svc.ReadPublished(ctx, owner.ID, p.ID, def.ID, second.ID)
	if err != nil || !current.IsCurrent || current.BodySHA256 == old.BodySHA256 || current.Body.Instructions != changed.Instructions {
		t.Fatalf("current body: %+v %v", current, err)
	}
	source, err := d.CreateSubmission(ctx, owner.ID, p.ID, discussion.Submission{ClientSubmissionID: uuid.NewString(), Purpose: "message", Source: "cli", Text: "Read published workflow"})
	if err != nil {
		t.Fatal(err)
	}
	analysis, err := collaboration.NewService(pool).SubmissionAnalysis(ctx, owner.ID, p.ID, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	var run uuid.UUID
	if err = pool.QueryRow(ctx, `UPDATE agent_runs SET state='running',lease_token='read-fixed' WHERE batch_id=$1 RETURNING id`, analysis.BatchID).Scan(&run); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.ReadPublishedForRun(ctx, p.ID, run, "stale", def.ID, second.ID); !apierrors.IsCode(err, apierrors.Forbidden) {
		t.Fatalf("stale lease read: %v", err)
	}
	if _, err = svc.ReadPublishedForRun(ctx, p.ID, run, "read-fixed", def.ID, second.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE project_members SET state='removed' WHERE project_id=$1 AND user_id=$2`, p.ID, owner.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.ReadPublishedForRun(ctx, p.ID, run, "read-fixed", def.ID, second.ID); !apierrors.IsCode(err, apierrors.Forbidden) {
		t.Fatalf("removed member read: %v", err)
	}
}
