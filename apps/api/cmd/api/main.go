// Command api runs the HTTP API (`serve`, default), manages the schema
// (`migrate up|down|version`) and bootstraps the first administrator (`create-admin`).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"licoreria/api/internal/mail"
	"licoreria/api/internal/platform/config"
	"licoreria/api/internal/platform/database"
	"licoreria/api/internal/platform/logx"
	"licoreria/api/internal/server"
	"licoreria/api/internal/users"
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
	case "create-admin":
		return createAdmin(cfg, args[1:])
	default:
		return fmt.Errorf("unknown command %q (use serve, migrate or create-admin)", command)
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

	sender, err := mail.NewSMTPSender(cfg.Mail)
	if err != nil {
		return err
	}
	go mail.NewWorker(pool, sender, logger).Run(ctx)

	router := server.NewRouter(server.Deps{Config: cfg, Logger: logger, DB: pool})
	return server.Run(ctx, cfg, logger, router)
}

// createAdmin bootstraps the first administrator. The temporary password is printed
// to the terminal once and never logged.
func createAdmin(cfg config.Config, args []string) error {
	flags := flag.NewFlagSet("create-admin", flag.ContinueOnError)
	name := flags.String("name", "", "full name")
	email := flags.String("email", "", "email address")
	username := flags.String("username", "admin", "login username")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *name == "" || *email == "" {
		return errors.New(`usage: create-admin -name "Lucía Barberena" -email lucia@example.com [-username admin]`)
	}

	ctx := context.Background()
	if err := database.MigrateUp(cfg.DatabaseURL); err != nil {
		return err
	}
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	admin, temp, err := users.NewService(pool, cfg.AppBaseURL, time.Now).CreateInitialAdmin(ctx, *name, *email, *username)
	if err != nil {
		return err
	}
	fmt.Printf("\nAdministrador creado: %s (%s)\nUsuario: %s\nContraseña temporal: %s\n\nSe pedirá cambiarla en el primer inicio de sesión.\n\n",
		admin.FullName, admin.Email, admin.Username, temp)
	return nil
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
