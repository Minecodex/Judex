// SPDX-License-Identifier: Apache-2.0
package backend_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
	"github.com/kakj-go/Judex/internal/config"
	transport "github.com/kakj-go/Judex/internal/transport/http"
)

func newTestRouter(t *testing.T, draining *atomic.Bool) *gin.Engine {
	t.Helper()
	router, err := transport.NewRouter(transport.Options{
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Draining: draining,
		Assets: fstest.MapFS{
			"index.html":                       {Data: []byte("<!doctype html><title>Judex</title>")},
			"assets/app.js":                    {Data: []byte("export const ready=true")},
			"downloads/judex-windows-test.zip": {Data: []byte("PK\x03\x04download bytes")},
			"downloads/manifest.json":          {Data: []byte(`{"version":"test"}`)},
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return router
}

func TestClientDownloadsAreAttachmentsAndMissingFilesStay404(t *testing.T) {
	router := newTestRouter(t, &atomic.Bool{})
	for _, method := range []string{"GET", "HEAD"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(method, "/downloads/judex-windows-test.zip", nil))
		if response.Code != 200 || response.Header().Get("Content-Disposition") != `attachment; filename="judex-windows-test.zip"` || response.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("download response: %d %v", response.Code, response.Header())
		}
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("GET", "/downloads/missing.zip", nil))
	if response.Code != 404 || strings.Contains(response.Body.String(), "<!doctype") {
		t.Fatal("download miss returned the SPA")
	}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("GET", "/downloads/missing", nil))
	if response.Code != 404 {
		t.Fatal("download directory miss returned the SPA")
	}
}

func TestRoutingAndScaffoldBoundaries(t *testing.T) {
	draining := &atomic.Bool{}
	router := newTestRouter(t, draining)
	cases := []struct {
		method, path string
		status       int
		contains     string
	}{
		{"GET", "/healthz", 200, "ok"}, {"GET", "/readyz", 200, "\"scope\":\"http\""},
		{"GET", "/api/v1/system", 200, "Judex"},
		{"GET", "/api/v1/workspace", 404, "NOT_FOUND"},
		{"POST", "/api/v1/auth/register", 501, "NOT_IMPLEMENTED"}, {"POST", "/api/v1/auth/login", 501, "NOT_IMPLEMENTED"},
		{"GET", "/api/v1/projects", 401, "UNAUTHENTICATED"},
		{"GET", "/api/v1/missing", 404, "NOT_FOUND"}, {"GET", "/assets/missing.js", 404, "NOT_FOUND"},
		{"GET", "/projects/example", 200, "<title>Judex"}, {"GET", "/assets/app.js", 200, "export const"},
		{"POST", "/healthz", 405, "METHOD_NOT_ALLOWED"},
	}
	for _, tc := range cases {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			r := httptest.NewRecorder()
			router.ServeHTTP(r, httptest.NewRequest(tc.method, tc.path, nil))
			if r.Code != tc.status || !strings.Contains(r.Body.String(), tc.contains) {
				t.Fatalf("status=%d body=%s", r.Code, r.Body)
			}
			if r.Header().Get("X-Request-ID") == "" {
				t.Fatal("missing request id")
			}
			if strings.HasPrefix(tc.path, "/api/") {
				var parsed map[string]any
				if err := json.Unmarshal(r.Body.Bytes(), &parsed); err != nil {
					t.Fatal("API returned non-JSON", err)
				}
			}
		})
	}
	draining.Store(true)
	r := httptest.NewRecorder()
	router.ServeHTTP(r, httptest.NewRequest("GET", "/readyz", nil))
	if r.Code != 503 {
		t.Fatal("draining server is still ready")
	}
}
func TestAPIWithoutWebBuild(t *testing.T) {
	r := httptest.NewRecorder()
	router, err := transport.NewRouter(transport.Options{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	router.ServeHTTP(r, httptest.NewRequest("GET", "/", nil))
	if r.Code != 404 || !strings.Contains(r.Body.String(), "WEB_BUILD_MISSING") {
		t.Fatal(r.Body.String())
	}
}
func TestConfiguration(t *testing.T) {
	apiOnly := func(k string) string {
		if k == "JUDEX_MODE" {
			return "api"
		}
		return ""
	}
	c, err := config.FromEnv(apiOnly)
	if err != nil || c.HTTPAddress != "127.0.0.1:8080" || c.Mode != config.ModeAPI {
		t.Fatal(c, err)
	}
	invalid := map[string]string{
		"JUDEX_ENV": "invalid", "JUDEX_HTTP_ADDR": "bad", "JUDEX_SHUTDOWN_TIMEOUT": "0s",
		"JUDEX_MODE": "bogus", "JUDEX_WORKER_COUNT": "0",
	}
	for key, value := range invalid {
		_, err := config.FromEnv(func(k string) string {
			if k == "JUDEX_MODE" && key != "JUDEX_MODE" {
				return "api"
			}
			if k == key {
				return value
			}
			return ""
		})
		if err == nil {
			t.Fatal("invalid configuration accepted", key)
		}
	}
	// Persistence modes require a database URL; production always does.
	for _, mode := range []string{"all", "worker", "migrate"} {
		_, err := config.FromEnv(func(k string) string {
			if k == "JUDEX_MODE" {
				return mode
			}
			return ""
		})
		if err == nil {
			t.Fatal("mode without database accepted", mode)
		}
	}
	_, err = config.FromEnv(func(k string) string {
		switch k {
		case "JUDEX_MODE":
			return "api"
		case "JUDEX_ENV":
			return "production"
		}
		return ""
	})
	if err == nil {
		t.Fatal("production without database accepted")
	}
}
