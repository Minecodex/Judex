// SPDX-License-Identifier: Apache-2.0
package app

import (
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/kakj-go/Judex/internal/config"
	httptransport "github.com/kakj-go/Judex/internal/transport/http"
)

type Application struct {
	Server   *http.Server
	Draining *atomic.Bool
	root     *os.Root
}

func New(cfg config.Config, logger *slog.Logger) (*Application, error) {
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
	router, err := httptransport.NewRouter(httptransport.Options{Logger: logger, Assets: assets, Draining: draining}, nil)
	if err != nil {
		root.Close()
		return nil, err
	}
	return &Application{root: root, Draining: draining, Server: &http.Server{
		Addr: cfg.HTTPAddress, Handler: router, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}}, nil
}
func (a *Application) Close() error {
	if a.root != nil {
		return a.root.Close()
	}
	return nil
}
