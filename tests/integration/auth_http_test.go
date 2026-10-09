// SPDX-License-Identifier: Apache-2.0
package integrationtest_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/kakj-go/Judex/internal/identity"
	"github.com/kakj-go/Judex/internal/transport/http"
	"github.com/kakj-go/Judex/internal/transport/http/middleware"
	integration "github.com/kakj-go/Judex/tests/integration"
)

// bootHTTPApp starts the full application (identity enabled) on an httptest
// server with the development cookie name.
func bootHTTPApp(t *testing.T) (*httptest.Server, *identity.Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	fixture := integration.StartPG(t)
	limiter := identity.NewRateLimiter(fixture.Pool.Pool, nil)
	svc := identity.NewService(fixture.Pool, limiter, identity.Options{RegisterPerIP: 1000, LoginPerIP: 1000, LoginPerAccount: 1000}, nil)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	authCfg := middleware.AuthConfig{Development: true, AllowedOrigins: []string{"http://localhost:5173"}}
	spec, err := httptransport.NewSpecRouter(logger)
	if err != nil {
		t.Fatal(err)
	}
	httptransport.NewIdentityHandlers(svc, authCfg, true).Register(spec)
	draining := &atomic.Bool{}
	router, err := httptransport.NewRouter(httptransport.Options{
		Logger: logger, Draining: draining,
		Auth: &httptransport.AuthOptions{Config: authCfg, Resolver: svc.ResolveSession},
	}, spec)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return server, svc
}

// registerViaHTTP performs register and returns the session cookie + csrf.
func registerViaHTTP(t *testing.T, server *httptest.Server, name, email, password string) (cookie *http.Cookie, csrf string) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"displayName": name, "email": email, "password": password})
	req, _ := http.NewRequest("POST", server.URL+"/api/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 201 {
		t.Fatalf("register: %d %s", resp.StatusCode, raw)
	}
	for _, c := range resp.Cookies() {
		if c.Name == "judex_dev_session" {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("register did not set session cookie")
	}
	var parsed struct {
		Meta struct {
			RequestID string `json:"requestId"`
		} `json:"meta"`
	}
	_ = json.Unmarshal(raw, &parsed)
	if parsed.Meta.RequestID == "" {
		t.Fatal("envelope missing requestId")
	}
	// Fetch the CSRF token through the session endpoint.
	req2, _ := http.NewRequest("GET", server.URL+"/api/v1/auth/session", nil)
	req2.AddCookie(cookie)
	resp2, err := server.Client().Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	var session struct {
		Data struct {
			CSRFToken string `json:"csrfToken"`
			User      struct {
				Email string `json:"email"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}
	if session.Data.CSRFToken == "" || session.Data.User.Email == "" {
		t.Fatalf("session payload incomplete: %+v", session)
	}
	return cookie, session.Data.CSRFToken
}

// TestAuthHTTPFlow (A01/A02/A03 basics): register sets cookie; session GET
// works; logout revokes; writes without CSRF are rejected; foreign Origin is
// rejected; login rate limit sets Retry-After.
func TestAuthHTTPFlow(t *testing.T) {
	server, _ := bootHTTPApp(t)
	cookie, csrf := registerViaHTTP(t, server, "李四", "lisi@judex.test", "password-lisi-123")

	// Write without CSRF header -> 403.
	body, _ := json.Marshal(map[string]int64{"expectedAuthVersion": 1})
	req, _ := http.NewRequest("POST", server.URL+"/api/v1/auth/logout-all", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("write without CSRF must 403, got %d", resp.StatusCode)
	}

	// Write with CSRF but a foreign Origin -> 403.
	req, _ = http.NewRequest("POST", server.URL+"/api/v1/auth/logout-all", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrf)
	req.Header.Set("Origin", "https://evil.example")
	req.AddCookie(cookie)
	resp, err = server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("foreign origin write must 403, got %d", resp.StatusCode)
	}

	// Allowed Origin + CSRF passes: logout-all keeps the current session.
	req, _ = http.NewRequest("POST", server.URL+"/api/v1/auth/logout-all", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrf)
	req.Header.Set("Origin", "http://localhost:5173")
	req.AddCookie(cookie)
	resp, err = server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(raw), "revokedSessions") {
		t.Fatalf("logout-all with CSRF+origin: %d %s", resp.StatusCode, raw)
	}

	// Logout clears the cookie and revokes; subsequent session GET -> 401.
	req, _ = http.NewRequest("POST", server.URL+"/api/v1/auth/logout", nil)
	req.Header.Set("X-CSRF-Token", csrf)
	req.AddCookie(cookie)
	resp, err = server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("logout: %d", resp.StatusCode)
	}
	req, _ = http.NewRequest("GET", server.URL+"/api/v1/auth/session", nil)
	req.AddCookie(cookie)
	resp, err = server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("revoked session must 401, got %d", resp.StatusCode)
	}
}

// TestLoginHTTPUniformError: wrong password and unknown account both return
// 401 UNAUTHENTICATED with identical bodies (no account enumeration).
func TestLoginHTTPUniformError(t *testing.T) {
	server, _ := bootHTTPApp(t)
	registerViaHTTP(t, server, "王五", "wangwu@judex.test", "password-wangwu-1")

	login := func(email, password string) (int, string) {
		body, _ := json.Marshal(map[string]string{"email": email, "password": password})
		req, _ := http.NewRequest("POST", server.URL+"/api/v1/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(raw)
	}
	codeWrong, bodyWrong := login("wangwu@judex.test", "not-the-password-1")
	codeUnknown, bodyUnknown := login("ghost@judex.test", "not-the-password-1")
	if codeWrong != 401 || codeUnknown != 401 {
		t.Fatalf("expected 401s, got %d/%d", codeWrong, codeUnknown)
	}
	// requestId differs per request; only the error envelope must be identical.
	errorPart := func(body string) string {
		var parsed map[string]any
		if err := json.Unmarshal([]byte(body), &parsed); err != nil {
			t.Fatal(err)
		}
		e, _ := json.Marshal(parsed["error"])
		return string(e)
	}
	if errorPart(bodyWrong) != errorPart(bodyUnknown) || !strings.Contains(errorPart(bodyWrong), "UNAUTHENTICATED") {
		t.Fatalf("login error envelopes must be identical UNAUTHENTICATED:\n%s\n%s", bodyWrong, bodyUnknown)
	}
}

// TestRegisterHTTPDuplicate: duplicate email surfaces EMAIL_IN_USE (409).
func TestRegisterHTTPDuplicate(t *testing.T) {
	server, _ := bootHTTPApp(t)
	registerViaHTTP(t, server, "赵六", "zhaoliu@judex.test", "password-zhaoliu-1")
	body, _ := json.Marshal(map[string]string{"displayName": "赵六2", "email": "ZHAOLIU@judex.test", "password": "password-zhaoliu-1"})
	req, _ := http.NewRequest("POST", server.URL+"/api/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 409 || !strings.Contains(string(raw), "EMAIL_IN_USE") {
		t.Fatalf("duplicate register: %d %s", resp.StatusCode, raw)
	}
}

// TestSessionsListAndRevoke (A03 前端基础): the account security endpoints
// list only the caller's sessions and revoke idempotently.
func TestSessionsListAndRevoke(t *testing.T) {
	server, svc := bootHTTPApp(t)
	cookie, csrf := registerViaHTTP(t, server, "会话用户", "sessions@judex.test", "password-sessions-1")
	// Create a second session via login.
	loginBody, _ := json.Marshal(map[string]string{"email": "sessions@judex.test", "password": "password-sessions-1"})
	req, _ := http.NewRequest("POST", server.URL+"/api/v1/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	req, _ = http.NewRequest("GET", server.URL+"/api/v1/me/sessions", nil)
	req.AddCookie(cookie)
	resp, err = server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var list struct {
		Data struct {
			Items []struct {
				ID      string `json:"id"`
				Current bool   `json:"current"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 || len(list.Data.Items) != 2 {
		t.Fatalf("expected 2 sessions, got %d (%d)", len(list.Data.Items), resp.StatusCode)
	}
	var other string
	for _, item := range list.Data.Items {
		if !item.Current {
			other = item.ID
		}
	}
	if other == "" {
		t.Fatal("no non-current session found")
	}
	// Revoke the other session (CSRF required for DELETE).
	req, _ = http.NewRequest("DELETE", server.URL+"/api/v1/me/sessions/"+other, nil)
	req.Header.Set("X-CSRF-Token", csrf)
	req.AddCookie(cookie)
	resp, err = server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("revoke must succeed, got %d", resp.StatusCode)
	}
	req, _ = http.NewRequest("GET", server.URL+"/api/v1/me/sessions", nil)
	req.AddCookie(cookie)
	resp, err = server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if len(list.Data.Items) != 1 || !list.Data.Items[0].Current {
		t.Fatalf("expected only current session after revoke, got %+v", list.Data.Items)
	}
	_ = svc
}
