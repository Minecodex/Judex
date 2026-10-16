package agent_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/kakj-go/Judex/internal/agent/runner"
	"github.com/kakj-go/Judex/internal/agent/tools"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
)

type quotaScript struct {
	class       *model.ErrorClass
	calls       int
	streamError bool
}

func (p *quotaScript) Stream(_ context.Context, req model.Request) (<-chan model.Event, error) {
	p.calls++
	if p.class != nil && !p.streamError {
		return nil, &model.ProviderError{Class: *p.class, Err: fmt.Errorf("controlled provider failure")}
	}
	events := make(chan model.Event, 3)
	if p.class != nil {
		events <- model.Event{Type: "error", Err: &model.ProviderError{Class: *p.class, Err: fmt.Errorf("controlled stream failure")}}
	} else {
		events <- model.Event{Type: "textDelta", TextDelta: "completed"}
		events <- model.Event{Type: "usage", Usage: model.UsageSnapshot{ModelName: req.Model, InputTokens: model.TokenCount(50), OutputTokens: model.TokenCount(3), CacheReadTokens: model.TokenCount(20)}}
		events <- model.Event{Type: "finish"}
	}
	close(events)
	return events, nil
}

type fallbackJournal struct {
	checkpointJournal
	started, ended []string
	failEnd        bool
}

func (j *fallbackJournal) ModelStart(_ context.Context, _ int, r model.Request) error {
	j.started = append(j.started, r.Model)
	return nil
}
func (j *fallbackJournal) ModelEnd(_ context.Context, _ int, u model.UsageSnapshot, _ bool, _ error) error {
	j.ended = append(j.ended, u.ModelName)
	if j.failEnd {
		return fmt.Errorf("journal unavailable")
	}
	return nil
}

func TestQuotaFallbackAttemptsAreJournalledAndRetainedAcrossPrepare(t *testing.T) {
	quota := model.ErrorQuota
	primary, flash, free := &quotaScript{class: &quota}, &quotaScript{class: &quota, streamError: true}, &quotaScript{}
	target := func(p model.Provider, n string) model.Target {
		return model.Target{Provider: p, Name: n, MaxInputTokens: 50000, MaxOutputTokens: 64}
	}
	policy := model.WithQuotaFallbacks(target(primary, "glm-5.3"), []model.Target{target(flash, "glm-5.3-flash"), target(free, "glm-4.5-flash")})
	j := &fallbackJournal{}
	harness := runner.NewRunnerForTest(policy, tools.New())
	out := harness.Run(context.Background(), runner.RunRequest{ModelName: "glm-5.3", MaxOutputTokens: 64, Budget: runner.DefaultBudget(), Journal: j, Prepare: func(context.Context) (runner.CallContext, error) {
		return runner.CallContext{Provider: policy, ModelName: "glm-5.3", MaxInputTokens: 50000, MaxOutputTokens: 64}, nil
	}})
	want := []string{"glm-5.3", "glm-5.3-flash", "glm-4.5-flash"}
	if out.State != "succeeded" || out.ModelCalls != 3 || !reflect.DeepEqual(j.started, want) || !reflect.DeepEqual(j.ended, want) || primary.calls != 1 || flash.calls != 1 || free.calls != 1 {
		t.Fatalf("outcome=%+v journal=%+v", out, j)
	}
}
func TestFallbackRejectsNonQuotaErrorsAndJournalFailure(t *testing.T) {
	for _, class := range []model.ErrorClass{model.ErrorRateLimited, model.ErrorAuth, model.ErrorInvalidRequest, model.ErrorTransient, model.ErrorQuota} {
		primary, free := &quotaScript{class: &class}, &quotaScript{}
		policy := model.WithQuotaFallbacks(model.Target{Provider: primary, Name: "primary", MaxInputTokens: 50000, MaxOutputTokens: 64}, []model.Target{{Provider: free, Name: "free", MaxInputTokens: 50000, MaxOutputTokens: 64}})
		j := &fallbackJournal{failEnd: class == model.ErrorQuota}
		out := runner.NewRunnerForTest(policy, tools.New()).Run(context.Background(), runner.RunRequest{ModelName: "primary", Budget: runner.DefaultBudget(), Journal: j})
		if primary.calls != 1 || free.calls != 0 || out.State == "succeeded" {
			t.Fatalf("class=%v outcome=%+v calls=%d/%d", class, out, primary.calls, free.calls)
		}
	}
}
