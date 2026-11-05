// Command api runs the HTTP API (`serve`, default) or manages the schema (`migrate up|down|version`).
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"licoreria/api/internal/platform/config"
	"licoreria/api/internal/platform/database"
	"licoreria/api/internal/platform/logx"
	"licoreria/api/internal/server"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logger := logx.New(os.Stdout, logx.Format(cfg.LogFormat), logx.ParseLevel(cfg.LogLevel))
	slog.SetDefault(logger)

	command := "serve"
	if len(args) > 0 {
		command = args[0]
	}

	switch command {
	case "serve":
		return serve(cfg, logger)
	case "migrate":
		return migrate(cfg, logger, args[1:])
	default:
		return fmt.Errorf("unknown command %q (use serve or migrate)", command)
	}
}

func serve(cfg config.Config, logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := database.MigrateUp(cfg.DatabaseURL); err != nil {
		return err
	}

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	router := server.NewRouter(server.Deps{Config: cfg, Logger: logger, DB: pool})
	return server.Run(ctx, cfg, logger, router)
}

func migrate(cfg config.Config, logger *slog.Logger, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: migrate up|down|version")
	}
	if args[0] == "up" {
		if err := database.MigrateUp(cfg.DatabaseURL); err != nil {
			return err
		}
		logger.Info("migrations applied")
		return nil
	}

	m, err := database.Migrator(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer m.Close()

	switch args[0] {
	case "down":
		if err := database.IgnoreNoChange(m.Steps(-1)); err != nil {
			return err
		}
		logger.Info("rolled back one migration")
		return nil
	case "version":
		version, dirty, err := m.Version()
		if err != nil {
			return err
		}
		logger.Info("schema version", "version", version, "dirty", dirty)
		return nil
	default:
		return fmt.Errorf("unknown migrate action %q", args[0])
	}
}
