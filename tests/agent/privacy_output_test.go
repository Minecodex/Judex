package agent_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	agentcontext "github.com/kakj-go/Judex/internal/agent/context"
	"github.com/kakj-go/Judex/internal/agent/runner"
	"github.com/kakj-go/Judex/internal/agent/tools"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
)

type privacyProvider struct{ events []model.Event }

func (p privacyProvider) Stream(_ context.Context, _ model.Request) (<-chan model.Event, error) {
	stream := make(chan model.Event, len(p.events)+2)
	for _, event := range p.events {
		stream <- event
	}
	stream <- model.Event{Type: "usage", Usage: model.UsageSnapshot{InputTokens: model.TokenCount(100), OutputTokens: model.TokenCount(10)}}
	stream <- model.Event{Type: "finish"}
	close(stream)
	return stream, nil
}

func TestPrivateOutputStopsBeforeCheckpointAndToolDispatch(t *testing.T) {
	private := "PRIVATE_ROLE_MARKER：这是仅用于个人工作的私有偏好，不能成为公开标签。"
	for _, test := range []struct {
		name   string
		events []model.Event
	}{
		{"full_text", []model.Event{{Type: "textDelta", TextDelta: private}}},
		{"label_only", []model.Event{{Type: "textDelta", TextDelta: "（PRIVATE_ROLE_MARKER）岗位公开意见"}}},
		{"escaped_tool_argument", []model.Event{{Type: "toolCallReady", ToolCallID: "unsafe-write", ToolName: "write", ArgsJSON: `{"path":"output.txt","content":"\u0050RIVATE_ROLE_MARKER"}`}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			identity := uuid.New()
			journal := &checkpointJournal{}
			registry := tools.New()
			tools.RegisterDefaults(registry)
			r := runner.NewRunnerForTest(privacyProvider{test.events}, registry)
			outcome := r.Run(context.Background(), runner.RunRequest{RunID: uuid.New(), ModelName: "test", PrincipalKind: "position", Manifest: agentcontext.Build(agentcontext.Facts{IdentityID: &identity, PreferencePrompt: private}), Journal: journal, Budget: runner.DefaultBudget()})
			if outcome.State != "failed" || !strings.Contains(outcome.Summary, "private_context_disclosure") {
				t.Fatalf("private output must be held: %+v", outcome)
			}
			if strings.Contains(outcome.Summary, "PRIVATE_ROLE_MARKER") || len(journal.Snapshots) != 0 || len(journal.Results) != 0 || outcome.ToolCalls != 0 {
				t.Fatal("rejected content was persisted or dispatched")
			}
			if !outcome.UsageKnown || outcome.TokensUsed != 110 || outcome.ModelCalls != 1 {
				t.Fatal("real provider usage must still be accounted for")
			}
		})
	}
}

func TestPrivateOutputGuardRetainsRevisedValuesAndAllowsPublicStyle(t *testing.T) {
	guard := agentcontext.PublicOutputGuard{}
	identity := uuid.New()
	guard.Add(agentcontext.Build(agentcontext.Facts{IdentityID: &identity, PreferencePrompt: "旧私有约束：这个完整句子仅限本人上下文使用。"}))
	guard.Add(agentcontext.Build(agentcontext.Facts{IdentityID: &identity, PreferencePrompt: "新私有约束：另一个完整句子仅限本人上下文使用。"}))
	for _, value := range []string{"旧私有约束：这个完整句子仅限本人上下文使用。", "新私有约束：另一个完整句子仅限本人上下文使用。"} {
		if guard.Check(value) == nil {
			t.Fatal("old and fresh private values must both remain protected")
		}
	}
	guard.Add(agentcontext.Build(agentcontext.Facts{IdentityID: &identity, PreferencePrompt: "PostgreSQL"}))
	if err := guard.Check("公开接口仍需核对；数据库采用 PostgreSQL，尚未验证并发写。"); err != nil {
		t.Fatal("ordinary public wording and style choices must remain usable")
	}
	raw, _ := json.Marshal(map[string]any{"nested": []string{"新私有约束：另一个完整句子仅限本人上下文使用。"}})
	if guard.CheckJSON(string(raw)) == nil {
		t.Fatal("nested tool strings must not bypass the guard")
	}
}
