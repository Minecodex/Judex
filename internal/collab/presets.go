package collab

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/collab/catalog"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/workflow"
)

type PresetImport struct {
	CatalogVersion string   `json:"catalogVersion"`
	Locale         string   `json:"locale"`
	PositionIDs    []string `json:"positionIds"`
	WorkflowIDs    []string `json:"workflowIds"`
}
type PresetResult struct {
	Created int         `json:"created"`
	Skipped int         `json:"skipped"`
	IDs     []uuid.UUID `json:"ids"`
}

func (s *Service) Presets(ctx context.Context) (catalog.Catalog, error) {
	if _, err := actor(ctx); err != nil {
		return catalog.Catalog{}, err
	}
	return catalog.Load(), nil
}

func (s *Service) ImportPresets(ctx context.Context, project uuid.UUID, in PresetImport) (PresetResult, error) {
	out := PresetResult{IDs: []uuid.UUID{}}
	if err := s.requireHuman(ctx); err != nil {
		return out, err
	}
	if in.CatalogVersion != catalog.Version {
		return out, conflict()
	}
	if (in.Locale != "en" && in.Locale != "zh-CN") || len(in.PositionIDs)+len(in.WorkflowIDs) == 0 || len(in.PositionIDs)+len(in.WorkflowIDs) > 200 {
		return out, apierrors.Fields("selection", "select 1 to 200 templates and a supported locale")
	}
	c := catalog.Load()
	type entry struct {
		kind, preset, name, content string
		names                       catalog.Text
		graph                       *workflow.Graph
	}
	entries := []entry{}
	seen := map[string]bool{}
	for _, id := range in.PositionIDs {
		found := false
		for _, role := range c.Roles {
			if role.ID == id && !seen["positions:"+id] {
				entries = append(entries, entry{"positions", id, role.Name.In(in.Locale), role.Prompt.In(in.Locale), role.Name, nil})
				found = true
			}
		}
		if !found {
			return out, apierrors.Fields("positionIds", "unknown or duplicate preset")
		}
		seen["positions:"+id] = true
	}
	for _, id := range in.WorkflowIDs {
		found := false
		for _, flow := range c.Workflows {
			if flow.ID == id && !seen["workflows:"+id] {
				entries = append(entries, entry{"workflows", id, flow.Name.In(in.Locale), flow.Content(in.Locale), flow.Name, flow.Graph(in.Locale)})
				found = true
			}
		}
		if !found {
			return out, apierrors.Fields("workflowIds", "unknown or duplicate preset")
		}
		seen["workflows:"+id] = true
	}
	err := s.write(ctx, func(ctx context.Context) error {
		out = PresetResult{IDs: []uuid.UUID{}}
		// Imports have a project-scoped lock; ordinary template editing retains
		// the existing optimistic version contract.
		if _, e := s.Pool.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 13))`, project.String()); e != nil {
			return e
		}
		if e := s.projectAccess(ctx, project, true); e != nil {
			return e
		}
		for _, item := range entries {
			// Matching a manually created bilingual name skips without replacing it.
			rows, e := s.Pool.Query(ctx, `SELECT id FROM `+item.kind+` WHERE project_id=$1 AND (preset_id=$2 OR lower(trim(name))=ANY($3::text[])) LIMIT 1`, project, item.preset, []string{strings.ToLower(item.names.ZH), strings.ToLower(item.names.EN)})
			if e != nil {
				return e
			}
			var existing uuid.UUID
			if rows.Next() {
				e = rows.Scan(&existing)
			}
			if e == nil {
				e = rows.Err()
			}
			rows.Close()
			if e != nil {
				return e
			}
			if existing != uuid.Nil {
				out.Skipped++
				out.IDs = append(out.IDs, existing)
				continue
			}
			id, e := s.SaveTemplate(ctx, project, uuid.Nil, item.kind, TemplateInput{Graph: item.graph, Name: item.name, Prompt: contentFor(item.kind == "positions", item.content), Content: contentFor(item.kind == "workflows", item.content)})
			if e != nil {
				return e
			}
			if _, e = s.Pool.Exec(ctx, `UPDATE `+item.kind+` SET preset_id=$2 WHERE id=$1`, id, item.preset); e != nil {
				return e
			}
			out.Created++
			out.IDs = append(out.IDs, id)
		}
		return nil
	})
	if err != nil {
		return PresetResult{}, err
	}
	return out, nil
}

func contentFor(enabled bool, value string) string {
	if enabled {
		return value
	}
	return ""
}
