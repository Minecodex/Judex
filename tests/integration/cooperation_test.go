package integrationtest_test

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/discussion"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/paging"
	projectdomain "github.com/kakj-go/Judex/internal/project"
	"github.com/kakj-go/Judex/internal/work"
	"net/url"
	"sync"
	"testing"
	"time"
)

func TestCooperationTaskTopicAndAssociations(t *testing.T) {
	d, p, ids, pool := newDiscussionEnv(t)
	ctx := context.Background()
	w := work.NewService(pool, nil)
	owner, _, e := ids.Register(ctx, "Owner", "cooperation@topic.test", "cooperation-password-123", "127.0.0.1")
	if e != nil {
		t.Fatal(e)
	}
	other, _, e := ids.Register(ctx, "Member", "cooperation-member@topic.test", "cooperation-password-123", "127.0.0.1")
	if e != nil {
		t.Fatal(e)
	}
	project, e := p.Create(ctx, owner.ID, projectRequest("Cooperation"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role,state,joined_at) VALUES($1,$2,'member','active',$3)`, project.ID, other.ID, time.Now()); e != nil {
		t.Fatal(e)
	}
	a, e := w.CreatePlanDraft(ctx, owner.ID, project.ID, "Alpha 100%", "", "", nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	b, e := w.CreatePlanDraft(ctx, owner.ID, project.ID, "Beta", "", "", nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	task, e := w.CreateTaskDraft(ctx, owner.ID, project.ID, work.TaskDraft{Title: "Task", PlanID: &a.ID})
	if e != nil {
		t.Fatal(e)
	}
	var before int
	if e = pool.QueryRow(ctx, `SELECT count(*) FROM topics WHERE project_id=$1`, project.ID).Scan(&before); e != nil {
		t.Fatal(e)
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	created := 0
	results := []uuid.UUID{}
	errs := []error{}
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			u := owner.ID
			if i%2 == 1 {
				u = other.ID
			}
			id, new, e := w.EnsureTaskMainTopic(ctx, u, project.ID, task.ID)
			mu.Lock()
			defer mu.Unlock()
			if e != nil {
				errs = append(errs, e)
			}
			if new {
				created++
			}
			results = append(results, id)
		}(i)
	}
	wg.Wait()
	if len(errs) > 0 || created != 1 {
		t.Fatalf("concurrent creation: %d %v", created, errs)
	}
	for _, id := range results {
		if id != results[0] {
			t.Fatalf("different main topics: %v", results)
		}
	}
	main := results[0]
	got, e := w.GetTask(ctx, owner.ID, project.ID, task.ID)
	if e != nil || got.MainTopicID == nil || *got.MainTopicID != main || got.Version != task.Version || got.Status != "draft" {
		t.Fatalf("task metadata changed formal work: %+v %v", got, e)
	}
	var after int
	pool.QueryRow(ctx, `SELECT count(*) FROM topics WHERE project_id=$1`, project.ID).Scan(&after)
	if after != before+1 {
		t.Fatalf("orphan topics: %d -> %d", before, after)
	}
	filtered, e := d.ListTopics(ctx, owner.ID, project.ID, discussion.TopicFilter{PlanID: &a.ID})
	if e != nil || len(filtered) != 2 {
		t.Fatalf("implicit task visibility: %+v %v", filtered, e)
	}
	topic, e := d.GetTopic(ctx, owner.ID, project.ID, main)
	if e != nil {
		t.Fatal(e)
	}
	if e = d.ReplaceTopicLinks(ctx, owner.ID, project.ID, main, topic.LinksVersion, []discussion.TopicLink{{ObjectType: "plan", ObjectID: b.ID}}); !apierrors.IsCode(e, apierrors.InvalidReference) {
		t.Fatalf("removed required main association: %v", e)
	}
	refs := []discussion.TopicLink{{ObjectType: "task", ObjectID: task.ID}, {ObjectType: "plan", ObjectID: b.ID}}
	if e = d.ReplaceTopicLinks(ctx, owner.ID, project.ID, main, topic.LinksVersion, refs); e != nil {
		t.Fatal(e)
	}
	if e = d.ReplaceTopicLinks(ctx, other.ID, project.ID, main, topic.LinksVersion, refs); !apierrors.IsCode(e, apierrors.VersionConflict) {
		t.Fatalf("lost concurrent association edit: %v", e)
	}
	for _, plan := range []uuid.UUID{a.ID, b.ID} {
		listed, e := d.ListTopics(ctx, other.ID, project.ID, discussion.TopicFilter{PlanID: &plan, Query: "Task"})
		if e != nil || len(listed) != 1 || listed[0].ID != main {
			t.Fatalf("shared discussion visibility: %+v %v", listed, e)
		}
	}
	foreign, e := p.Create(ctx, owner.ID, projectRequest("Foreign"))
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = w.EnsureTaskMainTopic(ctx, owner.ID, foreign.ID, task.ID); !apierrors.IsCode(e, apierrors.NotFound) {
		t.Fatalf("foreign task: %v", e)
	}
	if e = d.LinkTopic(ctx, owner.ID, project.ID, uuid.New(), refs); !apierrors.IsCode(e, apierrors.NotFound) {
		t.Fatalf("missing topic link: %v", e)
	}
	plans, e := w.ListPlanCards(ctx, owner.ID, project.ID, work.PlanFilter{Query: "100%", Status: "draft"})
	if e != nil || len(plans) != 1 || plans[0].ID != a.ID {
		t.Fatalf("literal plan filter: %+v %v", plans, e)
	}
}

func TestCooperationScopedPaginationAndProjection(t *testing.T) {
	d, p, ids, pool := newDiscussionEnv(t)
	ctx := context.Background()
	w := work.NewService(pool, nil)
	user, _, e := ids.Register(ctx, "Paging", "cooperation@paging.test", "cooperation-password-123", "127.0.0.1")
	if e != nil {
		t.Fatal(e)
	}
	project, e := p.Create(ctx, user.ID, projectRequest("Paging"))
	if e != nil {
		t.Fatal(e)
	}
	plan, e := w.CreatePlanDraft(ctx, user.ID, project.ID, "Plan", "", "", nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	task, e := w.CreateTaskDraft(ctx, user.ID, project.ID, work.TaskDraft{Title: "Task", PlanID: &plan.ID})
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 8; i++ {
		links := []discussion.TopicLink{}
		if i%2 == 0 {
			links = append(links, discussion.TopicLink{ObjectType: "task", ObjectID: task.ID})
		}
		if _, e = d.CreateTopic(ctx, user.ID, project.ID, fmt.Sprintf("Target %d", i), "", links); e != nil {
			t.Fatal(e)
		}
	}
	seen := map[uuid.UUID]bool{}
	cursor := ""
	for {
		q := url.Values{"limit": {"2"}, "taskId": {task.ID.String()}, "q": {"Target"}}
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		c, e := paging.Parse(ctx, "scoped-topics", q)
		if e != nil {
			t.Fatal(e)
		}
		rows, e := d.ListTopics(c, user.ID, project.ID, discussion.TopicFilter{TaskID: &task.ID, Query: "Target"})
		if e != nil {
			t.Fatal(e)
		}
		for _, topic := range rows {
			if seen[topic.ID] {
				t.Fatal("duplicated topic")
			}
			seen[topic.ID] = true
		}
		next := paging.Next(c)
		if next == nil {
			break
		}
		cursor = *next
	}
	if len(seen) != 4 {
		t.Fatalf("filter after pagination lost matches: %d", len(seen))
	}
	count, e := d.CountTopics(ctx, user.ID, project.ID, discussion.TopicFilter{TaskID: &task.ID, Query: "Target"})
	if e != nil || count != 4 {
		t.Fatalf("scope count: %d %v", count, e)
	}
	planCount, e := w.CountPlanCards(ctx, user.ID, project.ID, work.PlanFilter{Query: "Plan"})
	if e != nil || planCount != 1 {
		t.Fatalf("plan count: %d %v", planCount, e)
	}
	actions, e := w.MyActions(ctx, user.ID, &project.ID, work.ActionFilter{Category: "decision"})
	if e != nil || len(actions) != 0 {
		t.Fatalf("unexpected actions: %+v %v", actions, e)
	}
	if _, e = w.MyActions(ctx, user.ID, &project.ID, work.ActionFilter{Kind: "invalid"}); !apierrors.IsCode(e, apierrors.Validation) {
		t.Fatalf("kind validation: %v", e)
	}
	deliveries, e := w.ListDeliveries(ctx, user.ID, project.ID, work.DeliveryFilter{})
	if e != nil || len(deliveries) != 0 {
		t.Fatalf("draft is delivery: %+v %v", deliveries, e)
	}
}

func TestCooperationDeliveryAndDecisionProjection(t *testing.T) {
	w, p, ids, pool := newWorkEnv(t)
	ctx := context.Background()
	user, _, e := ids.Register(ctx, "Reporter", "cooperation@delivery.test", "cooperation-password-123", "127.0.0.1")
	if e != nil {
		t.Fatal(e)
	}
	project, e := p.Create(ctx, user.ID, projectRequest("Delivery projection"))
	if e != nil {
		t.Fatal(e)
	}
	position, e := p.CreatePosition(ctx, user.ID, project.ID, projectdomain.PositionDraft{Name: "Executor"})
	if e != nil {
		t.Fatal(e)
	}
	seat, e := p.CreateIdentity(ctx, user.ID, project.ID, position.ID, user.ID)
	if e != nil {
		t.Fatal(e)
	}
	plan, e := w.CreatePlanDraft(ctx, user.ID, project.ID, "Plan", "", "", &seat.ID, nil)
	if e != nil {
		t.Fatal(e)
	}
	first, e := w.CreateTaskDraft(ctx, user.ID, project.ID, work.TaskDraft{Title: "Delivery", PlanID: &plan.ID, ParticipantIDs: []uuid.UUID{seat.ID}, ReviewerIdentityID: &seat.ID})
	if e != nil {
		t.Fatal(e)
	}
	second, e := w.CreateTaskDraft(ctx, user.ID, project.ID, work.TaskDraft{Title: "Receiver", PlanID: &plan.ID, ParticipantIDs: []uuid.UUID{seat.ID}, ReviewerIdentityID: &seat.ID})
	if e != nil {
		t.Fatal(e)
	}
	report, handoff, source, version := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	now := time.Now()
	for _, row := range []struct {
		sql  string
		args []any
	}{
		{`UPDATE plans SET status='active' WHERE id=$1`, []any{plan.ID}},
		{`INSERT INTO work_reports(project_id,id,task_id,identity_id,report_kind,progress_hint,created_by,created_at) VALUES($1,$2,$3,$4,'delivery','固定报告正文',$5,$6)`, []any{project.ID, report, first.ID, seat.ID, user.ID, now}},
		{`UPDATE tasks SET status='delivered',latest_report_id=$2 WHERE id=$1`, []any{first.ID, report}},
		{`INSERT INTO handoffs(project_id,id,target_task_id,receiver_identity_id,kind,title,created_at,updated_at) VALUES($1,$2,$3,$4,'dependency','固定交接',$5,$5)`, []any{project.ID, handoff, second.ID, seat.ID, now}},
		{`INSERT INTO handoff_sources(project_id,id,handoff_id,source_task_id,sender_identity_id) VALUES($1,$2,$3,$4,$5)`, []any{project.ID, source, handoff, first.ID, seat.ID}},
		{`INSERT INTO source_versions(project_id,id,source_id,revision,report_id,summary,state,created_at) VALUES($1,$2,$3,1,$4,'固定交接正文','pending',$5)`, []any{project.ID, version, source, report, now}},
		{`UPDATE handoff_sources SET current_source_version_id=$2 WHERE id=$1`, []any{source, version}},
	} {
		if _, e = pool.Exec(ctx, row.sql, row.args...); e != nil {
			t.Fatal(e)
		}
	}
	all, e := w.ListDeliveries(ctx, user.ID, project.ID, work.DeliveryFilter{})
	if e != nil || len(all) != 2 {
		t.Fatalf("mixed delivery cards: %+v %v", all, e)
	}
	receive, e := w.ListDeliveries(ctx, user.ID, project.ID, work.DeliveryFilter{Filter: "receive"})
	if e != nil || len(receive) != 1 || receive[0].ObjectID != handoff {
		t.Fatalf("receipt filter: %+v %v", receive, e)
	}
	waiting, e := w.ListDeliveries(ctx, user.ID, project.ID, work.DeliveryFilter{Filter: "waiting"})
	if e != nil || len(waiting) != 2 {
		t.Fatalf("waiting filter: %+v %v", waiting, e)
	}
	taskCards, e := w.ListDeliveries(ctx, user.ID, project.ID, work.DeliveryFilter{Type: "task"})
	if e != nil || len(taskCards) != 1 || taskCards[0].Summary != "固定报告正文" {
		t.Fatalf("canonical report projection: %+v %v", taskCards, e)
	}
	count, e := w.CountMyActions(ctx, user.ID, &project.ID, work.ActionFilter{Category: "decision"})
	if e != nil || count != 2 {
		t.Fatalf("decision count: %d %v", count, e)
	}
	c, e := paging.Parse(ctx, "decisions", url.Values{"limit": {"1"}, "category": {"decision"}})
	if e != nil {
		t.Fatal(e)
	}
	page, e := w.MyActions(c, user.ID, &project.ID, work.ActionFilter{Category: "decision"})
	if e != nil || len(page) != 1 || paging.Next(c) == nil {
		t.Fatalf("decision pagination: %+v %v", page, e)
	}
	accepted, e := w.MyActions(ctx, user.ID, &project.ID, work.ActionFilter{Kind: "accept"})
	if e != nil || len(accepted) != 1 || accepted[0].ObjectType != "task" {
		t.Fatalf("kind filter: %+v %v", accepted, e)
	}
}
