package integrationtest_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/agent"
	"github.com/kakj-go/Judex/internal/app"
	"github.com/kakj-go/Judex/internal/config"
)

func TestHTTPProjectCreateIntentConfirmAndResultPreserveModel(t *testing.T) {
	_, _, _, pool := newDiscussionEnv(t)
	ctx := context.Background()
	modelID := uuid.New()
	err := agent.SyncCatalog(ctx, pool, []agent.CatalogEntry{{ID: modelID.String(), DisplayName: "HTTP creation", Provider: "openai-compatible", BaseURL: "https://model.test", APIKeyEnv: "TEST_KEY", ModelName: "http-create", MaxInputTokens: 1000, MaxOutputTokens: 100, ToolCalling: true, Streaming: true}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.FromEnv(func(key string) string {
		switch key {
		case "JUDEX_DATABASE_URL":
			return pool.Config().ConnConfig.ConnString()
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
	defer application.Close(ctx)
	cookie, csrf := "", ""
	request := func(method, path string, body any) map[string]any {
		t.Helper()
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(method, "/api/v1"+path, strings.NewReader(string(raw)))
		req.Header.Set("Content-Type", "application/json")
		if cookie != "" {
			req.Header.Set("Cookie", cookie)
		}
		if csrf != "" {
			req.Header.Set("X-CSRF-Token", csrf)
		}
		if method != "GET" {
			req.Header.Set("Idempotency-Key", uuid.NewString())
		}
		response := httptest.NewRecorder()
		application.Server.Handler.ServeHTTP(response, req)
		if response.Code < 200 || response.Code >= 300 {
			t.Fatalf("%s %s status=%d body=%s", method, path, response.Code, response.Body.String())
		}
		if set := response.Header().Get("Set-Cookie"); set != "" {
			cookie = strings.Split(set, ";")[0]
		}
		var envelope map[string]any
		if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		return envelope["data"].(map[string]any)
	}
	request("POST", "/auth/register", map[string]any{"displayName": "HTTP intent owner", "email": "http@project-create.test", "password": "http-project-create-123"})
	session := request("GET", "/auth/session", nil)
	csrf, _ = session["csrfToken"].(string)
	body := map[string]any{"title": "Reviewed creation", "defaultModelId": modelID.String(), "kind": "software"}
	raw, _ := json.Marshal(body)
	sum := sha256.Sum256(raw)
	intent := request("POST", "/confirmation-intents", map[string]any{"operation": "project.create", "objectId": uuid.NewString(), "reviewHash": hex.EncodeToString(sum[:]), "payload": body})
	id := intent["id"].(string)
	stored := request("GET", "/confirmation-intents/"+id, nil)
	confirmBody := map[string]any{"decision": "approve", "intentHash": stored["intentHash"]}
	first := request("POST", "/confirmation-intents/"+id+"/confirm", confirmBody)
	second := request("POST", "/confirmation-intents/"+id+"/confirm", confirmBody)
	if first["state"] != "committed" || first["resultRef"] != second["resultRef"] {
		t.Fatalf("confirmation results %v %v", first, second)
	}
	projectID := strings.TrimPrefix(first["resultRef"].(string), "project:")
	created := request("GET", "/projects/"+projectID, nil)
	if created["defaultModelId"] != modelID.String() {
		t.Fatalf("model lost in HTTP confirmation: %v", created)
	}
}
