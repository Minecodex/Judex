package integrationtest_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/agent"
	"github.com/kakj-go/Judex/internal/decision"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/project"
)

func TestProjectCreationRetainsModelChoiceAndRejectsDisabledModel(t *testing.T) {
	_, projects, ids, pool := newDiscussionEnv(t)
	ctx := context.Background()
	user, _, err := ids.Register(ctx, "Model creator", "creator@models.test", "model-create-1234", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	modelID := uuid.New()
	err = agent.SyncCatalog(ctx, pool, []agent.CatalogEntry{{ID: modelID.String(), DisplayName: "Creation model", Provider: "openai-compatible", BaseURL: "https://model.test", APIKeyEnv: "TEST_KEY", ModelName: "creation", MaxInputTokens: 1000, MaxOutputTokens: 100, ToolCalling: true, Streaming: true}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	created, err := projects.Create(ctx, user.ID, project.CreateRequest{Title: "Chosen model", DefaultModelID: &modelID})
	if err != nil {
		t.Fatal(err)
	}
	if created.DefaultModelID == nil || *created.DefaultModelID != modelID {
		t.Fatalf("model lost in creation: %+v", created)
	}
	var stored uuid.UUID
	if err = pool.QueryRow(ctx, `SELECT default_model_id FROM projects WHERE id=$1`, created.ID).Scan(&stored); err != nil || stored != modelID {
		t.Fatalf("stored model=%s err=%v", stored, err)
	}
	if _, err = pool.Exec(ctx, `UPDATE model_catalog SET enabled=false WHERE id=$1`, modelID); err != nil {
		t.Fatal(err)
	}
	if _, err = projects.Create(ctx, user.ID, project.CreateRequest{Title: "Disabled model", DefaultModelID: &modelID}); !apierrors.IsCode(err, apierrors.ModelUnavailable) {
		t.Fatalf("disabled model allowed: %v", err)
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM projects WHERE owner_user_id=$1`, user.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("failed model choice created objects count=%d err=%v", count, err)
	}
}

func TestProjectCreationIntentAcceptsModelAndRejectsInvalidDraft(t *testing.T) {
	_, _, ids, pool := newDiscussionEnv(t)
	ctx := context.Background()
	user, _, err := ids.Register(ctx, "Intent creator", "intent@models.test", "model-intent-1234", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	service := decision.NewService(pool, nil, 3600)
	body := map[string]any{"title": "Named project", "defaultModelId": uuid.NewString()}
	raw, _ := json.Marshal(body)
	sum := sha256.Sum256(raw)
	intent, err := service.CreateIntent(ctx, user.ID, uuid.Nil, nil, "project.create", uuid.New(), hex.EncodeToString(sum[:]), body)
	if err != nil || intent["state"] != "pending" {
		t.Fatalf("valid model selection rejected: %+v %v", intent, err)
	}
	for _, invalid := range []map[string]any{{}, {"title": "Bad model", "defaultModelId": "bad"}, {"title": "Bad integer", "maxDiscussionRounds": 1.5}, {"title": "Extra field", "unexpected": "value"}} {
		raw, _ := json.Marshal(invalid)
		sum := sha256.Sum256(raw)
		if _, err = service.CreateIntent(ctx, user.ID, uuid.Nil, nil, "project.create", uuid.New(), hex.EncodeToString(sum[:]), invalid); !apierrors.IsCode(err, apierrors.Validation) {
			t.Fatalf("invalid project intent accepted: %+v %v", invalid, err)
		}
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM confirmation_intents WHERE user_id=$1`, user.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("invalid intent persisted count=%d err=%v", count, err)
	}
}
