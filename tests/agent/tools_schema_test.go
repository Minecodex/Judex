package agent_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/kakj-go/Judex/internal/agent/tools"
)

func TestRegisteredObjectSchemasHaveValidWireKeywordTypes(t *testing.T) {
	registry := tools.New()
	tools.RegisterDefaults(registry)
	for _, principal := range []string{"coordinator", "position"} {
		raw, err := json.Marshal(registry.SchemasForAuthority(principal, true))
		if err != nil {
			t.Fatal(err)
		}
		var schemas []struct {
			Name  string                     `json:"name"`
			Input map[string]json.RawMessage `json:"input_schema"`
		}
		if err := json.Unmarshal(raw, &schemas); err != nil {
			t.Fatal(err)
		}
		if len(schemas) == 0 {
			t.Fatalf("no schemas for %s", principal)
		}
		for _, tool := range schemas {
			var properties map[string]json.RawMessage
			if err := json.Unmarshal(tool.Input["properties"], &properties); err != nil || properties == nil {
				t.Fatalf("%s/%s properties must serialize as an object: %s", principal, tool.Name, tool.Input["properties"])
			}
			if required, present := tool.Input["required"]; present {
				var names []string
				if err := json.Unmarshal(required, &names); err != nil || names == nil {
					t.Fatalf("%s/%s required must be absent or an array: %s", principal, tool.Name, required)
				}
				for _, name := range names {
					if _, exists := properties[name]; !exists {
						t.Fatalf("%s/%s requires undeclared property %q", principal, tool.Name, name)
					}
				}
			}
		}
	}
}

func TestNoArgumentToolAcceptsEmptyObjectAndRejectsExtraFields(t *testing.T) {
	registry := tools.New()
	tools.RegisterDefaults(registry)
	calls := 0
	env := tools.Env{ListAgents: func(context.Context, string) ([]map[string]any, error) {
		calls++
		return nil, nil
	}}
	if _, err := registry.Execute(context.Background(), "list_agent", "coordinator", map[string]any{}, env); err != nil {
		t.Fatalf("empty arguments rejected: %v", err)
	}
	if _, err := registry.Execute(context.Background(), "list_agent", "coordinator", map[string]any{"unexpected": true}, env); err == nil {
		t.Fatal("unexpected argument bypassed schema validation")
	}
	if calls != 1 {
		t.Fatalf("unexpected arguments reached the executor: calls=%d", calls)
	}
}

func TestProposalKindIsValidatedBeforeDraftEffect(t *testing.T) {
	registry := tools.New()
	tools.RegisterDefaults(registry)
	calls := 0
	env := tools.Env{ProposeDraft: func(_ context.Context, _ string, args map[string]any) (string, error) {
		calls++
		if args["kind"] != "work_change" {
			t.Fatal("explicit work change kind lost before draft adapter")
		}
		return "11111111-1111-4111-8111-111111111111", nil
	}}
	args := map[string]any{"kind": "release_acceptance", "changes": []any{map[string]any{"operation": "update_scope", "targetType": "plan"}}}
	if _, err := registry.Execute(context.Background(), "propose_changes", "coordinator", args, env); err == nil || calls != 0 {
		t.Fatal("unsupported proposal kind reached draft effect")
	}
	args["kind"] = "work_change"
	result, err := registry.Execute(context.Background(), "propose_changes", "coordinator", args, env)
	if err != nil || calls != 1 || result.Data["kind"] != "work_change" || result.Data["state"] != "draft" {
		t.Fatalf("explicit kind did not remain draft-only: %+v %v", result, err)
	}
}

func TestWorkflowToolRejectsMissingHardTargetBeforeExecutor(t *testing.T) {
	registry := tools.New()
	tools.RegisterDefaults(registry)
	calls := 0
	env := tools.Env{CanManage: true, DraftWorkflow: func(context.Context, map[string]any) (map[string]any, error) { calls++; return nil, nil }}
	args := map[string]any{"body": map[string]any{"name": "Gate", "instructions": "Real target required", "nodes": []any{map[string]any{"id": "review", "name": "Review", "responsibility": "Check", "allowedPositionIds": []any{}, "defaultApprovalPolicy": "all"}}, "advisoryEdges": []any{}, "approvalPolicies": map[string]any{}, "hardRules": []any{map[string]any{"kind": "material_ready", "phase": "start"}}}}
	if _, err := registry.Execute(context.Background(), "draft_workflow", "coordinator", args, env); err == nil {
		t.Fatal("missing hard-rule target accepted")
	}
	if calls != 0 {
		t.Fatal("invalid hard rule reached draft executor")
	}
}
