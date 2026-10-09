package project

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/project/catalog"
)

type PresetText = catalog.Text
type PositionPreset = catalog.PositionPreset
type PositionScenario = catalog.PositionScenario
type PositionPresetCatalog = catalog.PositionCatalog

// Each read owns its value; neither callers nor imported project edits mutate
// the global built-in catalog. The same JSON feeds the explicit demo mode.
func BuiltinPositionCatalog() PositionPresetCatalog {
	return catalog.Positions()
}

type ImportPositionPresetsRequest struct {
	CatalogVersion string   `json:"catalogVersion"`
	ScenarioID     string   `json:"scenarioId"`
	RoleIDs        []string `json:"roleIds"`
	Locale         string   `json:"locale"`
}
type SkippedPositionPreset struct {
	PresetID   string    `json:"presetId"`
	PositionID uuid.UUID `json:"positionId"`
	Name       string    `json:"name"`
}
type ImportedPositionPresets struct {
	Items   []Position              `json:"items"`
	Skipped []SkippedPositionPreset `json:"skipped"`
}

func selectedPositionPresets(request ImportPositionPresetsRequest) ([]PositionPreset, error) {
	catalog := BuiltinPositionCatalog()
	if request.CatalogVersion != catalog.Version {
		return nil, apierrors.New(apierrors.VersionConflict, "position preset catalog changed; reload it")
	}
	if request.Locale != "zh-CN" && request.Locale != "en" {
		return nil, apierrors.Fields("locale", "invalid")
	}
	if len(request.RoleIDs) < 1 || len(request.RoleIDs) > 32 {
		return nil, apierrors.Fields("roleIds", "length")
	}
	allowed := map[string]bool{}
	for _, scenario := range catalog.Scenarios {
		if scenario.ID == request.ScenarioID {
			for _, id := range scenario.RoleIDs {
				allowed[id] = true
			}
		}
	}
	if len(allowed) == 0 {
		return nil, apierrors.Fields("scenarioId", "invalid")
	}
	roles, seen := map[string]PositionPreset{}, map[string]bool{}
	for _, role := range catalog.Roles {
		roles[role.ID] = role
	}
	out := make([]PositionPreset, 0, len(request.RoleIDs))
	for _, id := range request.RoleIDs {
		role, exists := roles[id]
		if !exists || !allowed[id] || seen[id] {
			return nil, apierrors.Fields("roleIds", "invalid")
		}
		seen[id] = true
		out = append(out, role)
	}
	return out, nil
}

// ImportPositionPresets copies selected responsibilities in one project-locked
// transaction. It never assigns people, workflow nodes, models or authority.
func (s *Service) ImportPositionPresets(ctx context.Context, requester, projectID uuid.UUID, request ImportPositionPresetsRequest) (ImportedPositionPresets, error) {
	out := ImportedPositionPresets{Items: []Position{}, Skipped: []SkippedPositionPreset{}}
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, projectID.String()); err != nil {
			return err
		}
		membership, err := s.MembershipForTx(ctx, tx, requester, projectID)
		if err != nil {
			return err
		}
		if membership.Role != "owner" && membership.Role != "manager" {
			return apierrors.New(apierrors.Forbidden, "manager or owner required")
		}
		presets, err := selectedPositionPresets(request)
		if err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT id, name, preset_id FROM position_templates WHERE project_id=$1 AND status='active' ORDER BY created_at, id`, projectID)
		if err != nil {
			return apierrors.New(apierrors.Internal, "existing positions failed").Wrap(err)
		}
		type existingPosition struct {
			id       uuid.UUID
			name     string
			presetID *string
		}
		existing := []existingPosition{}
		for rows.Next() {
			var position existingPosition
			if err := rows.Scan(&position.id, &position.name, &position.presetID); err != nil {
				rows.Close()
				return apierrors.New(apierrors.Internal, "existing position scan failed").Wrap(err)
			}
			existing = append(existing, position)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, preset := range presets {
			var duplicate *existingPosition
			for i := range existing {
				position := &existing[i]
				if (position.presetID != nil && *position.presetID == preset.ID) || strings.EqualFold(strings.TrimSpace(position.name), preset.Name.Zh) || strings.EqualFold(strings.TrimSpace(position.name), preset.Name.En) {
					duplicate = position
					break
				}
			}
			if duplicate != nil {
				out.Skipped = append(out.Skipped, SkippedPositionPreset{PresetID: preset.ID, PositionID: duplicate.id, Name: duplicate.name})
				continue
			}
			id := preset.ID
			position, err := s.insertPosition(ctx, tx, requester, projectID, PositionDraft{
				Name: preset.Name.Localized(request.Locale), Prompt: preset.Prompt.Localized(request.Locale), PublicSummary: preset.Summary.Localized(request.Locale),
			}, &id)
			if err != nil {
				return err
			}
			out.Items = append(out.Items, position)
			existing = append(existing, existingPosition{position.ID, position.Name, position.PresetID})
		}
		return nil
	})
	if err != nil {
		return ImportedPositionPresets{}, err
	}
	return out, nil
}
