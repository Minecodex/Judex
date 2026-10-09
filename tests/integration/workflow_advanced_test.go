package integrationtest_test

import (
	"context"
	"github.com/kakj-go/Judex/internal/project"
	"github.com/kakj-go/Judex/internal/project/catalog"
	"github.com/kakj-go/Judex/internal/workflow"
	"testing"
)

func TestAdvancedWorkflowPresetLifecycle(t *testing.T) {
	svc, projects, identity, _ := newWorkflowService(t)
	ctx := context.Background()
	owner, _, err := identity.Register(ctx, "Advanced owner", "advanced@workflow.test", "password-advanced-123", "10.9.2.1")
	if err != nil {
		t.Fatal(err)
	}
	p, err := projects.Create(ctx, owner.ID, project.CreateRequest{Title: "Advanced workflows"})
	if err != nil {
		t.Fatal(err)
	}
	imported, err := svc.ImportPresets(ctx, owner.ID, p.ID, workflow.ImportPresetsRequest{CatalogVersion: catalog.Workflows().Version, ScenarioID: "software", WorkflowIDs: []string{"software-delivery-advanced"}, Locale: "en"})
	if err != nil || len(imported.Items) != 1 {
		t.Fatal(imported, err)
	}
	head := imported.Items[0]
	versions, err := svc.ListVersions(ctx, owner.ID, p.ID, head.ID, true)
	if err != nil || len(versions) != 1 {
		t.Fatal(versions, err)
	}
	body := *versions[0].Body
	oldReviewHash := versions[0].DraftHash
	if len(body.Nodes) < 18 || body.Nodes[0].Phase == "" {
		t.Fatal("stages lost")
	}
	returns := 0
	for _, edge := range body.AdvisoryEdges {
		if edge.Kind == "feedback" {
			returns++
			if edge.Label == "" {
				t.Fatal("condition lost")
			}
		}
	}
	if returns < 5 {
		t.Fatal("returns lost")
	}
	authority := workflow.ConstraintHash(body)
	body.Nodes[0].Phase = "Project-specific grouping"
	if workflow.ConstraintHash(body) != authority {
		t.Fatal("display metadata changed authority")
	}
	if _, err = svc.UpdateDraft(ctx, owner.ID, p.ID, head.ID, 1, body); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Publish(ctx, owner.ID, p.ID, head.ID, 2, oldReviewHash); err == nil {
		t.Fatal("stale graph review accepted")
	}
	versions, _ = svc.ListVersions(ctx, owner.ID, p.ID, head.ID, true)
	if versions[0].DraftHash == oldReviewHash || versions[0].Body.Nodes[0].Phase != body.Nodes[0].Phase {
		t.Fatal("diagram review or stage not preserved")
	}
	if _, err = svc.Publish(ctx, owner.ID, p.ID, head.ID, 2, versions[0].DraftHash); err != nil {
		t.Fatal(err)
	}
	published, _ := svc.ListVersions(ctx, owner.ID, p.ID, head.ID, false)
	if len(published) != 1 || published[0].Body.Nodes[0].Phase != body.Nodes[0].Phase {
		t.Fatal("published annotations lost")
	}
	if len(published[0].Body.AdvisoryEdges) != len(body.AdvisoryEdges) {
		t.Fatal("published connections lost")
	}
}
