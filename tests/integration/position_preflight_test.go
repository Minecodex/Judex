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
)

func TestInvalidPositionLookupCanBeCorrectedWithoutInventingAFailedOpinion(t *testing.T) {
	d, projects, ids, pool := newDiscussionEnv(t)
	ctx := context.Background()
	owner, _, err := ids.Register(ctx, "Preflight", "position-preflight@test.local", "preflight-password", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	p, err := projects.Create(ctx, owner.ID, projectRequest("Delegation preflight"))
	if err != nil {
		t.Fatal(err)
	}
	source, err := d.CreateSubmission(ctx, owner.ID, p.ID, discussion.Submission{ClientSubmissionID: uuid.NewString(), Purpose: "message", Source: "cli", Text: "Clarify the incomplete request"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := collaboration.NewService(pool).SubmissionAnalysis(ctx, owner.ID, p.ID, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	provider := contextProvider(func(_ context.Context, req model.Request) (<-chan model.Event, error) {
		calls++
		out := make(chan model.Event, 3)
		if calls == 1 {
			out <- model.Event{Type: "toolCallReady", ToolCallID: "bad-name", ToolName: "call_agent", ArgsJSON: `{"position":"unresolved holder display name","question":"Clarify"}`}
		} else {
			if !strings.Contains(req.Messages[len(req.Messages)-1].Content, "use identityId") {
				t.Fatal("lookup failure was not returned for correction")
			}
			out <- model.Event{Type: "toolCallReady", ToolCallID: "record", ToolName: "record_task_analysis", ArgsJSON: `{"summary":"岗位名未解析，未委派；请求缺少信息，保留澄清而不伪造岗位意见。","needsClarification":true,"basis":["controlled unresolvable position request"]}`}
		}
		out <- model.Event{Type: "usage", Usage: model.UsageSnapshot{InputTokens: model.TokenCount(100), OutputTokens: model.TokenCount(20)}}
		out <- model.Event{Type: "finish"}
		close(out)
		return out, nil
	})
	e := batch.Executor{Pool: pool.Pool, Provider: provider, ModelName: "controlled-delegation-preflight"}
	if err = e.ExecuteBatch(ctx, p.ID, *a.BatchID); err != nil {
		t.Fatal(err)
	}
	finished, err := collaboration.NewService(pool).SubmissionAnalysis(ctx, owner.ID, p.ID, source.ID)
	if err != nil || finished.State != "completed" {
		t.Fatalf("no invocation became a failed opinion: %+v %v", finished, err)
	}
	var children int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM agent_runs WHERE batch_id=$1 AND parent_run_id IS NOT NULL`, a.BatchID).Scan(&children); err != nil || children != 0 {
		t.Fatalf("preflight created a child: %d %v", children, err)
	}
}
