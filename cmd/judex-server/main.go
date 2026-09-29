// SPDX-License-Identifier: Apache-2.0

// judex-server entry point: thin wiring only — flags, config, app lifecycle
// (docs/plans/v1/01 §1). Mode selection: all | api | worker | migrate.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/kakj-go/Judex/internal/app"
	"github.com/kakj-go/Judex/internal/config"
	"github.com/kakj-go/Judex/internal/version"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "judex-server:", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) == 2 && os.Args[1] == "verify-recovery" {
		return verifyRecovery()
	}
	if len(os.Args) == 3 && os.Args[1] == "verify-sandbox" {
		return verifySandbox(os.Args[2])
	}
	if len(os.Args) == 3 && os.Args[1] == "objects" {
		return runObjectArchive(os.Args[2])
	}
	mode := flag.String("mode", "", "all | api | worker | migrate (overrides JUDEX_MODE)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Printf("judex-server %s (%s)\n", version.Version, version.Commit)
		return nil
	}

	// Operator subcommands (docs/plans/v1/02 §3): audited maintenance that
	// runs against the database directly and exits; never a business role.
	if flag.NArg() > 0 && flag.Arg(0) == "account" {
		return runAccountCommand()
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if *mode != "" {
		cfg.Mode = config.Mode(*mode)
	}
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLevel(cfg.LogLevel)}))
	slog.SetDefault(logger)
	logger.Info("judex-server starting", "version", version.Version, "commit", version.Commit, "mode", cfg.Mode, "env", cfg.Environment)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	application, err := app.New(cfg, logger)
	if err != nil {
		return err
	}

	switch cfg.Mode {
	case config.ModeMigrate:
		if err := application.RunMigrations(ctx); err != nil {
			application.Close(context.Background())
			return fmt.Errorf("migration failed: %w", err)
		}
		logger.Info("migrations applied")
		return application.Close(context.Background())

	case config.ModeWorker:
		application.StartWorkers(ctx, logger)
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		return application.Close(shutdownCtx)

	default: // all | api
		if cfg.NeedsDatabase() {
			if err := application.RunMigrations(ctx); err != nil {
				application.Close(context.Background())
				return fmt.Errorf("startup migration failed: %w", err)
			}
			application.StartWorkers(ctx, logger)
		}
		errCh := make(chan error, 1)
		go func() {
			logger.Info("http listening", "addr", cfg.HTTPAddress)
			if err := application.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- err
			}
		}()
		select {
		case err := <-errCh:
			application.Close(context.Background())
			return err
		case <-ctx.Done():
			logger.Info("shutdown signal received")
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := application.Close(shutdownCtx); err != nil {
			return err
		}
		logger.Info("shutdown complete")
		return nil
	}
}

func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
