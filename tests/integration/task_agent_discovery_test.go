package integrationtest_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/agent/batch"
	"github.com/kakj-go/Judex/internal/collaboration"
	"github.com/kakj-go/Judex/internal/discussion"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
	"github.com/kakj-go/Judex/internal/project"
	"github.com/kakj-go/Judex/internal/work"
)

func TestTaskAgentDiscoveryMatchesDelegationScope(t *testing.T) {
	d, projects, users, pool := newDiscussionEnv(t)
	ctx := context.Background()
	user, _, err := users.Register(ctx, "Scope", "scope@agent.test", "scope-password-123", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	p, err := projects.Create(ctx, user.ID, projectRequest("Scope"))
	if err != nil {
		t.Fatal(err)
	}
	identities := []uuid.UUID{}
	for _, name := range []string{"Assigned", "Reviewer", "Unrelated"} {
		role, err := projects.CreatePosition(ctx, user.ID, p.ID, project.PositionDraft{Name: name, Prompt: name + " duty"})
		if err != nil {
			t.Fatal(err)
		}
		identity, err := projects.CreateIdentity(ctx, user.ID, p.ID, role.ID, user.ID)
		if err != nil {
			t.Fatal(err)
		}
		identities = append(identities, identity.ID)
	}
	task, err := work.NewService(pool, nil).CreateTaskDraft(ctx, user.ID, p.ID, work.TaskDraft{Title: "Read current task", ParticipantIDs: identities[:1], ReviewerIdentityID: &identities[1]})
	if err != nil {
		t.Fatal(err)
	}
	source, err := d.CreateSubmission(ctx, user.ID, p.ID, discussion.Submission{ClientSubmissionID: uuid.NewString(), Purpose: "message", DiscussionIntent: "question", Source: "web", Text: "Discover callable identities", TaskID: &task.ID})
	if err != nil {
		t.Fatal(err)
	}
	c := collaboration.NewService(pool)
	activities, err := c.ListActivity(ctx, user.ID, p.ID, task.ID)
	if err != nil || len(activities) != 1 {
		t.Fatalf("activity: %+v %v", activities, err)
	}
	seen := false
	provider := contextProvider(func(ctx context.Context, r model.Request) (<-chan model.Event, error) {
		out := make(chan model.Event, 4)
		if len(r.Messages) == 2 {
			out <- model.Event{Type: "toolCallReady", ToolCallID: "callable", ToolName: "list_agent", ArgsJSON: "{}"}
		} else if !seen {
			var result struct {
				Data struct {
					Agents []struct {
						IdentityID uuid.UUID `json:"identityId"`
					}
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(r.Messages[len(r.Messages)-1].Content), &result); err != nil {
				t.Fatal(err)
			}
			found := map[uuid.UUID]bool{}
			for _, a := range result.Data.Agents {
				found[a.IdentityID] = true
			}
			if len(found) != 2 || !found[identities[0]] || !found[identities[1]] || found[identities[2]] {
				t.Fatalf("unassignable identities advertised: %+v", found)
			}
			seen = true
			args, _ := json.Marshal(map[string]any{"summary": "Assigned and reviewer are discoverable; unrelated is excluded", "basis": []string{source.ID.String()}})
			out <- model.Event{Type: "toolCallReady", ToolCallID: "analysis", ToolName: "record_task_analysis", ArgsJSON: string(args)}
		} else {
			out <- model.Event{Type: "textDelta", TextDelta: "Discovery scoped correctly"}
		}
		out <- model.Event{Type: "usage", Usage: model.UsageSnapshot{InputTokens: model.TokenCount(100), OutputTokens: model.TokenCount(50)}}
		out <- model.Event{Type: "finish"}
		close(out)
		return out, nil
	})
	executor := batch.Executor{Pool: pool.Pool, Provider: provider, ModelName: "controlled-scope-contract"}
	if err = executor.ExecuteBatch(ctx, p.ID, *activities[0].Analysis.BatchID); err != nil {
		t.Fatal(err)
	}
	if !seen {
		t.Fatal("discovery did not reach the model")
	}
}
