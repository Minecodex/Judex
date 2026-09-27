// SPDX-License-Identifier: Apache-2.0

package httptransport

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kakj-go/Judex/internal/identity"
	"github.com/kakj-go/Judex/internal/platform/auth"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/transport/http/middleware"
	"github.com/kakj-go/Judex/internal/version"
)

const requestIDKey = "request_id"

type Options struct {
	Logger   *slog.Logger
	Assets   fs.FS
	Draining *atomic.Bool
	// ReadyCheck validates deeper readiness (DB reachable, not draining);
	// nil means HTTP-only readiness.
	ReadyCheck func(ctx context.Context) error
	// Auth, when non-nil, enables session resolution + CSRF/Origin
	// enforcement for the whole API surface (docs/plans/v1/02 §2).
	Auth *AuthOptions
}

// AuthOptions carries the wiring the middleware needs.
type AuthOptions struct {
	Config        middleware.AuthConfig
	Resolver      func(ctx context.Context, secret string) (identity.ResolvedSession, error)
	GrantResolver func(ctx context.Context, secret string) (*auth.Principal, error)
}

// NewRouter builds the HTTP layer: probes, system info, the full contract
// route table (real handlers where registered, 501 elsewhere) and the SPA
// hosting rules (API 404 must never return HTML).
func NewRouter(opts Options, spec *SpecRouter) (*gin.Engine, error) {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.Draining == nil {
		opts.Draining = &atomic.Bool{}
	}
	if spec == nil {
		var err error
		spec, err = NewSpecRouter(opts.Logger)
		if err != nil {
			return nil, err
		}
	}
	router := gin.New()
	_ = router.SetTrustedProxies(nil)
	router.HandleMethodNotAllowed = true
	router.Use(func(c *gin.Context) {
		b := make([]byte, 16)
		_, _ = rand.Read(b)
		id := hex.EncodeToString(b)
		c.Set(requestIDKey, id)
		c.Header("X-Request-ID", id)
		c.Header("X-Content-Type-Options", "nosniff")
		start := time.Now()
		c.Next()
		opts.Logger.Info("http request", "request_id", id, "method", c.Request.Method, "route", c.FullPath(), "status", c.Writer.Status(), "duration_ms", time.Since(start).Milliseconds())
	})
	router.Use(gin.CustomRecovery(func(c *gin.Context, recovered any) {
		opts.Logger.Error("request panic", "request_id", c.GetString(requestIDKey))
		respond{}.error(c, errInternalPanic)
	}))
	if opts.Auth != nil && opts.Auth.Resolver != nil {
		router.Use(middleware.SessionMiddleware(opts.Auth.Config, opts.Auth.Resolver))
		if opts.Auth.GrantResolver != nil {
			router.Use(middleware.BearerMiddleware(opts.Auth.GrantResolver))
		}
		router.Use(middleware.CheckOriginOnly(opts.Auth.Config))
		router.Use(middleware.RequireCSRF(opts.Auth.Config))
	}
	router.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	router.GET("/readyz", func(c *gin.Context) {
		if opts.Draining.Load() {
			respond{}.error(c, errDraining)
			return
		}
		if opts.ReadyCheck != nil {
			if err := opts.ReadyCheck(c.Request.Context()); err != nil {
				respond{}.error(c, apierrors.Newf(apierrors.DependencyDown, "readiness check failed").WithRetryable(true).Wrap(err))
				return
			}
		}
		scope := "http"
		if opts.ReadyCheck != nil {
			scope = "http+db"
		}
		c.JSON(200, gin.H{"status": "ready", "scope": scope})
	})
	spec.Register("getSystem", func(c *gin.Context) {
		respond{}.ok(c, gin.H{
			"name": "Judex", "version": version.Version, "commit": version.Commit,
			"protocolVersion": "1", "schemaRange": "1",
			"capabilities": gin.H{"identity": false, "projects": false, "persistence": false, "agentExecution": false},
		})
	})
	spec.Mount(&router.RouterGroup)
	router.NoMethod(func(c *gin.Context) {
		respond{}.error(c, errMethodNotAllowed)
	})
	router.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, apiPrefix) || c.Request.URL.Path == "/api" {
			respond{}.error(c, errAPINotFound)
			return
		}
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			respond{}.error(c, errAPINotFound)
			return
		}
		if opts.Assets == nil {
			respond{}.error(c, errWebBuildMissing)
			return
		}
		name := strings.TrimPrefix(path.Clean(c.Request.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		info, err := fs.Stat(opts.Assets, name)
		if err == nil && !info.IsDir() {
			c.Header("Cache-Control", "no-cache")
			http.ServeFileFS(c.Writer, c.Request, opts.Assets, name)
			return
		}
		// Asset misses must remain errors rather than returning an HTML document.
		if path.Ext(name) != "" || strings.HasPrefix(name, "assets/") {
			respond{}.error(c, errAssetNotFound)
			return
		}
		if _, err := fs.Stat(opts.Assets, "index.html"); err != nil {
			respond{}.error(c, errWebBuildMissing)
			return
		}
		c.Header("Cache-Control", "no-cache")
		http.ServeFileFS(c.Writer, c.Request, opts.Assets, "index.html")
	})
	return router, nil
}
