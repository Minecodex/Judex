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
	"github.com/kakj-go/Judex/internal/project"
)

func TestCoordinatorKeepsOnlyCompatiblePublicOpinionsFromItsBatch(t *testing.T) {
	d, projects, ids, pool := newDiscussionEnv(t)
	ctx := context.Background()
	owner, _, err := ids.Register(ctx, "Opinion context", "opinion-context@test.local", "opinion-context-password", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	p, err := projects.Create(ctx, owner.ID, projectRequest("Public opinions"))
	if err != nil {
		t.Fatal(err)
	}
	position, err := projects.CreatePosition(ctx, owner.ID, p.ID, project.PositionDraft{Name: "QA", Prompt: "Independent QA"})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := projects.CreateIdentity(ctx, owner.ID, p.ID, position.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	source, err := d.CreateSubmission(ctx, owner.ID, p.ID, discussion.Submission{ClientSubmissionID: uuid.NewString(), Purpose: "message", Source: "cli", Text: "Summarize existing public opinions"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := collaboration.NewService(pool).SubmissionAnalysis(ctx, owner.ID, p.ID, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	var parent uuid.UUID
	if err = pool.QueryRow(ctx, `SELECT id FROM agent_runs WHERE batch_id=$1 AND parent_run_id IS NULL`, a.BatchID).Scan(&parent); err != nil {
		t.Fatal(err)
	}
	session := uuid.New()
	if _, err = pool.Exec(ctx, `INSERT INTO agent_sessions(project_id,id,identity_id,created_at) VALUES($1,$2,$3,now())`, p.ID, session, identity.ID); err != nil {
		t.Fatal(err)
	}
	// Real persisted shape, controlled results. No private prompt is selected by
	// the reader; wrong preference revision and failed opinions are negative controls.
	for _, sample := range []struct{ state, summary, manifest string }{
		{"succeeded", "VALID_PUBLIC_OPINION", `{"positionVersion":1,"preferenceRevision":0,"workflowVersions":{}}`},
		{"succeeded", "STALE_PREFERENCE_OPINION", `{"positionVersion":1,"preferenceRevision":999,"workflowVersions":{}}`},
		{"failed", "FAILED_PRIVATE_ECHO", `{"positionVersion":1,"preferenceRevision":0,"workflowVersions":{}}`},
		{"succeeded", "STALE_WORKFLOW_OPINION", `{"positionVersion":1,"preferenceRevision":0,"workflowVersions":{"old":"version"}}`},
	} {
		if _, err = pool.Exec(ctx, `INSERT INTO agent_runs(project_id,id,session_id,batch_id,parent_run_id,identity_id,binding_version,state,requested_by,manifest,budget_snapshot,created_at,updated_at)
 VALUES($1,$2,$3,$4,$5,$6,1,$7,$8,$9::jsonb,jsonb_build_object('reason',$10::text),now(),now())`, p.ID, uuid.New(), session, a.BatchID, parent, identity.ID, sample.state, owner.ID, sample.manifest, sample.summary); err != nil {
			t.Fatal(err)
		}
	}
	checked := false
	provider := contextProvider(func(_ context.Context, req model.Request) (<-chan model.Event, error) {
		text := req.Messages[1].Content
		if !strings.Contains(text, "VALID_PUBLIC_OPINION") || strings.Contains(text, "STALE_PREFERENCE_OPINION") || strings.Contains(text, "FAILED_PRIVATE_ECHO") || strings.Contains(text, "STALE_WORKFLOW_OPINION") {
			t.Fatal("compatible public outcome lost or stale/private failed result exposed")
		}
		checked = true
		out := make(chan model.Event, 3)
		out <- model.Event{Type: "toolCallReady", ToolCallID: "record", ToolName: "record_task_analysis", ArgsJSON: `{"summary":"保留当前绑定与版本匹配的已返回公开意见及其范围；不复述失败回声。","basis":["controlled compatible public outcome"]}`}
		out <- model.Event{Type: "usage", Usage: model.UsageSnapshot{InputTokens: model.TokenCount(100), OutputTokens: model.TokenCount(50)}}
		out <- model.Event{Type: "finish"}
		close(out)
		return out, nil
	})
	e := batch.Executor{Pool: pool.Pool, Provider: provider, ModelName: "controlled-public-opinion-context"}
	if err = e.ExecuteBatch(ctx, p.ID, *a.BatchID); err != nil {
		t.Fatal(err)
	}
	if !checked {
		t.Fatal("coordinator context never checked")
	}
}
