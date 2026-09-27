// SPDX-License-Identifier: Apache-2.0

// Package config loads server configuration from the environment
// (docs/plans/v1/11 §2). Validation is strict: a missing required field fails
// startup with a clear message instead of a half-working process.
package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// Mode selects what one server process runs (docs/plans/v1/01 §1).
type Mode string

const (
	ModeAll     Mode = "all"     // HTTP + workers (default single process)
	ModeAPI     Mode = "api"     // HTTP only
	ModeWorker  Mode = "worker"  // background workers only
	ModeMigrate Mode = "migrate" // run migrations and exit
)

type Config struct {
	Environment     string
	HTTPAddress     string
	WebDirectory    string
	ShutdownTimeout time.Duration
	Mode            Mode
	DatabaseURL     string
	WorkerCount     int
	LogLevel        string
	// AllowedOrigins lists exact browser origins accepted for writes
	// (02 §2); production must include the public origin.
	AllowedOrigins []string
	// ModelCatalogFile points at the operator YAML (02 §8); empty = no models.
	ModelCatalogFile string
}

func Load() (Config, error) { return FromEnv(os.Getenv) }

func FromEnv(get func(string) string) (Config, error) {
	value := func(key, fallback string) string {
		if v := get(key); v != "" {
			return v
		}
		return fallback
	}
	c := Config{
		Environment:  value("JUDEX_ENV", "development"),
		HTTPAddress:  value("JUDEX_HTTP_ADDR", "127.0.0.1:8080"),
		WebDirectory: value("JUDEX_WEB_DIR", "web/dist"),
		Mode:         Mode(value("JUDEX_MODE", string(ModeAll))),
		DatabaseURL:  value("JUDEX_DATABASE_URL", ""),
		LogLevel:     value("JUDEX_LOG_LEVEL", "info"),
	}
	if c.Environment != "development" && c.Environment != "production" && c.Environment != "test" {
		return c, fmt.Errorf("JUDEX_ENV must be development, production, or test")
	}
	if _, _, err := net.SplitHostPort(c.HTTPAddress); err != nil {
		return c, fmt.Errorf("invalid JUDEX_HTTP_ADDR: %w", err)
	}
	var err error
	c.ShutdownTimeout, err = time.ParseDuration(value("JUDEX_SHUTDOWN_TIMEOUT", "15s"))
	if err != nil || c.ShutdownTimeout <= 0 {
		return c, fmt.Errorf("JUDEX_SHUTDOWN_TIMEOUT must be a positive duration")
	}
	switch c.Mode {
	case ModeAll, ModeAPI, ModeWorker, ModeMigrate:
	default:
		return c, fmt.Errorf("JUDEX_MODE must be one of all, api, worker, migrate")
	}
	// Persistence-requiring modes need a database; api-only development keeps
	// running against the 501 scaffold without one.
	if c.NeedsDatabase() && c.DatabaseURL == "" {
		return c, fmt.Errorf("JUDEX_DATABASE_URL is required for mode %s", c.Mode)
	}
	c.WorkerCount, err = strconv.Atoi(value("JUDEX_WORKER_COUNT", "2"))
	if err != nil || c.WorkerCount < 1 || c.WorkerCount > 64 {
		return c, fmt.Errorf("JUDEX_WORKER_COUNT must be an integer in [1,64]")
	}
	if c.Environment == "production" && c.DatabaseURL == "" {
		return c, fmt.Errorf("production requires JUDEX_DATABASE_URL")
	}
	if raw := get("JUDEX_ALLOWED_ORIGINS"); raw != "" {
		for _, o := range strings.Split(raw, ",") {
			if o = strings.TrimSpace(o); o != "" {
				c.AllowedOrigins = append(c.AllowedOrigins, o)
			}
		}
	}
	if c.Environment == "production" && len(c.AllowedOrigins) == 0 {
		return c, fmt.Errorf("production requires JUDEX_ALLOWED_ORIGINS (comma-separated exact origins)")
	}
	c.ModelCatalogFile = value("JUDEX_MODEL_CATALOG_FILE", "")
	if c.ModelCatalogFile != "" {
		if _, err := os.Stat(c.ModelCatalogFile); err != nil {
			return c, fmt.Errorf("JUDEX_MODEL_CATALOG_FILE not readable: %w", err)
		}
	}
	return c, nil
}

// NeedsDatabase reports whether the mode touches PostgreSQL.
func (c Config) NeedsDatabase() bool {
	return c.Mode == ModeAll || c.Mode == ModeWorker || c.Mode == ModeMigrate
}

// RunsHTTP reports whether the mode serves HTTP.
func (c Config) RunsHTTP() bool { return c.Mode == ModeAll || c.Mode == ModeAPI }
