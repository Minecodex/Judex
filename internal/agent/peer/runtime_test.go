package peer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/kakj-go/Judex/internal/collab"
	"github.com/kakj-go/Judex/internal/identity"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	"github.com/kakj-go/Judex/internal/job"
	"github.com/kakj-go/Judex/internal/platform/auth"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

type fixture struct {
	ctx                               context.Context
	s                                 *collab.Service
	project, group, user, other, a, b uuid.UUID
	engine                            *job.Engine
}

func setup(t *testing.T) fixture {
	t.Helper()
	dsn := os.Getenv("JUDEX_V2_TEST_DSN")
	if dsn == "" {
		t.Skip("set JUDEX_V2_TEST_DSN to run isolated PostgreSQL integration tests")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	name := "judex_v2_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(dsn)
	u.Path = "/" + name
	pool, err := postgres.Open(ctx, postgres.Options{URL: u.String()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(ctx, "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
		admin.Close(ctx)
	})
	if err = pool.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	ids := identity.NewService(pool, identity.NewRateLimiter(pool.Pool, nil), identity.Options{RegisterPerIP: 1000}, nil)
	first, _, err := ids.Register(ctx, "Role owner", "one@test.local", "V2-test-password-123", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := ids.Register(ctx, "Peer owner", "two@test.local", "V2-test-password-123", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	ctx = auth.WithPrincipal(ctx, &auth.Principal{Kind: auth.KindWeb, UserID: first.ID, AuthVersion: 1})
	s := &collab.Service{Pool: pool}
	var p struct {
		ID uuid.UUID `json:"id"`
	}
	raw, err := s.SaveProject(ctx, uuid.Nil, collab.ProjectInput{Name: "Controlled roles", LoopLimit: 10})
	if err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal(raw, &p)
	if err = s.SaveMember(ctx, p.ID, collab.ProjectMemberInput{UserID: second.ID, Role: "member"}, false); err != nil {
		t.Fatal(err)
	}
	position, err := s.SaveTemplate(ctx, p.ID, uuid.Nil, "positions", collab.TemplateInput{Name: "Engineer", Prompt: "Role guidance"})
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		ID      uuid.UUID `json:"id"`
		Members []struct {
			UserID uuid.UUID `json:"userId"`
			Roles  []struct {
				ID uuid.UUID `json:"id"`
			} `json:"roles"`
		} `json:"members"`
	}
	raw, err = s.SaveGroup(ctx, p.ID, uuid.Nil, collab.GroupInput{Name: "Peer discussion", Members: []collab.MemberRoles{{UserID: first.ID, PositionIDs: []uuid.UUID{position}}, {UserID: second.ID, PositionIDs: []uuid.UUID{position}}}})
	if err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal(raw, &g)
	f := fixture{ctx: ctx, s: s, project: p.ID, group: g.ID, user: first.ID, other: second.ID, engine: job.NewEngine(pool.Pool, job.Options{PollInterval: time.Millisecond}, slog.Default(), nil)}
	for _, m := range g.Members {
		if m.UserID == first.ID {
			f.a = m.Roles[0].ID
		} else {
			f.b = m.Roles[0].ID
		}
	}
	return f
}

type peerModel struct {
	a, b    uuid.UUID
	mu      sync.Mutex
	calls   int
	prompts []string
}

func (p *peerModel) Stream(ctx context.Context, req model.Request) (<-chan model.Event, error) {
	p.mu.Lock()
	p.calls++
	p.prompts = append(p.prompts, req.Messages[0].Content)
	p.mu.Unlock()
	events := make(chan model.Event, 4)
	if req.Messages[len(req.Messages)-1].Role == "tool" {
		events <- model.Event{Type: "finish", FinishReason: "stop"}
	} else {
		target := p.a
		if strings.Contains(req.Messages[0].Content, "ID: "+p.a.String()) {
			target = p.b
		}
		args, _ := json.Marshal(map[string]any{"body": "A public peer perspective", "mentions": []collab.Mention{{Kind: "role_agent", TargetID: target}}})
		events <- model.Event{Type: "toolCallReady", ToolCallID: uuid.NewString(), ToolName: "send_message", ArgsJSON: string(args)}
		events <- model.Event{Type: "finish", FinishReason: "tool_calls"}
	}
	close(events)
	return events, nil
}
func (f fixture) executor(provider model.Provider) *Executor {
	return &Executor{Service: f.s, ResolveModel: func(context.Context, uuid.UUID, uuid.UUID) (model.Provider, string, int64, int64, error) {
		return provider, "controlled", 128000, 2048, nil
	}}
}
func (f fixture) step(t *testing.T, e *Executor) {
	t.Helper()
	j, err := f.engine.Claim(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	err = e.ExecuteJob(f.ctx, *j)
	if err != nil && err != job.ErrBusy {
		t.Fatal(err)
	}
	if err = f.engine.Finish(f.ctx, j, err); err != nil {
		t.Fatal(err)
	}
}
func (f fixture) state(t *testing.T) (uuid.UUID, string, int, int, int64) {
	t.Helper()
	var id uuid.UUID
	var state string
	var loops, allowance int
	var version int64
	err := f.s.Pool.QueryRow(f.ctx, `SELECT id,state,loop_count,allowance,version FROM discussions WHERE group_id=$1 ORDER BY started_at DESC LIMIT 1`, f.group).Scan(&id, &state, &loops, &allowance, &version)
	if err != nil {
		t.Fatal(err)
	}
	return id, state, loops, allowance, version
}
func TestPeerLoopPauseMergeContinueTerminate(t *testing.T) {
	f := setup(t)
	model := &peerModel{a: f.a, b: f.b}
	executor := f.executor(model)
	if _, err := f.s.Post(f.ctx, f.group, collab.MessageInput{Body: "Ordinary text @Engineer"}, false); err != nil {
		t.Fatal(err)
	}
	var count int
	_ = f.s.Pool.QueryRow(f.ctx, `SELECT count(*) FROM discussions`).Scan(&count)
	if count != 0 {
		t.Fatal("ordinary message triggered AI")
	}
	if _, err := f.s.Post(f.ctx, f.group, collab.MessageInput{Body: "Discuss", Mentions: []collab.Mention{{Kind: "role_agent", TargetID: f.a}}}, false); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 30; i++ {
		_, state, _, _, _ := f.state(t)
		if state == "paused" {
			break
		}
		f.step(t, executor)
	}
	id, state, loops, allowance, version := f.state(t)
	if state != "paused" || loops != 10 || allowance != 10 {
		t.Fatalf("state=%s loops=%d allowance=%d", state, loops, allowance)
	}
	// New triggers cannot acquire a new discussion or allowance while paused.
	if _, err := f.s.Post(f.ctx, f.group, collab.MessageInput{Body: "New local work"}, true); err != nil {
		t.Fatal(err)
	}
	same, _, newLoops, newAllowance, _ := f.state(t)
	if same != id || newLoops != 10 || newAllowance != 10 {
		t.Fatal("paused trigger bypassed allowance")
	}
	// Competing human controls use optimistic version checking.
	if err := f.s.Control(f.ctx, f.group, id, version, "continue", "Prioritize evidence"); err != nil {
		t.Fatal(err)
	}
	if err := f.s.Control(f.ctx, f.group, id, version, "continue", ""); !apierrors.IsCode(err, apierrors.VersionConflict) {
		t.Fatalf("duplicate continue: %v", err)
	}
	_, state, _, allowance, version = f.state(t)
	if state != "active" || allowance != 20 {
		t.Fatal("continue did not extend one allowance")
	}
	if err := f.s.Control(f.ctx, f.group, id, version, "terminate", ""); err != nil {
		t.Fatal(err)
	}
	var pub collab.Publication
	pub.GroupID = f.group
	pub.UserID = f.user
	pub.RoleID = f.a
	pub.DiscussionID = id
	_ = f.s.Pool.QueryRow(f.ctx, `SELECT run.id,run.fence FROM role_runs run WHERE run.discussion_id=$1 ORDER BY created_at LIMIT 1`, id).Scan(&pub.RunID, &pub.Fence)
	if _, err := f.s.Publish(f.ctx, pub, collab.MessageInput{Body: "late result"}); err == nil {
		t.Fatal("late publication accepted")
	}
	if _, err := f.s.Post(f.ctx, f.group, collab.MessageInput{Body: "Fresh discussion", Mentions: []collab.Mention{{Kind: "role_agent", TargetID: f.a}}}, false); err != nil {
		t.Fatal(err)
	}
	next, _, _, _, _ := f.state(t)
	if next == id {
		t.Fatal("new trigger could not create discussion after termination")
	}
}
func TestSharedSnapshotCompressionAndOriginalReferences(t *testing.T) {
	f := setup(t)
	for i := 0; i < 35; i++ {
		if _, err := f.s.Post(f.ctx, f.group, collab.MessageInput{Body: fmt.Sprintf("Question %d: disagreement remains unresolved. ", i) + strings.Repeat("evidence ", 600)}, false); err != nil {
			t.Fatal(err)
		}
	}
	_, err := f.s.Pool.Exec(f.ctx, `INSERT INTO model_catalog(id,display_name,provider,model_name,limits,created_at,updated_at) VALUES($1,'small','openai-compatible','controlled','{"maxInputTokens":20000}',now(),now())`, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	// Bound the group budget through its actual project model.
	_, err = f.s.Pool.Exec(f.ctx, `UPDATE projects SET default_model_id=(SELECT id FROM model_catalog LIMIT 1) WHERE id=$1`, f.project)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	summary := func(ctx context.Context, input string, budget int64) (string, error) {
		calls++
		if !strings.Contains(input, "Question") || !strings.Contains(input, "refId") {
			t.Fatal("summary lost original references")
		}
		return "Unresolved questions and disagreements. Original message references remain readable through read_context.", nil
	}
	first, err := f.s.Snapshot(f.ctx, f.group, summary)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || first.CoveredThrough == 0 {
		t.Fatal("shared compression did not run")
	}
	otherCtx := auth.WithPrincipal(f.ctx, &auth.Principal{Kind: auth.KindWeb, UserID: f.other})
	second, err := f.s.Snapshot(otherCtx, f.group, summary)
	if err != nil {
		t.Fatal(err)
	}
	if first.Version != second.Version || first.Summary != second.Summary || calls != 1 {
		t.Fatal("roles did not share one snapshot")
	}
	var ref string
	if err = f.s.Pool.QueryRow(f.ctx, `SELECT ref_id FROM knowledge_entries WHERE group_id=$1 ORDER BY seq LIMIT 1`, f.group).Scan(&ref); err != nil {
		t.Fatal(err)
	}
	raw, err := f.s.ReadReference(otherCtx, f.group, "message", ref, 0, 8000)
	if err != nil || !strings.Contains(string(raw), "Question 0") {
		t.Fatalf("original unreadable: %v", err)
	}
}
