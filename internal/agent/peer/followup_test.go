package peer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/agent"
	"github.com/kakj-go/Judex/internal/collab"
	"github.com/kakj-go/Judex/internal/config"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
	"github.com/kakj-go/Judex/internal/job"
	"github.com/kakj-go/Judex/internal/platform/auth"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

func TestMessageEffectsRequireAgentScopeBeforeWriteAndReplay(t *testing.T) {
	f := setup(t)
	ctx := auth.WithPrincipal(f.ctx, &auth.Principal{Kind: auth.KindCLI, UserID: f.user, Scopes: []string{auth.ScopeMessagesWrite}, ProjectScope: []uuid.UUID{f.project}})
	in := collab.MessageInput{Body: "Explicit AI request", Mentions: []collab.Mention{{Kind: "role_agent", TargetID: f.a}}}
	for _, push := range []bool{false, true} {
		for _, record := range []bool{false, true} {
			in.RecordOnly = record
			if _, err := f.s.Post(ctx, f.group, in, push); !apierrors.IsCode(err, apierrors.Forbidden) {
				t.Fatalf("push=%v record=%v: %v", push, record, err)
			}
			raw, _ := json.Marshal(in)
			op := "postMessage"
			if push {
				op = "pushGroup"
			}
			if err := f.s.CheckOperation(ctx, op, uuid.Nil, f.group, uuid.Nil, raw); !apierrors.IsCode(err, apierrors.Forbidden) {
				t.Fatalf("pre-replay effect check: %v", err)
			}
		}
	}
	for _, in := range []collab.MessageInput{{Body: "Ordinary @Engineer"}, {Body: "Human mention", Mentions: []collab.Mention{{Kind: "human", TargetID: f.other}}}} {
		if _, err := f.s.Post(ctx, f.group, in, false); err != nil {
			t.Fatal(err)
		}
	}
	var messages, deliveries int
	_ = f.s.Pool.QueryRow(f.ctx, `SELECT count(*) FROM messages`).Scan(&messages)
	_ = f.s.Pool.QueryRow(f.ctx, `SELECT count(*) FROM deliveries`).Scan(&deliveries)
	if messages != 2 || deliveries != 0 {
		t.Fatalf("denied effects persisted: messages=%d deliveries=%d", messages, deliveries)
	}
}

func TestSummaryOnlyRecompressionAfterConfigurationChange(t *testing.T) {
	for _, change := range []string{"smaller-model", "larger-prompt"} {
		t.Run(change, func(t *testing.T) {
			f := setup(t)
			for range 3 {
				if _, err := f.s.Post(f.ctx, f.group, collab.MessageInput{Body: "An unanswered disagreement. " + strings.Repeat("e", 60000)}, false); err != nil {
					t.Fatal(err)
				}
			}
			first, err := f.s.Snapshot(f.ctx, f.group, func(context.Context, string, int64) (string, error) {
				return strings.Repeat("public-memory ", 3000), nil
			})
			if err != nil || len(first.Entries) != 0 || first.CoveredThrough != first.SourceSeq {
				t.Fatalf("initial compression: %+v %v", first, err)
			}
			if change == "smaller-model" {
				id := uuid.New()
				_, err = f.s.Pool.Exec(f.ctx, `INSERT INTO model_catalog(id,display_name,provider,model_name,limits,created_at,updated_at) VALUES($1,'Small','openai-compatible','fixture','{"maxInputTokens":20000,"maxOutputTokens":2048}',now(),now())`, id)
				if err == nil {
					_, err = f.s.SaveProject(f.ctx, f.project, collab.ProjectInput{Name: "Controlled roles", DefaultModelID: &id, LoopLimit: 10, ExpectedVersion: 1})
				}
			} else {
				_, err = f.s.SaveProject(f.ctx, f.project, collab.ProjectInput{Name: "Controlled roles", Description: strings.Repeat("p", 32000), LoopLimit: 10, ExpectedVersion: 1})
				var position uuid.UUID
				if err == nil {
					err = f.s.Pool.QueryRow(f.ctx, `SELECT position_id FROM role_agents WHERE id=$1`, f.a).Scan(&position)
				}
				if err == nil {
					_, err = f.s.SaveTemplate(f.ctx, f.project, position, "positions", collab.TemplateInput{Name: "Engineer", Prompt: strings.Repeat("p", 32000), ExpectedVersion: 1})
				}
				if err == nil {
					err = f.s.SavePrompt(f.ctx, f.project, position, strings.Repeat("p", 16000), 0)
				}
				if err == nil {
					_, err = f.s.SaveGroup(f.ctx, f.project, f.group, collab.GroupInput{Name: "Peer discussion", Purpose: strings.Repeat("p", 16000), ExpectedVersion: 1})
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = f.s.Snapshot(f.ctx, f.group, func(context.Context, string, int64) (string, error) {
				return "", errors.New("controlled summary failure")
			}); !apierrors.IsCode(err, apierrors.DependencyDown) {
				t.Fatalf("summary failure must retain old memory: %v", err)
			}
			var version int64
			_ = f.s.Pool.QueryRow(f.ctx, `SELECT version FROM context_heads WHERE group_id=$1`, f.group).Scan(&version)
			if version != first.Version {
				t.Fatal("failed compression advanced coverage")
			}
			started, release := make(chan struct{}), make(chan struct{})
			type answer struct {
				snapshot collab.Snapshot
				err      error
			}
			result := make(chan answer, 1)
			go func() {
				snapshot, err := f.s.Snapshot(f.ctx, f.group, func(_ context.Context, input string, budget int64) (string, error) {
					if !strings.Contains(input, first.Summary) || budget <= 0 {
						return "", errors.New("old summary missing")
					}
					close(started)
					<-release
					return "Public memory retains unresolved disagreement; original references stay readable.", nil
				})
				result <- answer{snapshot, err}
			}()
			<-started
			other := auth.WithPrincipal(f.ctx, &auth.Principal{Kind: auth.KindWeb, UserID: f.other})
			_, concurrent := f.s.Snapshot(other, f.group, nil)
			close(release)
			if !errors.Is(concurrent, job.ErrBusy) {
				t.Fatalf("concurrent compressor: %v", concurrent)
			}
			next := <-result
			if next.err != nil || next.snapshot.CoveredThrough != first.CoveredThrough || next.snapshot.SourceSeq != first.SourceSeq {
				t.Fatalf("coverage changed: %+v %v", next.snapshot, next.err)
			}
			shared, err := f.s.Snapshot(other, f.group, nil)
			if err != nil || shared.Version != next.snapshot.Version || shared.Summary != next.snapshot.Summary {
				t.Fatalf("roles read different memory: %v", err)
			}
			var ref string
			_ = f.s.Pool.QueryRow(f.ctx, `SELECT ref_id FROM knowledge_entries WHERE group_id=$1 ORDER BY seq LIMIT 1`, f.group).Scan(&ref)
			raw, err := f.s.ReadReference(other, f.group, "message", ref, 0, 8000)
			if err != nil || !strings.Contains(string(raw), "unanswered disagreement") {
				t.Fatalf("original reference lost: %v", err)
			}
		})
	}
}

type humanRequester struct{ target uuid.UUID }

func (p humanRequester) Stream(_ context.Context, req model.Request) (<-chan model.Event, error) {
	out := make(chan model.Event, 2)
	if req.Messages[len(req.Messages)-1].Role != "tool" {
		args, _ := json.Marshal(map[string]any{"body": "Please respond", "mentions": []collab.Mention{{Kind: "human", TargetID: p.target, RequiresResponse: true}}})
		out <- model.Event{Type: "toolCallReady", ToolCallID: uuid.NewString(), ToolName: "send_message", ArgsJSON: string(args)}
	}
	out <- model.Event{Type: "finish"}
	close(out)
	return out, nil
}

func TestUnassignRolesSettlesOnlyAfterLastBranchIsCancelled(t *testing.T) {
	for _, phase := range []string{"pending", "running", "waiting_human", "paused"} {
		t.Run(phase, func(t *testing.T) {
			f := setup(t)
			if _, err := f.s.Post(f.ctx, f.group, collab.MessageInput{Body: "Two branches", Mentions: []collab.Mention{{Kind: "role_agent", TargetID: f.a}, {Kind: "role_agent", TargetID: f.b}}}, false); err != nil {
				t.Fatal(err)
			}
			e := f.executor(humanRequester{f.other})
			if phase == "waiting_human" {
				f.step(t, e)
				f.step(t, e)
			}
			var claimed *run
			if phase == "running" {
				claimed = claimNextForTest(t, f, e)
			}
			id, _, _, _, _ := f.state(t)
			if phase == "paused" {
				if err := f.s.Pause(f.ctx, f.group, id, "technical_budget"); err != nil {
					t.Fatal(err)
				}
			}
			var position uuid.UUID
			_ = f.s.Pool.QueryRow(f.ctx, `SELECT position_id FROM role_agents WHERE id=$1`, f.b).Scan(&position)
			members := []collab.MemberRoles{{UserID: f.user}, {UserID: f.other, PositionIDs: []uuid.UUID{position}}}
			if _, err := f.s.SaveGroup(f.ctx, f.project, f.group, collab.GroupInput{Name: "Branches", ExpectedVersion: 1, Members: members}); err != nil {
				t.Fatal(err)
			}
			_, state, _, _, _ := f.state(t)
			if state == "settled" {
				t.Fatal("cancelled another valid branch")
			}
			for i := range members {
				members[i].PositionIDs = nil
			}
			if _, err := f.s.SaveGroup(f.ctx, f.project, f.group, collab.GroupInput{Name: "Branches", ExpectedVersion: 2, Members: members}); err != nil {
				t.Fatal(err)
			}
			_, state, _, _, _ = f.state(t)
			if state != "settled" {
				t.Fatalf("last branch did not settle: %s", state)
			}
			if claimed != nil {
				if _, err := f.s.Publish(f.ctx, claimed.Publication, collab.MessageInput{Body: "late"}); err == nil {
					t.Fatal("unassigned run published late result")
				}
			}
			for i := range members {
				if members[i].UserID == f.user {
					members[i].PositionIDs = []uuid.UUID{position}
				}
			}
			if _, err := f.s.SaveGroup(f.ctx, f.project, f.group, collab.GroupInput{Name: "Branches", ExpectedVersion: 3, Members: members}); err != nil {
				t.Fatal(err)
			}
			if _, err := f.s.Post(f.ctx, f.group, collab.MessageInput{Body: "New discussion", Mentions: []collab.Mention{{Kind: "role_agent", TargetID: f.a}}}, false); err != nil {
				t.Fatal(err)
			}
			next, _, loops, _, _ := f.state(t)
			if next == id || loops != 0 {
				t.Fatal("new trigger reused cancelled discussion")
			}
		})
	}
}

func TestCatalogQuotaFallbackExecutesWithJournalAndSmallerWindow(t *testing.T) {
	for _, failure := range []string{"quota-http", "quota-stream", "rate-limit", "auth", "partial-quota"} {
		t.Run(failure, func(t *testing.T) {
			f := setup(t)
			var primary, fallback atomic.Int32
			gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					Model     string          `json:"model"`
					Messages  []model.Message `json:"messages"`
					MaxTokens int64           `json:"max_tokens"`
					Tools     json.RawMessage `json:"tools"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				if req.Model == "primary" {
					primary.Add(1)
					if failure == "quota-stream" || failure == "partial-quota" {
						w.Header().Set("Content-Type", "text/event-stream")
						if failure == "partial-quota" {
							fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"partial response\"}}]}\n\n")
						}
						fmt.Fprint(w, "data: {\"error\":{\"code\":\"insufficient_quota\",\"message\":\"Controlled exhausted quota\"}}\n\n")
						return
					}
					status, code := 429, "insufficient_quota"
					if failure == "rate-limit" {
						code = "rate_limit_exceeded"
					}
					if failure == "auth" {
						status, code = 401, "invalid_api_key"
					}
					w.WriteHeader(status)
					fmt.Fprintf(w, `{"error":{"code":%q,"message":"controlled failure"}}`, code)
					return
				}
				fallback.Add(1)
				raw, _ := json.Marshal(req.Messages)
				if collab.Tokens(string(raw))+req.MaxTokens > 20000 {
					t.Error("fallback received oversized context")
					w.WriteHeader(400)
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				text := "Configured fallback answered using shared references."
				if len(req.Tools) == 0 {
					text = "Public memory: unanswered disagreement remains, read original message references."
				}
				chunk, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": text}, "finish_reason": "stop"}}, "usage": map[string]int{"prompt_tokens": 300, "completion_tokens": 30}})
				fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", chunk)
			}))
			defer gateway.Close()
			t.Setenv("JUDEX_MODEL_KEY_QUOTA_TEST", "fixture-only")
			mainID, backupID := uuid.New(), uuid.New()
			entries := []agent.CatalogEntry{}
			for _, c := range []struct {
				id   uuid.UUID
				name string
				size int
			}{{mainID, "primary", 128000}, {backupID, "fallback", 20000}} {
				entries = append(entries, agent.CatalogEntry{ID: c.id.String(), DisplayName: c.name, Provider: "openai-compatible", BaseURL: gateway.URL + "/v1", APIKeyEnv: "JUDEX_MODEL_KEY_QUOTA_TEST", ModelName: c.name, MaxInputTokens: c.size, MaxOutputTokens: 2048, ToolCalling: true, Streaming: true})
			}
			entries[0].QuotaFallbackIDs = []string{backupID.String()}
			if err := agent.SyncCatalog(f.ctx, f.s.Pool, entries, time.Now()); err != nil {
				t.Fatal(err)
			}
			if _, err := f.s.SaveProject(f.ctx, f.project, collab.ProjectInput{Name: "Quota fallback", DefaultModelID: &mainID, ExpectedVersion: 1}); err != nil {
				t.Fatal(err)
			}
			budget, err := f.s.ContextBudget(f.ctx, f.group)
			if err != nil || budget > 20000-2048-8192 {
				t.Fatalf("shared fallback budget: %d %v", budget, err)
			}
			if failure == "quota-http" {
				for range 3 {
					if _, err := f.s.Post(f.ctx, f.group, collab.MessageInput{Body: "Unanswered disagreement " + strings.Repeat("evidence ", 2500)}, false); err != nil {
						t.Fatal(err)
					}
				}
			}
			if _, err := f.s.Post(f.ctx, f.group, collab.MessageInput{Body: "Use configured fallback", Mentions: []collab.Mention{{Kind: "role_agent", TargetID: f.a}}}, false); err != nil {
				t.Fatal(err)
			}
			e := &Executor{Service: f.s, ResolveModel: agent.Resolver(f.s.Pool, config.ModelGatewayConfig{})}
			f.step(t, e)
			_, state, loops, _, _ := f.state(t)
			success := failure == "quota-http" || failure == "quota-stream"
			if success && (state != "settled" || fallback.Load() == 0) {
				t.Fatalf("fallback not used: %s primary=%d fallback=%d", state, primary.Load(), fallback.Load())
			}
			if !success && (state != "paused" || fallback.Load() != 0) {
				t.Fatalf("unsafe fallback: %s primary=%d fallback=%d", state, primary.Load(), fallback.Load())
			}
			if loops != 0 {
				t.Fatal("model fallback changed causal loop count")
			}
			var attempts, calls, unknown int
			var total, reserved int64
			if err := f.s.Pool.QueryRow(f.ctx, `SELECT count(*),COALESCE(sum(COALESCE(charged_tokens,reserved_tokens)),0),count(*) FILTER(WHERE charged_tokens IS NULL),COALESCE(sum(reserved_tokens) FILTER(WHERE charged_tokens IS NULL),0) FROM role_model_calls`).Scan(&attempts, &total, &unknown, &reserved); err != nil {
				t.Fatal(err)
			}
			var tokens int64
			_ = f.s.Pool.QueryRow(f.ctx, `SELECT model_calls,tokens FROM role_runs LIMIT 1`).Scan(&calls, &tokens)
			if attempts != int(primary.Load()+fallback.Load()) || calls != attempts || tokens != total || unknown == 0 || reserved == 0 {
				t.Fatalf("lost attempt/cost: attempts=%d calls=%d actual=%d tokens=%d total=%d unknown=%d", attempts, calls, primary.Load()+fallback.Load(), tokens, total, unknown)
			}
		})
	}
}

type attemptProvider func(context.Context, model.Request) (<-chan model.Event, error)

func (p attemptProvider) Stream(ctx context.Context, req model.Request) (<-chan model.Event, error) {
	return p(ctx, req)
}

func TestFallbackSharesTechnicalBudgetAndRetainsKnownFailureUsage(t *testing.T) {
	for _, limit := range []bool{false, true} {
		t.Run(fmt.Sprintf("last-call-%v", limit), func(t *testing.T) {
			f := setup(t)
			if _, err := f.s.Post(f.ctx, f.group, collab.MessageInput{Body: "Work", Mentions: []collab.Mention{{Kind: "role_agent", TargetID: f.a}}}, false); err != nil {
				t.Fatal(err)
			}
			e := &Executor{Service: f.s}
			r := claimNextForTest(t, f, e)
			if limit {
				_, err := f.s.Pool.Exec(f.ctx, `UPDATE role_runs SET model_calls=29,budget_model_calls=29 WHERE id=$1`, r.Publication.RunID)
				if err != nil {
					t.Fatal(err)
				}
				r.Calls = 29
			}
			secondary := 0
			primary := attemptProvider(func(context.Context, model.Request) (<-chan model.Event, error) {
				out := make(chan model.Event, 2)
				out <- model.Event{Type: "usage", Usage: model.UsageSnapshot{InputTokens: model.TokenCount(150), OutputTokens: model.TokenCount(50)}}
				out <- model.Event{Type: "error", Err: &model.ProviderError{Class: model.ErrorQuota, Err: errors.New("explicit quota")}}
				close(out)
				return out, nil
			})
			backup := attemptProvider(func(context.Context, model.Request) (<-chan model.Event, error) {
				secondary++
				var charged int64
				if err := f.s.Pool.QueryRow(f.ctx, `SELECT charged_tokens FROM role_model_calls WHERE model_id='primary' AND state='quota_failed'`).Scan(&charged); err != nil || charged != 200 {
					t.Errorf("primary attempt not committed before fallback: %d %v", charged, err)
				}
				out := make(chan model.Event, 1)
				out <- model.Event{Type: "finish"}
				close(out)
				return out, nil
			})
			targets := []model.Target{{Provider: primary, Name: "primary", MaxInputTokens: 128000, MaxOutputTokens: 2048}, {Provider: backup, Name: "backup", MaxInputTokens: 20000, MaxOutputTokens: 1024}}
			_, err := e.callTargets(f.ctx, r, targets, model.Request{Messages: []model.Message{{Role: "user", Content: "Work"}}})
			if limit && (failureReason(err, "") != "technical_budget" || secondary != 0 || r.Calls != 30) {
				t.Fatalf("fallback reset budget: calls=%d secondary=%d err=%v", r.Calls, secondary, err)
			}
			if !limit && (err != nil || secondary != 1 || r.Calls != 2) {
				t.Fatalf("known usage fallback: %v", err)
			}
			var tokens int64
			_ = f.s.Pool.QueryRow(f.ctx, `SELECT tokens FROM role_runs WHERE id=$1`, r.Publication.RunID).Scan(&tokens)
			if tokens != r.Tokens || tokens < 200 {
				t.Fatalf("known failed usage lost: db=%d memory=%d", tokens, r.Tokens)
			}
		})
	}
}
