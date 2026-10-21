// Package workflow defines advisory graphs. It does not execute a state machine.
package workflow

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
)

type Node struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	Phase          string `json:"phase"`
	Responsibility string `json:"responsibility"`
}
type Edge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
}
type Graph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}
type Draft struct {
	Name    string `json:"name"`
	Content string `json:"content"`
	Graph   Graph  `json:"graph"`
}

var nodeID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,80}$`)

// Cycles and parallel branches are valid reference relationships.
func (g Graph) Validate() error {
	raw, _ := json.Marshal(g)
	if len(g.Nodes) > 100 || len(g.Edges) > 300 || len(raw) > 128000 {
		return fmt.Errorf("graph exceeds 100 nodes, 300 edges or 128 KB")
	}
	ids := map[string]bool{}
	for _, n := range g.Nodes {
		if !nodeID.MatchString(n.ID) || ids[n.ID] || strings.TrimSpace(n.Name) == "" || len(n.Name) > 240 || len(n.Phase) > 240 || len(n.Responsibility) > 4000 {
			return fmt.Errorf("invalid or duplicate node")
		}
		switch n.Kind {
		case "activity", "decision", "start", "end":
		default:
			return fmt.Errorf("invalid node kind")
		}
		ids[n.ID] = true
	}
	edges := map[string]bool{}
	for _, e := range g.Edges {
		key := e.From + "\x00" + e.To + "\x00" + e.Label
		if !ids[e.From] || !ids[e.To] || e.From == e.To || edges[key] || len(e.Label) > 240 || (e.Kind != "sequence" && e.Kind != "feedback") {
			return fmt.Errorf("invalid or duplicate connection")
		}
		edges[key] = true
	}
	return nil
}

func ParseDraft(text string) (Draft, error) {
	var out Draft
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		if i := strings.Index(text, "\n"); i >= 0 {
			text = strings.TrimSuffix(strings.TrimSpace(text[i+1:]), "```")
		}
	}
	d := json.NewDecoder(strings.NewReader(text))
	d.DisallowUnknownFields()
	if err := d.Decode(&out); err != nil {
		return out, fmt.Errorf("model did not return a graph")
	}
	var extra any
	if d.Decode(&extra) != io.EOF || strings.TrimSpace(out.Name) == "" || len(out.Name) > 120 || len(out.Content) > 32000 || len(out.Graph.Nodes) == 0 {
		return out, fmt.Errorf("invalid workflow draft")
	}
	return out, out.Graph.Validate()
}
