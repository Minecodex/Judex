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
	"github.com/kakj-go/Judex/internal/work"
	integration "github.com/kakj-go/Judex/tests/integration"
)

func newWorkEnv(t *testing.T) (*work.Service, *project.Service, *identity.Service, *postgres.Pool) {
	t.Helper()
	fixture := integration.StartPG(t)
	limiter := identity.NewRateLimiter(fixture.Pool.Pool, nil)
	ids := identity.NewService(fixture.Pool, limiter, identity.Options{RegisterPerIP: 1000}, nil)
	return work.NewService(fixture.Pool, nil), project.NewService(fixture.Pool, nil), ids, fixture.Pool
}

// TestPlanTaskDrafts (B01/B02 核心): 草稿不自动成为正式工作；跨项目 plan
// 引用拒绝；父子必须同 plan；requirements kind/phase 白名单。
func TestPlanTaskDrafts(t *testing.T) {
	svc, projects, ids, _ := newWorkEnv(t)
	ctx := context.Background()
	owner, _, _ := ids.Register(ctx, "W", "w@w.test", "password-ww-ww-11", "10.0.0.1")
	proj, err := projects.Create(ctx, owner.ID, project.CreateRequest{Title: "工作项目"})
	if err != nil {
		t.Fatal(err)
	}
	otherProj, err := projects.Create(ctx, owner.ID, project.CreateRequest{Title: "另一项目"})
	if err != nil {
		t.Fatal(err)
	}

	plan, err := svc.CreatePlanDraft(ctx, owner.ID, proj.ID, "主线计划", "目标", "验收标准", nil, nil)
	if err != nil || plan.Status != "draft" {
		t.Fatalf("plan draft: %v %+v", err, plan)
	}
	plans, err := svc.ListPlans(ctx, owner.ID, proj.ID)
	if err != nil || len(plans) != 1 || plans[0].TaskStats.Total != 0 {
		t.Fatalf("plans: %v %+v", err, plans)
	}

	// Task draft under the plan.
	task, err := svc.CreateTaskDraft(ctx, owner.ID, proj.ID, work.TaskDraft{
		Title: "实现筛选", PlanID: &plan.ID, Kind: "task",
	})
	if err != nil || task.Status != "draft" {
		t.Fatalf("task draft: %v %+v", err, task)
	}
	// Parent must share the plan.
	if _, err := svc.CreateTaskDraft(ctx, owner.ID, proj.ID, work.TaskDraft{
		Title: "孤儿子任务", ParentTaskID: &task.ID,
	}); errors.IsCode(err, errors.InvalidReference) == false {
		t.Fatalf("parent without shared plan must be rejected, got %v", err)
	}
	child, err := svc.CreateTaskDraft(ctx, owner.ID, proj.ID, work.TaskDraft{
		Title: "子任务", PlanID: &plan.ID, ParentTaskID: &task.ID,
	})
	if err != nil {
		t.Fatalf("child with shared plan: %v", err)
	}
	// Foreign plan rejected.
	if _, err := svc.CreateTaskDraft(ctx, owner.ID, proj.ID, work.TaskDraft{
		Title: "越权任务", PlanID: &otherProj.ID,
	}); err == nil {
		t.Fatal("foreign plan reference must be rejected")
	}
	// Requirement kind whitelist.
	if _, err := svc.CreateTaskDraft(ctx, owner.ID, proj.ID, work.TaskDraft{
		Title: "非法前置", Requirements: []work.Requirement{{Phase: "start", Kind: "exec_sql", TargetID: task.ID}},
	}); errors.IsCode(err, errors.Validation) == false {
		t.Fatalf("non-whitelisted requirement must be rejected, got %v", err)
	}
	// Drafts stay drafts: stats count only after proposals activate them.
	plans, _ = svc.ListPlans(ctx, owner.ID, proj.ID)
	if plans[0].TaskStats.Total != 2 || plans[0].TaskStats.Active != 2 {
		t.Fatalf("draft tasks should aggregate as active drafts: %+v", plans[0].TaskStats)
	}
	_ = child
	_ = uuid.Nil
}

// TestStartBlockingAndMap (B09 核心): 未验收前置阻塞开始（REQUIREMENT_UNMET
// + blockers）；验收后可开始；执行图列序按前置深度。
func TestStartBlockingAndMap(t *testing.T) {
	svc, projects, ids, pool := newWorkEnv(t)
	ctx := context.Background()
	owner, _, _ := ids.Register(ctx, "S", "s@s.test", "password-ss-ss-11", "10.0.0.1")
	proj, _ := projects.Create(ctx, owner.ID, project.CreateRequest{Title: "前置项目"})

	plan, err := svc.CreatePlanDraft(ctx, owner.ID, proj.ID, "计划", "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	first, err := svc.CreateTaskDraft(ctx, owner.ID, proj.ID, work.TaskDraft{Title: "第一步", PlanID: &plan.ID})
	if err != nil {
		t.Fatal(err)
	}
	// Manually activate + accept the first task (proposal path lands P3-03;
	// the state machine gates are what matters here).
	if _, err := pool.Exec(ctx, `UPDATE tasks SET status='ready', version=2 WHERE id=$1`, first.ID); err != nil {
		t.Fatal(err)
	}
	second, err := svc.CreateTaskDraft(ctx, owner.ID, proj.ID, work.TaskDraft{
		Title: "第二步", PlanID: &plan.ID,
		Requirements: []work.Requirement{{Phase: "start", Kind: "task_acceptance", TargetID: first.ID, Hard: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE tasks SET status='ready', version=2 WHERE id=$1`, second.ID); err != nil {
		t.Fatal(err)
	}
	pos, err := projects.CreatePosition(ctx, owner.ID, proj.ID, project.PositionDraft{Name: "Executor"})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := projects.CreateIdentity(ctx, owner.ID, proj.ID, pos.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO task_participants(project_id,task_id,identity_id) VALUES($1,$2,$3)`, proj.ID, second.ID, identity.ID); err != nil {
		t.Fatal(err)
	}
	// Blocked: first not accepted.
	if _, err := svc.Start(ctx, owner.ID, proj.ID, second.ID, nil, 2); errors.IsCode(err, errors.RequirementUnmet) == false {
		t.Fatalf("unmet precondition must block start, got %v", err)
	}
	// Accept first, then second starts.
	if _, err := pool.Exec(ctx, `UPDATE tasks SET status='accepted', version=3 WHERE id=$1`, first.ID); err != nil {
		t.Fatal(err)
	}
	started, err := svc.Start(ctx, owner.ID, proj.ID, second.ID, nil, 2)
	if err != nil || started.Status != "working" {
		t.Fatalf("start after acceptance: %v %+v", err, started)
	}
	// Execution map: accepted first at column 0, working second at column 1.
	m, err := svc.ExecutionMap(ctx, owner.ID, proj.ID, &plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	columns := map[string]int{}
	for _, node := range m.Nodes {
		columns[node.Title] = node.Column
	}
	if columns["第一步"] != 0 || columns["第二步"] != 1 {
		t.Fatalf("columns must follow precondition depth: %+v", columns)
	}
	edgeFound := false
	for _, edge := range m.Edges {
		if edge.Kind == "hard" && edge.ToTaskID == second.ID.String() {
			edgeFound = true
		}
	}
	if !edgeFound {
		t.Fatalf("hard edge missing: %+v", m.Edges)
	}
}
