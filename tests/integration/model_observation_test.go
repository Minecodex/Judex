// SPDX-License-Identifier: Apache-2.0
package integrationtest_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/agent/batch"
	"github.com/kakj-go/Judex/internal/discussion"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
)

func TestModelObservationRetentionPreservesRecoveryAndBudget(t *testing.T) {
	_, projects, ids, pool := newDecisionEnv(t)
	ctx := context.Background()
	user, _, err := ids.Register(ctx, "Observation", "observation@test.local", "observation-password-123", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	p, err := projects.Create(ctx, user.ID, projectRequest("Observation"))
	if err != nil {
		t.Fatal(err)
	}
	var topic uuid.UUID
	if err = pool.QueryRow(ctx, `SELECT id FROM topics WHERE project_id=$1`, p.ID).Scan(&topic); err != nil {
		t.Fatal(err)
	}
	submission, err := discussion.NewService(pool, nil).CreateSubmission(ctx, user.ID, p.ID, discussion.Submission{ClientSubmissionID: uuid.NewString(), Purpose: "message", Source: "web", Text: "Observation fixture", TopicID: &topic})
	if err != nil {
		t.Fatal(err)
	}
	svc := batch.Service{Pool: pool}
	batchID, runID, err := svc.Start(ctx, user.ID, p.ID, topic, submission.ID)
	if err != nil {
		t.Fatal(err)
	}
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: " + `{"choices":[{"delta":{"content":"Observed","finish_reason":"stop"}}]}` + "\n\n"))
		_, _ = w.Write([]byte("data: " + `{"id":"provider-fixture","model":"returned-fixture","choices":[],"usage":{"prompt_tokens":1000,"completion_tokens":50,"prompt_tokens_details":{"cached_tokens":800}}}` + "\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer gateway.Close()
	executor := batch.Executor{Pool: pool.Pool, Provider: model.NewOpenAIProvider(model.OpenAIConfig{BaseURL: gateway.URL, APIKey: "fixture"}), ModelName: "requested-fixture"}
	if err = executor.ExecuteBatch(ctx, p.ID, batchID); err != nil {
		t.Fatal(err)
	}
	var callID uuid.UUID
	var raw []byte
	var known bool
	var providerID *string
	if err = pool.QueryRow(ctx, `SELECT id,usage_json,usage_known,provider_request_id FROM model_calls WHERE run_id=$1`, runID).Scan(&callID, &raw, &known, &providerID); err != nil {
		t.Fatal(err)
	}
	var record struct {
		Total        int64               `json:"totalTokens"`
		Conservative bool                `json:"conservative"`
		Observation  model.UsageSnapshot `json:"observation"`
	}
	if err = json.Unmarshal(raw, &record); err != nil {
		t.Fatal(err)
	}
	if !known || record.Conservative || record.Total != 1050 || record.Observation.ModelName != "returned-fixture" || record.Observation.CacheReadTokens == nil || *record.Observation.CacheReadTokens != 800 || record.Observation.CacheWriteTokens != nil || providerID == nil || *providerID != "provider-fixture" {
		t.Fatalf("usage=%s known=%v provider=%v", raw, known, providerID)
	}
	if removed, err := batch.PruneModelObservations(ctx, pool.Pool); err != nil || removed != 0 {
		t.Fatalf("fresh observation pruned: %d %v", removed, err)
	}
	if _, err = pool.Exec(ctx, `UPDATE model_calls SET ended_at=now()-interval '25 hours' WHERE id=$1`, callID); err != nil {
		t.Fatal(err)
	}
	var visible int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM model_calls WHERE run_id=$1 AND ended_at>now()-interval '24 hours' AND usage_json ? 'observation'`, runID).Scan(&visible); err != nil || visible != 0 {
		t.Fatalf("expired observation visible: %d %v", visible, err)
	}
	maintenanceCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); batch.MaintainModelObservations(maintenanceCtx, pool.Pool, nil) }()
	defer func() { cancel(); <-done }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var exists bool
		if err = pool.QueryRow(ctx, `SELECT usage_json ? 'observation' FROM model_calls WHERE id=$1`, callID).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("automatic startup expiry did not remove observation")
		}
		time.Sleep(20 * time.Millisecond)
	}
	var total, used int64
	var attempt int
	if err = pool.QueryRow(ctx, `SELECT (usage_json->>'totalTokens')::bigint,attempt,usage_known,provider_request_id FROM model_calls WHERE id=$1`, callID).Scan(&total, &attempt, &known, &providerID); err != nil {
		t.Fatal(err)
	}
	if total != 1050 || attempt != 1 || !known || providerID != nil {
		t.Fatalf("recovery metadata damaged: total=%d attempt=%d known=%v provider=%v", total, attempt, known, providerID)
	}
	if err = pool.QueryRow(ctx, `SELECT used_tokens FROM discussion_batches WHERE id=$1`, batchID).Scan(&used); err != nil || used != 1050 {
		t.Fatalf("budget changed: %d %v", used, err)
	}
	if removed, err := batch.PruneModelObservations(ctx, pool.Pool); err != nil || removed != 0 {
		t.Fatalf("expiry not idempotent: %d %v", removed, err)
	}
}
