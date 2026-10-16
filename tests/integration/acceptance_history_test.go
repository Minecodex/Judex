package integrationtest_test

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/collaboration"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/paging"
	"github.com/kakj-go/Judex/internal/project"
	"github.com/kakj-go/Judex/internal/work"
)

func TestFormalFeedbackHistoryRetainsRejectAcceptReopenAndScopes(t *testing.T) {
	_, projects, ids, pool := newDiscussionEnv(t)
	ctx := context.Background()
	owner, _, err := ids.Register(ctx, "Feedback owner", "feedback@history.test", "feedback-history-password", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	outsider, _, err := ids.Register(ctx, "Other", "other@history.test", "feedback-other-password", "127.0.0.2")
	if err != nil {
		t.Fatal(err)
	}
	p, err := projects.Create(ctx, owner.ID, projectRequest("Feedback"))
	if err != nil {
		t.Fatal(err)
	}
	other, err := projects.Create(ctx, owner.ID, projectRequest("Other"))
	if err != nil {
		t.Fatal(err)
	}
	position, err := projects.CreatePosition(ctx, owner.ID, p.ID, project.PositionDraft{Name: "Reviewer"})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := projects.CreateIdentity(ctx, owner.ID, p.ID, position.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	w := work.NewService(pool, nil)
	plan, err := w.CreatePlanDraft(ctx, owner.ID, p.ID, "Review goal", "Goal", "Criteria", &identity.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	task, err := w.CreateTaskDraft(ctx, owner.ID, p.ID, work.TaskDraft{Title: "Review artifact", PlanID: &plan.ID, ParticipantIDs: []uuid.UUID{identity.ID}, ReviewerIdentityID: &identity.ID})
	if err != nil {
		t.Fatal(err)
	}
	// Controlled activation fixture; all delivery and human decisions below go
	// through the actual domain transactions and frozen review hashes.
	if _, err = pool.Exec(ctx, `UPDATE plans SET status='active' WHERE id=$1`, plan.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE tasks SET status='ready',version=2 WHERE id=$1`, task.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = w.Start(ctx, owner.ID, p.ID, task.ID, nil, 2); err != nil {
		t.Fatal(err)
	}
	deliver := func(version int64) {
		t.Helper()
		_, _, e := w.Report(ctx, owner.ID, p.ID, work.ReportInput{TaskID: task.ID, IdentityID: &identity.ID, Kind: "delivery", Text: "Fixed evidence", ExpectedTaskVersion: version})
		if e != nil {
			t.Fatal(e)
		}
	}
	decide := func(accept bool, reason string) {
		t.Helper()
		r, e := w.TaskAcceptanceReview(ctx, owner.ID, p.ID, task.ID)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = w.DecideTaskAcceptance(ctx, owner.ID, p.ID, task.ID, r.ReviewHash, accept, reason, r.TargetVersion); e != nil {
			t.Fatal(e)
		}
	}
	deliver(3)
	decide(false, "Missing actual execution log")
	history, err := w.AcceptanceHistory(ctx, owner.ID, p.ID, task.ID, "task")
	if err != nil || len(history) != 1 || history[0].Decision != "reject" || history[0].Reason != "Missing actual execution log" || history[0].Provenance != "work_record" || len(history[0].Snapshot) == 0 {
		t.Fatalf("rejection not public and frozen: %+v %v", history, err)
	}
	if _, err = w.AcceptanceHistory(ctx, outsider.ID, p.ID, task.ID, "task"); !apierrors.IsCode(err, apierrors.NotFound) {
		t.Fatalf("outsider history: %v", err)
	}
	if _, err = w.AcceptanceHistory(ctx, owner.ID, other.ID, task.ID, "task"); !apierrors.IsCode(err, apierrors.NotFound) {
		t.Fatalf("cross project history: %v", err)
	}
	deliver(5)
	decide(true, "")
	accepted, err := w.GetTask(ctx, owner.ID, p.ID, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.ReopenTask(ctx, owner.ID, p.ID, task.ID, *accepted.LatestAcceptanceID, "New scope requires rework", accepted.Version); err != nil {
		t.Fatal(err)
	}
	history, err = w.AcceptanceHistory(ctx, owner.ID, p.ID, task.ID, "task")
	if err != nil || len(history) != 3 || history[0].Decision != "reopen" || history[0].PreviousAcceptanceID == nil || *history[0].PreviousAcceptanceID != *accepted.LatestAcceptanceID {
		t.Fatalf("history duplicated or lost reopen: %+v %v", history, err)
	}
	// Page across the union of actual decisions: no duplicate or lost event,
	// and cursors must not be reusable for another work object.
	scope, cursor, seen := "acceptance-history:"+task.ID.String(), "", map[uuid.UUID]bool{}
	for {
		page, e := paging.Parse(ctx, scope, url.Values{"limit": {"1"}, "cursor": {cursor}})
		if e != nil {
			t.Fatal(e)
		}
		items, e := w.AcceptanceHistory(page, owner.ID, p.ID, task.ID, "task")
		if e != nil || len(items) != 1 || seen[items[0].ID] {
			t.Fatalf("feedback pagination: %+v %v", items, e)
		}
		seen[items[0].ID] = true
		next := paging.Next(page)
		if next == nil {
			break
		}
		cursor = *next
		if _, e = paging.Parse(ctx, scope+":other", url.Values{"cursor": {cursor}}); e == nil {
			t.Fatal("feedback cursor crossed work scope")
		}
	}
	if len(seen) != len(history) {
		t.Fatal("feedback pagination lost formal decisions")
	}
	latest, err := work.LatestFormalFeedback(ctx, pool, p.ID, task.ID, "task")
	if err != nil {
		t.Fatal(err)
	}
	var feedback map[string]any
	if json.Unmarshal(latest, &feedback) != nil || feedback["reason"] != "New scope requires rework" {
		t.Fatalf("agent feedback: %s", latest)
	}
	activities, err := collaboration.NewService(pool).ListActivity(ctx, owner.ID, p.ID, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	decisions := 0
	for _, a := range activities {
		if a.Kind == "decision" {
			decisions++
		}
	}
	if decisions != 3 {
		t.Fatalf("Web/CLI activity did not expose formal feedback: %d", decisions)
	}
	r, err := w.PlanAcceptanceReview(ctx, owner.ID, p.ID, plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.DecidePlanAcceptance(ctx, owner.ID, p.ID, plan.ID, r.ReviewHash, false, "Overall goal not met"); err != nil {
		t.Fatal(err)
	}
	deliver(8)
	decide(true, "")
	r, err = w.PlanAcceptanceReview(ctx, owner.ID, p.ID, plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.DecidePlanAcceptance(ctx, owner.ID, p.ID, plan.ID, r.ReviewHash, true, ""); err != nil {
		t.Fatal(err)
	}
	plansBeforeReopen, err := w.ListPlans(ctx, owner.ID, p.ID, plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	pread := plansBeforeReopen[0]
	if _, err = w.ReopenPlan(ctx, owner.ID, p.ID, plan.ID, *pread.LatestAcceptanceID, "Revisit the overall criteria"); err != nil {
		t.Fatal(err)
	}
	plans, err := w.AcceptanceHistory(ctx, owner.ID, p.ID, plan.ID, "plan")
	if err != nil || len(plans) != 3 || plans[0].Decision != "reopen" {
		t.Fatalf("plan history: %+v %v", plans, err)
	}
}

func TestLegacyRejectedFeedbackUsesActualAuditWithoutInventingSnapshot(t *testing.T) {
	_, projects, ids, pool := newDiscussionEnv(t)
	ctx := context.Background()
	u, _, err := ids.Register(ctx, "Legacy reader", "legacy-feedback@history.test", "legacy-feedback-password", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	p, err := projects.Create(ctx, u.ID, projectRequest("Legacy feedback"))
	if err != nil {
		t.Fatal(err)
	}
	w := work.NewService(pool, nil)
	task, err := w.CreateTaskDraft(ctx, u.ID, p.ID, work.TaskDraft{Title: "Legacy input"})
	if err != nil {
		t.Fatal(err)
	}
	reason := "Actual retained legacy rejection"
	err = pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		return audit.Append(ctx, tx, audit.Entry{ProjectID: &p.ID, ActorType: audit.ActorUser, ActorUserID: &u.ID, Source: audit.SourceWeb, Operation: "task.reject", ObjectType: "task", ObjectID: task.ID.String(), Reason: &reason, OccurredAt: time.Now().UTC()})
	})
	if err != nil {
		t.Fatal(err)
	}
	items, err := w.AcceptanceHistory(ctx, u.ID, p.ID, task.ID, "task")
	if err != nil || len(items) != 1 || items[0].Provenance != "legacy_audit" || items[0].Reason != reason || items[0].TargetVersion != nil || len(items[0].Snapshot) != 0 {
		t.Fatalf("legacy facts fabricated or lost: %+v %v", items, err)
	}
	activities, err := collaboration.NewService(pool).ListActivity(ctx, u.ID, p.ID, task.ID)
	if err != nil || len(activities) != 1 || activities[0].Text != reason || activities[0].SourceType != "acceptance_feedback" {
		t.Fatalf("existing Web/CLI timeline lost legacy feedback: %+v %v", activities, err)
	}
}
