// SPDX-License-Identifier: Apache-2.0

// Package integrationtest provides disposable real-PostgreSQL fixtures via
// Docker (docs/plans/v1/10 §1). Each test gets its own container on a random
// port, fully migrated, removed on cleanup — no shared state between tests.
package integrationtest

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
)

// PGFixture is one throwaway postgres container.
type PGFixture struct {
	Container string
	URL       string
	Pool      *postgres.Pool
}

func run(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v failed: %v\n%s", args, err, out)
	}
	return string(out)
}

func tryRun(args ...string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
// StartPG boots postgres:17-alpine on a random host port and migrates it.
// It skips (not fails) when Docker is unavailable so unit-only runs work.
func StartPG(t *testing.T) *PGFixture {
	t.Helper()
	if out, err := tryRun("docker", "info"); err != nil {
		t.Skipf("docker unavailable: %v\n%s", err, out)
	}
	name := fmt.Sprintf("judex-test-pg-%d", time.Now().UnixNano())
	password := "judex-test"
	run(t, "docker", "run", "-d", "--rm", "--name", name,
		"-e", "POSTGRES_PASSWORD="+password,
		"-e", "POSTGRES_DB=judex",
		"-p", "127.0.0.1::5432",
		"postgres:17-alpine")
	t.Cleanup(func() {
		_, _ = tryRun("docker", "rm", "-f", name)
	})

	var port string
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		out, err := tryRun("docker", "inspect", name,
			"--format", "{{(index (index .NetworkSettings.Ports \"5432/tcp\") 0).HostPort}}")
		if err == nil {
			port = strings.TrimSpace(out)
			if port != "" {
				break
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	if port == "" {
		t.Fatal("could not discover postgres host port")
	}
	host := "127.0.0.1"
	if runtime.GOOS == "windows" {
		host = "127.0.0.1"
	}
	url := fmt.Sprintf("postgres://postgres:%s@%s:%s/judex?sslmode=disable", password, host, port)

	// Wait for readiness at the TCP level first, then via the pool ping.
	deadline = time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), time.Second)
		if err == nil {
			conn.Close()
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var pool *postgres.Pool
	for time.Now().Before(deadline) {
		p, err := postgres.Open(ctx, postgres.Options{URL: url, MaxConns: 4}, nil)
		if err == nil {
			pool = p
			break
		}
		time.Sleep(700 * time.Millisecond)
	}
	if pool == nil {
		t.Fatal("postgres did not become ready in time")
	}
	t.Cleanup(pool.Close)
	if err := pool.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return &PGFixture{Container: name, URL: url, Pool: pool}
}

// Port returns the host port integer of the fixture (helper for extra pools).
func (f *PGFixture) Port(t *testing.T) int {
	t.Helper()
	p, err := strconv.Atoi(strings.Split(strings.Split(f.URL, ":")[3], "/")[0])
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// ProjectPath resolves a repo-relative directory from this test file.
func ProjectPath(t *testing.T, elem ...string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test source path")
	}
	base := filepath.Dir(filepath.Dir(filepath.Dir(thisFile))) // tests/integration -> repo root
	return filepath.Join(append([]string{base}, elem...)...)
}
