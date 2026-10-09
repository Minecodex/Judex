package integrationtest_test

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/project"
	"github.com/kakj-go/Judex/internal/work"
	"github.com/kakj-go/Judex/internal/workflow"
	"os"
	"testing"
)

func TestRuntimeContinuousSkipRetainsEvidenceAndFrozenImpact(t *testing.T) {
	s, projects, ids, pool := newWorkEnv(t)
	ctx := context.Background()
	u, _, err := ids.Register(ctx, "Runtime", "runtime-edges@test.local", "runtime-password-123", "10.11.1.1")
	if err != nil {
		t.Fatal(err)
	}
	p, err := projects.Create(ctx, u.ID, project.CreateRequest{Title: "Runtime boundaries"})
	if err != nil {
		t.Fatal(err)
	}
	create := func(title string, req []work.Requirement) work.Task {
		t.Helper()
		task, e := s.CreateTaskDraft(ctx, u.ID, p.ID, work.TaskDraft{Title: title, Requirements: req})
		if e != nil {
			t.Fatal(e)
		}
		if _, e = pool.Exec(ctx, `UPDATE tasks SET status='ready',version=2 WHERE id=$1`, task.ID); e != nil {
			t.Fatal(e)
		}
		return task
	}
	a := create("A", nil)
	b := create("B", []work.Requirement{{Kind: "task_acceptance", Phase: "accept", TargetID: a.ID, Hard: true}})
	c := create("C", []work.Requirement{{Kind: "task_acceptance", Phase: "start", TargetID: b.ID, Hard: true}})
	d := create("D", []work.Requirement{{Kind: "task_acceptance", Phase: "accept", TargetID: c.ID, Hard: true}})
	// Missing/deleted historical evidence must remain a blocker after bypass.
	evidence, receipt := uuid.New(), uuid.New()
	for _, r := range []struct {
		kind string
		id   uuid.UUID
	}{{"material_ready", evidence}, {"handoff_receipt", receipt}} {
		if _, err = pool.Exec(ctx, `INSERT INTO task_requirements(project_id,id,task_id,phase,kind,target_id,hard,label) VALUES($1,$2,$3,'accept',$4,$5,true,'Evidence still required')`, p.ID, uuid.New(), b.ID, r.kind, r.id); err != nil {
			t.Fatal(err)
		}
	}
	review, err := s.ExecutionExceptionReview(ctx, u.ID, p.ID, b.ID, "skip")
	if err != nil {
		t.Fatal(err)
	}
	in := work.ExceptionCommand{ExpectedVersion: 2, ReviewHash: review.ReviewHash, Reason: "Review bypass"}
	// Even a new report/version on a downstream task invalidates the impact list.
	pool.Exec(ctx, `UPDATE tasks SET version=version+1 WHERE id=$1`, d.ID)
	if err = s.ChangeExecutionException(ctx, u.ID, p.ID, b.ID, "skip", in); !apierrors.IsCode(err, apierrors.ReviewStale) {
		t.Fatalf("stale impact %v", err)
	}
	skip := func(id uuid.UUID) {
		t.Helper()
		r, e := s.ExecutionExceptionReview(ctx, u.ID, p.ID, id, "skip")
		if e != nil {
			t.Fatal(e)
		}
		waivers := []work.ExceptionWaiver{}
		for _, v := range r.AffectedTasks {
			for _, q := range v.Requirements {
				waivers = append(waivers, work.ExceptionWaiver{TaskID: v.TaskID, RequirementID: q.ID, Fingerprint: q.Fingerprint})
			}
		}
		if e = s.ChangeExecutionException(ctx, u.ID, p.ID, id, "skip", work.ExceptionCommand{ExpectedVersion: r.TargetVersion, ReviewHash: r.ReviewHash, Reason: "Explicit bypass", Waivers: waivers}); e != nil {
			t.Fatal(e)
		}
	}
	skip(b.ID)
	skip(c.ID)
	req, blockers, err := s.RequirementsFor(ctx, pool, p.ID, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, q := range blockers {
		found[q.ObjectID] = q.Phase == "accept"
	}
	for _, id := range []uuid.UUID{a.ID, evidence, receipt} {
		if !found[id.String()] {
			t.Fatalf("lost inherited condition %s: %+v", id, blockers)
		}
	}
	for _, r := range req {
		if r.Kind != "task_acceptance" && r.Waived {
			t.Fatalf("non-task condition waived %+v", r)
		}
	}
	mapping, err := s.ExecutionMap(ctx, u.ID, p.ID, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	bridge := false
	for _, e := range mapping.Edges {
		bridge = bridge || e.FromTaskID == a.ID.String() && e.ToTaskID == d.ID.String() && len(e.ViaTaskIDs) == 2
	}
	if !bridge {
		t.Fatalf("missing continuous bridge %+v", mapping.Edges)
	}
	restore, err := s.ExecutionExceptionReview(ctx, u.ID, p.ID, b.ID, "restore")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ChangeExecutionException(ctx, u.ID, p.ID, b.ID, "restore", work.ExceptionCommand{ExpectedVersion: restore.TargetVersion, ReviewHash: restore.ReviewHash, Reason: "Restore prior work"}); err != nil {
		t.Fatal(err)
	}
	_, blockers, err = s.RequirementsFor(ctx, pool, p.ID, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(blockers) != 1 || blockers[0].ObjectID != b.ID.String() {
		t.Fatalf("restoration did not reinstate prior constraint %+v", blockers)
	}
	pool.Exec(ctx, `UPDATE tasks SET status='delivered' WHERE id=$1`, b.ID)
	skip(b.ID)
	got, err := s.GetTask(ctx, u.ID, p.ID, b.ID)
	if err != nil || got.Status != "delivered" || got.ExecutionException.PreviousStatus != "delivered" {
		t.Fatalf("delivered phase lost %+v %v", got, err)
	}
}

func TestRuntimeMigrationKeepsWorkAndAcceptanceHistory(t *testing.T) {
	s, projects, ids, pool := newWorkEnv(t)
	ctx := context.Background()
	u, _, e := ids.Register(ctx, "Migration", "runtime-migration@test.local", "runtime-password-123", "10.11.3.1")
	if e != nil {
		t.Fatal(e)
	}
	p, e := projects.Create(ctx, u.ID, project.CreateRequest{Title: "Preserved history"})
	if e != nil {
		t.Fatal(e)
	}
	pos, e := projects.CreatePosition(ctx, u.ID, p.ID, project.PositionDraft{Name: "Review"})
	if e != nil {
		t.Fatal(e)
	}
	id, e := projects.CreateIdentity(ctx, u.ID, p.ID, pos.ID, u.ID)
	if e != nil {
		t.Fatal(e)
	}
	task, e := s.CreateTaskDraft(ctx, u.ID, p.ID, work.TaskDraft{Title: "Accepted before migration", ParticipantIDs: []uuid.UUID{id.ID}, ReviewerIdentityID: &id.ID})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, `UPDATE tasks SET status='ready' WHERE id=$1`, task.ID); e != nil {
		t.Fatal(e)
	}
	if _, _, e = s.Report(ctx, u.ID, p.ID, work.ReportInput{TaskID: task.ID, Kind: "delivery", Text: "Immutable original outcome", ExpectedTaskVersion: 1}); e != nil {
		t.Fatal(e)
	}
	review, e := s.TaskAcceptanceReview(ctx, u.ID, p.ID, task.ID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DecideTaskAcceptance(ctx, u.ID, p.ID, task.ID, review.ReviewHash, true, "", review.TargetVersion); e != nil {
		t.Fatal(e)
	}
	take := func() string {
		t.Helper()
		var raw string
		if e = pool.QueryRow(ctx, `SELECT jsonb_build_object('tasks',(SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM tasks t),'reports',(SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM work_reports r),'acceptances',(SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM task_acceptances a),'events',(SELECT jsonb_agg(to_jsonb(v) ORDER BY seq) FROM project_events v))::text`).Scan(&raw); e != nil {
			t.Fatal(e)
		}
		return raw
	}
	before := take()
	for _, direction := range []string{"down", "up"} {
		raw, e := os.ReadFile("../../internal/infrastructure/postgres/migrations/00025_task_runtime." + direction + ".sql")
		if e != nil {
			t.Fatal(e)
		}
		if _, e = pool.Exec(ctx, string(raw)); e != nil {
			t.Fatal(e)
		}
	}
	if before != take() {
		t.Fatal("migration changed existing work, reports, acceptance or events")
	}
}

func TestDraftDiscardProtectsPublishedWorkflowReferences(t *testing.T) {
	s, projects, ids, pool := newWorkEnv(t)
	ctx := context.Background()
	u, _, e := ids.Register(ctx, "Workflow owner", "runtime-flow@test.local", "runtime-password-123", "10.11.4.1")
	if e != nil {
		t.Fatal(e)
	}
	p, e := projects.Create(ctx, u.ID, project.CreateRequest{Title: "Reference protection"})
	if e != nil {
		t.Fatal(e)
	}
	task, e := s.CreateTaskDraft(ctx, u.ID, p.ID, work.TaskDraft{Title: "Referenced draft"})
	if e != nil {
		t.Fatal(e)
	}
	flows := workflow.NewService(pool, nil)
	empty, e := flows.Create(ctx, u.ID, p.ID, workflow.Body{Name: "No hard rules", Nodes: []workflow.Node{{ID: "empty", Name: "Empty", DefaultApprovalPolicy: "all"}}})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = flows.Publish(ctx, u.ID, p.ID, empty.ID, empty.Version, ""); e != nil {
		t.Fatal(e)
	}
	flow, e := flows.Create(ctx, u.ID, p.ID, workflow.Body{Name: "Published formal basis", Nodes: []workflow.Node{{ID: "review", Name: "Review", DefaultApprovalPolicy: "all"}}, HardRules: []workflow.HardRule{{NodeID: "review", Kind: "task_acceptance", Phase: "start", TargetID: task.ID.String()}}})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = flows.Publish(ctx, u.ID, p.ID, flow.ID, flow.Version, ""); e != nil {
		t.Fatal(e)
	}
	review, e := s.DiscardReview(ctx, u.ID, p.ID, task.ID, "task")
	if e != nil || len(review.Blockers) != 1 || review.Blockers[0].ObjectType != "workflow" {
		t.Fatalf("lost published reference %+v %v", review, e)
	}
	if e = s.Discard(ctx, u.ID, p.ID, task.ID, "task", 1, review.ReviewHash); !apierrors.IsCode(e, apierrors.DependencyChanged) {
		t.Fatalf("discard bypassed formal workflow %v", e)
	}
}

func TestRuntimePlanSnapshotExcludesSkippedButProtectsReferences(t *testing.T) {
	s, projects, ids, pool := newWorkEnv(t)
	ctx := context.Background()
	u, _, e := ids.Register(ctx, "Owner", "runtime-plan@test.local", "runtime-password-123", "10.11.2.1")
	if e != nil {
		t.Fatal(e)
	}
	p, e := projects.Create(ctx, u.ID, project.CreateRequest{Title: "Runtime plan"})
	if e != nil {
		t.Fatal(e)
	}
	pos, e := projects.CreatePosition(ctx, u.ID, p.ID, project.PositionDraft{Name: "Owner"})
	if e != nil {
		t.Fatal(e)
	}
	identity, e := projects.CreateIdentity(ctx, u.ID, p.ID, pos.ID, u.ID)
	if e != nil {
		t.Fatal(e)
	}
	plan, e := s.CreatePlanDraft(ctx, u.ID, p.ID, "Owned plan", "Goal", "Human review", &identity.ID, nil)
	if e != nil {
		t.Fatal(e)
	}
	referenced, e := s.CreatePlanDraft(ctx, u.ID, p.ID, "Referenced plan", "Goal", "Evidence still needed", &identity.ID, nil)
	if e != nil {
		t.Fatal(e)
	}
	task, e := s.CreateTaskDraft(ctx, u.ID, p.ID, work.TaskDraft{Title: "Exception", PlanID: &plan.ID, ReviewerIdentityID: &identity.ID, ParticipantIDs: []uuid.UUID{identity.ID}})
	if e != nil {
		t.Fatal(e)
	}
	pool.Exec(ctx, `UPDATE plans SET status='active' WHERE id=ANY($1)`, []uuid.UUID{plan.ID, referenced.ID})
	pool.Exec(ctx, `UPDATE tasks SET status='ready' WHERE id=$1`, task.ID)
	pool.Exec(ctx, `INSERT INTO plan_task_references(project_id,plan_id,task_id)VALUES($1,$2,$3)`, p.ID, referenced.ID, task.ID)
	impact, e := s.ExecutionExceptionReview(ctx, u.ID, p.ID, task.ID, "skip")
	if e != nil {
		t.Fatal(e)
	}
	if len(impact.ReferencingPlanIDs) != 1 {
		t.Fatal("reference plan absent")
	}
	if e = s.ChangeExecutionException(ctx, u.ID, p.ID, task.ID, "skip", work.ExceptionCommand{ExpectedVersion: 1, ReviewHash: impact.ReviewHash, Reason: "Owner will check goal separately"}); e != nil {
		t.Fatal(e)
	}
	own, e := s.PlanAcceptanceReview(ctx, u.ID, p.ID, plan.ID)
	if e != nil || len(own.Blockers) != 0 || len(own.TaskAcceptanceIDs) != 0 {
		t.Fatalf("own review %+v %v", own, e)
	}
	external, e := s.PlanAcceptanceReview(ctx, u.ID, p.ID, referenced.ID)
	if e != nil || len(external.Blockers) != 1 {
		t.Fatalf("reference incorrectly waived %+v %v", external, e)
	}
	var manifest map[string]any
	if e = json.Unmarshal(own.Manifest, &manifest); e != nil {
		t.Fatal(e)
	}
	if manifest["tasks"].([]any)[0].(map[string]any)["executionException"] == nil {
		t.Fatal("snapshot lacks skip basis")
	}
	if _, e = s.DecidePlanAcceptance(ctx, u.ID, p.ID, referenced.ID, external.ReviewHash, true, ""); !apierrors.IsCode(e, apierrors.RequirementUnmet) {
		t.Fatalf("reference accepted %v", e)
	}
	if _, e = s.DecidePlanAcceptance(ctx, u.ID, p.ID, plan.ID, own.ReviewHash, true, ""); e != nil {
		t.Fatal(e)
	}
	var audit []byte
	if e = pool.QueryRow(ctx, `SELECT review_manifest FROM plan_acceptances WHERE plan_id=$1`, plan.ID).Scan(&audit); e != nil {
		t.Fatal(e)
	}
	if !json.Valid(audit) {
		t.Fatal("acceptance snapshot invalid")
	}
}
