package workflow

import (
	"fmt"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"strings"
	"unicode/utf8"
)

func forwardEdges(edges []Edge) []Edge {
	out := []Edge{}
	for _, edge := range edges {
		if edge.Kind != "feedback" {
			out = append(out, edge)
		}
	}
	return out
}

func validateGraphAnnotations(body Body) error {
	nodes := map[string]Node{}
	for _, node := range body.Nodes {
		if node.Kind != "" && node.Kind != "activity" && node.Kind != "decision" {
			return apierrors.Fields("nodes[].kind", "enum")
		}
		if utf8.RuneCountInString(node.Phase) > 120 {
			return apierrors.Fields("nodes[].phase", "length")
		}
		nodes[node.ID] = node
	}
	adj := map[string][]string{}
	for _, edge := range forwardEdges(body.AdvisoryEdges) {
		adj[edge.From] = append(adj[edge.From], edge.To)
	}
	reachable := func(from, to string) bool {
		visited := map[string]bool{}
		var visit func(string) bool
		visit = func(id string) bool {
			if id == to {
				return true
			}
			if visited[id] {
				return false
			}
			visited[id] = true
			for _, next := range adj[id] {
				if visit(next) {
					return true
				}
			}
			return false
		}
		return visit(from)
	}
	for _, edge := range body.AdvisoryEdges {
		if edge.Kind != "" && edge.Kind != "sequence" && edge.Kind != "feedback" {
			return apierrors.Fields("advisoryEdges[].kind", "enum")
		}
		if utf8.RuneCountInString(edge.Label) > 200 {
			return apierrors.Fields("advisoryEdges[].label", "length")
		}
		if edge.Kind == "feedback" {
			if nodes[edge.From].Kind != "decision" || strings.TrimSpace(edge.Label) == "" || !reachable(edge.To, edge.From) {
				return apierrors.Fields("advisoryEdges", "feedback must return from a decision to an earlier node with a label")
			}
		}
	}
	return nil
}

// Annotations are a collaboration reference; feedback never becomes a
// cyclic task prerequisite, expression, delegation or automatic approval.
func GenerateMermaid(body Body) string {
	var b strings.Builder
	b.WriteString("flowchart LR\n")
	ids := map[string]string{}
	phases, order := map[string][]Node{}, []string{}
	for i, node := range body.Nodes {
		ids[node.ID] = fmt.Sprintf("node%d", i)
		if _, exists := phases[node.Phase]; !exists {
			order = append(order, node.Phase)
		}
		phases[node.Phase] = append(phases[node.Phase], node)
	}
	label := func(value string) string {
		return strings.NewReplacer("\"", "'", "\n", " ", "\r", " ", "<", " ", ">", " ").Replace(value)
	}
	for i, phase := range order {
		if phase != "" {
			fmt.Fprintf(&b, "  subgraph phase%d[\"%s\"]\n", i, label(phase))
		}
		for _, node := range phases[phase] {
			if node.Kind == "decision" {
				fmt.Fprintf(&b, "  %s{\"%s\"}\n", ids[node.ID], label(node.Name))
			} else {
				fmt.Fprintf(&b, "  %s[\"%s\"]\n", ids[node.ID], label(node.Name))
			}
		}
		if phase != "" {
			b.WriteString("  end\n")
		}
	}
	for _, edge := range body.AdvisoryEdges {
		if edge.Kind == "feedback" {
			fmt.Fprintf(&b, "  %s -. \"%s\" .-> %s\n", ids[edge.From], label(edge.Label), ids[edge.To])
		} else if edge.Label != "" {
			fmt.Fprintf(&b, "  %s -->|\"%s\"| %s\n", ids[edge.From], label(edge.Label), ids[edge.To])
		} else {
			fmt.Fprintf(&b, "  %s --> %s\n", ids[edge.From], ids[edge.To])
		}
	}
	return b.String()
}
