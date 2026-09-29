package agent

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/config"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"os"
)

// Resolver chooses the current position override, then project model. The
// deployment gateway is only a fallback when neither has an explicit choice.
func Resolver(pool *postgres.Pool, fallback config.ModelGatewayConfig) func(context.Context, uuid.UUID, uuid.UUID) (model.Provider, string, int64, int64, error) {
	return func(ctx context.Context, project, identity uuid.UUID) (model.Provider, string, int64, int64, error) {
		var selected *uuid.UUID
		err := pool.QueryRow(ctx, `SELECT COALESCE(v.model_id,p.default_model_id) FROM projects p
   LEFT JOIN agent_identities i ON i.id=$2 AND i.project_id=p.id
   LEFT JOIN position_templates t ON t.id=i.template_id
   LEFT JOIN position_versions v ON v.template_id=t.id AND v.revision=t.current_version
   WHERE p.id=$1 AND p.status='active'`, project, identity).Scan(&selected)
		if err != nil {
			return nil, "", 0, 0, apierrors.New(apierrors.ModelUnavailable, "project unavailable")
		}
		cfg := model.GatewayConfig{Protocol: fallback.Protocol, BaseURL: fallback.BaseURL, APIKey: fallback.APIKey}
		name := fallback.Model
		maxIn, maxOut := int64(128000), int64(8192)
		if selected != nil {
			var keyRef string
			var limits []byte
			var capabilities []byte
			if err = pool.QueryRow(ctx, `SELECT provider,endpoint_config_ref,credential_secret_ref,model_name,limits,capabilities FROM model_catalog WHERE id=$1 AND enabled=true`, selected).Scan(&cfg.Protocol, &cfg.BaseURL, &keyRef, &name, &limits, &capabilities); err != nil {
				return nil, "", 0, 0, apierrors.New(apierrors.ModelUnavailable, "selected model disabled or missing")
			}
			cfg.APIKey = os.Getenv(keyRef)
			var lim map[string]int64
			_ = json.Unmarshal(limits, &lim)
			maxIn, maxOut = lim["maxInputTokens"], lim["maxOutputTokens"]
			var caps map[string]bool
			_ = json.Unmarshal(capabilities, &caps)
			if !caps["toolCalling"] || !caps["streaming"] {
				return nil, "", 0, 0, apierrors.New(apierrors.ModelUnavailable, "selected model lacks tool calling or streaming")
			}
		}
		if cfg.APIKey == "" || cfg.BaseURL == "" || name == "" {
			return nil, "", 0, 0, apierrors.New(apierrors.ModelUnavailable, "model endpoint or credentials not configured")
		}
		provider, err := model.NewProvider(cfg)
		return provider, name, maxIn, maxOut, err
	}
}
