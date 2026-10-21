package workflow

import (
	"encoding/json"
	"testing"
)

func TestGraphValidationAndDraftParsing(t *testing.T) {
	g := Graph{Nodes: []Node{{ID: "a", Name: "需求", Kind: "start"}, {ID: "b", Name: "评估", Kind: "decision"}}, Edges: []Edge{{From: "a", To: "b", Kind: "sequence"}, {From: "b", To: "a", Kind: "feedback", Label: "补充"}}}
	if e := g.Validate(); e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(Draft{Name: "协作", Graph: g})
	if _, e := ParseDraft(string(raw)); e != nil {
		t.Fatal(e)
	}
	for _, v := range []string{string(raw) + " trailing", string(raw) + "{}", "{}", "```json\n{}\n```"} {
		if _, e := ParseDraft(v); e == nil {
			t.Fatalf("accepted invalid draft %s", v)
		}
	}
	bad := g
	bad.Edges = []Edge{{From: "a", To: "missing", Kind: "sequence"}}
	if bad.Validate() == nil {
		t.Fatal("dangling edge")
	}
	bad = g
	bad.Nodes = append(append([]Node{}, g.Nodes...), g.Nodes[0])
	if bad.Validate() == nil {
		t.Fatal("duplicate node")
	}
	bad = g
	bad.Edges = append(append([]Edge{}, g.Edges...), g.Edges[0])
	if bad.Validate() == nil {
		t.Fatal("duplicate edge")
	}
}
