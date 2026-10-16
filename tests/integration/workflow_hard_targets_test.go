package integrationtest_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/project"
	"github.com/kakj-go/Judex/internal/work"
	"github.com/kakj-go/Judex/internal/workflow"
)

func TestWorkflowHardTargetsRejectPlaceholdersAndForeignObjectsAtomically(t *testing.T) {
	svc, projects, ids, pool := newWorkflowService(t)
	ctx := context.Background()
	u, _, err := ids.Register(ctx, "Hard target owner", "hard-target@workflow.test", "password-hard-target-123", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	p, err := projects.Create(ctx, u.ID, project.CreateRequest{Title: "Real hard targets"})
	if err != nil {
		t.Fatal(err)
	}
	body := validBody()
	for _, target := range []string{"", uuid.Nil.String(), "future-material", uuid.NewString()} {
		body.HardRules = []workflow.HardRule{{NodeID: "dev", Kind: "material_ready", Phase: "start", TargetID: target}}
		if _, err = svc.Create(ctx, u.ID, p.ID, body); !apierrors.IsCode(err, apierrors.Validation) {
			t.Fatalf("invalid target %q accepted: %v", target, err)
		}
	}
	var count int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM workflow_definitions WHERE project_id=$1", p.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("invalid draft persisted: %d %v", count, err)
	}
	works := work.NewService(pool, nil)
	pre, err := works.CreateTaskDraft(ctx, u.ID, p.ID, work.TaskDraft{Title: "Actual predecessor"})
	if err != nil {
		t.Fatal(err)
	}
	body.HardRules = []workflow.HardRule{{NodeID: "dev", Kind: "task_acceptance", Phase: "start", TargetID: pre.ID.String()}}
	def, err := svc.Create(ctx, u.ID, p.ID, body)
	if err != nil {
		t.Fatal(err)
	}
	first, err := svc.Publish(ctx, u.ID, p.ID, def.ID, def.Version, "")
	if err != nil {
		t.Fatal(err)
	}
	other, err := projects.Create(ctx, u.ID, project.CreateRequest{Title: "Other project"})
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := works.CreateTaskDraft(ctx, u.ID, other.ID, work.TaskDraft{Title: "Foreign predecessor"})
	if err != nil {
		t.Fatal(err)
	}
	body.HardRules[0].TargetID = foreign.ID.String()
	if _, err = svc.UpdateDraft(ctx, u.ID, p.ID, def.ID, 2, body); !apierrors.IsCode(err, apierrors.Validation) {
		t.Fatalf("foreign target accepted: %v", err)
	}
	body.HardRules[0].TargetID = pre.ID.String()
	if _, err = svc.UpdateDraft(ctx, u.ID, p.ID, def.ID, 2, body); err != nil {
		t.Fatal(err)
	}
	// Simulate an already stored legacy draft: publication must revalidate it.
	if _, err = pool.Exec(ctx, `UPDATE workflow_versions SET hard_rules_json='[{"kind":"material_ready","phase":"start"}]'::jsonb WHERE definition_id=$1 AND state='draft'`, def.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Publish(ctx, u.ID, p.ID, def.ID, 3, ""); !apierrors.IsCode(err, apierrors.Validation) {
		t.Fatalf("legacy placeholder published: %v", err)
	}
	var version int64
	var published uuid.UUID
	if err = pool.QueryRow(ctx, "SELECT version,published_version_id FROM workflow_definitions WHERE id=$1", def.ID).Scan(&version, &published); err != nil || version != 3 || published != first.ID {
		t.Fatalf("failed publish mutated head: %d %s %v", version, published, err)
	}
	versions, err := svc.ListVersions(ctx, u.ID, p.ID, def.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range versions {
		if v.ID == first.ID && (v.State != "published" || v.Body.HardRules[0].TargetID != pre.ID.String()) {
			t.Fatal("published original changed")
		}
	}
}
