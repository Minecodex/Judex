package workflow

import (
	"github.com/kakj-go/Judex/internal/project/catalog"
	"net/url"
	"testing"
)

func TestBuiltinWorkflowCatalog(t *testing.T) {
	value := catalog.Workflows()
	positions := catalog.Positions()
	if len(value.Scenarios) != len(positions.Scenarios) || len(value.Workflows) < 20 {
		t.Fatal("missing shared scenarios or workflows")
	}
	roles, seen := map[string]bool{}, map[string]bool{}
	for _, role := range positions.Roles {
		roles[role.ID] = true
	}
	for i, scenario := range value.Scenarios {
		if scenario.ID != positions.Scenarios[i].ID || scenario.Name != positions.Scenarios[i].Name || len(scenario.WorkflowIDs) == 0 {
			t.Fatal("scenario drift", scenario)
		}
		advanced := 0
		for _, preset := range value.Workflows {
			for _, id := range scenario.WorkflowIDs {
				if preset.ID == id && preset.Complexity == "advanced" {
					advanced++
				}
			}
		}
		if advanced == 0 {
			t.Fatal("scenario lacks a researched full workflow", scenario.ID)
		}
		for _, id := range scenario.WorkflowIDs {
			found := false
			for _, preset := range value.Workflows {
				if preset.ID == id {
					found = true
				}
			}
			if !found {
				t.Fatal("unknown workflow", id)
			}
		}
	}
	for _, preset := range value.Workflows {
		if seen[preset.ID] || preset.Name.Zh == "" || preset.Name.En == "" || preset.Summary.Zh == "" || preset.Summary.En == "" {
			t.Fatal("invalid preset", preset.ID)
		}
		seen[preset.ID] = true
		if preset.Complexity == "advanced" {
			gates, returns, forks := 0, 0, 0
			outgoing := map[string]int{}
			for _, node := range preset.Nodes {
				if node.Kind == "decision" {
					gates++
				}
				if node.Phase == nil || node.Phase.Zh == "" || node.Phase.En == "" {
					t.Fatal("missing stage", preset.ID)
				}
			}
			for _, edge := range preset.Edges {
				if edge.Kind == "feedback" {
					returns++
				} else {
					outgoing[edge.From]++
				}
			}
			for _, n := range outgoing {
				if n > 1 {
					forks++
				}
			}
			if len(preset.Nodes) < 18 || gates < 5 || returns < 5 || forks < 2 || len(preset.Sources) == 0 {
				t.Fatal("full workflow lacks real branches, reviews or provenance", preset.ID)
			}
			for _, source := range preset.Sources {
				parsed, err := url.Parse(source.URL)
				if err != nil || parsed.Scheme != "https" || parsed.Host == "" || source.Title.Zh == "" || source.Title.En == "" {
					t.Fatal("invalid source", preset.ID)
				}
			}
		}
		for _, id := range preset.RoleIDs {
			if !roles[id] {
				t.Fatal("unknown role", id)
			}
		}
		for _, locale := range []string{"zh-CN", "en"} {
			body := presetBody(preset, locale)
			if err := Validate(body); err != nil {
				t.Fatal(preset.ID, locale, err)
			}
			if len(body.HardRules) != 0 || len(body.ApprovalPolicies) != 0 {
				t.Fatal("preset imported authority")
			}
			for _, node := range body.Nodes {
				if len(node.AllowedPositionIds) != 0 || len(node.DelegationUserIDs) != 0 || node.Responsibility == "" {
					t.Fatal("node side effect", node)
				}
			}
		}
	}
	value.Workflows[0].Nodes[0].Name.Zh = "modified"
	if catalog.Workflows().Workflows[0].Nodes[0].Name.Zh == "modified" {
		t.Fatal("catalog shares mutable storage")
	}
}

func TestWorkflowPresetSelection(t *testing.T) {
	request := ImportPresetsRequest{CatalogVersion: catalog.Workflows().Version, ScenarioID: "software", WorkflowIDs: []string{"software-standard", "software-bug"}, Locale: "zh-CN"}
	if items, err := selectedPresets(request); err != nil || len(items) != 2 {
		t.Fatal(items, err)
	}
	for _, mutate := range []func(*ImportPresetsRequest){
		func(r *ImportPresetsRequest) { r.CatalogVersion = "old" },
		func(r *ImportPresetsRequest) { r.Locale = "invalid" },
		func(r *ImportPresetsRequest) { r.ScenarioID = "missing" },
		func(r *ImportPresetsRequest) { r.WorkflowIDs = []string{} },
		func(r *ImportPresetsRequest) { r.WorkflowIDs = []string{"software-bug", "missing"} },
		func(r *ImportPresetsRequest) { r.WorkflowIDs = []string{"software-bug", "content-publish"} },
		func(r *ImportPresetsRequest) { r.WorkflowIDs = []string{"software-bug", "software-bug"} },
	} {
		invalid := request
		mutate(&invalid)
		if _, err := selectedPresets(invalid); err == nil {
			t.Fatal("invalid selection accepted", invalid)
		}
	}
}
