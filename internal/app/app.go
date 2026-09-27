// SPDX-License-Identifier: Apache-2.0

// Package app assembles runtime resources: HTTP transport, PostgreSQL pool,
// job engine workers, graceful shutdown. Entry points stay thin
// (docs/plans/v1/01 §1).
package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/kakj-go/Judex/internal/config"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	"github.com/kakj-go/Judex/internal/job"
	httptransport "github.com/kakj-go/Judex/internal/transport/http"
)

type Application struct {
	Server   *http.Server
	Draining *atomic.Bool
	Config   config.Config

	pool     *postgres.Pool
	engine   *job.Engine
	root     *os.Root
	workers  []context.CancelFunc
}

// New builds the application per config mode. When persistence is enabled it
// opens the pool immediately (fail fast on bad DSN).
func New(cfg config.Config, logger *slog.Logger) (*Application, error) {
	if logger == nil {
		logger = slog.Default()
	}
	var assets fs.FS
	root, err := os.OpenRoot(cfg.WebDirectory)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if root != nil {
		assets = root.FS()
	} else {
		logger.Info("web build absent; API-only development mode")
	}
	draining := &atomic.Bool{}

	app := &Application{Draining: draining, Config: cfg, root: root}

	if cfg.NeedsDatabase() {
		pool, err := postgres.Open(context.Background(), postgres.Options{URL: cfg.DatabaseURL}, logger)
		if err != nil {
			root.Close()
			return nil, err
		}
		app.pool = pool
	}

	if cfg.RunsHTTP() {
		router, err := httptransport.NewRouter(httptransport.Options{
			Logger: logger, Assets: assets, Draining: draining,
			ReadyCheck: app.readyCheck,
		}, nil)
		if err != nil {
			app.Close(context.Background())
			return nil, err
		}
		app.Server = &http.Server{
			Addr: cfg.HTTPAddress, Handler: router,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       30 * time.Second,
			// SSE/streaming handlers override the write deadline per request;
			// the global cap only bounds ordinary requests (docs/plans/v1/00).
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
			MaxHeaderBytes: 1 << 20,
		}
	}
	return app, nil
}

// readyCheck implements /readyz levels: draining -> not ready; with DB -> the
// pool must answer a trivial query (schema compatibility is asserted at
// migration time and re-checked cheaply here).
func (a *Application) readyCheck(ctx context.Context) error {
	if a.Draining.Load() {
		return errors.New("draining")
	}
	if a.pool == nil {
		return nil
	}
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var one int
	return a.pool.QueryRow(pingCtx, "SELECT 1").Scan(&one)
}

// RunMigrations applies all embedded migrations under the advisory lock.
func (a *Application) RunMigrations(ctx context.Context) error {
	if a.pool == nil {
		return errors.New("app: no database configured")
	}
	return a.pool.Migrate(ctx)
}

// StartWorkers launches the job engine loop; handlers are registered by
// callers before this point via RegisterJobHandler.
func (a *Application) StartWorkers(ctx context.Context, logger *slog.Logger) {
	if a.pool == nil {
		return
	}
	a.engine = job.NewEngine(a.pool.Pool, job.Options{}, logger, nil)
	for _, h := range jobHandlers {
		a.engine.Register(h)
	}
	for i := 0; i < a.Config.WorkerCount; i++ {
		wctx, cancel := context.WithCancel(ctx)
		a.workers = append(a.workers, cancel)
		go a.engine.Run(wctx)
	}
}

var jobHandlers []job.Handler

// RegisterJobHandler is called during wiring (before StartWorkers).
func RegisterJobHandler(h job.Handler) { jobHandlers = append(jobHandlers, h) }

// Close drains and releases all resources (graceful shutdown, 11 §3).
func (a *Application) Close(ctx context.Context) error {
	a.Draining.Store(true)
	var firstErr error
	if a.Server != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, a.Config.ShutdownTimeout)
		if err := a.Server.Shutdown(shutdownCtx); err != nil {
			firstErr = fmt.Errorf("http shutdown: %w", err)
		}
		cancel()
	}
	for _, cancel := range a.workers {
		cancel()
	}
	a.workers = nil
	if a.pool != nil {
		a.pool.Close()
		a.pool = nil
	}
	if a.root != nil {
		if err := a.root.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
