// SPDX-License-Identifier: Apache-2.0

// Package agent hosts the platform Agent domain: the deployment-synced
// model catalog here, the execution harness in later stages (P5).
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

// CatalogEntry is one operator-configured model (user decision 2026-09-27:
// baseUrl + apiKeyEnv Secret reference + model name + max in/out tokens).
// The API key itself is ONLY resolved from the environment at call time and
// never persisted or returned.
type CatalogEntry struct {
	ID              string `yaml:"id" json:"id"`
	DisplayName     string `yaml:"displayName" json:"displayName"`
	Provider        string `yaml:"provider" json:"provider"`
	BaseURL         string `yaml:"baseUrl" json:"baseUrl"`
	APIKeyEnv       string `yaml:"apiKeyEnv" json:"apiKeyEnv"`
	ModelName       string `yaml:"model" json:"model"`
	MaxInputTokens  int    `yaml:"maxInputTokens" json:"maxInputTokens"`
	MaxOutputTokens int    `yaml:"maxOutputTokens" json:"maxOutputTokens"`
	ToolCalling     bool   `yaml:"toolCalling" json:"toolCalling"`
	Vision          bool   `yaml:"vision" json:"vision"`
	Streaming       bool   `yaml:"streaming" json:"streaming"`
}

// LoadCatalogFile reads the deployment catalog YAML.
func LoadCatalogFile(path string) ([]CatalogEntry, error) {
	if path == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, apierrors.Newf(apierrors.DependencyDown, "model catalog file unreadable: %v", err)
	}
	var doc struct {
		Models []CatalogEntry `yaml:"models" json:"models"`
	}
	if err := parseYAML(raw, &doc); err != nil {
		return nil, apierrors.Newf(apierrors.DependencyDown, "model catalog invalid: %v", err)
	}
	seen := map[string]bool{}
	for _, entry := range doc.Models {
		if entry.ID == "" || entry.BaseURL == "" || entry.ModelName == "" || entry.APIKeyEnv == "" {
			return nil, apierrors.Newf(apierrors.Validation, "model entry %q missing id/baseUrl/model/apiKeyEnv", entry.ID)
		}
		if seen[entry.ID] {
			return nil, apierrors.Newf(apierrors.Validation, "duplicate model id %q", entry.ID)
		}
		seen[entry.ID] = true
		if entry.MaxInputTokens <= 0 || entry.MaxOutputTokens <= 0 {
			return nil, apierrors.Newf(apierrors.Validation, "model %q needs positive maxInputTokens/maxOutputTokens", entry.ID)
		}
	}
	return doc.Models, nil
}

// SyncCatalog upserts entries by stable id and disables missing ones
// (02 §8 部署配置同步：幂等 upsert/停用，保留历史 ID). No secrets stored.
func SyncCatalog(ctx context.Context, pool *postgres.Pool, entries []CatalogEntry, now time.Time) error {
	return pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		active := map[string]bool{}
		for _, entry := range entries {
			active[entry.ID] = true
			caps, _ := json.Marshal(map[string]bool{
				"toolCalling": entry.ToolCalling, "vision": entry.Vision, "streaming": entry.Streaming,
			})
			limits, _ := json.Marshal(map[string]int{
				"maxInputTokens": entry.MaxInputTokens, "maxOutputTokens": entry.MaxOutputTokens,
			})
			id, err := uuid.Parse(entry.ID)
			if err != nil {
				return apierrors.Newf(apierrors.Validation, "model id %q must be a uuid", entry.ID)
			}
			// endpoint_config_ref holds the baseUrl only; the API key stays an
			// env/Secret reference and is never written to a row the API can read.
			if _, err := tx.Exec(ctx, `
				INSERT INTO model_catalog (id, display_name, provider, endpoint_config_ref, credential_secret_ref,
				                           model_name, capabilities, limits, enabled, created_at, updated_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8::jsonb,true,$9,$9)
				ON CONFLICT (id) DO UPDATE SET
					display_name=EXCLUDED.display_name,
					provider=EXCLUDED.provider,
					endpoint_config_ref=EXCLUDED.endpoint_config_ref,
					credential_secret_ref=EXCLUDED.credential_secret_ref,
					model_name=EXCLUDED.model_name,
					capabilities=EXCLUDED.capabilities,
					limits=EXCLUDED.limits,
					enabled=true,
					updated_at=EXCLUDED.updated_at`,
				id, entry.DisplayName, entry.Provider, entry.BaseURL, entry.APIKeyEnv,
				entry.ModelName, string(caps), string(limits), now); err != nil {
				return apierrors.New(apierrors.Internal, "catalog upsert failed").Wrap(err)
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE model_catalog SET enabled=false, updated_at=$1`, now); err != nil {
			return apierrors.New(apierrors.Internal, "catalog disable failed").Wrap(err)
		}
		for id := range active {
			uid, _ := uuid.Parse(id)
			if _, err := tx.Exec(ctx, `UPDATE model_catalog SET enabled=true WHERE id=$1`, uid); err != nil {
				return apierrors.New(apierrors.Internal, "catalog enable failed").Wrap(err)
			}
		}
		return nil
	})
}

// PublicModel is the API projection: never includes baseUrl-with-key or the
// secret reference (02 §8 API 只获得 ID、公开能力与可用状态).
type PublicModel struct {
	ID           uuid.UUID       `json:"id"`
	DisplayName  string          `json:"displayName"`
	Provider     string          `json:"provider"`
	Enabled      bool            `json:"enabled"`
	Capabilities map[string]bool `json:"capabilities"`
	Limits       map[string]int  `json:"limits"`
}

// ListPublicModels reads the catalog for GET /models.
func ListPublicModels(ctx context.Context, pool *postgres.Pool) ([]PublicModel, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, display_name, provider, enabled, capabilities, limits
		FROM model_catalog ORDER BY display_name`)
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "catalog read failed").Wrap(err)
	}
	defer rows.Close()
	var out []PublicModel
	for rows.Next() {
		var (
			m    PublicModel
			caps []byte
			lim  []byte
		)
		if err := rows.Scan(&m.ID, &m.DisplayName, &m.Provider, &m.Enabled, &caps, &lim); err != nil {
			return nil, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		_ = json.Unmarshal(caps, &m.Capabilities)
		_ = json.Unmarshal(lim, &m.Limits)
		out = append(out, m)
	}
	return out, rows.Err()
}

// ModelEnabled checks a model id exists and is enabled (project config).
func ModelEnabled(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, id uuid.UUID) (bool, error) {
	var enabled bool
	err := q.QueryRow(ctx, `SELECT enabled FROM model_catalog WHERE id=$1`, id).Scan(&enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, apierrors.Newf(apierrors.InvalidReference, "model not found")
	}
	if err != nil {
		return false, apierrors.New(apierrors.Internal, "model lookup failed").Wrap(err)
	}
	if !enabled {
		return false, apierrors.Newf(apierrors.InvalidReference, "model disabled")
	}
	return true, nil
}
