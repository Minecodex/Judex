// SPDX-License-Identifier: Apache-2.0
package integrationtest_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/kakj-go/Judex/internal/app"
	"github.com/kakj-go/Judex/internal/config"
	integration "github.com/kakj-go/Judex/tests/integration"
)

// TestApplicationBootsAndShutsDown (A04/G01 basics): boot the real
// application against a fresh migrated PG on a random port, verify
// probes + system endpoint, then shut down gracefully.
func TestApplicationBootsAndShutsDown(t *testing.T) {
	fixture := integration.StartPG(t)

	// Pick a free local port for the test server.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	listener.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.Config{
		Environment: "test", HTTPAddress: addr, WebDirectory: "does-not-exist",
		ShutdownTimeout: 5 * time.Second, Mode: config.ModeAll,
		DatabaseURL: fixture.URL, WorkerCount: 1, LogLevel: "error",
	}
	application, err := app.New(cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	if err := application.RunMigrations(context.Background()); err != nil {
		t.Fatal(err)
	}
	workerCtx, cancelWorkers := context.WithCancel(context.Background())
	defer cancelWorkers()
	application.StartWorkers(workerCtx, logger)

	errCh := make(chan error, 1)
	go func() {
		errCh <- application.Server.ListenAndServe()
	}()
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := application.Close(shutdownCtx); err != nil {
			t.Errorf("graceful close: %v", err)
		}
	})

	base := "http://" + addr
	deadline := time.Now().Add(10 * time.Second)
	ready := false
	for time.Now().Before(deadline) {
		resp, err := http.Get(base + "/readyz")
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == 200 {
				ready = true
				break
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	if !ready {
		t.Fatal("application never became ready")
	}

	resp, err := http.Get(base + "/api/v1/system")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !contains(string(body), "Judex") {
		t.Fatalf("system endpoint: %d %s", resp.StatusCode, body)
	}
	// Identity is live: bad login body is a 400, not a 501.
	req, err := http.Post(base+"/api/v1/auth/login", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, req.Body)
	req.Body.Close()
	if req.StatusCode != 400 {
		t.Fatalf("login with empty body must be 400, got %d", req.StatusCode)
	}
	// Authenticated endpoints reject anonymous access with 401 now that the
	// module is live.
	resp, err = http.Get(base + "/api/v1/projects")
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("projects without a session must be 401, got %d", resp.StatusCode)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
