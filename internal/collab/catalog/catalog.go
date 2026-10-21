// Package catalog supplies versioned advisory templates, not executable workflows.
package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/kakj-go/Judex/internal/workflow"
	"strings"
)

const Version = "2026-10-20.v2.1"

type Text struct {
	ZH string `json:"zh"`
	EN string `json:"en"`
}

func (t Text) In(locale string) string {
	if locale == "en" {
		return t.EN
	}
	return t.ZH
}

type Role struct {
	ID      string `json:"id"`
	Name    Text   `json:"name"`
	Summary Text   `json:"summary"`
	Prompt  Text   `json:"prompt"`
}
type Node struct {
	ID             string `json:"id"`
	Name           Text   `json:"name"`
	Phase          Text   `json:"phase"`
	Kind           string `json:"kind"`
	Responsibility Text   `json:"responsibility"`
}
type Edge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Kind  string `json:"kind"`
	Label Text   `json:"label"`
}
type Workflow struct {
	ID           string   `json:"id"`
	ScenarioIDs  []string `json:"scenarioIds"`
	RoleIDs      []string `json:"roleIds"`
	Name         Text     `json:"name"`
	Summary      Text     `json:"summary"`
	Instructions Text     `json:"instructions"`
	Complexity   string   `json:"complexity"`
	Nodes        []Node   `json:"nodes"`
	Edges        []Edge   `json:"edges"`
}
type Scenario struct {
	ID          string   `json:"id"`
	Name        Text     `json:"name"`
	Description Text     `json:"description"`
	RoleIDs     []string `json:"roleIds"`
	WorkflowIDs []string `json:"workflowIds"`
}
type Catalog struct {
	Version   string     `json:"version"`
	Roles     []Role     `json:"roles"`
	Workflows []Workflow `json:"workflows"`
	Scenarios []Scenario `json:"scenarios"`
}

//go:embed roles.json
var roleJSON []byte

//go:embed workflows.json
var workflowJSON []byte

//go:embed scenarios.json
var scenarioJSON []byte

func Load() Catalog {
	c := Catalog{Version: Version}
	for _, item := range []struct {
		raw []byte
		dst any
	}{{roleJSON, &c.Roles}, {workflowJSON, &c.Workflows}, {scenarioJSON, &c.Scenarios}} {
		if err := json.Unmarshal(item.raw, item.dst); err != nil {
			panic(fmt.Errorf("embedded template catalog: %w", err))
		}
	}
	return c
}

// Content retains domain guidance and advisory relationships as readable text.
// No legacy task IDs, approval policy or workflow engine state is imported.
func (w Workflow) Content(locale string) string {
	var out strings.Builder
	out.WriteString(w.Instructions.In(locale) + "\n\n")
	names := map[string]string{}
	for _, n := range w.Nodes {
		names[n.ID] = n.Name.In(locale)
		title := n.Name.In(locale)
		if phase := n.Phase.In(locale); phase != "" {
			title = phase + " · " + title
		}
		fmt.Fprintf(&out, "## %s\n\n%s\n\n", title, n.Responsibility.In(locale))
	}
	if len(w.Edges) > 0 {
		if locale == "en" {
			out.WriteString("## Suggested collaboration\n\n")
		} else {
			out.WriteString("## 建议协作关系\n\n")
		}
		for _, e := range w.Edges {
			fmt.Fprintf(&out, "- %s → %s", names[e.From], names[e.To])
			if label := e.Label.In(locale); label != "" {
				out.WriteString(" · " + label)
			}
			out.WriteString("\n")
		}
	}
	return out.String()
}

func (w Workflow) Graph(locale string) *workflow.Graph {
	g := &workflow.Graph{Nodes: []workflow.Node{}, Edges: []workflow.Edge{}}
	for _, n := range w.Nodes {
		kind := n.Kind
		if kind == "" {
			kind = "activity"
		}
		g.Nodes = append(g.Nodes, workflow.Node{ID: n.ID, Name: n.Name.In(locale), Phase: n.Phase.In(locale), Responsibility: n.Responsibility.In(locale), Kind: kind})
	}
	for _, e := range w.Edges {
		kind := e.Kind
		if kind == "" {
			kind = "sequence"
		}
		g.Edges = append(g.Edges, workflow.Edge{From: e.From, To: e.To, Label: e.Label.In(locale), Kind: kind})
	}
	return g
}
