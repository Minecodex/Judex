// SPDX-License-Identifier: Apache-2.0

package opensandbox

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/kakj-go/Judex/internal/platform/errors"
)

// RunSandbox lazily provisions ONE sandbox per agent run (05 §7): nothing is
// created until the model actually invokes a sandbox tool, and the same
// sandbox serves every tool call of that run. Close kills the sandbox.
type RunSandbox struct {
	Client    Sandbox
	RunID     uuid.UUID
	ProjectID uuid.UUID

	once      sync.Once
	external  string
	createErr error
}

// Exec implements tools.SandboxExec (int/os timeout in milliseconds).
func (r *RunSandbox) Exec(ctx context.Context, command string, timeoutMs int) (exit int, stdout, stderr []byte, unknown bool, err error) {
	r.once.Do(func() {
		r.external, r.createErr = r.Client.Create(ctx, r.RunID, r.ProjectID)
	})
	if r.createErr != nil {
		return 0, nil, nil, true, r.createErr
	}
	res, err := r.Client.Exec(ctx, r.external, command, time.Duration(timeoutMs)*time.Millisecond)
	return res.ExitCode, res.Stdout, res.Stderr, res.Unknown, err
}

// Close kills the sandbox if one was created; safe to call when nothing
// was provisioned. Best effort — expiry reaps stragglers.
func (r *RunSandbox) Close(ctx context.Context) {
	if r.external != "" {
		_ = r.Client.Kill(ctx, r.external)
	}
}

// ExternalID reports the provisioned sandbox ID ("" when none was created).
func (r *RunSandbox) ExternalID() string { return r.external }

// Unavailable is a RunSandbox that fails every call — used when the
// deployment has no sandbox configured so tool output stays honest.
func Unavailable() *RunSandbox {
	return &RunSandbox{Client: unavailableClient{}}
}

type unavailableClient struct{}

func (unavailableClient) Create(context.Context, uuid.UUID, uuid.UUID) (string, error) {
	return "", errors.New(errors.SandboxUnavailable, "沙箱未配置")
}
func (unavailableClient) Exec(context.Context, string, string, time.Duration) (SandboxResult, error) {
	return SandboxResult{Unknown: true}, errors.New(errors.SandboxUnavailable, "沙箱未配置")
}
func (unavailableClient) Kill(context.Context, string) error { return nil }
