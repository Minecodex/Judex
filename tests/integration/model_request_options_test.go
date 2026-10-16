package integrationtest_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/agent"
	"github.com/kakj-go/Judex/internal/config"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
	"github.com/kakj-go/Judex/internal/project"
)

func TestConfiguredOptionsSurviveCatalogAndQuotaResolution(t *testing.T) {
	_, projects, ids, pool := newDiscussionEnv(t)
	ctx := context.Background()
	t.Setenv("JUDEX_TEST_OPTIONS_KEY", "controlled-key")
	primary, fallback := uuid.New(), uuid.New()
	entry := func(id uuid.UUID, thinking string) agent.CatalogEntry {
		return agent.CatalogEntry{ID: id.String(), DisplayName: thinking, Provider: model.ProtocolOpenAI, BaseURL: "https://model.test", APIKeyEnv: "JUDEX_TEST_OPTIONS_KEY", ModelName: "controlled", ToolCalling: true, Streaming: true, MaxInputTokens: 128000, MaxOutputTokens: 8192, RequestOptions: model.RequestOptions{ThinkingType: thinking}}
	}
	a, b := entry(primary, "disabled"), entry(fallback, "enabled")
	a.RequestOptions.ReasoningEffort = "low"
	b.RequestOptions.ReasoningEffort = "high"
	a.QuotaFallbackIDs = []string{fallback.String()}
	if err := agent.SyncCatalog(ctx, pool, []agent.CatalogEntry{a, b}, time.Now()); err != nil {
		t.Fatal(err)
	}
	user, _, err := ids.Register(ctx, "Options", "options@models.test", "model-options-1234", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	p, err := projects.Create(ctx, user.ID, project.CreateRequest{Title: "Explicit mode", DefaultModelID: &primary})
	if err != nil {
		t.Fatal(err)
	}
	provider, _, _, outCap, err := agent.Resolver(pool, config.ModelGatewayConfig{})(ctx, p.ID, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	targets := provider.(model.QuotaCandidates).QuotaTargets()
	if len(targets) != 2 || outCap != 8192 {
		t.Fatal("options changed model routing or token caps")
	}
	for i, thinking := range []string{"disabled", "enabled"} {
		options := targets[i].Provider.(model.ConfiguredOptions).RequestOptions()
		if options.ThinkingType != thinking {
			t.Fatal("per-model option lost in primary/fallback resolution")
		}
		if options.ReasoningEffort != []string{"low", "high"}[i] {
			t.Fatal("per-model reasoning effort lost in catalog/resolution")
		}
	}
	a.RequestOptions.ThinkingType = "invalid"
	if err = agent.SyncCatalog(ctx, pool, []agent.CatalogEntry{a, b}, time.Now()); err == nil {
		t.Fatal("invalid provider options were silently admitted")
	}
	a.RequestOptions.ThinkingType = "enabled"
	a.RequestOptions.ReasoningEffort = "invalid"
	if err = agent.SyncCatalog(ctx, pool, []agent.CatalogEntry{a, b}, time.Now()); err == nil {
		t.Fatal("invalid reasoning effort was silently admitted")
	}
}
