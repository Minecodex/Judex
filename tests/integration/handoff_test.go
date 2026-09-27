// SPDX-License-Identifier: Apache-2.0
package integrationtest_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/handoff"
	"github.com/kakj-go/Judex/internal/identity"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	"github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/project"
	"github.com/kakj-go/Judex/internal/work"
	integration "github.com/kakj-go/Judex/tests/integration"
)

func newHandoffEnv(t *testing.T) (*work.Service, *handoff.Service, *project.Service, *identity.Service, *postgres.Pool) {
	t.Helper()
	fixture := integration.StartPG(t)
	limiter := identity.NewRateLimiter(fixture.Pool.Pool, nil)
	ids := identity.NewService(fixture.Pool, limiter, identity.Options{RegisterPerIP: 1000}, nil)
	return work.NewService(fixture.Pool, nil), handoff.NewService(fixture.Pool, nil),
		project.NewService(fixture.Pool, nil), ids, fixture.Pool
}

// activateTask flips a draft task to ready directly (proposal activation is
// already proven; here we exercise reports/handoffs).
func activateTask(t *testing.T, pool *postgres.Pool, taskID uuid.UUID) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`UPDATE tasks SET status='ready', version=2 WHERE id=$1`, taskID); err != nil {
		t.Fatal(err)
	}
}

// TestWorkReports (B10 前置/03 §2): progress 保持状态；delivery → delivered；
// 身份校验拒绝非绑定人；同 submission 幂等不双建报告；accepted 任务拒绝报告。
func TestWorkReports(t *testing.T) {
	svc, _, projects, ids, pool := newHandoffEnv(t)
	ctx := context.Background()
	owner, _, _ := ids.Register(ctx, "RA", "ra@rp.test", "password-ra-ra-1", "10.0.0.1")
	worker, _, _ := ids.Register(ctx, "RB", "rb@rp.test", "password-rb-rb-1", "10.0.0.1")
	proj, _ := projects.Create(ctx, owner.ID, project.CreateRequest{Title: "报告项目"})
	if _, err := pool.Exec(ctx, `
		INSERT INTO project_members (project_id, user_id, role, state, joined_at)
		VALUES ($1,$2,'member','active',now())`, proj.ID, worker.ID); err != nil {
		t.Fatal(err)
	}
	pos, _ := projects.CreatePosition(ctx, owner.ID, proj.ID, project.PositionDraft{Name: "执行"})
	workerIdent, _ := projects.CreateIdentity(ctx, owner.ID, proj.ID, pos.ID, worker.ID)
	plan, _ := svc.CreatePlanDraft(ctx, owner.ID, proj.ID, "P", "", "", nil, nil)
	task, err := svc.CreateTaskDraft(ctx, owner.ID, proj.ID, work.TaskDraft{
		Title: "T", PlanID: &plan.ID, ParticipantIDs: []uuid.UUID{workerIdent.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	activateTask(t, pool, task.ID)

	// Non-holder identity report rejected.
	_, _, err = svc.Report(ctx, owner.ID, proj.ID, work.ReportInput{
		TaskID: task.ID, IdentityID: &workerIdent.ID, Kind: "progress",
		Text: "越权", ExpectedTaskVersion: 2,
	})
	if errors.IsCode(err, errors.Forbidden) == false {
		t.Fatalf("non-holder must be forbidden, got %v", err)
	}
	// Progress keeps working... first start? Report requires ready/working.
	_, version, err := svc.Report(ctx, worker.ID, proj.ID, work.ReportInput{
		TaskID: task.ID, IdentityID: &workerIdent.ID, Kind: "progress",
		Text: "进行中", ExpectedTaskVersion: 2,
	})
	if err != nil || version != 3 {
		t.Fatalf("progress report: %v version=%d", err, version)
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM tasks WHERE id=$1`, task.ID).Scan(&status); err != nil || status != "ready" {
		t.Fatalf("progress must keep status, got %s", status)
	}
	// Delivery flips delivered.
	if _, _, err := svc.Report(ctx, worker.ID, proj.ID, work.ReportInput{
		TaskID: task.ID, IdentityID: &workerIdent.ID, Kind: "delivery",
		Text: "完成", ExpectedTaskVersion: 3,
	}); err != nil {
		t.Fatalf("delivery: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT status FROM tasks WHERE id=$1`, task.ID).Scan(&status); err != nil || status != "delivered" {
		t.Fatalf("delivery must set delivered, got %s", status)
	}
	// Same submission idempotent.
	subID := uuid.New()
	first, _, err := svc.Report(ctx, worker.ID, proj.ID, work.ReportInput{
		TaskID: task.ID, IdentityID: &workerIdent.ID, Kind: "progress",
		SubmissionID: &subID, Text: "补充", ExpectedTaskVersion: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := svc.Report(ctx, worker.ID, proj.ID, work.ReportInput{
		TaskID: task.ID, IdentityID: &workerIdent.ID, Kind: "progress",
		SubmissionID: &subID, Text: "补充", ExpectedTaskVersion: 5,
	})
	if err != nil || first != second {
		t.Fatalf("same submission must return the same report: %v %s/%s", err, first, second)
	}
	// Accepted task rejects reports.
	if _, err := pool.Exec(ctx, `UPDATE tasks SET status='accepted', version=10 WHERE id=$1`, task.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Report(ctx, worker.ID, proj.ID, work.ReportInput{
		TaskID: task.ID, IdentityID: &workerIdent.ID, Kind: "progress",
		Text: "迟到", ExpectedTaskVersion: 10,
	}); errors.IsCode(err, errors.InvalidTransition) == false {
		t.Fatalf("accepted task must reject reports, got %v", err)
	}
}

// TestHandoffSourceLevel (B10/B13): 分来源发送/接收/拒收/补交；拒收未验收
// 来源任务→rework；整包聚合状态正确；补交保留旧决定。
func TestHandoffSourceLevel(t *testing.T) {
	svc, hsvc, projects, ids, pool := newHandoffEnv(t)
	ctx := context.Background()
	owner, _, _ := ids.Register(ctx, "HA", "ha@hp.test", "password-ha-ha-1", "10.0.0.1")
	sender, _, _ := ids.Register(ctx, "HB", "hb@hp.test", "password-hb-hb-1", "10.0.0.1")
	receiver, _, _ := ids.Register(ctx, "HC", "hc@hp.test", "password-hc-hc-1", "10.0.0.1")
	proj, _ := projects.Create(ctx, owner.ID, project.CreateRequest{Title: "交接项目"})
	for _, u := range []uuid.UUID{sender.ID, receiver.ID} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO project_members (project_id, user_id, role, state, joined_at)
			VALUES ($1,$2,'member','active',now())`, proj.ID, u); err != nil {
			t.Fatal(err)
		}
	}
	sendPos, _ := projects.CreatePosition(ctx, owner.ID, proj.ID, project.PositionDraft{Name: "发送岗"})
	recvPos, _ := projects.CreatePosition(ctx, owner.ID, proj.ID, project.PositionDraft{Name: "接收岗"})
	senderIdent, _ := projects.CreateIdentity(ctx, owner.ID, proj.ID, sendPos.ID, sender.ID)
	receiverIdent, _ := projects.CreateIdentity(ctx, owner.ID, proj.ID, recvPos.ID, receiver.ID)
	plan, _ := svc.CreatePlanDraft(ctx, owner.ID, proj.ID, "P", "", "", nil, nil)
	srcTask, _ := svc.CreateTaskDraft(ctx, sender.ID, proj.ID, work.TaskDraft{
		Title: "来源任务", PlanID: &plan.ID, ParticipantIDs: []uuid.UUID{senderIdent.ID},
	})
	dstTask, _ := svc.CreateTaskDraft(ctx, owner.ID, proj.ID, work.TaskDraft{
		Title: "目标任务", PlanID: &plan.ID, Requirements: []work.Requirement{{
			Phase: "start", Kind: "handoff_receipt", TargetID: srcTask.ID, Hard: true,
		}},
	})
	activateTask(t, pool, srcTask.ID)

	hand, err := hsvc.Create(ctx, owner.ID, proj.ID, "首次交接", dstTask.ID, receiverIdent.ID, "dependency",
		[]struct {
			SourceTaskID     uuid.UUID
			SenderIdentityID uuid.UUID
		}{{srcTask.ID, senderIdent.ID}})
	if err != nil {
		t.Fatal(err)
	}
	// Receiver cannot send; only sender.
	if _, err := hsvc.SendSource(ctx, receiver.ID, proj.ID, hand.ID, hand.Sources[0].ID, "冒充"); errors.IsCode(err, errors.Forbidden) == false {
		t.Fatalf("non-sender send must be forbidden, got %v", err)
	}
	if _, err := hsvc.SendSource(ctx, sender.ID, proj.ID, hand.ID, hand.Sources[0].ID, "首版成果"); err != nil {
		t.Fatalf("send: %v", err)
	}
	// Receiver rejects: source task not accepted -> rework.
	if _, err := hsvc.DecideSource(ctx, receiver.ID, proj.ID, hand.ID, hand.Sources[0].ID, false, "缺测试证据"); err != nil {
		t.Fatalf("reject: %v", err)
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM tasks WHERE id=$1`, srcTask.ID).Scan(&status); err != nil || status != "rework" {
		t.Fatalf("rejected unaccepted source task must go rework, got %s", status)
	}
	list, err := hsvc.List(ctx, owner.ID, proj.ID)
	if err != nil || list[0].State != "needs_revision" {
		t.Fatalf("aggregate must be needs_revision, got %+v", list)
	}
	// 补交: sender sends a new version (revision 2), old decision kept.
	revised, err := hsvc.SendSource(ctx, sender.ID, proj.ID, hand.ID, hand.Sources[0].ID, "补充测试后的版本")
	if err != nil || revised.CurrentVersion == nil || *revised.CurrentVersion != 2 {
		t.Fatalf("revision send: %+v %v", revised, err)
	}
	// Receiver accepts revision 2.
	if _, err := hsvc.DecideSource(ctx, receiver.ID, proj.ID, hand.ID, hand.Sources[0].ID, true, ""); err != nil {
		t.Fatalf("accept: %v", err)
	}
	list, _ = hsvc.List(ctx, owner.ID, proj.ID)
	if list[0].State != "accepted" {
		t.Fatalf("aggregate must be accepted, got %s", list[0].State)
	}
	// The rejected first version keeps its decision record.
	var rejected int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM source_decisions WHERE decision='reject'`).Scan(&rejected); err != nil || rejected != 1 {
		t.Fatalf("old decision must persist: %v %d", err, rejected)
	}

	// B13: accepted source task rejected -> no silent unacceptance.
	srcTask2, _ := svc.CreateTaskDraft(ctx, sender.ID, proj.ID, work.TaskDraft{
		Title: "已验收来源", PlanID: &plan.ID, ParticipantIDs: []uuid.UUID{senderIdent.ID},
	})
	if _, err := pool.Exec(ctx, `UPDATE tasks SET status='accepted', version=5 WHERE id=$1`, srcTask2.ID); err != nil {
		t.Fatal(err)
	}
	hand2, err := hsvc.Create(ctx, owner.ID, proj.ID, "二次交接", dstTask.ID, receiverIdent.ID, "stage",
		[]struct {
			SourceTaskID     uuid.UUID
			SenderIdentityID uuid.UUID
		}{{srcTask2.ID, senderIdent.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hsvc.SendSource(ctx, sender.ID, proj.ID, hand2.ID, hand2.Sources[0].ID, "成果"); err != nil {
		t.Fatal(err)
	}
	if _, err := hsvc.DecideSource(ctx, receiver.ID, proj.ID, hand2.ID, hand2.Sources[0].ID, false, "与目标不符"); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT status FROM tasks WHERE id=$1`, srcTask2.ID).Scan(&status); err != nil || status != "accepted" {
		t.Fatalf("accepted task must NOT be unaccepted by receiver, got %s", status)
	}
}

// TestMyActions (B12/B15 读模型): 按当前绑定计算统一待办——pending 提案、
// 待接收交接、待验收任务、全验收计划；无关用户为空。
func TestMyActions(t *testing.T) {
	svc, hsvc, projects, ids, pool := newHandoffEnv(t)
	ctx := context.Background()
	owner, _, _ := ids.Register(ctx, "OA", "oa@oa.test", "password-oa-oa-1", "10.0.0.1")
	worker, _, _ := ids.Register(ctx, "OB", "ob@oa.test", "password-ob-ob-1", "10.0.0.1")
	proj, _ := projects.Create(ctx, owner.ID, project.CreateRequest{Title: "待办项目"})
	if err := projects.JoinDirect(ctx, proj.ID, worker.ID); err != nil {
		t.Fatal(err)
	}
	revPos, _ := projects.CreatePosition(ctx, owner.ID, proj.ID, project.PositionDraft{Name: "验收"})
	revIdent, _ := projects.CreateIdentity(ctx, owner.ID, proj.ID, revPos.ID, owner.ID)
	wPos, _ := projects.CreatePosition(ctx, owner.ID, proj.ID, project.PositionDraft{Name: "执行"})
	wIdent, _ := projects.CreateIdentity(ctx, owner.ID, proj.ID, wPos.ID, worker.ID)

	plan, _ := svc.CreatePlanDraft(ctx, owner.ID, proj.ID, "P", "", "", &revIdent.ID, nil)
	task, _ := svc.CreateTaskDraft(ctx, owner.ID, proj.ID, work.TaskDraft{
		Title: "T", PlanID: &plan.ID, ParticipantIDs: []uuid.UUID{wIdent.ID},
		ReviewerIdentityID: &revIdent.ID,
	})
	activateTask(t, pool, task.ID)
	if _, err := pool.Exec(ctx, `UPDATE plans SET status='active', version=2 WHERE id=$1`, plan.ID); err != nil {
		t.Fatal(err)
	}
	// Deliver -> owner should see an accept action.
	if _, _, err := svc.Report(ctx, worker.ID, proj.ID, work.ReportInput{
		TaskID: task.ID, IdentityID: &wIdent.ID, Kind: "delivery",
		Text: "done", ExpectedTaskVersion: 2,
	}); err != nil {
		t.Fatal(err)
	}
	ownerActions, err := svc.MyActions(ctx, owner.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	hasAccept := false
	for _, a := range ownerActions {
		if a.ObjectType == "task" && a.Kind == "accept" {
			hasAccept = true
		}
	}
	if !hasAccept {
		t.Fatalf("owner must see a task accept action: %+v", ownerActions)
	}
	// Worker (not reviewer) sees none of that.
	workerActions, _ := svc.MyActions(ctx, worker.ID, nil)
	for _, a := range workerActions {
		if a.ObjectType == "task" && a.Kind == "accept" {
			t.Fatalf("worker must not see reviewer accept action: %+v", a)
		}
	}
	// Accept task -> owner sees plan-level accept action.
	review, _ := svc.TaskAcceptanceReview(ctx, owner.ID, proj.ID, task.ID)
	if _, err := svc.DecideTaskAcceptance(ctx, owner.ID, proj.ID, task.ID, review.ReviewHash, true, "", 3); err != nil {
		t.Fatal(err)
	}
	ownerActions, _ = svc.MyActions(ctx, owner.ID, nil)
	hasPlanAccept := false
	for _, a := range ownerActions {
		if a.ObjectType == "plan" && a.Kind == "accept" {
			hasPlanAccept = true
		}
	}
	if !hasPlanAccept {
		t.Fatalf("owner must see plan accept action after all tasks accepted: %+v", ownerActions)
	}
	// Handoff receipt waiting for receiver.
	dst, _ := svc.CreateTaskDraft(ctx, owner.ID, proj.ID, work.TaskDraft{Title: "D", PlanID: &plan.ID})
	h, err := hsvc.Create(ctx, owner.ID, proj.ID, "交接", dst.ID, wIdent.ID, "stage",
		[]struct {
			SourceTaskID     uuid.UUID
			SenderIdentityID uuid.UUID
		}{{task.ID, revIdent.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hsvc.SendSource(ctx, owner.ID, proj.ID, h.ID, h.Sources[0].ID, "成果"); err != nil {
		t.Fatal(err)
	}
	workerActions, _ = svc.MyActions(ctx, worker.ID, nil)
	hasReceive := false
	for _, a := range workerActions {
		if a.ObjectType == "handoff" && a.Kind == "receive" {
			hasReceive = true
		}
	}
	if !hasReceive {
		t.Fatalf("worker (receiver) must see handoff receipt action: %+v", workerActions)
	}
}
