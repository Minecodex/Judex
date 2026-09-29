// SPDX-License-Identifier: Apache-2.0
package integrationtest_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/project"
	"github.com/kakj-go/Judex/internal/work"
)

// TestTaskAndPlanAcceptance (B11/B12): 验收快照哈希（新报告→REVIEW_STALE）、
// reviewer 专属、退回→rework、验收→accepted+不可变 acceptance、任务全验收
// 不自动完成计划（REQUIREMENT_UNMET）→ owner 验收 → 重开保留历史。
func TestTaskAndPlanAcceptance(t *testing.T) {
	svc, _, projects, ids, pool := newHandoffEnv(t)
	ctx := context.Background()
	owner, _, _ := ids.Register(ctx, "VA", "va@va.test", "password-va-va-1", "10.0.0.1")
	worker, _, _ := ids.Register(ctx, "VB", "vb@va.test", "password-vb-vb-1", "10.0.0.1")
	proj, _ := projects.Create(ctx, owner.ID, project.CreateRequest{Title: "验收项目"})
	if _, err := pool.Exec(ctx, `
		INSERT INTO project_members (project_id, user_id, role, state, joined_at)
		VALUES ($1,$2,'member','active',now())`, proj.ID, worker.ID); err != nil {
		t.Fatal(err)
	}
	reviewPos, _ := projects.CreatePosition(ctx, owner.ID, proj.ID, project.PositionDraft{Name: "验收岗"})
	reviewerIdent, _ := projects.CreateIdentity(ctx, owner.ID, proj.ID, reviewPos.ID, owner.ID)
	workerPos, _ := projects.CreatePosition(ctx, owner.ID, proj.ID, project.PositionDraft{Name: "执行岗"})
	workerIdent, _ := projects.CreateIdentity(ctx, owner.ID, proj.ID, workerPos.ID, worker.ID)

	plan, _ := svc.CreatePlanDraft(ctx, owner.ID, proj.ID, "验收计划", "", "", &reviewerIdent.ID, nil)
	task, err := svc.CreateTaskDraft(ctx, owner.ID, proj.ID, work.TaskDraft{
		Title: "T", PlanID: &plan.ID,
		ParticipantIDs:     []uuid.UUID{workerIdent.ID},
		ReviewerIdentityID: &reviewerIdent.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	activateTask(t, pool, task.ID)
	// Deliver.
	if _, _, err := svc.Report(ctx, worker.ID, proj.ID, work.ReportInput{
		TaskID: task.ID, IdentityID: &workerIdent.ID, Kind: "delivery",
		Text: "完成", ExpectedTaskVersion: 2,
	}); err != nil {
		t.Fatal(err)
	}
	review, err := svc.TaskAcceptanceReview(ctx, owner.ID, proj.ID, task.ID)
	if err != nil || review.ReviewHash == "" {
		t.Fatalf("review: %v %+v", err, review)
	}
	// Worker (not reviewer) cannot accept.
	if _, err := svc.DecideTaskAcceptance(ctx, worker.ID, proj.ID, task.ID, review.ReviewHash, true, "", 3); errors.IsCode(err, errors.Forbidden) == false {
		t.Fatalf("non-reviewer accept must be forbidden, got %v", err)
	}
	// Changing the reviewed agreement invalidates the open snapshot. A delivered
	// task rejects progress; use a fixture version change to exercise stale review.
	if _, _, err := svc.Report(ctx, worker.ID, proj.ID, work.ReportInput{TaskID: task.ID, IdentityID: &workerIdent.ID, Kind: "progress", Text: "late", ExpectedTaskVersion: 3}); !errors.IsCode(err, errors.InvalidTransition) {
		t.Fatalf("delivered progress: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE tasks SET version=4 WHERE id=$1`, task.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.DecideTaskAcceptance(ctx, owner.ID, proj.ID, task.ID, review.ReviewHash, true, "", 4); errors.IsCode(err, errors.ReviewStale) == false {
		t.Fatalf("stale review must be rejected, got %v", err)
	}
	fresh, _ := svc.TaskAcceptanceReview(ctx, owner.ID, proj.ID, task.ID)
	// Reject with reason -> rework.
	if _, err := svc.DecideTaskAcceptance(ctx, owner.ID, proj.ID, task.ID, fresh.ReviewHash, false, "缺文档", 4); err != nil {
		t.Fatalf("reject: %v", err)
	}
	// Redeliver and accept.
	if _, _, err := svc.Report(ctx, worker.ID, proj.ID, work.ReportInput{
		TaskID: task.ID, IdentityID: &workerIdent.ID, Kind: "delivery",
		Text: "补齐文档", ExpectedTaskVersion: 5,
	}); err != nil {
		t.Fatal(err)
	}
	final, _ := svc.TaskAcceptanceReview(ctx, owner.ID, proj.ID, task.ID)
	accepted, err := svc.DecideTaskAcceptance(ctx, owner.ID, proj.ID, task.ID, final.ReviewHash, true, "", 6)
	if err != nil || accepted.Status != "accepted" {
		t.Fatalf("accept: %v %+v", err, accepted)
	}
	var acceptanceCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM task_acceptances WHERE task_id=$1`, task.ID).Scan(&acceptanceCount); err != nil || acceptanceCount != 1 {
		t.Fatalf("immutable acceptance row expected, got %d", acceptanceCount)
	}

	// Activate the plan, then take the review (hash covers plan version).
	if _, err := pool.Exec(ctx, `UPDATE plans SET status='active', version=2 WHERE id=$1`, plan.ID); err != nil {
		t.Fatal(err)
	}
	planReview, err := svc.PlanAcceptanceReview(ctx, owner.ID, proj.ID, plan.ID)
	if err != nil || len(planReview.Blockers) != 0 {
		t.Fatalf("plan review after all tasks accepted: %+v %v", planReview, err)
	}
	acceptedPlan, err := svc.DecidePlanAcceptance(ctx, owner.ID, proj.ID, plan.ID, planReview.ReviewHash, true, "")
	if err != nil || acceptedPlan.Status != "accepted" {
		t.Fatalf("plan accept: %v %+v", err, acceptedPlan)
	}
	// Reopen keeps history.
	var acceptanceID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM plan_acceptances WHERE plan_id=$1`, plan.ID).Scan(&acceptanceID); err != nil {
		t.Fatal(err)
	}
	reopened, err := svc.ReopenPlan(ctx, owner.ID, proj.ID, plan.ID, acceptanceID, "线上问题")
	if err != nil || reopened.Status != "active" {
		t.Fatalf("plan reopen: %v %+v", err, reopened)
	}
	var planAcceptanceCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM plan_acceptances WHERE plan_id=$1`, plan.ID).Scan(&planAcceptanceCount); err != nil || planAcceptanceCount != 1 {
		t.Fatalf("plan acceptance history must persist, got %d", planAcceptanceCount)
	}
}

// TestPlanAcceptanceRequiresAllTasks (B12): 任务未全部验收时 owner 验收被
// REQUIREMENT_UNMET 拒绝。
func TestPlanAcceptanceRequiresAllTasks(t *testing.T) {
	svc, _, projects, ids, pool := newHandoffEnv(t)
	ctx := context.Background()
	owner, _, _ := ids.Register(ctx, "WA", "wa@wa.test", "password-wa-wa-1", "10.0.0.1")
	proj, _ := projects.Create(ctx, owner.ID, project.CreateRequest{Title: "部分验收项目"})
	plan, _ := svc.CreatePlanDraft(ctx, owner.ID, proj.ID, "P", "", "", nil, nil)
	task, _ := svc.CreateTaskDraft(ctx, owner.ID, proj.ID, work.TaskDraft{Title: "T", PlanID: &plan.ID})
	activateTask(t, pool, task.ID)
	if _, err := pool.Exec(ctx, `UPDATE plans SET status='active', version=2 WHERE id=$1`, plan.ID); err != nil {
		t.Fatal(err)
	}
	review, err := svc.PlanAcceptanceReview(ctx, owner.ID, proj.ID, plan.ID)
	if err != nil || len(review.Blockers) == 0 {
		t.Fatalf("partial plan must expose blockers: %+v %v", review, err)
	}
	if _, err := svc.DecidePlanAcceptance(ctx, owner.ID, proj.ID, plan.ID, review.ReviewHash, true, ""); errors.IsCode(err, errors.RequirementUnmet) == false {
		t.Fatalf("plan accept with unaccepted tasks must be REQUIREMENT_UNMET, got %v", err)
	}
}
