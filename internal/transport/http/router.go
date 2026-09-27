// SPDX-License-Identifier: Apache-2.0

package httptransport

import (
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
	"github.com/kakj-go/Judex/internal/version"
)

const requestIDKey = "request_id"

type Options struct {
	Logger   *slog.Logger
	Assets   fs.FS
	Draining *atomic.Bool
	// APIPath is the sub-path prefix the SPA is served under ("/").
	APIPath string
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
	router.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	router.GET("/readyz", func(c *gin.Context) {
		if opts.Draining.Load() {
			respond{}.error(c, errDraining)
			return
		}
		c.JSON(200, gin.H{"status": "ready", "scope": "http-scaffold"})
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
