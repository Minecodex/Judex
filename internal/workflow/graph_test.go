package workflow

import (
	"strings"
	"testing"
)

func annotatedBody() Body {
	return Body{Name: "Reviewable process", Nodes: []Node{
		{ID: "scope", Name: "Scope", Phase: "Planning", DefaultApprovalPolicy: "all"},
		{ID: "build", Name: "Build", Phase: "Delivery", DefaultApprovalPolicy: "all"},
		{ID: "review", Name: "Review", Kind: "decision", Phase: "Delivery", DefaultApprovalPolicy: "all"},
		{ID: "close", Name: "Close", Phase: "Receipt", DefaultApprovalPolicy: "all"},
	}, AdvisoryEdges: []Edge{
		{From: "scope", To: "build"}, {From: "build", To: "review"}, {From: "review", To: "close", Label: "Accepted"},
		{From: "review", To: "build", Kind: "feedback", Label: "Needs revision"},
	}, HardRules: []HardRule{}, ApprovalPolicies: map[string]string{}}
}
func TestWorkflowFeedbackIsAnAnnotation(t *testing.T) {
	body := annotatedBody()
	if err := Validate(body); err != nil {
		t.Fatal(err)
	}
	if !hasCycle(body.Nodes, body.AdvisoryEdges) || hasCycle(body.Nodes, forwardEdges(body.AdvisoryEdges)) {
		t.Fatal("return was treated as an executable dependency")
	}
	code := GenerateMermaid(body)
	for _, fragment := range []string{"subgraph", "{\"Review\"}", "-. \"Needs revision\" .->", "-->|\"Accepted\"|"} {
		if !strings.Contains(code, fragment) {
			t.Fatal("diagram lost annotation", fragment, code)
		}
	}
	for _, mutate := range []func(*Body){
		func(b *Body) { b.AdvisoryEdges[3].From = "build" },
		func(b *Body) { b.AdvisoryEdges[3].To = "close" },
		func(b *Body) { b.AdvisoryEdges[3].Label = "" },
		func(b *Body) { b.AdvisoryEdges[3].Kind = "expression" },
		func(b *Body) { b.Nodes[2].Kind = "auto-approve" },
		func(b *Body) { b.AdvisoryEdges = append(b.AdvisoryEdges, Edge{From: "close", To: "scope"}) },
	} {
		invalid := annotatedBody()
		mutate(&invalid)
		if err := Validate(invalid); err == nil {
			t.Fatal("invalid return or forward cycle accepted", invalid)
		}
	}
}
func TestWorkflowDisplayAnnotationsDoNotGrantAuthority(t *testing.T) {
	body := annotatedBody()
	hash := ConstraintHash(body)
	body.Nodes[0].Phase = "Display-only grouping"
	body.Nodes[0].Kind = "decision"
	body.AdvisoryEdges[2].Label = "Human-readable condition"
	if ConstraintHash(body) != hash {
		t.Fatal("display-only annotations invalidated business authority")
	}
	body.Nodes[0].Responsibility = "Changed formal responsibility"
	if ConstraintHash(body) == hash {
		t.Fatal("responsibility changes disappeared from the constraint hash")
	}
}
