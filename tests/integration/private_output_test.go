package integrationtest_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/agent/batch"
	"github.com/kakj-go/Judex/internal/discussion"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
	"github.com/kakj-go/Judex/internal/project"
)

func TestPrivatePositionEchoIsHeldBeforePublicJournal(t *testing.T) {
	_, projects, ids, pool := newDecisionEnv(t)
	ctx := context.Background()
	user, _, err := ids.Register(ctx, "Private", "private-output@test.local", "private-output-password", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	p, err := projects.Create(ctx, user.ID, project.CreateRequest{Title: "Private output"})
	if err != nil {
		t.Fatal(err)
	}
	role, err := projects.CreatePosition(ctx, user.ID, p.ID, project.PositionDraft{Name: "Review", Prompt: "private-output-role-duty"})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := projects.CreateIdentity(ctx, user.ID, p.ID, role.ID, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = projects.UpdateMyPreferences(ctx, user.ID, p.ID, role.ID, 0, "SENSITIVE_ROLE_MARKER：这是不能复制到工具日志的个人偏好内容。"); err != nil {
		t.Fatal(err)
	}
	var topic uuid.UUID
	if err = pool.QueryRow(ctx, `SELECT id FROM topics WHERE project_id=$1`, p.ID).Scan(&topic); err != nil {
		t.Fatal(err)
	}
	sub, err := discussion.NewService(pool, nil).CreateSubmission(ctx, user.ID, p.ID, discussion.Submission{ClientSubmissionID: uuid.NewString(), Purpose: "message", Source: "web", Text: "征询公开岗位意见", TopicID: &topic})
	if err != nil {
		t.Fatal(err)
	}
	service := batch.Service{Pool: pool}
	bid, _, err := service.Start(ctx, user.ID, p.ID, topic, sub.ID)
	if err != nil {
		t.Fatal(err)
	}
	parentCalls, childCalls := 0, 0
	provider := contextProvider(func(_ context.Context, request model.Request) (<-chan model.Event, error) {
		out := make(chan model.Event, 4)
		if strings.Contains(request.Messages[0].Content, "private-output-role-duty") {
			childCalls++
			if !strings.Contains(request.Messages[0].Content, "SENSITIVE_ROLE_MARKER") {
				t.Error("preference must remain in the actual private model input")
			}
			out <- model.Event{Type: "textDelta", TextDelta: "（SENSITIVE_ROLE_MARKER）公开意见"}
		} else {
			parentCalls++
			raw, _ := json.Marshal(request.Messages)
			if strings.Contains(string(raw), "SENSITIVE_ROLE_MARKER") {
				t.Error("coordinator received private role output")
			}
			if parentCalls == 1 {
				args, _ := json.Marshal(map[string]string{"position": identity.ID.String(), "question": "请给公开意见"})
				out <- model.Event{Type: "toolCallReady", ToolCallID: "private-review", ToolName: "call_agent", ArgsJSON: string(args)}
			} else {
				out <- model.Event{Type: "textDelta", TextDelta: "岗位意见被边界检查阻止，保留缺口等待核对。"}
			}
		}
		out <- model.Event{Type: "usage", Usage: model.UsageSnapshot{InputTokens: model.TokenCount(30), OutputTokens: model.TokenCount(30)}}
		out <- model.Event{Type: "finish"}
		close(out)
		return out, nil
	})
	executor := batch.Executor{Pool: pool.Pool, Provider: provider, ModelName: "test"}
	if err = executor.ExecuteBatch(ctx, p.ID, bid); err != nil {
		t.Fatal(err)
	}
	var state string
	if err = pool.QueryRow(ctx, `SELECT state FROM discussion_batches WHERE id=$1`, bid).Scan(&state); err != nil || state != "waiting_human" {
		t.Fatalf("missing role opinion must hold the batch: %s %v", state, err)
	}
	var leaked bool
	if err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM context_checkpoints WHERE project_id=$1 AND manifest::text LIKE '%SENSITIVE_ROLE_MARKER%') OR EXISTS(SELECT 1 FROM tool_calls WHERE project_id=$1 AND result_json::text LIKE '%SENSITIVE_ROLE_MARKER%') OR EXISTS(SELECT 1 FROM agent_runs WHERE project_id=$1 AND manifest::text LIKE '%SENSITIVE_ROLE_MARKER%')`, p.ID).Scan(&leaked); err != nil || leaked {
		t.Fatalf("private echo reached a durable/public output: %v", err)
	}
	if parentCalls != 2 || childCalls != 1 {
		t.Fatalf("no silent retry or fabricated opinion: parent=%d child=%d", parentCalls, childCalls)
	}
}
