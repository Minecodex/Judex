package integrationtest_test

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/collaboration"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/project"
	"github.com/kakj-go/Judex/internal/work"
	"testing"
)

func TestTaskRuntimeSkipRestoreAndInheritedConditions(t *testing.T) {
	s, projects, ids, pool := newWorkEnv(t)
	ctx := context.Background()
	admin, _, e := ids.Register(ctx, "Runtime Admin", "runtime-admin@test.local", "runtime-password-123", "10.10.1.1")
	if e != nil {
		t.Fatal(e)
	}
	member, _, e := ids.Register(ctx, "Runtime Member", "runtime-member@test.local", "runtime-password-123", "10.10.1.2")
	if e != nil {
		t.Fatal(e)
	}
	p, e := projects.Create(ctx, admin.ID, project.CreateRequest{Title: "Runtime"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role,state,joined_at)VALUES($1,$2,'member','active',now())`, p.ID, member.ID); e != nil {
		t.Fatal(e)
	}
	plan, e := s.CreatePlanDraft(ctx, admin.ID, p.ID, "Runtime plan", "Deliver clear evidence", "Human review", nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	create := func(title string, reqs []work.Requirement) work.Task {
		t.Helper()
		v, e := s.CreateTaskDraft(ctx, admin.ID, p.ID, work.TaskDraft{PlanID: &plan.ID, Title: title, Requirements: reqs})
		if e != nil {
			t.Fatal(e)
		}
		if _, e = pool.Exec(ctx, `UPDATE tasks SET status='ready',version=2 WHERE id=$1`, v.ID); e != nil {
			t.Fatal(e)
		}
		return v
	}
	a := create("A", nil)
	b := create("B", []work.Requirement{{Phase: "start", Kind: "task_acceptance", TargetID: a.ID, Hard: true}})
	d := create("D", nil)
	c := create("C", []work.Requirement{{Phase: "start", Kind: "task_acceptance", TargetID: b.ID, Hard: true}, {Phase: "accept", Kind: "task_acceptance", TargetID: d.ID, Hard: true}, {Phase: "start", Kind: "task_acceptance", TargetID: d.ID, Hard: false}})
	if _, e = s.ExecutionExceptionReview(ctx, member.ID, p.ID, b.ID, "skip"); !apierrors.IsCode(e, apierrors.Forbidden) {
		t.Fatalf("member preview %v", e)
	}
	review, e := s.ExecutionExceptionReview(ctx, admin.ID, p.ID, b.ID, "skip")
	if e != nil {
		t.Fatal(e)
	}
	var selected work.ExceptionWaiver
	for _, impact := range review.AffectedTasks {
		if impact.TaskID == c.ID {
			r := impact.Requirements[0]
			selected = work.ExceptionWaiver{TaskID: c.ID, RequirementID: r.ID, Fingerprint: r.Fingerprint}
		}
	}
	if selected.TaskID == uuid.Nil {
		t.Fatal("missing successor")
	}
	if e = s.ChangeExecutionException(ctx, admin.ID, p.ID, b.ID, "skip", work.ExceptionCommand{ExpectedVersion: 2, ReviewHash: review.ReviewHash, Reason: "Review already covered", Waivers: []work.ExceptionWaiver{selected}}); e != nil {
		t.Fatal(e)
	}
	got, e := s.GetTask(ctx, admin.ID, p.ID, b.ID)
	if e != nil || got.Status != "ready" || got.ExecutionException == nil || !got.Capabilities.Restore || got.Capabilities.Skip {
		t.Fatalf("skipped task %+v %v", got, e)
	}
	if _, e = s.Start(ctx, admin.ID, p.ID, b.ID, nil, got.Version); !apierrors.IsCode(e, apierrors.InvalidTransition) {
		t.Fatalf("skipped start %v", e)
	}
	_, blocked, e := s.RequirementsFor(ctx, pool, p.ID, c.ID)
	if e != nil {
		t.Fatal(e)
	}
	start, accept := false, false
	for _, v := range blocked {
		start = start || v.ObjectID == a.ID.String() && v.Phase == "start"
		accept = accept || v.ObjectID == d.ID.String() && v.Phase == "accept"
	}
	if !start || !accept {
		t.Fatalf("lost inherited or phased condition %+v", blocked)
	}
	if _, e = pool.Exec(ctx, `UPDATE tasks SET status='accepted',version=3 WHERE id=$1`, a.ID); e != nil {
		t.Fatal(e)
	}
	_, blocked, e = s.RequirementsFor(ctx, pool, p.ID, c.ID)
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range blocked {
		if v.Phase == "start" {
			t.Fatalf("start remains blocked %+v", blocked)
		}
	}
	pos, e := projects.CreatePosition(ctx, admin.ID, p.ID, project.PositionDraft{Name: "Executor"})
	if e != nil {
		t.Fatal(e)
	}
	identity, e := projects.CreateIdentity(ctx, admin.ID, p.ID, pos.ID, admin.ID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, `INSERT INTO task_participants(project_id,task_id,identity_id)VALUES($1,$2,$3)`, p.ID, c.ID, identity.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Start(ctx, admin.ID, p.ID, c.ID, nil, 2); e != nil {
		t.Fatal(e)
	}
	mapping, e := s.ExecutionMap(ctx, admin.ID, p.ID, &plan.ID, true)
	if e != nil {
		t.Fatal(e)
	}
	bypass, original := false, false
	for _, edge := range mapping.Edges {
		bypass = bypass || edge.FromTaskID == a.ID.String() && edge.ToTaskID == c.ID.String() && !edge.Original
		original = original || edge.FromTaskID == b.ID.String() && edge.ToTaskID == c.ID.String() && edge.Original
	}
	for _, node := range mapping.Nodes {
		if node.TaskID == b.ID.String() && (!node.Capabilities.Restore || node.Capabilities.Skip) {
			t.Fatalf("map permission differs from task %+v", node)
		}
	}
	if !bypass || !original {
		t.Fatalf("map %+v", mapping)
	}
	plans, e := s.ListPlans(ctx, admin.ID, p.ID, plan.ID)
	if e != nil || plans[0].TaskStats.Skipped != 1 || plans[0].TaskStats.Required != 3 || plans[0].TaskStats.Accepted != 1 {
		t.Fatalf("stats %+v %v", plans, e)
	}
	restore, e := s.ExecutionExceptionReview(ctx, admin.ID, p.ID, b.ID, "restore")
	if e != nil {
		t.Fatal(e)
	}
	input := work.ExceptionCommand{ExpectedVersion: got.Version, ReviewHash: restore.ReviewHash, Reason: "Need another review"}
	if e = s.ChangeExecutionException(ctx, admin.ID, p.ID, b.ID, "restore", input); !apierrors.IsCode(e, apierrors.RequirementUnmet) {
		t.Fatalf("restore acknowledgement %v", e)
	}
	input.AcknowledgeStarted = true
	if e = s.ChangeExecutionException(ctx, admin.ID, p.ID, b.ID, "restore", input); e != nil {
		t.Fatal(e)
	}
	current, e := s.GetTask(ctx, admin.ID, p.ID, c.ID)
	if e != nil || current.Status != "working" {
		t.Fatalf("successor changed %+v %v", current, e)
	}
	originalB, e := s.GetTask(ctx, admin.ID, p.ID, b.ID)
	if e != nil || originalB.ExecutionException != nil || originalB.Status != "ready" {
		t.Fatalf("restore %+v %v", originalB, e)
	}
	activity, e := collaboration.NewService(pool).ListActivity(ctx, admin.ID, p.ID, b.ID)
	if e != nil || len(activity) != 2 || activity[0].Kind != "decision" {
		t.Fatalf("exception history %+v %v", activity, e)
	}
}

func TestWorkDraftRevisionPermissionsAndDiscardReview(t *testing.T) {
	s, projects, ids, pool := newWorkEnv(t)
	ctx := context.Background()
	admin, _, _ := ids.Register(ctx, "Admin", "draft-admin@test.local", "runtime-password-123", "10.10.2.1")
	author, _, _ := ids.Register(ctx, "Author", "draft-author@test.local", "runtime-password-123", "10.10.2.2")
	owner, _, _ := ids.Register(ctx, "Plan owner", "draft-owner@test.local", "runtime-password-123", "10.10.2.3")
	other, _, _ := ids.Register(ctx, "Other", "draft-other@test.local", "runtime-password-123", "10.10.2.4")
	p, e := projects.Create(ctx, admin.ID, project.CreateRequest{Title: "Draft scopes"})
	if e != nil {
		t.Fatal(e)
	}
	for _, u := range []uuid.UUID{author.ID, owner.ID, other.ID} {
		if _, e = pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role,state,joined_at)VALUES($1,$2,'member','active',now())`, p.ID, u); e != nil {
			t.Fatal(e)
		}
	}
	pos, e := projects.CreatePosition(ctx, admin.ID, p.ID, project.PositionDraft{Name: "Plan owner"})
	if e != nil {
		t.Fatal(e)
	}
	identity, e := projects.CreateIdentity(ctx, admin.ID, p.ID, pos.ID, owner.ID)
	if e != nil {
		t.Fatal(e)
	}
	plan, e := s.CreatePlanDraft(ctx, author.ID, p.ID, "Draft plan", "Goal", "Criteria", &identity.ID, nil)
	if e != nil {
		t.Fatal(e)
	}
	first, e := s.CreateTaskDraft(ctx, author.ID, p.ID, work.TaskDraft{PlanID: &plan.ID, Title: "First"})
	if e != nil {
		t.Fatal(e)
	}
	change := work.DraftUpdate{ExpectedVersion: 1, Fields: map[string]any{"title": "Revised", "expectedOutput": "Reviewable output", "acceptanceCriteria": "Clear evidence"}}
	if e = s.UpdateDraft(ctx, other.ID, p.ID, first.ID, "task", change); !apierrors.IsCode(e, apierrors.Forbidden) {
		t.Fatalf("stranger edits %v", e)
	}
	proposal, version := uuid.New(), uuid.New()
	raw, _ := json.Marshal([]work.Change{{Operation: "activate_object", TargetType: "task", TargetID: first.ID.String(), ExpectedVersion: 1}})
	if _, e = pool.Exec(ctx, `INSERT INTO proposals(project_id,id,kind,status,current_review_id,created_by,created_at,updated_at)VALUES($1,$2,'work_arrangement','pending',$3,$4,now(),now());`, p.ID, proposal, version, author.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, `INSERT INTO proposal_versions(project_id,id,proposal_id,revision,changes_json,created_at)VALUES($1,$2,$3,1,$4,now())`, p.ID, version, proposal, raw); e != nil {
		t.Fatal(e)
	}
	if e = s.UpdateDraft(ctx, owner.ID, p.ID, first.ID, "task", change); e != nil {
		t.Fatal(e)
	}
	updated, e := s.GetTask(ctx, admin.ID, p.ID, first.ID)
	if e != nil || updated.Version != 2 || updated.Status != "draft" || updated.Title != "Revised" {
		t.Fatalf("draft revision %+v %v", updated, e)
	}
	var status string
	pool.QueryRow(ctx, `SELECT status FROM proposals WHERE id=$1`, proposal).Scan(&status)
	if status != "cancelled" {
		t.Fatalf("old review %s", status)
	}
	if e = s.UpdateDraft(ctx, author.ID, p.ID, first.ID, "task", change); !apierrors.IsCode(e, apierrors.VersionConflict) {
		t.Fatalf("stale draft %v", e)
	}
	linked, e := s.CreateTaskDraft(ctx, author.ID, p.ID, work.TaskDraft{PlanID: &plan.ID, Title: "Linked", Requirements: []work.Requirement{{Kind: "task_acceptance", Phase: "start", TargetID: first.ID, Hard: true}}})
	if e != nil {
		t.Fatal(e)
	}
	review, e := s.DiscardReview(ctx, owner.ID, p.ID, first.ID, "task")
	if e != nil || len(review.Blockers) == 0 {
		t.Fatalf("reference review %+v %v", review, e)
	}
	if e = s.Discard(ctx, owner.ID, p.ID, first.ID, "task", 2, review.ReviewHash); !apierrors.IsCode(e, apierrors.DependencyChanged) {
		t.Fatalf("removed referenced draft %v", e)
	}
	planReview, e := s.DiscardReview(ctx, owner.ID, p.ID, plan.ID, "plan")
	if e != nil || len(planReview.Tasks) != 2 || len(planReview.Blockers) != 0 {
		t.Fatalf("plan review %+v %v", planReview, e)
	}
	if e = s.Discard(ctx, admin.ID, p.ID, plan.ID, "plan", 1, planReview.ReviewHash); e != nil {
		t.Fatal(e)
	}
	list, e := s.ListPlans(ctx, owner.ID, p.ID)
	if e != nil || len(list) != 0 {
		t.Fatalf("discarded plan visible %+v %v", list, e)
	}
	tasks, e := s.ListTasks(ctx, owner.ID, p.ID, &plan.ID)
	if e != nil || len(tasks) != 0 {
		t.Fatalf("discarded tasks visible %+v %v", tasks, e)
	}
	historical, e := s.GetTask(ctx, owner.ID, p.ID, linked.ID)
	if e != nil || historical.DiscardedAt == nil || historical.Capabilities.EditDraft {
		t.Fatalf("history lost %+v %v", historical, e)
	}
}
