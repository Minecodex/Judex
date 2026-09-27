// SPDX-License-Identifier: Apache-2.0

// Package integrationtest provides disposable real-PostgreSQL fixtures via
// Docker (docs/plans/v1/10 §1). One postgres container per test PROCESS is
// started lazily; every test receives its own database inside it, so tests
// stay fully isolated while container churn stays at one per suite run
// (repeated container starts were destabilizing the local Docker daemon).
package integrationtest

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
)

// PGFixture is one per-test database on the shared container.
type PGFixture struct {
	Container string
	Database  string
	URL       string
	Pool      *postgres.Pool
}

const (
	containerName = "judex-it-pg"
	containerPass = "judex-test"
)

var (
	once      sync.Once
	container struct {
		port string
		err  error
	}
	adminPool *postgres.Pool
)

func tryRun(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// startContainer boots the shared postgres container once per process.
func startContainer() (string, error) {
	// Reuse an existing healthy container from a crashed run if present.
	if out, err := tryRun("docker", "inspect", containerName, "--format",
		"{{(index (index .NetworkSettings.Ports \"5432/tcp\") 0).HostPort}}"); err == nil {
		if port := strings.TrimSpace(out); port != "" {
			return port, nil
		}
	}
	if _, err := tryRun("docker", "rm", "-f", containerName); err != nil {
		// Non-fatal: the container may not exist.
		_ = err
	}
	if out, err := tryRun("docker", "run", "-d", "--rm", "--name", containerName,
		"-e", "POSTGRES_PASSWORD="+containerPass,
		"-e", "POSTGRES_DB=judex",
		"-p", "127.0.0.1::5432",
		"postgres:17-alpine"); err != nil {
		return "", fmt.Errorf("docker run: %v\n%s", err, out)
	}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		out, err := tryRun("docker", "inspect", containerName, "--format",
			"{{(index (index .NetworkSettings.Ports \"5432/tcp\") 0).HostPort}}")
		if err == nil {
			if port := strings.TrimSpace(out); port != "" {
				return port, nil
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	return "", fmt.Errorf("postgres container did not expose a port")
}

// StartPG gives the test a fresh database on the shared container, fully
// migrated. It skips (not fails) when Docker is unavailable.
func StartPG(t *testing.T) *PGFixture {
	t.Helper()
	if out, err := tryRun("docker", "info"); err != nil {
		t.Skipf("docker unavailable: %v\n%s", err, out)
	}
	once.Do(func() {
		container.port, container.err = startContainer()
	})
	if container.err != nil {
		t.Fatalf("shared postgres container: %v", container.err)
	}
	host := "127.0.0.1"
	_ = runtime.GOOS

	// Wait for TCP readiness (first test in the process may race startup).
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, container.port), time.Second)
		if err == nil {
			conn.Close()
			break
		}
		time.Sleep(400 * time.Millisecond)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	adminURL := fmt.Sprintf("postgres://postgres:%s@%s:%s/postgres?sslmode=disable", containerPass, host, container.port)
	if adminPool == nil {
		for time.Now().Before(deadline) {
			p, err := postgres.Open(ctx, postgres.Options{URL: adminURL, MaxConns: 2}, nil)
			if err == nil {
				adminPool = p
				break
			}
			time.Sleep(600 * time.Millisecond)
		}
		if adminPool == nil {
			t.Fatal("postgres did not become ready in time")
		}
	}

	database := fmt.Sprintf("judex_it_%d", time.Now().UnixNano())
	if _, err := adminPool.Exec(ctx, `CREATE DATABASE `+database); err != nil {
		t.Fatalf("create database: %v", err)
	}
	url := fmt.Sprintf("postgres://postgres:%s@%s:%s/%s?sslmode=disable", containerPass, host, container.port, database)
	pool, err := postgres.Open(ctx, postgres.Options{URL: url, MaxConns: 4}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		// Drop the per-test database; terminate any stragglers first.
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer dropCancel()
		_, _ = adminPool.Exec(dropCtx, fmt.Sprintf(
			`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='%s' AND pid<>pg_backend_pid()`, database))
		_, _ = adminPool.Exec(dropCtx, `DROP DATABASE IF EXISTS `+database)
	})
	if err := pool.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return &PGFixture{Container: containerName, Database: database, URL: url, Pool: pool}
}

// ShutdownContainer removes the shared container; call from TestMain after
// all tests finished (tests/integration/main_test.go).
func ShutdownContainer() {
	if adminPool != nil {
		adminPool.Close()
		adminPool = nil
	}
	_, _ = tryRun("docker", "rm", "-f", containerName)
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

var _ = os.Getenv
