// SPDX-License-Identifier: Apache-2.0
package integrationtest_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/identity"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	"github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/project"
	"github.com/kakj-go/Judex/internal/workflow"
	integration "github.com/kakj-go/Judex/tests/integration"
)

func newWorkflowService(t *testing.T) (*workflow.Service, *project.Service, *identity.Service, *postgres.Pool) {
	t.Helper()
	fixture := integration.StartPG(t)
	limiter := identity.NewRateLimiter(fixture.Pool.Pool, nil)
	ids := identity.NewService(fixture.Pool, limiter, identity.Options{RegisterPerIP: 1000}, nil)
	return workflow.NewService(fixture.Pool, nil), project.NewService(fixture.Pool, nil), ids, fixture.Pool
}

func validBody() workflow.Body {
	return workflow.Body{
		Name:         "交付流程",
		Instructions: "按节点推进",
		Nodes: []workflow.Node{
			{ID: "dev", Name: "开发", Responsibility: "实现", DefaultApprovalPolicy: "all"},
			{ID: "review", Name: "评审", Responsibility: "审查", DefaultApprovalPolicy: "all"},
			{ID: "release", Name: "发布", Responsibility: "上线", DefaultApprovalPolicy: "none"},
		},
		AdvisoryEdges: []workflow.Edge{{From: "dev", To: "review"}, {From: "review", To: "release"}},
		HardRules:     []workflow.HardRule{{Kind: "task_acceptance", Phase: "accept"}},
		ApprovalPolicies: map[string]string{"release": "all"},
	}
}

// TestWorkflowDraftPublishImmutable (B02/B08 后端): structure validation
// rejects duplicates/cycles/unknown edges/unknown hard rules; publish makes
// the version immutable and updates the head; stale draft hash rejected.
func TestWorkflowDraftPublishImmutable(t *testing.T) {
	svc, projects, ids, _ := newWorkflowService(t)
	ctx := context.Background()
	owner, _, _ := ids.Register(ctx, "WF", "wf@wf.test", "password-wf-wf-11", "10.0.0.1")
	proj, err := projects.Create(ctx, owner.ID, project.CreateRequest{Title: "流程项目"})
	if err != nil {
		t.Fatal(err)
	}

	// Validation negatives.
	dup := validBody()
	dup.Nodes = append(dup.Nodes, workflow.Node{ID: "dev", Name: "重复"})
	if _, err := svc.Create(ctx, owner.ID, proj.ID, dup); errors.IsCode(err, errors.Validation) == false {
		t.Fatalf("duplicate node id must be rejected, got %v", err)
	}
	cyclic := validBody()
	cyclic.AdvisoryEdges = append(cyclic.AdvisoryEdges, workflow.Edge{From: "release", To: "dev"})
	if _, err := svc.Create(ctx, owner.ID, proj.ID, cyclic); errors.IsCode(err, errors.Validation) == false {
		t.Fatalf("cycle must be rejected, got %v", err)
	}
	badEdge := validBody()
	badEdge.AdvisoryEdges = append(badEdge.AdvisoryEdges, workflow.Edge{From: "dev", To: "ghost"})
	if _, err := svc.Create(ctx, owner.ID, proj.ID, badEdge); errors.IsCode(err, errors.Validation) == false {
		t.Fatalf("unknown edge target must be rejected, got %v", err)
	}
	badRule := validBody()
	badRule.HardRules = append(badRule.HardRules, workflow.HardRule{Kind: "exec_sql", Phase: "start"})
	if _, err := svc.Create(ctx, owner.ID, proj.ID, badRule); errors.IsCode(err, errors.Validation) == false {
		t.Fatalf("non-whitelisted hard rule must be rejected, got %v", err)
	}

	def, err := svc.Create(ctx, owner.ID, proj.ID, validBody())
	if err != nil {
		t.Fatal(err)
	}
	if !def.HasDraft {
		t.Fatal("new definition must have a draft")
	}
	// Publish with stale hash rejected.
	if _, err := svc.Publish(ctx, owner.ID, proj.ID, def.ID, def.Version, "deadbeef"); errors.IsCode(err, errors.ReviewStale) == false {
		t.Fatalf("stale draft hash must be REVIEW_STALE, got %v", err)
	}
	published, err := svc.Publish(ctx, owner.ID, proj.ID, def.ID, def.Version, "")
	if err != nil {
		t.Fatal(err)
	}
	if published.State != "published" || published.Mermaid == "" {
		t.Fatalf("publish result: %+v", published)
	}
	// Published version immutable: updating the draft creates a NEW revision,
	// the published one stays untouched.
	body2 := validBody()
	body2.Name = "交付流程 v2"
	body2.Nodes = append(body2.Nodes, workflow.Node{ID: "audit", Name: "审计", DefaultApprovalPolicy: "all"})
	draft2, err := svc.UpdateDraft(ctx, owner.ID, proj.ID, def.ID, def.Version+1, body2)
	if err != nil {
		t.Fatal(err)
	}
	if draft2.Revision <= published.Revision {
		t.Fatalf("new draft revision must exceed published: %d vs %d", draft2.Revision, published.Revision)
	}
	versions, err := svc.ListVersions(ctx, owner.ID, proj.ID, def.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}
	for _, v := range versions {
		if v.Revision == published.Revision && v.State != "published" {
			t.Fatal("published revision mutated")
		}
	}
	// Mermaid is generated from structure, never parsed back.
	if workflow.GenerateMermaid(body2) == workflow.GenerateMermaid(validBody()) {
		t.Fatal("mermaid must reflect structure changes")
	}
	_ = uuid.Nil
}
