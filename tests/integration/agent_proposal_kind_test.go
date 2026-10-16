package integrationtest_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/agent/batch"
	"github.com/kakj-go/Judex/internal/discussion"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
	"github.com/kakj-go/Judex/internal/work"
)

func TestAgentProposalPersistsSelectedKindWithoutApplyingScope(t *testing.T) {
	_, projects, users, pool := newDecisionEnv(t)
	ctx := context.Background()
	owner, _, err := users.Register(ctx, "Draft owner", "proposal-kind@test.local", "proposal-kind-password", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	p, err := projects.Create(ctx, owner.ID, projectRequest("Draft kind"))
	if err != nil {
		t.Fatal(err)
	}
	w := work.NewService(pool, nil)
	plan, err := w.CreatePlanDraft(ctx, owner.ID, p.ID, "Original", "Original goal", "Original criteria", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var topic uuid.UUID
	if err = pool.QueryRow(ctx, `SELECT id FROM topics WHERE project_id=$1`, p.ID).Scan(&topic); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"", "work_change"} {
		t.Run("kind="+kind, func(t *testing.T) {
			source, err := discussion.NewService(pool, nil).CreateSubmission(ctx, owner.ID, p.ID, discussion.Submission{ClientSubmissionID: uuid.NewString(), Purpose: "message", Source: "cli", Text: "Prepare a scope draft; human approval remains required", TopicID: &topic})
			if err != nil {
				t.Fatal(err)
			}
			batchID, _, err := (&batch.Service{Pool: pool}).Start(ctx, owner.ID, p.ID, topic, source.ID)
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			provider := contextProvider(func(_ context.Context, _ model.Request) (<-chan model.Event, error) {
				calls++
				out := make(chan model.Event, 4)
				if calls == 1 {
					args := map[string]any{"reason": "Controlled draft only", "changes": []any{map[string]any{"operation": "update_scope", "targetType": "plan", "targetId": plan.ID.String(), "expectedVersion": 1, "fields": map[string]any{"goal": "Only after human approval"}}}}
					if kind != "" {
						args["kind"] = kind
					}
					raw, _ := json.Marshal(args)
					out <- model.Event{Type: "toolCallReady", ToolCallID: "draft-scope", ToolName: "propose_changes", ArgsJSON: string(raw)}
				} else {
					out <- model.Event{Type: "textDelta", TextDelta: "草稿已准备，等待人工确认"}
				}
				out <- model.Event{Type: "usage", Usage: model.UsageSnapshot{InputTokens: model.TokenCount(80), OutputTokens: model.TokenCount(40)}}
				out <- model.Event{Type: "finish"}
				close(out)
				return out, nil
			})
			if err = (&batch.Executor{Pool: pool.Pool, Provider: provider, ModelName: "controlled"}).ExecuteBatch(ctx, p.ID, batchID); err != nil {
				t.Fatal(err)
			}
			var stored, status string
			if err = pool.QueryRow(ctx, `SELECT kind,status FROM proposals WHERE project_id=$1 ORDER BY created_at DESC LIMIT 1`, p.ID).Scan(&stored, &status); err != nil {
				t.Fatal(err)
			}
			expected := kind
			if expected == "" {
				expected = "work_arrangement"
			}
			if stored != expected || status != "draft" {
				t.Fatalf("kind/state %s/%s, want %s/draft", stored, status, expected)
			}
			var goal string
			var version int64
			if err = pool.QueryRow(ctx, `SELECT goal,version FROM plans WHERE id=$1`, plan.ID).Scan(&goal, &version); err != nil || goal != "Original goal" || version != 1 {
				t.Fatalf("model draft altered formal scope: %s v%d %v", goal, version, err)
			}
		})
	}
}
