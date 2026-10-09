package integrationtest_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/agent/batch"
	"github.com/kakj-go/Judex/internal/collaboration"
	"github.com/kakj-go/Judex/internal/discussion"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
	"github.com/kakj-go/Judex/internal/work"
)

type taskAnalysisProvider struct {
	calls    atomic.Int32
	prompts  []string
	question bool
}

func (p *taskAnalysisProvider) Stream(ctx context.Context, req model.Request) (<-chan model.Event, error) {
	p.calls.Add(1)
	var transcript strings.Builder
	for _, message := range req.Messages {
		transcript.WriteString(message.Content)
		transcript.WriteByte('\n')
	}
	p.prompts = append(p.prompts, transcript.String())
	out := make(chan model.Event, 4)
	if len(req.Messages) == 2 {
		args := map[string]any{"summary": "已核对本任务，保留原始证据。", "disagreements": []string{}}
		if p.question {
			args["discussion"] = map[string]any{"title": "任务问题分析", "reason": "需要持续协调"}
		}
		raw, _ := json.Marshal(args)
		out <- model.Event{Type: "toolCallReady", ToolCallID: "task-analysis", ToolName: "record_task_analysis", ArgsJSON: string(raw)}
	} else {
		out <- model.Event{Type: "textDelta", TextDelta: "本次分析完成。"}
	}
	out <- model.Event{Type: "usage", InputTokens: 100, OutputTokens: 50}
	out <- model.Event{Type: "finish"}
	close(out)
	return out, nil
}
func TestTaskAnalysisSuggestionAndNoAutomaticConversation(t *testing.T) {
	d, p, ids, pool := newDiscussionEnv(t)
	ctx := context.Background()
	user, _, _ := ids.Register(ctx, "Analysis", "analysis@task.test", "analysis-password-123", "127.0.0.1")
	project, _ := p.Create(ctx, user.ID, projectRequest("Analysis"))
	w := work.NewService(pool, nil)
	plan, err := w.CreatePlanDraft(ctx, user.ID, project.ID, "相关计划", "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	task, err := w.CreateTaskDraft(ctx, user.ID, project.ID, work.TaskDraft{Title: "当前任务 ONLY_THIS_TASK", PlanID: &plan.ID})
	if err != nil {
		t.Fatal(err)
	}
	unrelated, err := w.CreateTaskDraft(ctx, user.ID, project.ID, work.TaskDraft{Title: "不得混入 UNRELATED_TASK", PlanID: &plan.ID})
	if err != nil {
		t.Fatal(err)
	}
	proposal, review := uuid.New(), uuid.New()
	raw, _ := json.Marshal([]map[string]any{{"operation": "update_scope", "targetType": "task", "targetId": unrelated.ID, "fields": map[string]any{"expectedOutput": "UNRELATED_TASK"}}})
	if _, err = pool.Exec(ctx, `INSERT INTO proposals(project_id,id,kind,status,reason,created_at,updated_at)VALUES($1,$2,'work_change','pending','UNRELATED_TASK pending proposal',now(),now())`, project.ID, proposal); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO proposal_versions(project_id,id,proposal_id,revision,changes_json,created_at)VALUES($1,$2,$3,1,$4,now())`, project.ID, review, proposal, raw); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE proposals SET current_review_id=$2 WHERE id=$1`, proposal, review); err != nil {
		t.Fatal(err)
	}
	source, err := d.CreateSubmission(ctx, user.ID, project.ID, discussion.Submission{ClientSubmissionID: uuid.NewString(), Purpose: "message", DiscussionIntent: "question", Source: "web", Text: "需要核对的问题", TaskID: &task.ID})
	if err != nil {
		t.Fatal(err)
	}
	c := collaboration.NewService(pool)
	activities, err := c.ListActivity(ctx, user.ID, project.ID, task.ID)
	if err != nil || len(activities) != 1 {
		t.Fatalf("activity %+v %v", activities, err)
	}
	analysis := activities[0].Analysis
	provider := &taskAnalysisProvider{question: true}
	executor := batch.Executor{Pool: pool.Pool, Provider: provider, ModelName: "controlled-task-test"}
	if err = executor.ExecuteBatch(ctx, project.ID, *analysis.BatchID); err != nil {
		t.Fatal(err)
	}
	finished, err := c.GetAnalysis(ctx, user.ID, project.ID, analysis.ID)
	if err != nil || finished.State != "completed" || !strings.Contains(finished.Summary, "核对") {
		t.Fatalf("analysis %+v %v", finished, err)
	}
	for _, prompt := range provider.prompts {
		if !strings.Contains(prompt, "ONLY_THIS_TASK") || strings.Contains(prompt, "UNRELATED_TASK") {
			t.Fatalf("task context contamination: %s", prompt)
		}
	}
	suggestions, err := c.ListSuggestions(ctx, user.ID, project.ID, nil, nil)
	if err != nil || len(suggestions) != 1 {
		t.Fatalf("suggestions %+v %v", suggestions, err)
	}
	var topics int
	pool.QueryRow(ctx, `SELECT count(*) FROM topics WHERE project_id=$1`, project.ID).Scan(&topics)
	if topics != 2 {
		t.Fatalf("AI created a conversation: %d", topics)
	}
	results := make(chan collaboration.Suggestion, 8)
	failures := make(chan error, 8)
	var concurrent sync.WaitGroup
	for range 8 {
		concurrent.Add(1)
		go func() {
			defer concurrent.Done()
			value, e := c.Resolve(ctx, user.ID, project.ID, suggestions[0].ID, collaboration.ResolveInput{ExpectedVersion: 1, Mode: "create"})
			results <- value
			failures <- e
		}()
	}
	concurrent.Wait()
	close(results)
	close(failures)
	for e := range failures {
		if e != nil {
			t.Fatal(e)
		}
	}
	result := <-results
	for value := range results {
		if value.ResultTopicID == nil || *value.ResultTopicID != *result.ResultTopicID {
			t.Fatal("concurrent confirmation created multiple outcomes")
		}
	}
	if err != nil || result.ResultTopicID == nil {
		t.Fatalf("resolve %+v %v", result, err)
	}
	duplicate, err := c.Resolve(ctx, user.ID, project.ID, suggestions[0].ID, collaboration.ResolveInput{ExpectedVersion: 1, Mode: "create"})
	if err != nil || *duplicate.ResultTopicID != *result.ResultTopicID {
		t.Fatalf("duplicate %+v %v", duplicate, err)
	}
	pool.QueryRow(ctx, `SELECT count(*) FROM topics WHERE project_id=$1`, project.ID).Scan(&topics)
	if topics != 3 {
		t.Fatalf("duplicate conversation: %d", topics)
	}
	topic, err := d.GetTopic(ctx, user.ID, project.ID, *result.ResultTopicID)
	if err != nil || len(topic.SourceRefs) != 1 || topic.SourceRefs[0].ID != source.ID {
		t.Fatalf("source refs %+v %v", topic, err)
	}
	if err = executor.ExecuteBatch(ctx, project.ID, *analysis.BatchID); err != nil {
		t.Fatal(err)
	}
	if provider.calls.Load() != 2 {
		t.Fatal("completed analysis repeated")
	}
}

type retryAnalysisProvider struct {
	delegate    taskAnalysisProvider
	interrupted bool
}

func (p *retryAnalysisProvider) Stream(ctx context.Context, req model.Request) (<-chan model.Event, error) {
	if len(req.Messages) > 2 && !p.interrupted {
		p.interrupted = true
		return nil, errors.New("controlled provider disconnect after durable tool result")
	}
	return p.delegate.Stream(ctx, req)
}
func TestTaskAnalysisRetryKeepsToolTranscriptAndBudget(t *testing.T) {
	d, p, ids, pool := newDiscussionEnv(t)
	ctx := context.Background()
	user, _, err := ids.Register(ctx, "Retry", "retry@analysis.test", "retry-password-123", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	project, err := p.Create(ctx, user.ID, projectRequest("Retry"))
	if err != nil {
		t.Fatal(err)
	}
	w := work.NewService(pool, nil)
	task, err := w.CreateTaskDraft(ctx, user.ID, project.ID, work.TaskDraft{Title: "Retry task"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = d.CreateSubmission(ctx, user.ID, project.ID, discussion.Submission{ClientSubmissionID: uuid.NewString(), Purpose: "message", DiscussionIntent: "question", Source: "web", Text: "needs review", TaskID: &task.ID})
	if err != nil {
		t.Fatal(err)
	}
	c := collaboration.NewService(pool)
	activities, err := c.ListActivity(ctx, user.ID, project.ID, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	a := activities[0].Analysis
	provider := &retryAnalysisProvider{}
	e := batch.Executor{Pool: pool.Pool, Provider: provider, ModelName: "controlled"}
	if err = e.ExecuteBatch(ctx, project.ID, *a.BatchID); err != nil {
		t.Fatal(err)
	}
	failed, err := c.GetAnalysis(ctx, user.ID, project.ID, a.ID)
	if err != nil || failed.State != "failed" {
		t.Fatalf("failure %+v %v", failed, err)
	}
	if _, err = c.Retry(ctx, user.ID, project.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	if err = e.ExecuteBatch(ctx, project.ID, *a.BatchID); err != nil {
		t.Fatal(err)
	}
	done, err := c.GetAnalysis(ctx, user.ID, project.ID, a.ID)
	if err != nil || done.State != "completed" || !strings.Contains(done.Summary, "已核对本任务") {
		t.Fatalf("recovery %+v %v", done, err)
	}
	var tools, attempts int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM tool_calls WHERE run_id IN(SELECT id FROM agent_runs WHERE batch_id=$1)`, *a.BatchID).Scan(&tools); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT model_attempts FROM discussion_batches WHERE id=$1`, *a.BatchID).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if tools != 1 || attempts != 3 {
		t.Fatalf("replayed effects or reset budget: tools=%d attempts=%d", tools, attempts)
	}
}

type noStructuredAnalysisProvider struct{}

func (*noStructuredAnalysisProvider) Stream(ctx context.Context, req model.Request) (<-chan model.Event, error) {
	out := make(chan model.Event, 2)
	out <- model.Event{Type: "textDelta", TextDelta: "A successful model response without a structured record"}
	out <- model.Event{Type: "finish"}
	close(out)
	return out, nil
}
func TestTaskAnalysisNeverCompletesWithoutStructuredOutputAfterRetry(t *testing.T) {
	d, p, ids, pool := newDiscussionEnv(t)
	ctx := context.Background()
	user, _, err := ids.Register(ctx, "Structure", "missing-structure@analysis.test", "analysis-password-123", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	project, err := p.Create(ctx, user.ID, projectRequest("Output"))
	if err != nil {
		t.Fatal(err)
	}
	w := work.NewService(pool, nil)
	task, err := w.CreateTaskDraft(ctx, user.ID, project.ID, work.TaskDraft{Title: "Source retained"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.CreateSubmission(ctx, user.ID, project.ID, discussion.Submission{ClientSubmissionID: uuid.NewString(), Purpose: "message", DiscussionIntent: "question", Source: "web", Text: "original question", TaskID: &task.ID}); err != nil {
		t.Fatal(err)
	}
	c := collaboration.NewService(pool)
	activities, err := c.ListActivity(ctx, user.ID, project.ID, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	a := activities[0].Analysis
	e := batch.Executor{Pool: pool.Pool, Provider: &noStructuredAnalysisProvider{}, ModelName: "controlled-missing-output"}
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			if _, err = c.Retry(ctx, user.ID, project.ID, a.ID); err != nil {
				t.Fatal(err)
			}
		}
		if err = e.ExecuteBatch(ctx, project.ID, *a.BatchID); err != nil {
			t.Fatal(err)
		}
		result, err := c.GetAnalysis(ctx, user.ID, project.ID, a.ID)
		if err != nil || result.State != "failed" || !strings.Contains(result.Summary, "结构化") {
			t.Fatalf("false completion %+v %v", result, err)
		}
	}
}
