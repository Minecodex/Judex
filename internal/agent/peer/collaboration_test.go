package peer

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/collab"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
	"github.com/kakj-go/Judex/internal/platform/auth"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

func TestNarrowedCLIListsOnlyGrantedProjects(t *testing.T) {
	f := setup(t)
	raw, err := f.s.SaveProject(f.ctx, uuid.Nil, collab.ProjectInput{Name: "Outside the device grant"})
	if err != nil {
		t.Fatal(err)
	}
	var other struct {
		ID uuid.UUID `json:"id"`
	}
	if err = json.Unmarshal(raw, &other); err != nil {
		t.Fatal(err)
	}
	principal := &auth.Principal{Kind: auth.KindCLI, UserID: f.user, ProjectScope: []uuid.UUID{f.project}}
	ctx := auth.WithPrincipal(f.ctx, principal)
	items, err := f.s.Projects(ctx)
	if err != nil || len(items) != 1 {
		t.Fatalf("narrowed list: %d %v", len(items), err)
	}
	if _, err = f.s.Project(ctx, other.ID); err == nil {
		t.Fatal("out-of-scope detail was readable")
	}
	principal.ProjectScope = nil
	items, err = f.s.Projects(ctx)
	if err != nil || len(items) != 2 {
		t.Fatalf("project discovery: %d %v", len(items), err)
	}
	if _, err = f.s.Project(ctx, f.project); err == nil {
		t.Fatal("empty discovery scope granted project access")
	}
}

func claimNextForTest(t *testing.T, f fixture, e *Executor) *run {
	t.Helper()
	var delivery uuid.UUID
	if err := f.s.Pool.QueryRow(f.ctx, `SELECT id FROM deliveries WHERE state='pending' ORDER BY created_at,id LIMIT 1`).Scan(&delivery); err != nil {
		t.Fatal(err)
	}
	r, err := e.claim(f.ctx, delivery)
	if err != nil || r == nil {
		t.Fatalf("claim: %v", err)
	}
	return r
}

func TestHumanRepliesResolveOnlyTheRequestedBranches(t *testing.T) {
	f := setup(t)
	e := &Executor{Service: f.s}
	if _, err := f.s.Post(f.ctx, f.group, collab.MessageInput{Body: "Analyze", Mentions: []collab.Mention{{Kind: "role_agent", TargetID: f.a}}}, false); err != nil {
		t.Fatal(err)
	}
	r := claimNextForTest(t, f, e)
	questions := map[uuid.UUID]uuid.UUID{}
	for _, user := range []uuid.UUID{f.user, f.other} {
		question, err := f.s.Publish(f.ctx, r.Publication, collab.MessageInput{Body: "Please confirm", Mentions: []collab.Mention{{Kind: "human", TargetID: user, RequiresResponse: true}}})
		if err != nil {
			t.Fatal(err)
		}
		questions[user] = question
	}
	if err := e.finish(f.ctx, r); err != nil {
		t.Fatal(err)
	}
	otherQuestion := questions[f.other]
	if _, err := f.s.Post(f.ctx, f.group, collab.MessageInput{Body: "Wrong respondent", ReplyTo: &otherQuestion}, false); err != nil {
		t.Fatal(err)
	}
	var unresolved int
	_ = f.s.Pool.QueryRow(f.ctx, `SELECT count(*) FROM human_waits WHERE NOT resolved`).Scan(&unresolved)
	if unresolved != 2 {
		t.Fatal("another human resolved a wait")
	}
	myQuestion := questions[f.user]
	if _, err := f.s.Post(f.ctx, f.group, collab.MessageInput{Body: "First confirmation", ReplyTo: &myQuestion}, false); err != nil {
		t.Fatal(err)
	}
	_, state, _, _, _ := f.state(t)
	if state != "active" {
		t.Fatal("one response ended another person's branch")
	}
	other := auth.WithPrincipal(f.ctx, &auth.Principal{Kind: auth.KindWeb, UserID: f.other})
	if _, err := f.s.Post(other, f.group, collab.MessageInput{Body: "Ordinary additional context"}, false); err != nil {
		t.Fatal(err)
	}
	var count int
	_ = f.s.Pool.QueryRow(f.ctx, `SELECT count(*) FROM deliveries`).Scan(&count)
	if count != 1 {
		t.Fatal("ordinary response woke an agent")
	}
	if _, err := f.s.Post(other, f.group, collab.MessageInput{Body: "Second confirmation; continue", ReplyTo: &otherQuestion, Mentions: []collab.Mention{{Kind: "role_agent", TargetID: f.a}}}, false); err != nil {
		t.Fatal(err)
	}
	_ = f.s.Pool.QueryRow(f.ctx, `SELECT count(*) FROM human_waits WHERE NOT resolved`).Scan(&unresolved)
	if unresolved != 0 {
		t.Fatal("explicit reply did not resolve the final wait")
	}
	next := claimNextForTest(t, f, e)
	if err := e.finish(f.ctx, next); err != nil {
		t.Fatal(err)
	}
	_, state, _, _, _ = f.state(t)
	if state != "settled" {
		t.Fatal("answered discussion did not naturally settle")
	}
}

func TestConcurrentContinueGrantsOnlyOneAllowance(t *testing.T) {
	f := setup(t)
	if _, err := f.s.Post(f.ctx, f.group, collab.MessageInput{Body: "Work", Mentions: []collab.Mention{{Kind: "role_agent", TargetID: f.a}}}, false); err != nil {
		t.Fatal(err)
	}
	id, _, _, _, _ := f.state(t)
	if err := f.s.Pause(f.ctx, f.group, id, "loop_limit"); err != nil {
		t.Fatal(err)
	}
	_, _, _, _, version := f.state(t)
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, user := range []uuid.UUID{f.user, f.other} {
		go func(user uuid.UUID) {
			<-start
			results <- f.s.Control(auth.WithPrincipal(f.ctx, &auth.Principal{Kind: auth.KindWeb, UserID: user}), f.group, id, version, "continue", "")
		}(user)
	}
	close(start)
	success, conflict := 0, 0
	for range 2 {
		err := <-results
		if err == nil {
			success++
		} else if apierrors.IsCode(err, apierrors.VersionConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("success=%d conflicts=%d", success, conflict)
	}
	_, _, _, allowance, _ := f.state(t)
	if allowance != 20 {
		t.Fatalf("allowance=%d", allowance)
	}
}

func TestParallelClosuresShareTheLastAllowanceAndDeduplicateTools(t *testing.T) {
	f := setup(t)
	var position uuid.UUID
	_ = f.s.Pool.QueryRow(f.ctx, `SELECT position_id FROM role_agents WHERE id=$1`, f.a).Scan(&position)
	extra, err := f.s.SaveTemplate(f.ctx, f.project, uuid.Nil, "positions", collab.TemplateInput{Name: "Third perspective"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.SaveGroup(f.ctx, f.project, f.group, collab.GroupInput{Name: "Parallel branches", ExpectedVersion: 1, Members: []collab.MemberRoles{{UserID: f.user, PositionIDs: []uuid.UUID{position, extra}}, {UserID: f.other, PositionIDs: []uuid.UUID{position}}}}); err != nil {
		t.Fatal(err)
	}
	var c uuid.UUID
	_ = f.s.Pool.QueryRow(f.ctx, `SELECT id FROM role_agents WHERE position_id=$1`, extra).Scan(&c)
	if _, err = f.s.Pool.Exec(f.ctx, `UPDATE projects SET loop_limit=2 WHERE id=$1`, f.project); err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.Post(f.ctx, f.group, collab.MessageInput{Body: "Branch", Mentions: []collab.Mention{{Kind: "role_agent", TargetID: f.a}}}, false); err != nil {
		t.Fatal(err)
	}
	e := &Executor{Service: f.s}
	first := claimNextForTest(t, f, e)
	if _, err = f.s.Publish(f.ctx, first.Publication, collab.MessageInput{Body: "Two peer branches", Mentions: []collab.Mention{{Kind: "role_agent", TargetID: f.b}, {Kind: "role_agent", TargetID: c}}}); err != nil {
		t.Fatal(err)
	}
	if err = e.finish(f.ctx, first); err != nil {
		t.Fatal(err)
	}
	runs := []*run{claimNextForTest(t, f, e), claimNextForTest(t, f, e)}
	start := make(chan struct{})
	results := make(chan error, 2)
	calls := make([]model.ToolCall, 2)
	for i, r := range runs {
		args, _ := json.Marshal(map[string]any{"body": "Close this branch", "mentions": []collab.Mention{{Kind: "role_agent", TargetID: f.a}}})
		calls[i] = model.ToolCall{ID: uuid.NewString(), Name: "send_message", Arguments: string(args)}
		r.Checkpoint = []model.Message{{Role: "assistant", ToolCalls: []model.ToolCall{calls[i]}}}
		if err = e.checkpoint(f.ctx, r); err != nil {
			t.Fatal(err)
		}
		go func(r *run, call model.ToolCall) { <-start; results <- e.executeTool(f.ctx, r, call, nil) }(r, calls[i])
	}
	close(start)
	for range 2 {
		if err = <-results; err != nil {
			t.Fatal(err)
		}
	}
	_, state, loops, allowance, _ := f.state(t)
	if state != "paused" || loops != 2 || allowance != 2 {
		t.Fatalf("%s loops=%d allowance=%d", state, loops, allowance)
	}
	for i, r := range runs {
		if err = e.executeTool(f.ctx, r, calls[i], nil); err != nil {
			t.Fatal(err)
		}
	}
	_, _, loops, _, _ = f.state(t)
	if loops != 2 {
		t.Fatal("repeated tool counted another closure")
	}
	var messages int
	_ = f.s.Pool.QueryRow(f.ctx, `SELECT count(*) FROM messages WHERE role_agent_id IS NOT NULL`).Scan(&messages)
	if messages != 3 {
		t.Fatalf("duplicate branch publications: %d", messages)
	}
}

func TestCompletedClosureJournalSurvivesLostCheckpoint(t *testing.T) {
	f := setup(t)
	e := &Executor{Service: f.s}
	if _, err := f.s.Post(f.ctx, f.group, collab.MessageInput{Body: "Start", Mentions: []collab.Mention{{Kind: "role_agent", TargetID: f.a}}}, false); err != nil {
		t.Fatal(err)
	}
	a := claimNextForTest(t, f, e)
	if _, err := f.s.Publish(f.ctx, a.Publication, collab.MessageInput{Body: "To peer", Mentions: []collab.Mention{{Kind: "role_agent", TargetID: f.b}}}); err != nil {
		t.Fatal(err)
	}
	if err := e.finish(f.ctx, a); err != nil {
		t.Fatal(err)
	}
	b := claimNextForTest(t, f, e)
	args, _ := json.Marshal(map[string]any{"body": "Back to first role", "mentions": []collab.Mention{{Kind: "role_agent", TargetID: f.a}}})
	call := model.ToolCall{ID: "completed-closure", Name: "send_message", Arguments: string(args)}
	before := []model.Message{{Role: "assistant", ToolCalls: []model.ToolCall{call}}}
	b.Checkpoint = before
	if err := e.checkpoint(f.ctx, b); err != nil {
		t.Fatal(err)
	}
	if err := e.executeTool(f.ctx, b, call, nil); err != nil {
		t.Fatal(err)
	}
	// Crash after the effect/journal transaction, before persisting its paired response.
	raw, _ := json.Marshal(before)
	if _, err := f.s.Pool.Exec(f.ctx, `UPDATE role_runs SET checkpoint=$2,lease_until=now()-interval '1 second' WHERE id=$1`, b.Publication.RunID, raw); err != nil {
		t.Fatal(err)
	}
	restarted := &Executor{Service: f.s}
	restored, err := restarted.claim(f.ctx, b.Publication.DeliveryID)
	if err != nil || restored == nil {
		t.Fatalf("restart claim: %v", err)
	}
	if err = restarted.recoverPending(f.ctx, restored, nil); err != nil {
		t.Fatal(err)
	}
	_, _, loops, _, _ := f.state(t)
	if loops != 1 {
		t.Fatalf("recovery repeated closure: %d", loops)
	}
	var publications int
	_ = f.s.Pool.QueryRow(f.ctx, `SELECT count(*) FROM messages WHERE role_agent_id=$1`, f.b).Scan(&publications)
	if publications != 1 {
		t.Fatal("completed public effect replayed")
	}
	if restored.Checkpoint[len(restored.Checkpoint)-1].Role != "tool" {
		t.Fatal("tool result not paired after recovery")
	}
}

func TestMultipleClosedMentionsCannotSpendTheNextAllowance(t *testing.T) {
	f := setup(t)
	e := &Executor{Service: f.s}
	var position uuid.UUID
	_ = f.s.Pool.QueryRow(f.ctx, `SELECT position_id FROM role_agents WHERE id=$1`, f.a).Scan(&position)
	third, err := f.s.SaveTemplate(f.ctx, f.project, uuid.Nil, "positions", collab.TemplateInput{Name: "Third role"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.SaveGroup(f.ctx, f.project, f.group, collab.GroupInput{Name: "Three-role path", ExpectedVersion: 1, Members: []collab.MemberRoles{{UserID: f.user, PositionIDs: []uuid.UUID{position, third}}, {UserID: f.other, PositionIDs: []uuid.UUID{position}}}}); err != nil {
		t.Fatal(err)
	}
	var c uuid.UUID
	_ = f.s.Pool.QueryRow(f.ctx, `SELECT id FROM role_agents WHERE position_id=$1`, third).Scan(&c)
	if _, err = f.s.Pool.Exec(f.ctx, `UPDATE projects SET loop_limit=1 WHERE id=$1`, f.project); err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.Post(f.ctx, f.group, collab.MessageInput{Body: "Start", Mentions: []collab.Mention{{Kind: "role_agent", TargetID: f.a}}}, false); err != nil {
		t.Fatal(err)
	}
	for _, target := range []uuid.UUID{f.b, c} {
		r := claimNextForTest(t, f, e)
		if _, err = f.s.Publish(f.ctx, r.Publication, collab.MessageInput{Body: "Forward", Mentions: []collab.Mention{{Kind: "role_agent", TargetID: target}}}); err != nil {
			t.Fatal(err)
		}
		if err = e.finish(f.ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	r := claimNextForTest(t, f, e)
	if _, err = f.s.Publish(f.ctx, r.Publication, collab.MessageInput{Body: "Two closures in one message", Mentions: []collab.Mention{{Kind: "role_agent", TargetID: f.a}, {Kind: "role_agent", TargetID: f.b}}}); err != nil {
		t.Fatal(err)
	}
	id, state, loops, allowance, version := f.state(t)
	if state != "paused" || loops != 1 || allowance != 1 {
		t.Fatalf("%s loops=%d allowance=%d", state, loops, allowance)
	}
	var pending int
	_ = f.s.Pool.QueryRow(f.ctx, `SELECT count(*) FROM deliveries WHERE closure_pending`).Scan(&pending)
	if pending != 1 {
		t.Fatal("remaining closure was lost")
	}
	if err = f.s.Control(f.ctx, f.group, id, version, "continue", ""); err != nil {
		t.Fatal(err)
	}
	_, state, loops, allowance, version = f.state(t)
	if state != "paused" || loops != 2 || allowance != 2 {
		t.Fatalf("deferred closure was not charged exactly once: %s %d/%d", state, loops, allowance)
	}
	if err = f.s.Control(f.ctx, f.group, id, version, "continue", ""); err != nil {
		t.Fatal(err)
	}
	_, state, loops, allowance, _ = f.state(t)
	if state != "active" || loops != 2 || allowance != 3 {
		t.Fatalf("deferred closure repeated: %s %d/%d", state, loops, allowance)
	}
}
