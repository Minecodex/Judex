package project

import (
	"testing"
	"unicode/utf8"
)

func TestBuiltinPositionCatalog(t *testing.T) {
	catalog := BuiltinPositionCatalog()
	if catalog.Version == "" || len(catalog.Scenarios) != 20 {
		t.Fatal("catalog version or scenarios missing")
	}
	roles, scenarios, used := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, role := range catalog.Roles {
		if role.ID == "" || roles[role.ID] {
			t.Fatal("duplicate or empty position ID", role.ID)
		}
		roles[role.ID] = true
		for _, locale := range []string{"zh-CN", "en"} {
			if length := utf8.RuneCountInString(role.Name.Localized(locale)); length < 1 || length > 120 {
				t.Fatal("invalid name", role.ID, locale)
			}
			if role.Summary.Localized(locale) == "" || role.Prompt.Localized(locale) == "" || utf8.RuneCountInString(role.Prompt.Localized(locale)) > 20000 {
				t.Fatal("missing responsibilities", role.ID, locale)
			}
		}
	}
	for _, scenario := range catalog.Scenarios {
		if scenario.ID == "" || scenarios[scenario.ID] || scenario.Name.Zh == "" || scenario.Name.En == "" || len(scenario.RoleIDs) < 1 || len(scenario.RoleIDs) > 32 {
			t.Fatal("invalid scenario", scenario.ID)
		}
		scenarios[scenario.ID] = true
		selected := map[string]bool{}
		for _, id := range scenario.RoleIDs {
			if !roles[id] || selected[id] {
				t.Fatal("unknown or duplicate scenario position", scenario.ID, id)
			}
			selected[id], used[id] = true, true
		}
	}
	if len(used) != len(roles) {
		t.Fatal("unused position templates")
	}
	// Returned values must not allow project edits to mutate the global defaults.
	catalog.Roles[0].Name.Zh = "edited"
	if BuiltinPositionCatalog().Roles[0].Name.Zh == "edited" {
		t.Fatal("catalog is shared mutable state")
	}
}
