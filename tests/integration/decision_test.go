// SPDX-License-Identifier: Apache-2.0
package integrationtest_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/decision"
	"github.com/kakj-go/Judex/internal/identity"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	"github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/project"
	integration "github.com/kakj-go/Judex/tests/integration"
)

func newDecisionEnv(t *testing.T) (*decision.Service, *project.Service, *identity.Service, *postgres.Pool) {
	t.Helper()
	fixture := integration.StartPG(t)
	limiter := identity.NewRateLimiter(fixture.Pool.Pool, nil)
	ids := identity.NewService(fixture.Pool, limiter, identity.Options{RegisterPerIP: 1000}, nil)
	return decision.NewService(fixture.Pool, nil, 3600), project.NewService(fixture.Pool, nil), ids, fixture.Pool
}

// TestProposalAllSlotsAtomicApply (B03/B04/B06 核心): 固定审阅+ALL 会签；
// 同一真人多身份一次覆盖；任一退回取消整案；全部同意后原子建计划+任务。
func TestProposalAllSlotsAtomicApply(t *testing.T) {
	svc, projects, ids, pool := newDecisionEnv(t)
	ctx := context.Background()
	owner, _, _ := ids.Register(ctx, "PA", "pa@pa.test", "password-pa-pa-1", "10.0.0.1")
	worker, _, _ := ids.Register(ctx, "PW", "pw@pw.test", "password-pw-pw-1", "10.0.0.1")
	proj, err := projects.Create(ctx, owner.ID, project.CreateRequest{Title: "审批项目"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO project_members (project_id, user_id, role, state, joined_at)
		VALUES ($1,$2,'member','active',now())`, proj.ID, worker.ID); err != nil {
		t.Fatal(err)
	}
	// owner 持两职（同真人多身份），worker 持一职。
	var ownerBackend, ownerReviewer, workerDev uuid.UUID
	for _, spec := range []struct {
		id     *uuid.UUID
		name   string
		holder uuid.UUID
	}{
		{&ownerBackend, "后端", owner.ID},
		{&ownerReviewer, "评审", owner.ID},
		{&workerDev, "开发", worker.ID},
	} {
		pos, err := projects.CreatePosition(ctx, owner.ID, proj.ID, project.PositionDraft{Name: spec.name})
		if err != nil {
			t.Fatal(err)
		}
		ident, err := projects.CreateIdentity(ctx, owner.ID, proj.ID, pos.ID, spec.holder)
		if err != nil {
			t.Fatal(err)
		}
		*spec.id = ident.ID
	}
	_ = ownerBackend
	_ = ownerReviewer
	_ = workerDev

	changes := []decision.Change{
		{Operation: "create_plan", TargetType: "plan", ClientRef: "plan1",
			Fields: map[string]any{"title": "正式计划", "ownerIdentityId": ownerBackend.String()}},
		{Operation: "create_task", TargetType: "task", ClientRef: "task1",
			Fields: map[string]any{
				"title": "实现功能", "planId": "plan1",
				"reviewerIdentityId": ownerReviewer.String(),
				"participantIdentityIds": []any{workerDev.String()},
			}},
	}
	proposalID, err := svc.CreateDraft(ctx, owner.ID, proj.ID, "work_arrangement", nil, "", changes)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Submit(ctx, owner.ID, proj.ID, proposalID, 1, ""); err != nil {
		t.Fatalf("submit: %v", err)
	}
	// Fetch review hash.
	var reviewID uuid.UUID
	var reviewHash string
	if err := pool.QueryRow(ctx, `SELECT id, review_hash FROM proposal_versions WHERE proposal_id=$1`, proposalID).
		Scan(&reviewID, &reviewHash); err != nil {
		t.Fatal(err)
	}
	// Worker (non-holder of any seat... wait, worker IS participant) tries an
	// unrelated third user: outsider cannot decide.
	outsider, _, _ := ids.Register(ctx, "PX", "px@px.test", "password-px-px-1", "10.0.0.1")
	if _, err := svc.Decide(ctx, outsider.ID, proj.ID, proposalID, reviewHash, true, ""); errors.IsCode(err, errors.Forbidden) == false {
		t.Fatalf("outsider decide must be forbidden, got %v", err)
	}
	// Worker approves their seat.
	if _, err := svc.Decide(ctx, worker.ID, proj.ID, proposalID, reviewHash, true, ""); err != nil {
		t.Fatal(err)
	}
	// Proposal still pending (owner seats unsatisfied).
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM proposals WHERE id=$1`, proposalID).Scan(&status); err != nil || status != "pending" {
		t.Fatalf("expected pending after partial approval, got %s", status)
	}
	// Owner (covering TWO seats: plan owner + reviewer) approves once.
	result, err := svc.Decide(ctx, owner.ID, proj.ID, proposalID, reviewHash, true, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT status FROM proposals WHERE id=$1`, proposalID).Scan(&status); err != nil || status != "approved" {
		t.Fatalf("expected approved after ALL seats, got %s", status)
	}
	// Atomic apply: plan active + task ready with participants.
	var planStatus, taskStatus string
	var planID, taskID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id, status FROM plans WHERE project_id=$1`, proj.ID).Scan(&planID, &planStatus); err != nil || planStatus != "active" {
		t.Fatalf("plan not applied: %v %s", err, planStatus)
	}
	if err := pool.QueryRow(ctx, `SELECT id, status FROM tasks WHERE project_id=$1`, proj.ID).Scan(&taskID, &taskStatus); err != nil || taskStatus != "ready" {
		t.Fatalf("task not applied: %v %s", err, taskStatus)
	}
	var participants int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM task_participants WHERE task_id=$1`, taskID).Scan(&participants); err != nil || participants != 1 {
		t.Fatalf("participants: %v %d", err, participants)
	}
	if result.CreatedIDs["plan1"] != planID.String() || result.CreatedIDs["task1"] != taskID.String() {
		t.Fatalf("createdIds mapping: %+v", result.CreatedIDs)
	}
	// Late decisions conflict (approved proposal).
	if _, err := svc.Decide(ctx, worker.ID, proj.ID, proposalID, reviewHash, false, "迟到反对"); errors.IsCode(err, errors.InvalidTransition) == false {
		t.Fatalf("late decision must conflict, got %v", err)
	}
}

// TestProposalRejectCancels (B04): 任一退回取消整案并停止计时。
func TestProposalRejectCancels(t *testing.T) {
	svc, projects, ids, pool := newDecisionEnv(t)
	ctx := context.Background()
	owner, _, _ := ids.Register(ctx, "RA", "ra@ra.test", "password-ra-ra-1", "10.0.0.1")
	other, _, _ := ids.Register(ctx, "RB", "rb@rb.test", "password-rb-rb-1", "10.0.0.1")
	proj, _ := projects.Create(ctx, owner.ID, project.CreateRequest{Title: "退回项目"})
	pos, _ := projects.CreatePosition(ctx, owner.ID, proj.ID, project.PositionDraft{Name: "工程"})
	ownerIdent, _ := projects.CreateIdentity(ctx, owner.ID, proj.ID, pos.ID, owner.ID)
	// add other as member with a seat
	if _, err := pool.Exec(ctx, `
		INSERT INTO project_members (project_id, user_id, role, state, joined_at)
		VALUES ($1,$2,'member','active',now())`, proj.ID, other.ID); err != nil {
		t.Fatal(err)
	}
	otherIdent, err := projects.CreateIdentity(ctx, owner.ID, proj.ID, pos.ID, other.ID)
	if err != nil {
		t.Fatal(err)
	}
	proposalID, err := svc.CreateDraft(ctx, owner.ID, proj.ID, "work_change", nil, "",
		[]decision.Change{{Operation: "create_task", TargetType: "task",
			Fields: map[string]any{"title": "变更任务", "participantIdentityIds": []any{otherIdent.ID.String()}}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Submit(ctx, owner.ID, proj.ID, proposalID, 1, ""); err != nil {
		t.Fatal(err)
	}
	var reviewHash string
	if err := pool.QueryRow(ctx, `SELECT review_hash FROM proposal_versions WHERE proposal_id=$1`, proposalID).Scan(&reviewHash); err != nil {
		t.Fatal(err)
	}
	// other rejects.
	if _, err := svc.Decide(ctx, other.ID, proj.ID, proposalID, reviewHash, false, "方向不对"); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM proposals WHERE id=$1`, proposalID).Scan(&status); err != nil || status != "cancelled" {
		t.Fatalf("reject must cancel the proposal, got %s", status)
	}
	// No partial application happened.
	var tasks int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM tasks WHERE project_id=$1`, proj.ID).Scan(&tasks); err != nil || tasks != 0 {
		t.Fatalf("cancelled proposal must not apply changes, got %d tasks", tasks)
	}
	_ = ownerIdent
}

// TestProposalTimeoutSettle (B05/B06): 首票后超时 worker 结算为通过；
// 无首票不超时通过。
func TestProposalTimeoutSettle(t *testing.T) {
	shortTimeout := decision.NewService(nil, func() time.Time { return time.Now().UTC() }, 1)
	fixture := integration.StartPG(t)
	svc := decision.NewService(fixture.Pool, nil, 1)
	_ = shortTimeout
	limiter := identity.NewRateLimiter(fixture.Pool.Pool, nil)
	ids := identity.NewService(fixture.Pool, limiter, identity.Options{RegisterPerIP: 1000}, nil)
	projects := project.NewService(fixture.Pool, nil)
	ctx := context.Background()
	owner, _, _ := ids.Register(ctx, "TA", "ta@ta.test", "password-ta-ta-1", "10.0.0.1")
	other, _, _ := ids.Register(ctx, "TB", "tb@tb.test", "password-tb-tb-1", "10.0.0.1")
	proj, _ := projects.Create(ctx, owner.ID, project.CreateRequest{Title: "超时项目"})
	if _, err := fixture.Pool.Exec(ctx, `
		INSERT INTO project_members (project_id, user_id, role, state, joined_at)
		VALUES ($1,$2,'member','active',now())`, proj.ID, other.ID); err != nil {
		t.Fatal(err)
	}
	pos, _ := projects.CreatePosition(ctx, owner.ID, proj.ID, project.PositionDraft{Name: "岗"})
	otherIdent, _ := projects.CreateIdentity(ctx, owner.ID, proj.ID, pos.ID, other.ID)

	ownerPos, _ := projects.CreatePosition(ctx, owner.ID, proj.ID, project.PositionDraft{Name: "负责人岗"})
	ownerIdent, _ := projects.CreateIdentity(ctx, owner.ID, proj.ID, ownerPos.ID, owner.ID)
	proposalID, err := svc.CreateDraft(ctx, owner.ID, proj.ID, "work_arrangement", nil, "",
		[]decision.Change{
			{Operation: "create_plan", TargetType: "plan", ClientRef: "p1",
				Fields: map[string]any{"title": "超时计划", "ownerIdentityId": ownerIdent.ID.String()}},
			{Operation: "create_task", TargetType: "task",
				Fields: map[string]any{"title": "超时任务", "planId": "p1",
					"participantIdentityIds": []any{otherIdent.ID.String()}}},
		})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Submit(ctx, owner.ID, proj.ID, proposalID, 1, ""); err != nil {
		t.Fatal(err)
	}
	var reviewID uuid.UUID
	var reviewHash string
	if err := fixture.Pool.QueryRow(ctx, `SELECT id, review_hash FROM proposal_versions WHERE proposal_id=$1`, proposalID).
		Scan(&reviewID, &reviewHash); err != nil {
		t.Fatal(err)
	}
	// No first approval: timeout settle must NOT pass the proposal.
	if err := svc.SettleTimeout(ctx, proj.ID, proposalID, reviewID); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := fixture.Pool.QueryRow(ctx, `SELECT status FROM proposals WHERE id=$1`, proposalID).Scan(&status); err != nil || status != "pending" {
		t.Fatalf("no-first-approval timeout must not apply, got %s", status)
	}
	// First approval by other, then timeout settles to approved.
	if _, err := svc.Decide(ctx, other.ID, proj.ID, proposalID, reviewHash, true, ""); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond)
	if err := svc.SettleTimeout(ctx, proj.ID, proposalID, reviewID); err != nil {
		t.Fatal(err)
	}
	if err := fixture.Pool.QueryRow(ctx, `SELECT status FROM proposals WHERE id=$1`, proposalID).Scan(&status); err != nil || status != "approved" {
		t.Fatalf("timeout settle must apply after first approval + deadline, got %s", status)
	}
	var decisionSource string
	if err := fixture.Pool.QueryRow(ctx, `SELECT decision_source FROM approval_decisions WHERE review_id=$1 AND decision_source='timeout'`, reviewID).Scan(&decisionSource); err != nil || decisionSource != "timeout" {
		t.Fatalf("timeout decision must be recorded as timeout actor, got %v %s", err, decisionSource)
	}
}

// TestDelegateAndRevision (B07/B04 修订): manager 代批 pending 席位（记录
// delegate 来源，不冒充原人）；修订创建新草稿 revision 且旧票不继承。
func TestDelegateAndRevision(t *testing.T) {
	svc, projects, ids, pool := newDecisionEnv(t)
	ctx := context.Background()
	owner, _, _ := ids.Register(ctx, "MA", "ma@ma.test", "password-ma-ma-1", "10.0.0.1")
	manager, _, _ := ids.Register(ctx, "MB", "mb@mb.test", "password-mb-mb-1", "10.0.0.1")
	member, _, _ := ids.Register(ctx, "MC", "mc@mc.test", "password-mc-mc-1", "10.0.0.1")
	proj, _ := projects.Create(ctx, owner.ID, project.CreateRequest{Title: "代批项目"})
	for _, u := range []uuid.UUID{manager.ID, member.ID} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO project_members (project_id, user_id, role, state, joined_at)
			VALUES ($1,$2,'member','active',now())`, proj.ID, u); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := projects.UpdateMemberRole(ctx, owner.ID, proj.ID, manager.ID, proj.Version, "manager"); err != nil {
		t.Fatal(err)
	}
	pos, _ := projects.CreatePosition(ctx, owner.ID, proj.ID, project.PositionDraft{Name: "岗"})
	memberIdent, _ := projects.CreateIdentity(ctx, owner.ID, proj.ID, pos.ID, member.ID)

	proposalID, err := svc.CreateDraft(ctx, owner.ID, proj.ID, "work_arrangement", nil, "",
		[]decision.Change{{Operation: "create_task", TargetType: "task",
			Fields: map[string]any{"title": "代批任务", "participantIdentityIds": []any{memberIdent.ID.String()}}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Submit(ctx, owner.ID, proj.ID, proposalID, 1, ""); err != nil {
		t.Fatal(err)
	}
	review, err := svc.GetReview(ctx, owner.ID, proj.ID, proposalID)
	if err != nil {
		t.Fatal(err)
	}
	// Member (plain, no delegation authority) cannot delegate.
	if _, err := svc.Delegate(ctx, member.ID, proj.ID, proposalID, review.ReviewHash, "越权"); errors.IsCode(err, errors.Forbidden) == false {
		t.Fatalf("plain member delegate must be forbidden, got %v", err)
	}
	// Manager delegates the pending seat -> approved + applied.
	if _, err := svc.Delegate(ctx, manager.ID, proj.ID, proposalID, review.ReviewHash, "出差代批"); err != nil {
		t.Fatalf("manager delegate: %v", err)
	}
	var status, source string
	if err := pool.QueryRow(ctx, `SELECT status FROM proposals WHERE id=$1`, proposalID).Scan(&status); err != nil || status != "approved" {
		t.Fatalf("delegate must complete the proposal, got %s", status)
	}
	if err := pool.QueryRow(ctx, `SELECT decision_source FROM approval_decisions WHERE review_id=$1`, review.ReviewID).Scan(&source); err != nil || source != "delegate" {
		t.Fatalf("decision must record delegate source, got %v %s", err, source)
	}
	var actor uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT actor_user_id FROM approval_decisions WHERE review_id=$1`, review.ReviewID).Scan(&actor); err != nil || actor != manager.ID {
		t.Fatalf("delegate must record the REAL actor, got %v", actor)
	}

	// Revision: terminal proposal -> new draft revision; old tickets stay.
	revision, err := svc.CreateRevision(ctx, owner.ID, proj.ID, proposalID,
		[]decision.Change{{Operation: "create_task", TargetType: "task",
			Fields: map[string]any{"title": "修订任务", "participantIdentityIds": []any{memberIdent.ID.String()}}}},
		"按退回意见修改")
	if err != nil || revision != 2 {
		t.Fatalf("revision: %v %d", err, revision)
	}
	if err := pool.QueryRow(ctx, `SELECT status FROM proposals WHERE id=$1`, proposalID).Scan(&status); err != nil || status != "draft" {
		t.Fatalf("revision must reset to draft, got %s", status)
	}
	// Old review slots remain with the OLD review id; a fresh submit makes new slots.
	if _, err := svc.Submit(ctx, owner.ID, proj.ID, proposalID, 3, ""); err != nil {
		t.Fatalf("resubmit: %v", err)
	}
	var slotCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM approval_slots s
		WHERE s.review_id=(SELECT current_review_id FROM proposals WHERE id=$1)`, proposalID).Scan(&slotCount); err != nil || slotCount != 1 {
		t.Fatalf("new review must have fresh slots, got %d", slotCount)
	}
}

// TestBindingReplacementContinuesPending (B08): 身份换绑后 pending 职责由新
// 绑定人接续；已同意席位保持；旧绑定人失去操作资格。
func TestBindingReplacementContinuesPending(t *testing.T) {
	svc, projects, ids, pool := newDecisionEnv(t)
	ctx := context.Background()
	owner, _, _ := ids.Register(ctx, "NA", "na@na.test", "password-na-na-1", "10.0.0.1")
	oldHolder, _, _ := ids.Register(ctx, "NB", "nb@nb.test", "password-nb-nb-1", "10.0.0.1")
	newHolder, _, _ := ids.Register(ctx, "NC", "nc@nc.test", "password-nc-nc-1", "10.0.0.1")
	proj, _ := projects.Create(ctx, owner.ID, project.CreateRequest{Title: "换人项目"})
	// Join both via direct membership (invitation path already proven).
	for _, u := range []uuid.UUID{oldHolder.ID, newHolder.ID} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO project_members (project_id, user_id, role, state, joined_at)
			VALUES ($1,$2,'member','active',now())`, proj.ID, u); err != nil {
			t.Fatal(err)
		}
	}
	seatPos, _ := projects.CreatePosition(ctx, owner.ID, proj.ID, project.PositionDraft{Name: "座席"})
	oldIdent, _ := projects.CreateIdentity(ctx, owner.ID, proj.ID, seatPos.ID, oldHolder.ID)
	otherPos, _ := projects.CreatePosition(ctx, owner.ID, proj.ID, project.PositionDraft{Name: "另席"})
	otherIdent, _ := projects.CreateIdentity(ctx, owner.ID, proj.ID, otherPos.ID, owner.ID)

	proposalID, err := svc.CreateDraft(ctx, owner.ID, proj.ID, "work_arrangement", nil, "",
		[]decision.Change{
			{Operation: "create_plan", TargetType: "plan",
				Fields: map[string]any{"title": "P", "ownerIdentityId": otherIdent.ID.String()}},
			{Operation: "create_task", TargetType: "task",
				Fields: map[string]any{"title": "T", "participantIdentityIds": []any{oldIdent.ID.String()}}},
		})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Submit(ctx, owner.ID, proj.ID, proposalID, 1, ""); err != nil {
		t.Fatal(err)
	}
	review, _ := svc.GetReview(ctx, owner.ID, proj.ID, proposalID)
	// Owner approves their own seat (first approval, starts timer).
	if _, err := svc.Decide(ctx, owner.ID, proj.ID, proposalID, review.ReviewHash, true, ""); err != nil {
		t.Fatal(err)
	}
	// Replace the old holder's binding: same identity, new bindingVersion.
	if _, err := projects.ReplaceIdentityBinding(ctx, owner.ID, proj.ID, oldIdent.ID, newHolder.ID, 1, "休假接替"); err != nil {
		t.Fatal(err)
	}
	// Old holder can no longer decide.
	if _, err := svc.Decide(ctx, oldHolder.ID, proj.ID, proposalID, review.ReviewHash, true, ""); errors.IsCode(err, errors.Forbidden) == false {
		t.Fatalf("old holder must lose eligibility, got %v", err)
	}
	// New holder continues the pending seat; first-approval time kept.
	if _, err := svc.Decide(ctx, newHolder.ID, proj.ID, proposalID, review.ReviewHash, true, ""); err != nil {
		t.Fatalf("new holder must continue the pending seat: %v", err)
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM proposals WHERE id=$1`, proposalID).Scan(&status); err != nil || status != "approved" {
		t.Fatalf("proposal must complete after continuation, got %s", status)
	}
	// Both seats end approved: the pre-replacement approval persisted (not
	// reopened) and the continued seat completed the quorum.
	var approvedSlots int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM approval_slots WHERE review_id=$1 AND state='approved'`, review.ReviewID).Scan(&approvedSlots); err != nil || approvedSlots != 2 {
		t.Fatalf("approved seats must persist (2 expected), got %d", approvedSlots)
	}
}
