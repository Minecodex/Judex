// SPDX-License-Identifier: Apache-2.0

// Package client is the shared public-API client used by the judex CLI
// (docs/plans/v1/07 §2-§6): envelope handling, bearer auth, idempotency
// keys, pagination and exit-code-friendly errors.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// CLIError carries the API error code for exit-code mapping (07 §6).
type CLIError struct {
	Status    int
	Code      string
	Message   string
	Retryable bool
}

func (e *CLIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

// ExitCode maps API errors to CLI exit codes (07 §6).
func (e *CLIError) ExitCode() int {
	switch {
	case e.Code == "PENDING_CONFIRMATION":
		return 6
	case e.Status == 0:
		return 7 // network
	case e.Status == 401 || e.Code == "UNAUTHENTICATED" || e.Code == "GRANT_REVOKED":
		return 3
	case e.Status == 403 || e.Code == "FORBIDDEN":
		return 4
	case e.Status == 409:
		return 5
	case e.Status == 412:
		return 8
	case e.Status >= 500:
		return 7
	default:
		return 1
	}
}

// Client talks to a Judex server.
type Client struct {
	OutboxDirectory string
	Server          string // base URL, e.g. https://judex.internal
	Token           string // short-lived bearer access token
	RefreshToken    string
	AccessExpiresAt time.Time
	TokenSource     func(context.Context) (string, error)
	HTTP            *http.Client
}

func New(server string) *Client {
	return &Client{Server: strings.TrimSuffix(server, "/"), HTTP: &http.Client{Timeout: 60 * time.Second}}
}

type envelope struct {
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		Retryable bool   `json:"retryable"`
	} `json:"error"`
}

// Do performs one API call; JSON bodies in, unwrapped data out. Write
// operations generate an idempotency key automatically unless given.
func (c *Client) Do(ctx context.Context, method, path string, body any, out any, idempotencyKey string) error {
	var rawBody []byte
	if body != nil {
		var err error
		rawBody, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	pendingFile := ""
	if method != "GET" && method != "HEAD" && !strings.HasPrefix(path, "/auth/") {
		if idempotencyKey == "" {
			idempotencyKey = NewKey()
		}
		command, file, err := c.prepareCommand(method, path, rawBody, idempotencyKey)
		if err != nil {
			return err
		}
		idempotencyKey, rawBody, pendingFile = command.Key, command.Body, file
	}
	var reader io.Reader
	if rawBody != nil {
		reader = bytes.NewReader(rawBody)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Server+"/api/v1"+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if err := c.authorize(ctx, req); err != nil {
		return err
	}
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return &CLIError{Status: 0, Code: "NETWORK", Message: err.Error(), Retryable: true}
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return &CLIError{Status: resp.StatusCode, Code: "NETWORK", Message: err.Error(), Retryable: true}
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return &CLIError{Status: resp.StatusCode, Code: "HTTP_ERROR", Message: strings.TrimSpace(string(raw))}
	}
	if resp.StatusCode >= 400 {
		if resp.StatusCode < 500 {
			retireCommand(pendingFile)
		}
		if env.Error != nil {
			return &CLIError{Status: resp.StatusCode, Code: env.Error.Code, Message: env.Error.Message, Retryable: env.Error.Retryable}
		}
		return &CLIError{Status: resp.StatusCode, Code: "HTTP_ERROR"}
	}
	if out != nil && len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return &CLIError{Status: resp.StatusCode, Code: "SCHEMA", Message: err.Error()}
		}
	}
	retireCommand(pendingFile)
	return nil
}

// NewKey returns a fresh idempotency key.
func NewKey() string { return uuid.NewString() }

// DeviceLogin runs the full device flow (07 §3): request codes, poll until
// approved, return the refresh token.
func (c *Client) DeviceLogin(ctx context.Context, deviceName string, scopes []string) (string, error) {
	var auth struct {
		DeviceCode      string `json:"deviceCode"`
		UserCode        string `json:"userCode"`
		VerificationUri string `json:"verificationUri"`
		ExpiresIn       int    `json:"expiresIn"`
		Interval        int    `json:"interval"`
	}
	if err := c.Do(ctx, "POST", "/auth/device/authorizations",
		map[string]any{"deviceName": deviceName, "requestedScopes": scopes}, &auth, ""); err != nil {
		return "", err
	}
	fmt.Fprintf(os.Stderr, "请在浏览器打开 %s 并输入代码：%s\n", c.Server+auth.VerificationUri, auth.UserCode)
	deadline := time.Now().Add(time.Duration(auth.ExpiresIn) * time.Second)
	interval := time.Duration(auth.Interval) * time.Second
	if interval < time.Second {
		interval = 5 * time.Second
	}
	for time.Now().Before(deadline) {
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", ctx.Err()
		case <-timer.C:
		}
		var result struct {
			Status string `json:"status"`
			TokenPair
		}
		err := c.Do(ctx, "POST", "/auth/device/token", map[string]any{"deviceCode": auth.DeviceCode}, &result, "")
		if err != nil {
			var ce *CLIError
			if errors.As(err, &ce) && (ce.Status == 401 || ce.Status == 403) {
				return "", err
			}
			continue
		}
		if result.RefreshToken != "" {
			c.Token = result.AccessToken
			c.RefreshToken = result.RefreshToken
			c.AccessExpiresAt = result.AccessExpiresAt
			c.TokenSource = nil
			return result.RefreshToken, nil
		}
		if result.Status == "slow_down" {
			interval += 5 * time.Second
		}
	}

	return "", &CLIError{Status: 401, Code: "EXPIRED", Message: "设备授权超时"}
}

// UploadPart streams one part to the project upload route.
func (c *Client) UploadPart(ctx context.Context, projectID, uploadID string, partNumber int, body []byte, sha string) error {
	req, err := http.NewRequestWithContext(ctx, "PUT",
		fmt.Sprintf("%s/api/v1/projects/%s/uploads/%s/parts/%d", c.Server, projectID, uploadID, partNumber),
		bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Judex-Part-SHA256", sha)
	if err := c.authorize(ctx, req); err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return &CLIError{Status: 0, Code: "NETWORK", Message: err.Error(), Retryable: true}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		var env envelope
		_ = json.Unmarshal(raw, &env)
		if env.Error != nil {
			return &CLIError{Status: resp.StatusCode, Code: env.Error.Code, Message: env.Error.Message}
		}
		return &CLIError{Status: resp.StatusCode, Code: "HTTP_ERROR", Message: strings.TrimSpace(string(raw))}
	}
	return nil
}

// Download streams a material version entry to w.
func (c *Client) Download(ctx context.Context, projectID, materialID, versionID, entry string, w io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/api/v1/projects/%s/materials/%s/versions/%s/content?entry=%s",
			c.Server, projectID, materialID, versionID, url.QueryEscape(entry)), nil)
	if err != nil {
		return err
	}
	if err := c.authorize(ctx, req); err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return &CLIError{Status: 0, Code: "NETWORK", Message: err.Error(), Retryable: true}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return &CLIError{Status: resp.StatusCode, Code: "HTTP_ERROR", Message: resp.Status}
	}
	_, err = io.Copy(w, resp.Body)
	return err
}
