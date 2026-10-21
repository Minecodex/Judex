package catalog

import (
	"strings"
	"testing"
)

func TestCatalogReferencesAndAdvisoryContent(t *testing.T) {
	c := Load()
	if len(c.Scenarios) != 20 || len(c.Roles) != 118 || len(c.Workflows) != 51 {
		t.Fatal("the existing built-in library must remain complete")
	}
	roles, flows := map[string]bool{}, map[string]bool{}
	for _, r := range c.Roles {
		if roles[r.ID] || r.ID == "" {
			t.Fatalf("duplicate role %s", r.ID)
		}
		roles[r.ID] = true
		for _, locale := range []string{"zh-CN", "en"} {
			if r.Name.In(locale) == "" || r.Summary.In(locale) == "" || len(r.Prompt.In(locale)) > 32000 {
				t.Fatalf("invalid role %s", r.ID)
			}
		}
	}
	for _, w := range c.Workflows {
		if flows[w.ID] || w.ID == "" || len(w.Nodes) == 0 {
			t.Fatalf("invalid workflow %s", w.ID)
		}
		flows[w.ID] = true
		nodes := map[string]bool{}
		for _, n := range w.Nodes {
			if nodes[n.ID] || n.Name.ZH == "" || n.Name.EN == "" {
				t.Fatalf("invalid node in %s", w.ID)
			}
			nodes[n.ID] = true
		}
		for _, e := range w.Edges {
			if !nodes[e.From] || !nodes[e.To] {
				t.Fatalf("dangling edge in %s", w.ID)
			}
		}
		for _, id := range w.RoleIDs {
			if !roles[id] {
				t.Fatalf("missing role %s", id)
			}
		}
		for _, locale := range []string{"zh-CN", "en"} {
			content := w.Content(locale)
			if len(content) > 32000 || !strings.Contains(content, w.Nodes[0].Name.In(locale)) {
				t.Fatalf("workflow cannot be imported in %s: %s", locale, w.ID)
			}
		}
		if !strings.Contains(w.Content("en"), "Workflows are advisory") {
			t.Fatalf("missing V2 boundary in %s", w.ID)
		}
	}
	for _, s := range c.Scenarios {
		if len(s.RoleIDs) == 0 || len(s.WorkflowIDs) == 0 {
			t.Fatalf("empty scenario %s", s.ID)
		}
		for _, id := range s.RoleIDs {
			if !roles[id] {
				t.Fatalf("missing scenario role %s", id)
			}
		}
		for _, id := range s.WorkflowIDs {
			if !flows[id] {
				t.Fatalf("missing scenario workflow %s", id)
			}
		}
	}
}

func TestAllPresetGraphs(t *testing.T) {
	for _, w := range Load().Workflows {
		for _, locale := range []string{"zh-CN", "en"} {
			if e := w.Graph(locale).Validate(); e != nil {
				t.Fatalf("%s: %v", w.ID, e)
			}
		}
	}
}
