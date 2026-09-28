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

	"github.com/google/uuid"
	"time"

	"github.com/kakj-go/Judex/internal/agent"
	"github.com/kakj-go/Judex/internal/agent/batch"
	"github.com/kakj-go/Judex/internal/agent/runner"
	"github.com/kakj-go/Judex/internal/agent/tools"
	"github.com/kakj-go/Judex/internal/config"
	"github.com/kakj-go/Judex/internal/decision"
	"github.com/kakj-go/Judex/internal/discussion"
	"github.com/kakj-go/Judex/internal/handoff"
	"github.com/kakj-go/Judex/internal/identity"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
	"github.com/kakj-go/Judex/internal/infrastructure/objectstore"
	"github.com/kakj-go/Judex/internal/infrastructure/opensandbox"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	"github.com/kakj-go/Judex/internal/job"
	"github.com/kakj-go/Judex/internal/material"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/project"
	httptransport "github.com/kakj-go/Judex/internal/transport/http"
	"github.com/kakj-go/Judex/internal/transport/http/middleware"
	"github.com/kakj-go/Judex/internal/work"
	"github.com/kakj-go/Judex/internal/workflow"
)

type Application struct {
	Server   *http.Server
	Draining *atomic.Bool
	Config   config.Config

	pool       *postgres.Pool
	identity   *identity.Service
	projects   *project.Service
	workflows  *workflow.Service
	materials  *material.Service
	discussion *discussion.Service
	work       *work.Service
	decisions  *decision.Service
	handoffs   *handoff.Service
	objects    material.ObjectStore
	engine     *job.Engine
	root       *os.Root
	workers    []context.CancelFunc
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
		if cfg.ObjectStorage.Endpoint != "" {
			store, err := objectstore.New(context.Background(), objectstore.Options{
				Endpoint: cfg.ObjectStorage.Endpoint, Region: cfg.ObjectStorage.Region,
				AccessKeyID: cfg.ObjectStorage.AccessKeyID, SecretAccessKey: cfg.ObjectStorage.SecretAccessKey,
				Bucket: cfg.ObjectStorage.Bucket, PathStyle: cfg.ObjectStorage.PathStyle,
			})
			if err != nil {
				pool.Close()
				root.Close()
				return nil, err
			}
			app.objects = store
		}
		if cfg.ModelCatalogFile != "" {
			entries, err := agent.LoadCatalogFile(cfg.ModelCatalogFile)
			if err != nil {
				pool.Close()
				root.Close()
				return nil, err
			}
			if err := agent.SyncCatalog(context.Background(), pool, entries, time.Now().UTC()); err != nil {
				pool.Close()
				root.Close()
				return nil, err
			}
			logger.Info("model catalog synced", "entries", len(entries))
		}
		app.pool = pool
		limiter := identity.NewRateLimiter(pool.Pool, nil)
		app.identity = identity.NewService(pool, limiter, identity.Options{}, nil)
		app.projects = project.NewService(pool, nil)
		app.workflows = workflow.NewService(pool, nil)
		app.materials = material.NewService(pool, app.objects, material.DefaultLimits(), nil)
		app.discussion = discussion.NewService(pool, nil)
		app.work = work.NewService(pool, nil)
		app.decisions = decision.NewService(pool, nil, 86400)
		app.handoffs = handoff.NewService(pool, nil)
		handler := decision.TimeoutJobHandler{Service: app.decisions}
		timeoutExecutor = handler.Execute
		if cfg.ModelGateway.Protocol != "" {
			provider, err := model.NewProvider(model.GatewayConfig{
				Protocol: cfg.ModelGateway.Protocol,
				BaseURL:  cfg.ModelGateway.BaseURL,
				APIKey:   cfg.ModelGateway.APIKey,
			})
			if err == nil {
				registry := tools.New()
				tools.RegisterDefaults(registry)
				executor := &batch.Executor{
					Pool: pool.Pool, Provider: provider, ModelName: cfg.ModelGateway.Model,
					Runner: runner.NewRunnerForTest(provider, registry),
				}
				if cfg.Sandbox.Endpoint != "" {
					sbClient := opensandbox.New(opensandbox.Config{
						Endpoint: cfg.Sandbox.Endpoint,
						APIKey:   cfg.Sandbox.APIKey,
						Image:    cfg.Sandbox.Image,
						CPU:      cfg.Sandbox.CPU,
						Memory:   cfg.Sandbox.Memory,
					})
					executor.SandboxFactory = func(runID, projectID uuid.UUID) *opensandbox.RunSandbox {
						return &opensandbox.RunSandbox{Client: sbClient, RunID: runID, ProjectID: projectID}
					}
				}
				batchExecutor = executor.ExecuteJob
			}
		}
	}

	if cfg.RunsHTTP() {
		var authOpts *httptransport.AuthOptions
		spec, err := httptransport.NewSpecRouter(logger)
		if err != nil {
			app.Close(context.Background())
			return nil, err
		}
		if app.identity != nil {
			authCfg := middleware.AuthConfig{
				Development:    cfg.Environment == "development" || cfg.Environment == "test",
				AllowedOrigins: cfg.AllowedOrigins,
			}
			httptransport.NewIdentityHandlers(app.identity, authCfg, authCfg.Development).Register(spec)
			httptransport.NewProjectHandlers(app.projects,
				func(ctx context.Context, id uuid.UUID) error {
					_, err := agent.ModelEnabled(ctx, app.pool, id)
					return err
				},
				func(ctx context.Context) ([]agent.PublicModel, error) { return agent.ListPublicModels(ctx, app.pool) },
			).Register(spec)
			httptransport.NewMemberHandlers(app.projects).Register(spec)
			httptransport.NewWorkflowHandlers(app.workflows).Register(spec)
			httptransport.NewPositionHandlers(app.projects).Register(spec)
			httptransport.NewMaterialHandlers(app.materials).Register(spec)
			httptransport.NewDiscussionHandlers(app.discussion).Register(spec)
			httptransport.NewSSEHandlers(app.pool.Pool).Register(spec)
			httptransport.NewWorkHandlers(app.work).Register(spec)
			httptransport.NewProposalHandlers(app.decisions).Register(spec)
			httptransport.NewHandoffHandlers(app.handoffs, app.work).Register(spec)
			httptransport.NewResearchHandlers(app.work).Register(spec)
			httptransport.NewIntentHandlers(app.decisions, app.work).Register(spec)
			httptransport.NewAgentHandlers(app.pool.Pool).Register(spec)
			idSvc := app.identity
			authOpts = &httptransport.AuthOptions{
				Config:        authCfg,
				Resolver:      idSvc.ResolveSession,
				GrantResolver: idSvc.ResolveGrant,
			}
		}
		httptransport.PreviewOrigin = cfg.PreviewOrigin
		router, err := httptransport.NewRouter(httptransport.Options{
			Logger: logger, Assets: assets, Draining: draining,
			ReadyCheck: app.readyCheck,
			Auth:       authOpts,
			// 按实际接线如实上报能力（未配置模型网关时 agentExecution=false）。
			Capabilities: map[string]bool{
				"identity": app.identity != nil, "projects": app.projects != nil,
				"persistence": app.pool != nil, "agentExecution": batchExecutor != nil,
			},
		}, spec)
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
			WriteTimeout:   30 * time.Second,
			IdleTimeout:    60 * time.Second,
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

func init() {
	// Timeout settlement worker (P3-05): the service instance is bound per
	// application via BindDecisionTimeouts.
	RegisterJobHandler(job.HandlerFunc{KindName: "proposal.timeout", Attempts: 5, ExecuteFn: func(ctx context.Context, j job.Job) error {
		if timeoutExecutor == nil {
			return apierrors.Newf(apierrors.Internal, "timeout executor not bound")
		}
		return timeoutExecutor(ctx, j)
	}})
	RegisterJobHandler(job.HandlerFunc{KindName: "discussion.batch", Attempts: 3, ExecuteFn: func(ctx context.Context, j job.Job) error {
		if batchExecutor == nil {
			return apierrors.Newf(apierrors.ModelUnavailable, "模型网关未配置，批次无法自动执行").WithRetryable(false)
		}
		return batchExecutor(ctx, j)
	}})
}

// timeoutExecutor is set by New() once the decision service exists.
var timeoutExecutor func(ctx context.Context, j job.Job) error

// batchExecutor is set by New() when a real model gateway is configured.
var batchExecutor func(ctx context.Context, j job.Job) error

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
