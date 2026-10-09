package integrationtest_test

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/agent"
	"github.com/kakj-go/Judex/internal/app"
	"github.com/kakj-go/Judex/internal/config"
	"github.com/kakj-go/Judex/internal/project"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProjectAndPositionModelSelection(t *testing.T) {
	_, projects, ids, pool := newDecisionEnv(t)
	ctx := context.Background()
	user, _, err := ids.Register(ctx, "Models", "models@test.local", "model-selection-123", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	p, err := projects.Create(ctx, user.ID, project.CreateRequest{Title: "Models"})
	if err != nil {
		t.Fatal(err)
	}
	a, b := uuid.New(), uuid.New()
	t.Setenv("JUDEX_TEST_KEY_A", "test-a")
	t.Setenv("JUDEX_TEST_KEY_B", "test-b")
	entries := []agent.CatalogEntry{
		{ID: a.String(), DisplayName: "Project model", Provider: "openai-compatible", BaseURL: "http://project.test", APIKeyEnv: "JUDEX_TEST_KEY_A", ModelName: "model-a", MaxInputTokens: 12000, MaxOutputTokens: 1000, ToolCalling: true, Streaming: true},
		{ID: b.String(), DisplayName: "Position model", Provider: "openai-compatible", BaseURL: "http://position.test", APIKeyEnv: "JUDEX_TEST_KEY_B", ModelName: "model-b", MaxInputTokens: 20000, MaxOutputTokens: 2000, ToolCalling: true, Streaming: true},
	}
	if err = agent.SyncCatalog(ctx, pool, entries, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE projects SET default_model_id=$2 WHERE id=$1`, p.ID, a); err != nil {
		t.Fatal(err)
	}
	position, err := projects.CreatePosition(ctx, user.ID, p.ID, project.PositionDraft{Name: "Override", ModelID: &b})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := projects.CreateIdentity(ctx, user.ID, p.ID, position.ID, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	resolve := agent.Resolver(pool, config.ModelGatewayConfig{})
	_, name, input, output, err := resolve(ctx, p.ID, uuid.Nil)
	if err != nil || name != "model-a" || input != 12000 || output != 1000 {
		t.Fatalf("project model: %s %d/%d %v", name, input, output, err)
	}
	_, name, input, output, err = resolve(ctx, p.ID, identity.ID)
	if err != nil || name != "model-b" || input != 20000 || output != 2000 {
		t.Fatalf("position model: %s %d/%d %v", name, input, output, err)
	}
	if _, err = pool.Exec(ctx, `UPDATE model_catalog SET enabled=false WHERE id=$1`, b); err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, err = resolve(ctx, p.ID, identity.ID); err == nil {
		t.Fatal("disabled explicit model silently fell back")
	}
}

func TestCatalogOnlyProductionWiring(t *testing.T) {
	_, _, _, pool := newDecisionEnv(t)
	file := filepath.Join(t.TempDir(), "models.json")
	raw, _ := json.Marshal(map[string]any{"models": []agent.CatalogEntry{{ID: uuid.NewString(), DisplayName: "Catalog", Provider: "openai-compatible", BaseURL: "http://model.test", APIKeyEnv: "JUDEX_TEST_KEY", ModelName: "catalog-only", MaxInputTokens: 10000, MaxOutputTokens: 1000, ToolCalling: true, Streaming: true}}})
	if err := os.WriteFile(file, raw, 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.FromEnv(func(key string) string {
		switch key {
		case "JUDEX_DATABASE_URL":
			return pool.Config().ConnConfig.ConnString()
		case "JUDEX_MODEL_CATALOG_FILE":
			return file
		case "JUDEX_ENV":
			return "test"
		case "JUDEX_MODE":
			return "api"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	application, err := app.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close(context.Background())
	response := httptest.NewRecorder()
	application.Server.Handler.ServeHTTP(response, httptest.NewRequest("GET", "/api/v1/system", nil))
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"agentExecution":true`) {
		t.Fatalf("catalog-only production wiring missing: %s", response.Body.String())
	}
}
