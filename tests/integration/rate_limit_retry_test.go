package integrationtest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/agent/batch"
	"github.com/kakj-go/Judex/internal/collaboration"
	"github.com/kakj-go/Judex/internal/discussion"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
)

// Controlled HTTP429 plus real PG and explicit public retry. This is not a
// claim that either real provider rate-limited a business run.
func TestRateLimitedAnalysisExplicitRetryKeepsSourceAndBudget(t *testing.T) {
	d, projects, ids, pool := newDiscussionEnv(t)
	ctx := context.Background()
	user, _, err := ids.Register(ctx, "Rate retry", "rate-retry@test.local", "rate-retry-password", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	p, err := projects.Create(ctx, user.ID, projectRequest("Rate-limited analysis"))
	if err != nil {
		t.Fatal(err)
	}
	source, err := d.CreateSubmission(ctx, user.ID, p.ID, discussion.Submission{ClientSubmissionID: uuid.NewString(), Purpose: "message", Source: "cli", Text: "Keep the original request on rate limiting"})
	if err != nil {
		t.Fatal(err)
	}
	svc := collaboration.NewService(pool)
	a, err := svc.SubmissionAnalysis(ctx, user.ID, p.ID, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Model string `json:"model"`
		}
		if json.NewDecoder(r.Body).Decode(&request) != nil || request.Model != "controlled-rate-model" {
			t.Error("requested model changed")
			w.WriteHeader(400)
			return
		}
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(429)
			_, _ = w.Write([]byte(`{"error":{"type":"rate_limit_error","message":"controlled rate limit, not quota exhaustion"}}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		arguments, _ := json.Marshal(map[string]any{"summary": "Same source after an explicit bounded retry", "basis": []string{"controlled original source"}})
		frame, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"tool_calls": []any{map[string]any{"index": 0, "id": "record-after-rate", "type": "function", "function": map[string]any{"name": "record_task_analysis", "arguments": string(arguments)}}}}, "finish_reason": "tool_calls"}}, "usage": map[string]any{"prompt_tokens": 100, "completion_tokens": 30}})
		_, _ = fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", frame)
	}))
	defer gateway.Close()
	provider := model.NewOpenAIProvider(model.OpenAIConfig{BaseURL: gateway.URL, APIKey: "synthetic-rate-test"})
	e := batch.Executor{Pool: pool.Pool, Provider: provider, ModelName: "controlled-rate-model"}
	if err = e.ExecuteBatch(ctx, p.ID, *a.BatchID); err != nil {
		t.Fatal(err)
	}
	failed, err := svc.GetAnalysis(ctx, user.ID, p.ID, a.ID)
	if err != nil || failed.State != "failed" || calls.Load() != 1 {
		t.Fatalf("unbounded retry or wrong failure: %+v %v calls=%d", failed, err, calls.Load())
	}
	var beforeAttempts int
	var beforeTokens int64
	if err = pool.QueryRow(ctx, `SELECT model_attempts,used_tokens FROM discussion_batches WHERE id=$1`, *a.BatchID).Scan(&beforeAttempts, &beforeTokens); err != nil {
		t.Fatal(err)
	}
	if beforeAttempts != 1 || beforeTokens <= 0 {
		t.Fatalf("failed attempt reservation was lost: %d %d", beforeAttempts, beforeTokens)
	}
	var failureCode *string
	var usageKnown bool
	if err = pool.QueryRow(ctx, `SELECT m.error_code,m.usage_known FROM model_calls m JOIN agent_runs r ON r.id=m.run_id WHERE r.batch_id=$1 AND m.state='failed'`, a.BatchID).Scan(&failureCode, &usageKnown); err != nil || failureCode == nil || *failureCode != "RATE_LIMITED" || usageKnown {
		t.Fatalf("rate limit cause missing or usage incorrectly known: code=%v known=%v error=%v", failureCode, usageKnown, err)
	}
	if _, err = svc.Retry(ctx, user.ID, p.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	if err = e.ExecuteBatch(ctx, p.ID, *a.BatchID); err != nil {
		t.Fatal(err)
	}
	done, err := svc.SubmissionAnalysis(ctx, user.ID, p.ID, source.ID)
	if err != nil || done.State != "completed" || done.ID != a.ID || *done.BatchID != *a.BatchID || calls.Load() != 2 {
		t.Fatalf("retry replaced its source/batch: %+v %v calls=%d", done, err, calls.Load())
	}
	var attempts, formal int
	var tokens int64
	if err = pool.QueryRow(ctx, `SELECT model_attempts,used_tokens,(SELECT count(*) FROM tasks WHERE project_id=$2) FROM discussion_batches WHERE id=$1`, *a.BatchID, p.ID).Scan(&attempts, &tokens, &formal); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 || tokens < beforeTokens || formal != 0 {
		t.Fatalf("retry reset budget or created formal work: attempts=%d tokens=%d before=%d tasks=%d", attempts, tokens, beforeTokens, formal)
	}
	var successCode *string
	if err = pool.QueryRow(ctx, `SELECT m.error_code FROM model_calls m JOIN agent_runs r ON r.id=m.run_id WHERE r.batch_id=$1 AND m.state='succeeded'`, a.BatchID).Scan(&successCode); err != nil || successCode != nil {
		t.Fatalf("successful retry has a failure category: code=%v error=%v", successCode, err)
	}
}
