package integrationtest_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/agent/batch"
	"github.com/kakj-go/Judex/internal/collaboration"
	"github.com/kakj-go/Judex/internal/discussion"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
	"github.com/kakj-go/Judex/internal/work"
)

func TestTaskAnalysisExposesPublicReportsFromItsOwnTask(t *testing.T) {
	d, projects, ids, pool := newDiscussionEnv(t)
	ctx := context.Background()
	owner, _, err := ids.Register(ctx, "Report context", "report-context@test.local", "report-context-password", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	p, err := projects.Create(ctx, owner.ID, projectRequest("Public report sources"))
	if err != nil {
		t.Fatal(err)
	}
	w := work.NewService(pool, nil)
	task, err := w.CreateTaskDraft(ctx, owner.ID, p.ID, work.TaskDraft{Title: "Task with source"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := w.CreateTaskDraft(ctx, owner.ID, p.ID, work.TaskDraft{Title: "Other task"})
	if err != nil {
		t.Fatal(err)
	}
	report, foreign := uuid.New(), uuid.New()
	// Controlled public work facts, not a business acceptance or an upload.
	if _, err = pool.Exec(ctx, `INSERT INTO work_reports(project_id,id,task_id,identity_id,report_kind,progress_hint,created_by,created_at) VALUES($1,$2,$3,$4,'progress',$5,$6,now()),($1,$7,$8,$4,'progress','UNRELATED_REPORT',$6,now())`, p.ID, report, task.ID, uuid.New(), "OWN_REPORT_"+strings.Repeat("中", 2100), owner.ID, foreign, other.ID); err != nil {
		t.Fatal(err)
	}
	source, err := d.CreateSubmission(ctx, owner.ID, p.ID, discussion.Submission{ClientSubmissionID: uuid.NewString(), Purpose: "message", Source: "cli", Text: "Review public source IDs", TaskID: &task.ID})
	if err != nil {
		t.Fatal(err)
	}
	a, err := collaboration.NewService(pool).SubmissionAnalysis(ctx, owner.ID, p.ID, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	checked := false
	provider := contextProvider(func(_ context.Context, req model.Request) (<-chan model.Event, error) {
		text := req.Messages[1].Content
		if !strings.Contains(text, report.String()) || !strings.Contains(text, "OWN_REPORT_") || !strings.Contains(text, `"textTruncated":true`) || strings.Contains(text, "UNRELATED_REPORT") {
			t.Fatal("public report source missing, unmarked truncation or task scope leaked")
		}
		checked = true
		out := make(chan model.Event, 3)
		out <- model.Event{Type: "toolCallReady", ToolCallID: "record", ToolName: "record_task_analysis", ArgsJSON: `{"summary":"只读取当前任务公开报告摘录，正文截断与未读原件作为缺口保留。","basis":["` + report.String() + `"]}`}
		out <- model.Event{Type: "usage", Usage: model.UsageSnapshot{InputTokens: model.TokenCount(100), OutputTokens: model.TokenCount(50)}}
		out <- model.Event{Type: "finish"}
		close(out)
		return out, nil
	})
	e := batch.Executor{Pool: pool.Pool, Provider: provider, ModelName: "controlled-public-report-context"}
	if err = e.ExecuteBatch(ctx, p.ID, *a.BatchID); err != nil {
		t.Fatal(err)
	}
	if !checked {
		t.Fatal("model context was never inspected")
	}
}
