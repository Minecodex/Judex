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
	RegisterPerIP   int64
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
	// ModelGateway is a direct single-model shortcut for worker processes
	// (P5 discussion batches) without the full catalog file.
	ModelGateway ModelGatewayConfig
	// ObjectStorage mirrors the Object config contract (11 §2); required
	// from P2-04 on for materials, optional for pure identity deployments.
	ObjectStorage ObjectStorageOptions
	// PreviewOrigin is the isolated origin serving HTML material previews
	// (04 §4). Empty: no interactive preview, source download only.
	PreviewOrigin string
	// Sandbox wires the OpenSandbox execution boundary (05 §7). Empty
	// endpoint: agent sandbox tools report 沙箱未配置 instead of faking.
	Sandbox SandboxConfig
}

// SandboxConfig configures the OpenSandbox server + per-run sandboxes.
type SandboxConfig struct {
	Endpoint             string // JUDEX_OPENSANDBOX_ENDPOINT, e.g. http://osb:80
	APIKey               string // JUDEX_OPENSANDBOX_API_KEY (custom header)
	Image                string // JUDEX_OPENSANDBOX_IMAGE, pinned at deploy time
	CPU                  string // JUDEX_OPENSANDBOX_CPU, K8s quantity
	Memory               string // JUDEX_OPENSANDBOX_MEMORY, K8s quantity
	MaterialNamespace    string
	MaterialImage        string
	MaterialStorageClass string
}

// ModelGatewayConfig is the direct gateway wiring for background agents.
type ModelGatewayConfig struct {
	Protocol string // openai-compatible | anthropic-compatible
	BaseURL  string
	APIKey   string
	Model    string
}

// ObjectStorageOptions configures the S3-compatible store.
type ObjectStorageOptions struct {
	Endpoint        string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	PathStyle       bool
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
	c.RegisterPerIP, err = strconv.ParseInt(value("JUDEX_REGISTER_PER_IP", "10"), 10, 64)
	if err != nil || c.RegisterPerIP < 1 || c.RegisterPerIP > 10000 {
		return c, fmt.Errorf("JUDEX_REGISTER_PER_IP must be between 1 and 10000")
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
	c.ObjectStorage = ObjectStorageOptions{
		Endpoint:        value("JUDEX_S3_ENDPOINT", ""),
		Region:          value("JUDEX_S3_REGION", ""),
		AccessKeyID:     value("JUDEX_S3_ACCESS_KEY", ""),
		SecretAccessKey: value("JUDEX_S3_SECRET_KEY", ""),
		Bucket:          value("JUDEX_S3_BUCKET", ""),
		PathStyle:       value("JUDEX_S3_PATH_STYLE", "true") == "true",
	}
	c.PreviewOrigin = value("JUDEX_PREVIEW_ORIGIN", "")

	c.Sandbox = SandboxConfig{
		Endpoint:          value("JUDEX_OPENSANDBOX_ENDPOINT", ""),
		MaterialNamespace: value("JUDEX_MATERIAL_NAMESPACE", ""), MaterialImage: value("JUDEX_MATERIAL_IMAGE", ""), MaterialStorageClass: value("JUDEX_MATERIAL_STORAGE_CLASS", ""),
		APIKey: value("JUDEX_OPENSANDBOX_API_KEY", ""),
		Image:  value("JUDEX_OPENSANDBOX_IMAGE", ""),
		CPU:    value("JUDEX_OPENSANDBOX_CPU", ""),
		Memory: value("JUDEX_OPENSANDBOX_MEMORY", ""),
	}
	c.ModelGateway = ModelGatewayConfig{
		Protocol: value("JUDEX_MODEL_PROTOCOL", ""),
		BaseURL:  value("JUDEX_MODEL_BASE_URL", ""),
		APIKey:   value("JUDEX_MODEL_API_KEY", ""),
		Model:    value("JUDEX_MODEL_NAME", ""),
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
	return c.Mode == ModeAll || c.Mode == ModeWorker || c.Mode == ModeMigrate || (c.Mode == ModeAPI && c.DatabaseURL != "")
}

// RunsHTTP reports whether the mode serves HTTP.
func (c Config) RunsHTTP() bool { return c.Mode == ModeAll || c.Mode == ModeAPI }
