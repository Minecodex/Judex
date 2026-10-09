package integrationtest_test

import (
	"context"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/collaboration"
	"github.com/kakj-go/Judex/internal/discussion"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/paging"
	"github.com/kakj-go/Judex/internal/project"
	"github.com/kakj-go/Judex/internal/work"
	"net/url"
	"testing"
)

func projectRequest(title string) project.CreateRequest { return project.CreateRequest{Title: title} }

func TestConversationForkHistory(t *testing.T) {
	d, p, ids, pool := newDiscussionEnv(t)
	ctx := context.Background()
	user, _, err := ids.Register(ctx, "Fork", "fork@history.test", "history-password-123", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	project, err := p.Create(ctx, user.ID, projectRequest("Fork history"))
	if err != nil {
		t.Fatal(err)
	}
	original, err := d.CreateTopic(ctx, user.ID, project.ID, "探索", "第一条", nil)
	if err != nil {
		t.Fatal(err)
	}
	send := func(topic uuid.UUID, text string) {
		t.Helper()
		if _, err := d.CreateSubmission(ctx, user.ID, project.ID, discussion.Submission{ClientSubmissionID: uuid.NewString(), Purpose: "message", Source: "web", Text: text, TopicID: &topic}); err != nil {
			t.Fatal(err)
		}
	}
	send(original.ID, "第二条")
	send(original.ID, "第三条")
	point := int64(2)
	branch, err := d.Fork(ctx, user.ID, project.ID, original.ID, collaboration.ForkInput{Title: "分支", ForkAfterSeq: &point})
	if err != nil {
		t.Fatal(err)
	}
	send(original.ID, "父分支后来消息")
	send(branch.ID, "分支独立消息")
	messages, err := d.ListMessages(ctx, user.ID, project.ID, branch.ID, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 3 || messages[0].Content != "第一条" || messages[1].Content != "第二条" || messages[2].Content != "分支独立消息" {
		t.Fatalf("fork prefix: %+v", messages)
	}
	if !messages[0].Inherited || messages[0].OriginTopicID != original.ID || messages[2].Inherited || messages[2].Seq != 3 {
		t.Fatalf("source and suffix: %+v", messages)
	}
	point = 1
	grandchild, err := d.Fork(ctx, user.ID, project.ID, branch.ID, collaboration.ForkInput{Title: "从继承消息再分叉", ForkAfterSeq: &point})
	if err != nil {
		t.Fatal(err)
	}
	send(grandchild.ID, "孙分支独立消息")
	grand, err := d.ListMessages(ctx, user.ID, project.ID, grandchild.ID, 0, 50)
	if err != nil || len(grand) != 2 || grand[0].ID != messages[0].ID || grand[1].Seq != 2 {
		t.Fatalf("nested fork: %+v %v", grand, err)
	}
	before, err := d.ListMessages(ctx, user.ID, project.ID, branch.ID, 3, 50)
	if err != nil || len(before) != 2 {
		t.Fatalf("before cursor: %+v %v", before, err)
	}
	after, err := d.ListMessages(ctx, user.ID, project.ID, branch.ID, 0, 50, 2)
	if err != nil || len(after) != 1 || after[0].Seq != 3 {
		t.Fatalf("after cursor: %+v %v", after, err)
	}
	other, err := p.Create(ctx, user.ID, projectRequest("Other"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.Fork(ctx, user.ID, other.ID, original.ID, collaboration.ForkInput{Title: "跨项目", ForkAfterSeq: &point}); !apierrors.IsCode(err, apierrors.InvalidReference) {
		t.Fatalf("foreign parent: %v", err)
	}
	var physical int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE topic_id=$1`, branch.ID).Scan(&physical); err != nil || physical != 1 {
		t.Fatalf("history was copied: %d %v", physical, err)
	}
}

func TestPlanMainTopicsAreAtomic(t *testing.T) {
	d, p, ids, pool := newDiscussionEnv(t)
	ctx := context.Background()
	user, _, _ := ids.Register(ctx, "Plan", "plan@fork.test", "plan-password-123", "127.0.0.1")
	project, _ := p.Create(ctx, user.ID, projectRequest("Plans"))
	origin, err := d.CreateTopic(ctx, user.ID, project.ID, "先探索", "原始依据", nil)
	if err != nil {
		t.Fatal(err)
	}
	w := work.NewService(pool, nil)
	point := int64(1)
	plan, err := w.CreatePlanDraft(ctx, user.ID, project.ID, "主讨论", "目标", "条件", nil, nil, collaboration.Origin{TopicID: origin.ID, AfterSeq: &point})
	if err != nil || plan.MainTopicID == nil {
		t.Fatalf("main discussion: %+v %v", plan, err)
	}
	main, err := d.GetTopic(ctx, user.ID, project.ID, *plan.MainTopicID)
	if err != nil || main.ParentTopicID == nil || *main.ParentTopicID != origin.ID || len(main.Links) != 1 {
		t.Fatalf("main origin: %+v %v", main, err)
	}
	listed, err := w.ListPlans(ctx, user.ID, project.ID)
	if err != nil || len(listed) != 1 || listed[0].MainTopicID == nil {
		t.Fatalf("projection: %+v %v", listed, err)
	}
	foreign, _ := p.Create(ctx, user.ID, projectRequest("Foreign"))
	if _, err = w.CreatePlanDraft(ctx, user.ID, foreign.ID, "应回滚", "", "", nil, nil, collaboration.Origin{TopicID: origin.ID}); err == nil {
		t.Fatal("foreign fork succeeded")
	}
	var count int
	pool.QueryRow(ctx, `SELECT count(*) FROM plans WHERE project_id=$1`, foreign.ID).Scan(&count)
	if count != 0 {
		t.Fatalf("partial plan remained: %d", count)
	}
}

func TestTaskCommunicationDoesNotChangeWorkState(t *testing.T) {
	d, p, ids, pool := newDiscussionEnv(t)
	ctx := context.Background()
	user, _, _ := ids.Register(ctx, "Question", "question@task.test", "question-password-123", "127.0.0.1")
	project, _ := p.Create(ctx, user.ID, projectRequest("Task activities"))
	w := work.NewService(pool, nil)
	plan, err := w.CreatePlanDraft(ctx, user.ID, project.ID, "计划", "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	task, err := w.CreateTaskDraft(ctx, user.ID, project.ID, work.TaskDraft{Title: "尚未开始的任务", PlanID: &plan.ID})
	if err != nil {
		t.Fatal(err)
	}
	key := uuid.NewString()
	input := discussion.Submission{ClientSubmissionID: key, Purpose: "message", DiscussionIntent: "question", Source: "cli", TaskID: &task.ID, Text: "需要澄清目标"}
	first, err := d.CreateSubmission(ctx, user.ID, project.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := d.CreateSubmission(ctx, user.ID, project.ID, input)
	if err != nil || first.ID != second.ID {
		t.Fatalf("replay: %+v %v", second, err)
	}
	var status string
	var version int64
	pool.QueryRow(ctx, `SELECT status,version FROM tasks WHERE id=$1`, task.ID).Scan(&status, &version)
	if status != "draft" || version != task.Version {
		t.Fatalf("communication changed task: %s %d", status, version)
	}
	c := collaboration.NewService(pool)
	if _, err = c.PlanSummary(ctx, user.ID, project.ID, uuid.New()); !apierrors.IsCode(err, apierrors.NotFound) {
		t.Fatalf("unknown plan must be not found: %v", err)
	}
	activities, err := c.ListActivity(ctx, user.ID, project.ID, task.ID)
	if err != nil || len(activities) != 1 || activities[0].Kind != "question" || activities[0].Analysis == nil || activities[0].Analysis.State != "queued" {
		t.Fatalf("activities: %+v %v", activities, err)
	}
	var reports, batches, topics int
	pool.QueryRow(ctx, `SELECT count(*) FROM work_reports WHERE task_id=$1`, task.ID).Scan(&reports)
	pool.QueryRow(ctx, `SELECT count(*) FROM discussion_batches WHERE task_id=$1`, task.ID).Scan(&batches)
	pool.QueryRow(ctx, `SELECT count(*) FROM topics WHERE project_id=$1`, project.ID).Scan(&topics)
	if reports != 0 || batches != 1 || topics != 2 {
		t.Fatalf("unexpected formal work or conversation: reports=%d batches=%d topics=%d", reports, batches, topics)
	}
	if _, err = d.CreateSubmission(ctx, user.ID, project.ID, discussion.Submission{ClientSubmissionID: uuid.NewString(), Purpose: "message", DiscussionIntent: "question", Source: "web", TaskID: &task.ID, Text: "Second record"}); err != nil {
		t.Fatal(err)
	}
	firstPage, _ := paging.Parse(ctx, "test-activity:"+task.ID.String(), url.Values{"limit": {"1"}})
	one, err := c.ListActivity(firstPage, user.ID, project.ID, task.ID)
	if err != nil || len(one) != 1 {
		t.Fatalf("first page: %+v %v", one, err)
	}
	cursor := paging.Next(firstPage)
	if cursor == nil {
		t.Fatal("activity has no next cursor")
	}
	secondPage, err := paging.Parse(ctx, "test-activity:"+task.ID.String(), url.Values{"limit": {"1"}, "cursor": {*cursor}})
	if err != nil {
		t.Fatal(err)
	}
	two, err := c.ListActivity(secondPage, user.ID, project.ID, task.ID)
	if err != nil || len(two) != 1 || two[0].ID == one[0].ID || two[0].ID != first.ID {
		t.Fatalf("task history repeated or skipped: %+v %+v %v", one, two, err)
	}
}

func TestTaskRepliesUseIndependentBranchSessions(t *testing.T) {
	d, p, ids, pool := newDiscussionEnv(t)
	ctx := context.Background()
	user, _, err := ids.Register(ctx, "Branch", "branch-sessions@task.test", "branch-password-123", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	project, err := p.Create(ctx, user.ID, projectRequest("Branch sessions"))
	if err != nil {
		t.Fatal(err)
	}
	w := work.NewService(pool, nil)
	task, err := w.CreateTaskDraft(ctx, user.ID, project.ID, work.TaskDraft{Title: "Same task"})
	if err != nil {
		t.Fatal(err)
	}
	parent, err := d.CreateTopic(ctx, user.ID, project.ID, "Parent", "Fixed original", nil)
	if err != nil {
		t.Fatal(err)
	}
	point := int64(1)
	child, err := d.Fork(ctx, user.ID, project.ID, parent.ID, collaboration.ForkInput{Title: "Child", ForkAfterSeq: &point})
	if err != nil {
		t.Fatal(err)
	}
	sessions := map[uuid.UUID]bool{}
	for _, topic := range []uuid.UUID{parent.ID, child.ID} {
		sub, err := d.CreateSubmission(ctx, user.ID, project.ID, discussion.Submission{ClientSubmissionID: uuid.NewString(), Purpose: "message", DiscussionIntent: "reply", Source: "web", Text: "Task reply", TaskID: &task.ID, TopicID: &topic})
		if err != nil {
			t.Fatal(err)
		}
		var session uuid.UUID
		var sessionTopic *uuid.UUID
		err = pool.QueryRow(ctx, `SELECT r.session_id,s.topic_id FROM task_analyses a JOIN agent_runs r ON r.batch_id=a.batch_id AND r.parent_run_id IS NULL JOIN agent_sessions s ON s.id=r.session_id WHERE a.project_id=$1 AND a.source_id=$2`, project.ID, sub.ID).Scan(&session, &sessionTopic)
		if err != nil || sessionTopic == nil || *sessionTopic != topic {
			t.Fatalf("branch session %v %v", sessionTopic, err)
		}
		sessions[session] = true
	}
	if len(sessions) != 2 {
		t.Fatal("branches shared an AI session")
	}
}
