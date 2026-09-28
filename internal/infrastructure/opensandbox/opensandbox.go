// SPDX-License-Identifier: Apache-2.0

// Package opensandbox adapts the locked OpenSandbox lifecycle API per
// docs/plans/v1/05 §7. When the real lifecycle endpoint is configured the
// HTTP client implements Sandbox; without configuration every call returns
// SANDBOX_UNAVAILABLE — never a silent success.
package opensandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

// Config mirrors the deployment Sandbox section (11 §2).
type Config struct {
	LifecycleEndpoint string // e.g. http://judex-opensandbox-server/execd/api
	APIKey            string
	Namespace         string
	Image             string // allowlist entry, pinned digest at deploy time
	ProjectMount      string // read-only mount source for /workspace/project
	HTTP              *http.Client
}

// SandboxResult reports command execution outcomes; unknown outcomes must be
// surfaced, not guessed.
type SandboxResult struct {
	ExitCode   int
	Stdout     []byte
	Stderr     []byte
	Unknown    bool // stream lost / server restarted mid-run
}

// Sandbox is the execution boundary (05 §7): one run one sandbox; project
// mount is read-only; no platform credentials inside.
type Sandbox interface {
	Create(ctx context.Context, runID uuid.UUID, projectID uuid.UUID) (externalID string, err error)
	Exec(ctx context.Context, externalID, command string, timeout time.Duration) (SandboxResult, error)
	Read(ctx context.Context, externalID, path string) ([]byte, error)
	Write(ctx context.Context, externalID, path string, content []byte) error
	Kill(ctx context.Context, externalID string) error
}

type client struct {
	cfg Config
}

func New(cfg Config) Sandbox {
	if cfg.HTTP == nil {
		cfg.HTTP = &http.Client{Timeout: 5 * time.Minute}
	}
	return &client{cfg: cfg}
}

func unavailable() error {
	return apierrors.New(apierrors.SandboxUnavailable, "沙箱未配置（JUDEX_SANDBOX_ENDPOINT）").WithRetryable(true)
}

func (c *client) call(ctx context.Context, method, path string, body any, out any) error {
	if c.cfg.LifecycleEndpoint == "" {
		return unavailable()
	}
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method,
		strings.TrimSuffix(c.cfg.LifecycleEndpoint, "/")+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}
	resp, err := c.cfg.HTTP.Do(req)
	if err != nil {
		return apierrors.New(apierrors.SandboxUnavailable, "沙箱请求失败").WithRetryable(true).Wrap(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 400 {
		return apierrors.Newf(apierrors.SandboxUnavailable, "沙箱 %d: %s", resp.StatusCode, truncate(raw, 500))
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return apierrors.New(apierrors.Internal, "沙箱响应解析失败").Wrap(err)
		}
	}
	return nil
}

func truncate(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n]) + "..."
	}
	return string(b)
}

// Create starts one sandbox for a run with the project mounted read-only at
// /workspace/project (05 §7 挂载/权限由基础设施强制).
func (c *client) Create(ctx context.Context, runID, projectID uuid.UUID) (string, error) {
	var out struct {
		ID string `json:"id"`
	}
	mount := map[string]any{}
	if c.cfg.ProjectMount != "" {
		mount["project"] = map[string]any{
			"source": c.cfg.ProjectMount, "destination": "/workspace/project", "readOnly": true,
		}
	}
	body := map[string]any{
		"labels": map[string]string{
			"app": "judex", "runId": runID.String(), "projectId": projectID.String(),
		},
		"mounts": mount,
	}
	if c.cfg.Image != "" {
		body["image"] = c.cfg.Image
	}
	if err := c.call(ctx, "POST", "/sandboxes", body, &out); err != nil {
		return "", err
	}
	if out.ID == "" {
		return "", apierrors.New(apierrors.SandboxUnavailable, "沙箱创建未返回 ID")
	}
	return out.ID, nil
}

// Exec runs a command; a lost stream marks Unknown=true so callers do NOT
// blindly re-run side-effectful commands (05 §8).
func (c *client) Exec(ctx context.Context, externalID, command string, timeout time.Duration) (SandboxResult, error) {
	var out struct {
		ExitCode int    `json:"exitCode"`
		Stdout   string `json:"stdout"`
		Stderr   string `json:"stderr"`
		Unknown  bool   `json:"unknown"`
	}
	if err := c.call(ctx, "POST", fmt.Sprintf("/sandboxes/%s/exec", externalID), map[string]any{
		"command": command, "timeoutSeconds": int(timeout.Seconds()),
	}, &out); err != nil {
		return SandboxResult{Unknown: true}, err
	}
	return SandboxResult{ExitCode: out.ExitCode, Stdout: []byte(out.Stdout), Stderr: []byte(out.Stderr), Unknown: out.Unknown}, nil
}

func (c *client) Read(ctx context.Context, externalID, path string) ([]byte, error) {
	var out struct {
		Content string `json:"content"`
	}
	if err := c.call(ctx, "GET", fmt.Sprintf("/sandboxes/%s/files?path=%s", externalID, path), nil, &out); err != nil {
		return nil, err
	}
	return []byte(out.Content), nil
}

func (c *client) Write(ctx context.Context, externalID, path string, content []byte) error {
	return c.call(ctx, "PUT", fmt.Sprintf("/sandboxes/%s/files", externalID), map[string]any{
		"path": path, "content": string(content),
	}, nil)
}

func (c *client) Kill(ctx context.Context, externalID string) error {
	if c.cfg.LifecycleEndpoint == "" {
		return unavailable()
	}
	return c.call(ctx, "DELETE", fmt.Sprintf("/sandboxes/%s", externalID), nil, nil)
}
