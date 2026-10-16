package integrationtest_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/agent/batch"
	"github.com/kakj-go/Judex/internal/collaboration"
	"github.com/kakj-go/Judex/internal/discussion"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
	"github.com/kakj-go/Judex/internal/project"
	"github.com/kakj-go/Judex/internal/workflow"
)

// Real PostgreSQL with a controlled provider: model output is corrected at the
// storage boundary, within its original batch and budget. This is not vendor
// semantic evidence; the real-host failing samples are retained separately.
func TestCurrentWorkflowNodesRespectPositionAndFreeSubmissionScope(t *testing.T) {
	d, projects, ids, pool := newDiscussionEnv(t)
	ctx := context.Background()
	owner, _, err := ids.Register(ctx, "Current node scope", "current-node@test.local", "current-node-password", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	p, err := projects.Create(ctx, owner.ID, projectRequest("Current node scope"))
	if err != nil {
		t.Fatal(err)
	}
	backend, err := projects.CreatePosition(ctx, owner.ID, p.ID, project.PositionDraft{Name: "Backend"})
	if err != nil {
		t.Fatal(err)
	}
	qa, err := projects.CreatePosition(ctx, owner.ID, p.ID, project.PositionDraft{Name: "QA"})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := projects.CreateIdentity(ctx, owner.ID, p.ID, backend.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	body := validBody()
	body.Nodes[0].AllowedPositionIds = []string{backend.ID.String()}
	body.Nodes[1].AllowedPositionIds = []string{qa.ID.String()}
	wf := workflow.NewService(pool, nil)
	definition, err := wf.Create(ctx, owner.ID, p.ID, body)
	if err != nil {
		t.Fatal(err)
	}
	version, err := wf.Publish(ctx, owner.ID, p.ID, definition.ID, definition.Version, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO position_node_bindings(project_id,template_id,workflow_id,node_id)VALUES($1,$2,$3,'dev')`, p.ID, backend.ID, definition.ID); err != nil {
		t.Fatal(err)
	}
	ref := func(node string) collaboration.WorkflowNodeRef {
		return collaboration.WorkflowNodeRef{WorkflowID: definition.ID, VersionID: version.ID, NodeID: node, Reason: "Published node; scope does not authorize other roles to execute it"}
	}
	for _, mode := range []string{"position", "free"} {
		t.Run(mode, func(t *testing.T) {
			input := discussion.Submission{ClientSubmissionID: uuid.NewString(), Purpose: "message", Source: "cli", Text: "Only analyze references; do not create work"}
			if mode == "position" {
				input.IdentityID = &identity.ID
			}
			source, err := d.CreateSubmission(ctx, owner.ID, p.ID, input)
			if err != nil {
				t.Fatal(err)
			}
			service := collaboration.NewService(pool)
			a, err := service.SubmissionAnalysis(ctx, owner.ID, p.ID, source.ID)
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			provider := contextProvider(func(_ context.Context, request model.Request) (<-chan model.Event, error) {
				calls++
				var transcript strings.Builder
				for _, m := range request.Messages {
					transcript.WriteString(m.Content)
				}
				if mode == "position" && !strings.Contains(transcript.String(), `"candidateSources":["position"]`) {
					t.Fatal("coordinator context ignored the explicitly submitted position")
				}
				out := collaboration.TaskAnalysisOutput{Summary: "References require human clarification; no formal work changed", Basis: []string{source.ID.String()}, NeedsClarification: true}
				if calls == 1 {
					out.CurrentNodeCandidates = []collaboration.WorkflowNodeRef{ref("review")}
				} else {
					correction := "node is not allowed for the submitted position"
					if mode == "free" {
						correction = "a free submission has no current work scope"
					}
					if !strings.Contains(transcript.String(), correction) {
						t.Fatal("failed output did not receive a bounded correction")
					}
					if mode == "position" {
						out.CurrentNodeCandidates = []collaboration.WorkflowNodeRef{ref("dev")}
					}
					out.SuggestedNextNodes = []collaboration.WorkflowNodeRef{ref("review")}
				}
				args := map[string]any{"summary": out.Summary, "basis": out.Basis, "needsClarification": out.NeedsClarification}
				if len(out.CurrentNodeCandidates) != 0 {
					args["currentNodeCandidates"] = out.CurrentNodeCandidates
				}
				if len(out.SuggestedNextNodes) != 0 {
					args["suggestedNextNodes"] = out.SuggestedNextNodes
				}
				raw, _ := json.Marshal(args)
				events := make(chan model.Event, 3)
				events <- model.Event{Type: "toolCallReady", ToolCallID: uuid.NewString(), ToolName: "record_task_analysis", ArgsJSON: string(raw)}
				events <- model.Event{Type: "usage", Usage: model.UsageSnapshot{InputTokens: model.TokenCount(100), OutputTokens: model.TokenCount(50)}}
				events <- model.Event{Type: "finish"}
				close(events)
				return events, nil
			})
			executor := batch.Executor{Pool: pool.Pool, Provider: provider, ModelName: "controlled-current-scope"}
			if err = executor.ExecuteBatch(ctx, p.ID, *a.BatchID); err != nil {
				t.Fatal(err)
			}
			finished, err := service.SubmissionAnalysis(ctx, owner.ID, p.ID, source.ID)
			if err != nil || finished.ID != a.ID || finished.State != "completed" || calls != 2 {
				t.Fatalf("not corrected within the original analysis: calls=%d result=%+v error=%v", calls, finished, err)
			}
			var saved collaboration.TaskAnalysisOutput
			if err = json.Unmarshal(finished.Output, &saved); err != nil {
				t.Fatal(err)
			}
			if mode == "position" && (len(saved.CurrentNodeCandidates) != 1 || saved.CurrentNodeCandidates[0].NodeID != "dev") || mode == "free" && len(saved.CurrentNodeCandidates) != 0 {
				t.Fatalf("invalid current node persisted: %+v", saved.CurrentNodeCandidates)
			}
			var attempts, failed, succeeded, formal int
			if err = pool.QueryRow(ctx, `SELECT model_attempts,
 (SELECT count(*) FROM tool_calls c JOIN agent_runs r ON r.id=c.run_id WHERE r.batch_id=b.id AND c.state='failed'),
 (SELECT count(*) FROM tool_calls c JOIN agent_runs r ON r.id=c.run_id WHERE r.batch_id=b.id AND c.state='succeeded'),
 (SELECT count(*) FROM tasks WHERE project_id=b.project_id)
 FROM discussion_batches b WHERE b.id=$1`, a.BatchID).Scan(&attempts, &failed, &succeeded, &formal); err != nil || attempts != 2 || failed != 1 || succeeded != 1 || formal != 0 {
				t.Fatalf("budget/effects changed: attempts=%d failed=%d succeeded=%d formal=%d error=%v", attempts, failed, succeeded, formal, err)
			}
		})
	}
}
