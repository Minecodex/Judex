package agent_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/agent/tools"
)

func TestWorkflowReadIsPublicReadAndValidatesObjectIDs(t *testing.T) {
	r := tools.New()
	tools.RegisterDefaults(r)
	definition, version := uuid.NewString(), uuid.NewString()
	calls := 0
	env := tools.Env{ReadWorkflow: func(_ context.Context, d, v string) (map[string]any, error) {
		calls++
		if d != definition || v != version {
			t.Fatal("object IDs changed")
		}
		return map[string]any{"isCurrent": false, "bodySha256": "hash"}, nil
	}}
	for _, principal := range []string{"coordinator", "position"} {
		result, err := r.Execute(context.Background(), "read_workflow", principal, map[string]any{"workflowId": definition, "versionId": version}, env)
		if err != nil || result.Data["isCurrent"] != false {
			t.Fatalf("public history: %+v %v", result, err)
		}
	}
	for _, args := range []map[string]any{{"versionId": version}, {"workflowId": "not-uuid", "versionId": version}, {"workflowId": definition, "versionId": version, "publish": true}} {
		if _, err := r.Execute(context.Background(), "read_workflow", "coordinator", args, env); err == nil {
			t.Fatal("invalid arguments reached reader")
		}
	}
	if calls != 2 {
		t.Fatalf("unexpected reader calls %d", calls)
	}
}
