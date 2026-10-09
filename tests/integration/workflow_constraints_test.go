package integrationtest_test

import (
	"context"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/decision"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/project"
	"github.com/kakj-go/Judex/internal/work"
	"github.com/kakj-go/Judex/internal/workflow"
	"testing"
)

func TestWorkflowConstraintsAndExplicitDelegation(t *testing.T) {
	decisions, projects, ids, pool := newDecisionEnv(t)
	ctx := context.Background()
	owner, _, err := ids.Register(ctx, "Owner", "workflow-owner@test.local", "password-owner-123", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	delegate, _, err := ids.Register(ctx, "Delegate", "workflow-delegate@test.local", "password-delegate-123", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	p, err := projects.Create(ctx, owner.ID, project.CreateRequest{Title: "Workflow constraints"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role,state,joined_at) VALUES($1,$2,'manager','active',now())`, p.ID, delegate.ID); err != nil {
		t.Fatal(err)
	}
	roleA, err := projects.CreatePosition(ctx, owner.ID, p.ID, project.PositionDraft{Name: "Engineering"})
	if err != nil {
		t.Fatal(err)
	}
	roleB, err := projects.CreatePosition(ctx, owner.ID, p.ID, project.PositionDraft{Name: "Required quality"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := projects.CreateIdentity(ctx, owner.ID, p.ID, roleA.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	b, err := projects.CreateIdentity(ctx, owner.ID, p.ID, roleB.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	workflows := workflow.NewService(pool, nil)
	body := workflow.Body{Name: "Gate", Nodes: []workflow.Node{{ID: "quality", Name: "Quality", AllowedPositionIds: []string{roleB.ID.String()}, DefaultApprovalPolicy: "all", DelegationUserIDs: []string{delegate.ID.String()}}}}
	definition, err := workflows.Create(ctx, owner.ID, p.ID, body)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = workflows.Publish(ctx, delegate.ID, p.ID, definition.ID, definition.Version, ""); !apierrors.IsCode(err, apierrors.Forbidden) {
		t.Fatalf("manager self-grant: %v", err)
	}
	if _, err = workflows.Publish(ctx, owner.ID, p.ID, definition.ID, definition.Version, ""); err != nil {
		t.Fatal(err)
	}
	create := func() (uuid.UUID, decision.Review) {
		t.Helper()
		id, err := decisions.CreateDraft(ctx, owner.ID, p.ID, "work_arrangement", nil, "", []decision.Change{{Operation: "create_task", TargetType: "task", ClientRef: "task", Fields: map[string]any{"title": "Controlled task", "reviewerIdentityId": a.ID.String(), "participantIdentityIds": []any{b.ID.String()}, "workflowId": definition.ID.String(), "nodeId": "quality"}}})
		if err != nil {
			t.Fatal(err)
		}
		review, err := decisions.Submit(ctx, owner.ID, p.ID, id, 1, "")
		if err != nil {
			t.Fatal(err)
		}
		return id, review
	}
	id, review := create()
	if len(review.Slots) != 2 {
		t.Fatalf("required workflow seat missing: %+v", review.Slots)
	}
	var own, quality uuid.UUID
	for _, s := range review.Slots {
		if s.AuthorityID == a.ID {
			own = s.ID
		}
		if s.AuthorityID == b.ID {
			quality = s.ID
		}
	}
	if _, err = decisions.Delegate(ctx, delegate.ID, p.ID, id, review.ReviewHash, "outside node", own); !apierrors.IsCode(err, apierrors.Forbidden) {
		t.Fatalf("delegated foreign seat: %v", err)
	}
	if _, err = decisions.Delegate(ctx, delegate.ID, p.ID, id, review.ReviewHash, "explicit node grant", quality); err != nil {
		t.Fatal(err)
	}
	done, err := decisions.Decide(ctx, owner.ID, p.ID, id, review.ReviewHash, true, "", decision.DecisionSelection{SlotIDs: []uuid.UUID{own}, Bindings: []decision.ActingBinding{{IdentityID: a.ID, BindingVersion: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	task, err := uuid.Parse(done.CreatedIDs["task"])
	if err != nil {
		t.Fatal(err)
	}
	// A later concrete workflow hard rule participates in the task start gate.
	workSvc := work.NewService(pool, nil)
	pre, err := workSvc.CreateTaskDraft(ctx, owner.ID, p.ID, work.TaskDraft{Title: "Required predecessor"})
	if err != nil {
		t.Fatal(err)
	}
	pending, pendingReview := create()
	body.HardRules = []workflow.HardRule{{NodeID: "quality", Kind: "task_acceptance", Phase: "start", TargetID: pre.ID.String()}}
	if _, err = workflows.UpdateDraft(ctx, owner.ID, p.ID, definition.ID, definition.Version+1, body); err != nil {
		t.Fatal(err)
	}
	defs, err := workflows.List(ctx, owner.ID, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = workflows.Publish(ctx, owner.ID, p.ID, definition.ID, defs[0].Version, ""); err != nil {
		t.Fatal(err)
	}
	if _, err = workSvc.Start(ctx, owner.ID, p.ID, task, &b.ID, 1); !apierrors.IsCode(err, apierrors.RequirementUnmet) {
		t.Fatalf("workflow hard requirement ignored: %v", err)
	}
	if _, err = decisions.Decide(ctx, owner.ID, p.ID, pending, pendingReview.ReviewHash, true, ""); !apierrors.IsCode(err, apierrors.ReviewStale) {
		t.Fatalf("changed workflow applied old votes: %v", err)
	}
	final, err := decisions.GetReview(ctx, owner.ID, p.ID, pending)
	if err != nil || final.Status != "stale" {
		t.Fatalf("stale not persisted: %+v %v", final, err)
	}
}
