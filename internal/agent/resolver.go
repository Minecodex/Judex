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
   LEFT JOIN role_agents i ON i.id=$2 AND i.group_id IN(SELECT id FROM groups WHERE project_id=p.id)
   LEFT JOIN positions t ON t.id=i.position_id
   LEFT JOIN position_versions v ON v.position_id=t.id AND v.revision=t.current_version
   WHERE p.id=$1 AND p.status='active'`, project, identity).Scan(&selected)
		if err != nil {
			return nil, "", 0, 0, apierrors.New(apierrors.ModelUnavailable, "project unavailable")
		}
		cfg := model.GatewayConfig{Protocol: fallback.Protocol, BaseURL: fallback.BaseURL, APIKey: fallback.APIKey}
		name := fallback.Model
		maxIn, maxOut := int64(128000), int64(8192)
		var fallbackIDs []string
		if selected != nil {
			var keyRef string
			var limits []byte
			var capabilities []byte
			var quotaFallbacks []byte
			var options []byte
			if err = pool.QueryRow(ctx, `SELECT provider,endpoint_config_ref,credential_secret_ref,model_name,limits,capabilities,quota_fallback_ids,request_options FROM model_catalog WHERE id=$1 AND enabled=true`, selected).Scan(&cfg.Protocol, &cfg.BaseURL, &keyRef, &name, &limits, &capabilities, &quotaFallbacks, &options); err != nil {
				return nil, "", 0, 0, apierrors.New(apierrors.ModelUnavailable, "selected model disabled or missing")
			}
			cfg.APIKey = os.Getenv(keyRef)
			if err = json.Unmarshal(options, &cfg.RequestOptions); err != nil {
				return nil, "", 0, 0, err
			}
			if err = json.Unmarshal(quotaFallbacks, &fallbackIDs); err != nil {
				return nil, "", 0, 0, err
			}
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
		if err != nil {
			return nil, "", 0, 0, err
		}
		alternatives := []model.Target{}
		for _, fallbackID := range fallbackIDs {
			target, err := resolveQuotaTarget(ctx, pool, fallbackID)
			if err != nil {
				continue
			} // A disabled/unconfigured fallback never blocks the primary.
			alternatives = append(alternatives, target)
		}
		if len(alternatives) > 0 {
			provider = model.WithQuotaFallbacks(model.Target{Provider: provider, Name: name, MaxInputTokens: maxIn, MaxOutputTokens: maxOut}, alternatives)
		}
		return provider, name, maxIn, maxOut, err
	}
}

func resolveQuotaTarget(ctx context.Context, pool *postgres.Pool, id string) (model.Target, error) {
	var cfg model.GatewayConfig
	var name, keyRef string
	var limits, capabilities, options []byte
	err := pool.QueryRow(ctx, `SELECT provider,endpoint_config_ref,credential_secret_ref,model_name,limits,capabilities,request_options FROM model_catalog WHERE id=$1 AND enabled=true`, id).Scan(&cfg.Protocol, &cfg.BaseURL, &keyRef, &name, &limits, &capabilities, &options)
	if err != nil {
		return model.Target{}, err
	}
	cfg.APIKey = os.Getenv(keyRef)
	if err = json.Unmarshal(options, &cfg.RequestOptions); err != nil {
		return model.Target{}, err
	}
	var lim map[string]int64
	var caps map[string]bool
	if err = json.Unmarshal(limits, &lim); err != nil {
		return model.Target{}, err
	}
	if err = json.Unmarshal(capabilities, &caps); err != nil {
		return model.Target{}, err
	}
	if cfg.APIKey == "" || !caps["toolCalling"] || !caps["streaming"] || lim["maxInputTokens"] <= 0 || lim["maxOutputTokens"] <= 0 {
		return model.Target{}, apierrors.New(apierrors.ModelUnavailable, "quota fallback unavailable")
	}
	provider, err := model.NewProvider(cfg)
	return model.Target{Provider: provider, Name: name, MaxInputTokens: lim["maxInputTokens"], MaxOutputTokens: lim["maxOutputTokens"]}, err
}
