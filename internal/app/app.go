// Package app assembles the V2 modular monolith. Entry points stay thin.
package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/kakj-go/Judex/internal/agent"
	"github.com/kakj-go/Judex/internal/agent/peer"
	"github.com/kakj-go/Judex/internal/collab"
	"github.com/kakj-go/Judex/internal/config"
	"github.com/kakj-go/Judex/internal/identity"
	"github.com/kakj-go/Judex/internal/infrastructure/objectstore"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	"github.com/kakj-go/Judex/internal/job"
	httptransport "github.com/kakj-go/Judex/internal/transport/http"
	"github.com/kakj-go/Judex/internal/transport/http/middleware"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"sync/atomic"
	"time"
)

type Application struct {
	Server   *http.Server
	Draining *atomic.Bool
	Config   config.Config
	pool     *postgres.Pool
	identity *identity.Service
	collab   *collab.Service
	engine   *job.Engine
	root     *os.Root
	workers  []context.CancelFunc
	handlers []job.Handler
}

func New(cfg config.Config, logger *slog.Logger) (app *Application, err error) {
	if logger == nil {
		logger = slog.Default()
	}
	app = &Application{Draining: &atomic.Bool{}, Config: cfg}
	defer func() {
		if err != nil {
			app.Close(context.Background())
		}
	}()
	var assets fs.FS
	root, err := os.OpenRoot(cfg.WebDirectory)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return app, err
	}
	err = nil
	if root != nil {
		app.root = root
		assets = root.FS()
	}
	if cfg.NeedsDatabase() {
		app.pool, err = postgres.Open(context.Background(), postgres.Options{URL: cfg.DatabaseURL}, logger)
		if err != nil {
			return app, err
		}
		if err = app.pool.Migrate(context.Background()); err != nil {
			return app, err
		}
		if cfg.ModelCatalogFile != "" {
			entries, e := agent.LoadCatalogFile(cfg.ModelCatalogFile)
			if e != nil {
				return app, e
			}
			if err = agent.SyncCatalog(context.Background(), app.pool, entries, time.Now().UTC()); err != nil {
				return app, err
			}
		}
		app.identity = identity.NewService(app.pool, identity.NewRateLimiter(app.pool.Pool, nil), identity.Options{RegisterPerIP: cfg.RegisterPerIP}, nil)
		app.collab = &collab.Service{Pool: app.pool, ConverterURL: cfg.MaterialConverterURL}
		app.identity.ProfileChanged = app.collab.ProfileChanged
		if cfg.ObjectStorage.Endpoint != "" {
			app.collab.Objects, err = objectstore.New(context.Background(), objectstore.Options{Endpoint: cfg.ObjectStorage.Endpoint, Region: cfg.ObjectStorage.Region, AccessKeyID: cfg.ObjectStorage.AccessKeyID, SecretAccessKey: cfg.ObjectStorage.SecretAccessKey, Bucket: cfg.ObjectStorage.Bucket, PathStyle: cfg.ObjectStorage.PathStyle})
			if err != nil {
				return app, err
			}
		}
		app.handlers = append(app.handlers, app.collab.PreviewJob(), app.collab.WorkflowDraftJob(agent.Resolver(app.pool, cfg.ModelGateway)))
		executor := &peer.Executor{Service: app.collab, ResolveModel: agent.Resolver(app.pool, cfg.ModelGateway)}
		if err = app.wireSandbox(executor); err != nil {
			return app, err
		}
		app.handlers = append(app.handlers, executor.Handler())
	}
	if cfg.RunsHTTP() {
		spec, e := httptransport.NewSpecRouter(logger)
		if e != nil {
			return app, e
		}
		var authOpts *httptransport.AuthOptions
		if app.identity != nil {
			spec.CommandPool = app.pool
			authCfg := middleware.AuthConfig{Development: cfg.Environment != "production", AllowedOrigins: cfg.AllowedOrigins}
			httptransport.NewIdentityHandlers(app.identity, authCfg, authCfg.Development).Register(spec)
			httptransport.V2Handlers{Service: app.collab, PreviewOrigin: cfg.PreviewOrigin}.Register(spec)
			authOpts = &httptransport.AuthOptions{Config: authCfg, Resolver: app.identity.ResolveSession, GrantResolver: app.identity.ResolveGrant}
		}
		router, e := httptransport.NewRouter(httptransport.Options{Logger: logger, Assets: assets, Draining: app.Draining, ReadyCheck: app.readyCheck, Auth: authOpts, PreviewService: app.collab, PreviewOrigin: cfg.PreviewOrigin, Capabilities: map[string]bool{"identity": app.identity != nil, "projects": app.collab != nil, "persistence": app.pool != nil, "agentExecution": cfg.ModelCatalogFile != "" || cfg.ModelGateway.Model != ""}}, spec)
		if e != nil {
			return app, e
		}
		if app.pool != nil {
			if e = spec.ValidateRegistration(); e != nil {
				return app, e
			}
		}
		app.Server = &http.Server{Addr: cfg.HTTPAddress, Handler: router, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
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
	var schema int
	if err := a.pool.QueryRow(pingCtx, "SELECT version FROM judex_schema").Scan(&schema); err != nil {
		return err
	}
	if schema != 2 {
		return errors.New("unsupported database schema")
	}
	return nil
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
	if a.pool == nil || a.Config.Mode == config.ModeAPI {
		return
	}
	a.engine = job.NewEngine(a.pool.Pool, job.Options{Kinds: []string{"role.delivery", "workflow.draft"}}, logger, nil)
	backgroundHandlers := []job.Handler{}
	backgroundKinds := []string{}
	for _, h := range append(append([]job.Handler{}, jobHandlers...), a.handlers...) {
		if h.Kind() == "role.delivery" || h.Kind() == "workflow.draft" {
			a.engine.Register(h)
		} else {
			backgroundHandlers = append(backgroundHandlers, h)
			backgroundKinds = append(backgroundKinds, h.Kind())
		}
	}
	// Model I/O can occupy both role slots for minutes. Previews must still
	// progress without raising the configured model concurrency.
	if len(backgroundHandlers) > 0 {
		background := job.NewEngine(a.pool.Pool, job.Options{Kinds: backgroundKinds}, logger, nil)
		for _, h := range backgroundHandlers {
			background.Register(h)
		}
		bctx, cancel := context.WithCancel(ctx)
		a.workers = append(a.workers, cancel)
		go background.Run(bctx)
	}
	maintenanceCtx, cancelMaintenance := context.WithCancel(ctx)
	a.workers = append(a.workers, cancelMaintenance)
	go a.maintain(maintenanceCtx, logger)
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
