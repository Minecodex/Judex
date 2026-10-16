package agent_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kakj-go/Judex/internal/agent/tools"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

type failedPositionCaller struct{}

func (failedPositionCaller) CallPositionAgent(context.Context, string, string, string, string) (string, error) {
	return "", apierrors.New(apierrors.BudgetExhausted, "child analysis exhausted its budget")
}

func TestPositionFailureIsFailedToolAndPreservesCode(t *testing.T) {
	registry := tools.New()
	tools.RegisterCallAgent(registry, failedPositionCaller{})
	result, err := registry.Execute(context.Background(), "call_agent", "coordinator", map[string]any{"position": "actual-reviewer", "question": "Review the fixed original"}, tools.Env{ProjectID: "isolated-project"})
	var typed *apierrors.Error
	if err == nil || !errors.As(err, &typed) || typed.Code != apierrors.BudgetExhausted {
		t.Fatalf("position failure not preserved: %v", err)
	}
	if _, answered := result.Data["answer"]; answered {
		t.Fatal("failed role call exposed a completed answer")
	}
}
