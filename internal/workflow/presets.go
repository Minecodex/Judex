package workflow

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/project/catalog"
)

type ImportPresetsRequest struct {
	CatalogVersion string   `json:"catalogVersion"`
	ScenarioID     string   `json:"scenarioId"`
	WorkflowIDs    []string `json:"workflowIds"`
	Locale         string   `json:"locale"`
}
type SkippedPreset struct {
	PresetID   string    `json:"presetId"`
	WorkflowID uuid.UUID `json:"workflowId"`
	Name       string    `json:"name"`
}
type ImportedPresets struct {
	Items   []Definition    `json:"items"`
	Skipped []SkippedPreset `json:"skipped"`
}

func presetBody(preset catalog.WorkflowPreset, locale string) Body {
	body := Body{Name: preset.Name.Localized(locale), Instructions: preset.Instructions.Localized(locale),
		Nodes: []Node{}, AdvisoryEdges: []Edge{}, HardRules: []HardRule{}, ApprovalPolicies: map[string]string{}}
	for _, node := range preset.Nodes {
		phase := ""
		if node.Phase != nil {
			phase = node.Phase.Localized(locale)
		}
		body.Nodes = append(body.Nodes, Node{ID: node.ID, Name: node.Name.Localized(locale),
			Kind: node.Kind, Phase: phase, Responsibility: node.Responsibility.Localized(locale), AllowedPositionIds: []string{}, DefaultApprovalPolicy: "all"})
	}
	for _, edge := range preset.Edges {
		label := ""
		if edge.Label != nil {
			label = edge.Label.Localized(locale)
		}
		body.AdvisoryEdges = append(body.AdvisoryEdges, Edge{From: edge.From, To: edge.To, Kind: edge.Kind, Label: label})
	}
	return body
}

func selectedPresets(request ImportPresetsRequest) ([]catalog.WorkflowPreset, error) {
	value := catalog.Workflows()
	if request.CatalogVersion != value.Version {
		return nil, apierrors.New(apierrors.VersionConflict, "workflow preset catalog changed; reload it")
	}
	if request.Locale != "zh-CN" && request.Locale != "en" {
		return nil, apierrors.Fields("locale", "invalid")
	}
	if len(request.WorkflowIDs) < 1 || len(request.WorkflowIDs) > 32 {
		return nil, apierrors.Fields("workflowIds", "length")
	}
	allowed := map[string]bool{}
	for _, scenario := range value.Scenarios {
		if scenario.ID == request.ScenarioID {
			for _, id := range scenario.WorkflowIDs {
				allowed[id] = true
			}
		}
	}
	if len(allowed) == 0 {
		return nil, apierrors.Fields("scenarioId", "invalid")
	}
	byID, seen := map[string]catalog.WorkflowPreset{}, map[string]bool{}
	for _, preset := range value.Workflows {
		byID[preset.ID] = preset
	}
	out := []catalog.WorkflowPreset{}
	for _, id := range request.WorkflowIDs {
		preset, exists := byID[id]
		if !exists || !allowed[id] || seen[id] {
			return nil, apierrors.Fields("workflowIds", "invalid")
		}
		seen[id] = true
		if err := Validate(presetBody(preset, request.Locale)); err != nil {
			return nil, err
		}
		out = append(out, preset)
	}
	return out, nil
}

// ImportPresets creates only independent drafts. No node binding, person,
// project authority, hard task requirement or published version is imported.
func (s *Service) ImportPresets(ctx context.Context, requester, projectID uuid.UUID, request ImportPresetsRequest) (ImportedPresets, error) {
	out := ImportedPresets{Items: []Definition{}, Skipped: []SkippedPreset{}}
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, projectID.String()); err != nil {
			return err
		}
		role, err := s.membership(ctx, tx, requester, projectID)
		if err != nil {
			return err
		}
		if err = requireManager(role); err != nil {
			return err
		}
		presets, err := selectedPresets(request)
		if err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT id,name,preset_id FROM workflow_definitions WHERE project_id=$1 ORDER BY created_at,id`, projectID)
		if err != nil {
			return err
		}
		type existing struct {
			id       uuid.UUID
			name     string
			presetID *string
		}
		definitions := []existing{}
		for rows.Next() {
			var item existing
			if err = rows.Scan(&item.id, &item.name, &item.presetID); err != nil {
				rows.Close()
				return err
			}
			definitions = append(definitions, item)
		}
		rows.Close()
		if err = rows.Err(); err != nil {
			return err
		}
		for _, preset := range presets {
			var duplicate *existing
			for i := range definitions {
				item := &definitions[i]
				if (item.presetID != nil && *item.presetID == preset.ID) ||
					strings.EqualFold(strings.TrimSpace(item.name), preset.Name.Zh) ||
					strings.EqualFold(strings.TrimSpace(item.name), preset.Name.En) {
					duplicate = item
					break
				}
			}
			if duplicate != nil {
				out.Skipped = append(out.Skipped, SkippedPreset{PresetID: preset.ID, WorkflowID: duplicate.id, Name: duplicate.name})
				continue
			}
			id := preset.ID
			item, err := s.insertDefinition(ctx, tx, requester, projectID, presetBody(preset, request.Locale), &id)
			if err != nil {
				return err
			}
			out.Items = append(out.Items, item)
			definitions = append(definitions, existing{item.ID, item.Name, item.PresetID})
		}
		return nil
	})
	if err != nil {
		return ImportedPresets{}, err
	}
	return out, nil
}
