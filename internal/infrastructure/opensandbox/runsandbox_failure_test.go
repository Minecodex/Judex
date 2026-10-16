package opensandbox

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
)

type noDispatchSandbox struct {
	createError                  error
	execUnknown                  bool
	executions, creations, kills int
}

func (s *noDispatchSandbox) Create(context.Context, uuid.UUID, uuid.UUID) (string, error) {
	s.creations++
	if s.createError != nil {
		return "", s.createError
	}
	return "created-owned-sandbox", nil
}
func (s *noDispatchSandbox) Exec(context.Context, string, string, time.Duration) (SandboxResult, error) {
	s.executions++
	return SandboxResult{Unknown: s.execUnknown}, nil
}
func (s *noDispatchSandbox) Kill(context.Context, string) error { s.kills++; return nil }

func TestSandboxSetupFailureNeverClaimsUnknownCommandExecution(t *testing.T) {
	for _, phase := range []string{"unconfigured", "before", "create", "after"} {
		t.Run(phase, func(t *testing.T) {
			client := &noDispatchSandbox{}
			rs := &RunSandbox{Client: client}
			switch phase {
			case "unconfigured":
				rs = Unavailable()
			case "before":
				rs.BeforeCreate = func(context.Context) error { return fmt.Errorf("authority changed before create") }
			case "create":
				client.createError = fmt.Errorf("provisioning failed")
			case "after":
				rs.AfterCreate = func(context.Context, string) error { return fmt.Errorf("lease persistence failed") }
			}
			_, _, _, unknown, err := rs.Exec(context.Background(), "echo must-not-execute", 30000)
			if err == nil || unknown || client.executions != 0 {
				t.Fatalf("setup failure became unknown command: unknown=%v executions=%d err=%v", unknown, client.executions, err)
			}
			rs.Close(context.Background())
			if phase == "after" && client.kills != 1 {
				t.Fatal("created sandbox was not cleaned after lease hook failure")
			}
		})
	}
}

func TestDispatchedSandboxExecutionStillPreservesUnknownOutcome(t *testing.T) {
	client := &noDispatchSandbox{execUnknown: true}
	rs := &RunSandbox{Client: client}
	_, _, _, unknown, err := rs.Exec(context.Background(), "controlled command", 30000)
	if err != nil || !unknown || client.executions != 1 {
		t.Fatalf("real uncertain execution lost protection: unknown=%v err=%v", unknown, err)
	}
	rs.Close(context.Background())
}
