// SPDX-License-Identifier: Apache-2.0

// Package opensandbox adapts the real OpenSandbox server API (verified against
// opensandbox/server v0.2.2, controller v0.2.0). Contract highlights:
//
//   - Auth: custom header OPEN-SANDBOX-API-KEY (not Bearer).
//   - POST /sandboxes {"image":{"uri":...},"entrypoint":[...],
//     "resourceLimits":{"cpu":"500m","memory":"256Mi"},"timeout":seconds}.
//     The controller injects bootstrap.sh + execd into every sandbox; the
//     entrypoint is ONLY the workload keep-alive command ("sleep infinity").
//   - The injected execd daemon listens on port 44772 and is reached through
//     the server proxy: POST /sandboxes/{id}/proxy/44772/command with
//     {"command": "..."} returning an NDJSON event stream:
//     init / stdout / stderr / execution_complete | error(evalue=exit code).
//   - GET /sandboxes/{id} → status.state Pending|Running|Failed|Succeed.
//   - DELETE /sandboxes/{id} kills it.
//
// Without configuration every call returns SANDBOX_UNAVAILABLE — never a
// silent success.
package opensandbox

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

// ExecdPort is the fixed port the injected execd daemon listens on inside
// every OpenSandbox sandbox.
const ExecdPort = 44772

// DefaultExecImage is the allowlist entry operators pin at deploy time.
const DefaultExecImage = "docker.io/library/alpine:3.20"

// Config mirrors the deployment Sandbox section (11 §2).
type Config struct {
	Endpoint string // OpenSandbox server root, e.g. http://osb-server:80
	APIKey   string
	Image    string // allowlist entry, pinned digest at deploy time
	CPU      string // K8s quantity, e.g. 500m
	Memory   string // K8s quantity, e.g. 256Mi
	HTTP     *http.Client
}

// SandboxResult reports command execution outcomes; unknown outcomes must be
// surfaced, not guessed.
type SandboxResult struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
	Unknown  bool // stream lost / server restarted mid-run
}

// Sandbox is the execution boundary (05 §7): one run one sandbox; no
// platform credentials inside.
type Sandbox interface {
	// Create starts a sandbox and waits until Running (or fails).
	Create(ctx context.Context, runID, projectID uuid.UUID) (externalID string, err error)
	// Exec runs one shell command; a lost stream marks Unknown=true so
	// callers do NOT blindly re-run side-effectful commands (05 §8).
	Exec(ctx context.Context, externalID, command string, timeout time.Duration) (SandboxResult, error)
	Kill(ctx context.Context, externalID string) error
}

type client struct {
	cfg Config
}

func New(cfg Config) Sandbox {
	if cfg.HTTP == nil {
		cfg.HTTP = &http.Client{Timeout: 5 * time.Minute}
	}
	if cfg.Image == "" {
		cfg.Image = DefaultExecImage
	}
	if cfg.CPU == "" {
		cfg.CPU = "500m"
	}
	if cfg.Memory == "" {
		cfg.Memory = "256Mi"
	}
	return &client{cfg: cfg}
}

func unavailable() error {
	return apierrors.New(apierrors.SandboxUnavailable, "沙箱未配置（JUDEX_OPENSANDBOX_ENDPOINT）").WithRetryable(true)
}

func (c *client) do(ctx context.Context, method, path string, body any) (int, []byte, error) {
	if c.cfg.Endpoint == "" {
		return 0, nil, unavailable()
	}
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method,
		strings.TrimSuffix(c.cfg.Endpoint, "/")+path, reader)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" {
		// Confirmed from the server's /openapi.json: custom header, not Bearer.
		req.Header.Set("OPEN-SANDBOX-API-KEY", c.cfg.APIKey)
	}
	resp, err := c.cfg.HTTP.Do(req)
	if err != nil {
		return 0, nil, apierrors.New(apierrors.SandboxUnavailable, "沙箱请求失败").WithRetryable(true).Wrap(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 400 {
		return resp.StatusCode, raw, apierrors.Newf(apierrors.SandboxUnavailable,
			"沙箱 %d: %s", resp.StatusCode, truncate(raw, 500))
	}
	return resp.StatusCode, raw, nil
}

func truncate(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n]) + "..."
	}
	return string(b)
}

type sandboxObject struct {
	ID         string `json:"id"`
	Image      any    `json:"image"`
	Entrypoint []string
	Status     struct {
		State   string `json:"state"`
		Reason  string `json:"reason"`
		Message string `json:"message"`
	} `json:"status"`
}

// Create starts one sandbox for a run and polls until Running (05 §7).
// The entrypoint is only the keep-alive workload; execd + bootstrap are
// injected by the controller regardless of image.
func (c *client) Create(ctx context.Context, runID, projectID uuid.UUID) (string, error) {
	body := map[string]any{
		"image":    map[string]any{"uri": c.cfg.Image},
		"entrypoint": []string{"sleep", "infinity"},
		"resourceLimits": map[string]any{
			"cpu": c.cfg.CPU, "memory": c.cfg.Memory,
		},
		"timeout": 1800,
		"labels": map[string]string{
			"app": "judex", "runId": runID.String(), "projectId": projectID.String(),
		},
	}
	_, raw, err := c.do(ctx, "POST", "/sandboxes", body)
	if err != nil {
		return "", err
	}
	var created sandboxObject
	if err := json.Unmarshal(raw, &created); err != nil || created.ID == "" {
		return "", apierrors.New(apierrors.SandboxUnavailable, "沙箱创建未返回 ID")
	}
	// Poll until Running; the sandbox pod needs a few seconds to schedule.
	deadline := time.Now().Add(2 * time.Minute)
	for {
		var obj sandboxObject
		_, raw, err := c.do(ctx, "GET", "/sandboxes/"+created.ID, nil)
		if err == nil {
			if json.Unmarshal(raw, &obj) == nil {
				switch obj.Status.State {
				case "Running":
					return obj.ID, nil
				case "Failed":
					return "", apierrors.Newf(apierrors.SandboxUnavailable,
						"沙箱启动失败: %s", obj.Status.Message)
				}
			}
		}
		if time.Now().After(deadline) {
			_ = c.Kill(context.WithoutCancel(ctx), created.ID)
			return "", apierrors.New(apierrors.SandboxUnavailable, "沙箱等待 Running 超时")
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

// execEvent is one NDJSON line from the execd command stream.
type execEvent struct {
	Type          string `json:"type"`
	Text          string `json:"text"`
	ExecutionTime int64  `json:"execution_time"`
	Error         *struct {
		EName    string   `json:"ename"`
		EValue   string   `json:"evalue"`
		Traceback []string `json:"traceback"`
	} `json:"error"`
}

// Exec runs one command through the sandbox proxy into the injected execd.
func (c *client) Exec(ctx context.Context, externalID, command string, timeout time.Duration) (SandboxResult, error) {
	if c.cfg.Endpoint == "" {
		return SandboxResult{Unknown: true}, unavailable()
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	path := fmt.Sprintf("/sandboxes/%s/proxy/%d/command", externalID, ExecdPort)
	req, err := http.NewRequestWithContext(ctx, "POST",
		strings.TrimSuffix(c.cfg.Endpoint, "/")+path,
		strings.NewReader(fmt.Sprintf(`{"command":%s}`, mustJSON(command))))
	if err != nil {
		return SandboxResult{Unknown: true}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" {
		req.Header.Set("OPEN-SANDBOX-API-KEY", c.cfg.APIKey)
	}
	resp, err := c.cfg.HTTP.Do(req)
	if err != nil {
		return SandboxResult{Unknown: true}, apierrors.New(apierrors.SandboxUnavailable,
			"沙箱命令请求失败").WithRetryable(true).Wrap(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return SandboxResult{Unknown: true}, apierrors.Newf(apierrors.SandboxUnavailable,
			"沙箱 %d: %s", resp.StatusCode, truncate(raw, 500))
	}

	var out SandboxResult
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	complete := false
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var ev execEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue // unknown event types must not break known ones
		}
		switch ev.Type {
		case "stdout":
			out.Stdout = append(out.Stdout, ev.Text...)
			out.Stdout = append(out.Stdout, '\n')
		case "stderr":
			out.Stderr = append(out.Stderr, ev.Text...)
			out.Stderr = append(out.Stderr, '\n')
		case "error":
			// evalue carries the exit code, e.g. "3".
			code := 1
			if n, err := strconv.Atoi(strings.TrimSpace(ev.Error.EValue)); err == nil {
				code = n
			}
			out.ExitCode = code
			if ev.Error.EName != "CommandExecError" {
				out.Stderr = append(out.Stderr, []byte(ev.Error.EName+": "+ev.Error.EValue+"\n")...)
			}
			complete = true
		case "execution_complete":
			complete = true
		}
	}
	if err := scanner.Err(); err != nil || !complete {
		// Stream lost before the terminal event: outcome unknown.
		out.Unknown = true
		if err == nil {
			err = apierrors.New(apierrors.SandboxUnavailable, "沙箱命令流未完成")
		}
		return out, err
	}
	return out, nil
}

func mustJSON(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

// ReadFile fetches a file's content byte-exact via base64 (execd has no
// file API; plain cat would lose/normalize trailing newlines). BusyBox
// base64 wraps output, so all whitespace is stripped before decoding.
func (c *client) ReadFile(ctx context.Context, externalID, path string) ([]byte, error) {
	res, err := c.Exec(ctx, externalID, "base64 -- "+shellQuote(path), 30*time.Second)
	if err != nil {
		return nil, err
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("读取失败 exit=%d: %s", res.ExitCode, truncate(res.Stderr, 200))
	}
	b64 := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' {
			return -1
		}
		return r
	}, string(res.Stdout))
	return base64.StdEncoding.DecodeString(b64)
}

// WriteFile stores content via base64 to survive arbitrary bytes.
func (c *client) WriteFile(ctx context.Context, externalID, path string, content []byte) error {
	b64 := base64.StdEncoding.EncodeToString(content)
	res, err := c.Exec(ctx, externalID,
		fmt.Sprintf("printf %%s %s | base64 -d > %s", shellQuote(b64), shellQuote(path)), 30*time.Second)
	if err != nil {
		return err
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("写入失败 exit=%d: %s", res.ExitCode, truncate(res.Stderr, 200))
	}
	return nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func (c *client) Kill(ctx context.Context, externalID string) error {
	_, _, err := c.do(ctx, "DELETE", "/sandboxes/"+externalID, nil)
	return err
}
