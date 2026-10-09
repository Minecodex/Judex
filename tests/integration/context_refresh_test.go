package integrationtest_test

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/agent/batch"
	"github.com/kakj-go/Judex/internal/discussion"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
	"github.com/kakj-go/Judex/internal/project"
	"strings"
	"testing"
)

type contextProvider func(context.Context, model.Request) (<-chan model.Event, error)

func (p contextProvider) Stream(ctx context.Context, r model.Request) (<-chan model.Event, error) {
	return p(ctx, r)
}
func TestEveryModelCallRefreshesPrivateContextAndKeepsItPrivate(t *testing.T) {
	_, projects, ids, pool := newDecisionEnv(t)
	ctx := context.Background()
	user, _, err := ids.Register(ctx, "Refresh", "refresh@test.local", "refresh-password-123", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	p, err := projects.Create(ctx, user.ID, project.CreateRequest{Title: "Refresh"})
	if err != nil {
		t.Fatal(err)
	}
	role, err := projects.CreatePosition(ctx, user.ID, p.ID, project.PositionDraft{Name: "Review", Prompt: "runtime-refresh-duty"})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := projects.CreateIdentity(ctx, user.ID, p.ID, role.ID, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = projects.UpdateMyPreferences(ctx, user.ID, p.ID, role.ID, 0, "PRIVATE-OLD-PROMPT"); err != nil {
		t.Fatal(err)
	}
	otherRole, err := projects.CreatePosition(ctx, user.ID, p.ID, project.PositionDraft{Name: "Design", Prompt: "runtime-other-duty"})
	if err != nil {
		t.Fatal(err)
	}
	otherIdentity, err := projects.CreateIdentity(ctx, user.ID, p.ID, otherRole.ID, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = projects.UpdateMyPreferences(ctx, user.ID, p.ID, otherRole.ID, 0, "PRIVATE-OTHER-POSITION-PROMPT"); err != nil {
		t.Fatal(err)
	}
	var topic uuid.UUID
	if err = pool.QueryRow(ctx, `SELECT id FROM topics WHERE project_id=$1`, p.ID).Scan(&topic); err != nil {
		t.Fatal(err)
	}
	sub, err := discussion.NewService(pool, nil).CreateSubmission(ctx, user.ID, p.ID, discussion.Submission{ClientSubmissionID: uuid.NewString(), Purpose: "message", Source: "web", Text: "请保留异议", TopicID: &topic})
	if err != nil {
		t.Fatal(err)
	}
	svc := batch.Service{Pool: pool}
	bid, _, err := svc.Start(ctx, user.ID, p.ID, topic, sub.ID)
	if err != nil {
		t.Fatal(err)
	}
	childCalls := 0
	otherCalls := 0
	parentCalls := 0
	provider := contextProvider(func(ctx context.Context, r model.Request) (<-chan model.Event, error) {
		events := make(chan model.Event, 4)
		system := r.Messages[0].Content
		if strings.Contains(system, "runtime-refresh-duty") {
			childCalls++
			if strings.Contains(system, "PRIVATE-OTHER-POSITION-PROMPT") {
				t.Error("another position's preference entered the review context")
			}
			if childCalls == 1 {
				if !strings.Contains(system, "PRIVATE-OLD-PROMPT") {
					t.Error("missing initial preference")
				}
				if _, err := projects.UpdateMyPreferences(ctx, user.ID, p.ID, role.ID, 1, "PRIVATE-NEW-PROMPT"); err != nil {
					return nil, err
				}
				events <- model.Event{Type: "toolCallReady", ToolCallID: "refresh-read", ToolName: "query_work", ArgsJSON: `{"objectType":"task"}`}
			} else {
				if !strings.Contains(system, "PRIVATE-NEW-PROMPT") || strings.Contains(system, "PRIVATE-OLD-PROMPT") {
					t.Error("next model call reused old private prompt")
				}
				events <- model.Event{Type: "textDelta", TextDelta: "岗位公开意见：仍有反对，等待人工核对"}
			}
		} else if strings.Contains(system, "runtime-other-duty") {
			otherCalls++
			if !strings.Contains(system, "PRIVATE-OTHER-POSITION-PROMPT") || strings.Contains(system, "PRIVATE-OLD-PROMPT") || strings.Contains(system, "PRIVATE-NEW-PROMPT") {
				t.Error("design context did not isolate its own preference")
			}
			events <- model.Event{Type: "textDelta", TextDelta: "设计岗位的公开意见"}
		} else {
			parentCalls++
			raw, _ := json.Marshal(r.Messages)
			if strings.Contains(string(raw), "PRIVATE-") {
				t.Error("coordinator received private preference")
			}
			if parentCalls == 1 {
				args, _ := json.Marshal(map[string]string{"position": identity.ID.String(), "question": "审阅"})
				events <- model.Event{Type: "toolCallReady", ToolCallID: "position-review", ToolName: "call_agent", ArgsJSON: string(args)}
				args, _ = json.Marshal(map[string]string{"position": otherIdentity.ID.String(), "question": "设计"})
				events <- model.Event{Type: "toolCallReady", ToolCallID: "position-design", ToolName: "call_agent", ArgsJSON: string(args)}
			} else {
				events <- model.Event{Type: "textDelta", TextDelta: "保留岗位异议，待人确认"}
			}
		}
		events <- model.Event{Type: "usage", InputTokens: 30, OutputTokens: 30}
		events <- model.Event{Type: "finish"}
		close(events)
		return events, nil
	})
	executor := batch.Executor{Pool: pool.Pool, Provider: provider, ModelName: "test"}
	if err = executor.ExecuteBatch(ctx, p.ID, bid); err != nil {
		t.Fatal(err)
	}
	if childCalls != 2 || otherCalls != 1 || parentCalls != 2 {
		t.Fatalf("unexpected calls: parent=%d child=%d other=%d", parentCalls, childCalls, otherCalls)
	}
	var leaked bool
	if err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM context_checkpoints WHERE project_id=$1 AND manifest::text LIKE '%PRIVATE-%') OR EXISTS(SELECT 1 FROM agent_runs WHERE project_id=$1 AND manifest::text LIKE '%PRIVATE-%')`, p.ID).Scan(&leaked); err != nil || leaked {
		t.Fatalf("checkpoint leaked private text: %v", err)
	}
	var version int
	if err = pool.QueryRow(ctx, `SELECT (manifest->>'preferenceRevision')::int FROM agent_runs WHERE project_id=$1 AND identity_id=$2`, p.ID, identity.ID).Scan(&version); err != nil || version != 2 {
		t.Fatalf("preference revision not recorded: %d %v", version, err)
	}
}
