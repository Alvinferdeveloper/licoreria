// Package server assembles the router and runs the HTTP server with graceful shutdown.
package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"

	"licoreria/api/internal/apidocs"
	"licoreria/api/internal/auditlog"
	"licoreria/api/internal/auth"
	"licoreria/api/internal/backups"
	"licoreria/api/internal/cash"
	"licoreria/api/internal/catalog"
	"licoreria/api/internal/configuration"
	"licoreria/api/internal/dashboard"
	"licoreria/api/internal/health"
	"licoreria/api/internal/httpx"
	"licoreria/api/internal/inventory"
	"licoreria/api/internal/parties"
	"licoreria/api/internal/platform/config"
	"licoreria/api/internal/platform/storage"
	"licoreria/api/internal/products"
	"licoreria/api/internal/purchases"
	"licoreria/api/internal/rates"
	"licoreria/api/internal/reports"
	"licoreria/api/internal/roles"
	"licoreria/api/internal/sales"
	"licoreria/api/internal/settings"
	"licoreria/api/internal/users"
)

type Deps struct {
	Config  config.Config
	Logger  *slog.Logger
	DB      *pgxpool.Pool
	Backups *backups.Service
}

func NewRouter(deps Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(
		middleware.RealIP,
		httpx.RequestID(deps.Logger),
		httpx.AccessLog,
		httpx.Recoverer,
		httpx.SecurityHeaders,
		cors.Handler(cors.Options{
			AllowedOrigins:   deps.Config.AllowedOrigins,
			AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Authorization", "Content-Type", "Idempotency-Key", httpx.RequestIDHeader},
			ExposedHeaders:   []string{httpx.RequestIDHeader},
			AllowCredentials: true,
			MaxAge:           600,
		}),
	)
	r.NotFound(httpx.NotFound)
	r.MethodNotAllowed(httpx.MethodNotAllowed)

	authSvc := auth.NewService(deps.DB, deps.Config.Auth, deps.Config.AppBaseURL, time.Now)
	usersSvc := users.NewService(deps.DB, deps.Config.AppBaseURL, time.Now)
	store := storage.NewSupabase(storage.Config{
		URL:        deps.Config.Storage.SupabaseURL,
		ServiceKey: deps.Config.Storage.SupabaseServiceKey,
		Bucket:     deps.Config.Storage.Bucket,
	})
	ledger := inventory.NewLedger(deps.Config.AppBaseURL)
	if !store.Enabled() {
		deps.Logger.Warn("supabase storage not configured, product image uploads are disabled")
	}

	r.Route("/api/v1", func(api chi.Router) {
		health.NewHandler(deps.DB).Routes(api)
		apidocs.Routes(api)

		api.Group(func(gated chi.Router) {
			gated.Use(deps.Backups.Gate().Middleware)
			auth.NewHandler(authSvc, deps.Config.Auth.CookieSecure).Routes(gated)

			gated.Group(func(protected chi.Router) {
				protected.Use(auth.Authenticate(authSvc), auth.RequirePasswordChanged)
				users.NewHandler(usersSvc).Routes(protected)
				roles.NewHandler(roles.NewService(deps.DB)).Routes(protected)
				catalog.NewHandler(catalog.NewService(deps.DB)).Routes(protected)
				products.NewHandler(products.NewService(deps.DB, store)).Routes(protected)
				parties.NewCustomerHandler(parties.NewCustomerService(deps.DB)).Routes(protected)
				parties.NewSupplierHandler(parties.NewSupplierService(deps.DB)).Routes(protected)
				inventory.NewHandler(inventory.NewService(deps.DB, ledger)).Routes(protected)
				purchases.NewHandler(purchases.NewService(deps.DB, ledger)).Routes(protected)
				rates.NewHandler(rates.NewService(deps.DB)).Routes(protected)
				cash.NewHandler(cash.NewService(deps.DB, deps.Config.AppBaseURL)).Routes(protected)
				sales.NewHandler(sales.NewService(deps.DB, ledger)).Routes(protected)
				settings.NewHandler(deps.DB).Routes(protected)
				configuration.NewHandler(configuration.NewService(deps.DB, store)).Routes(protected)
				reports.NewHandler(reports.NewService(deps.DB)).Routes(protected)
				dashboard.NewHandler(deps.DB).Routes(protected)
				backups.NewHandler(deps.Backups).Routes(protected)
				auditlog.NewHandler(deps.DB).Routes(protected)
			})
		})
	})
	return r
}

// NewBackups builds the backup service, with its private bucket when one is configured.
func NewBackups(cfg config.Config, logger *slog.Logger, db *pgxpool.Pool) *backups.Service {
	remote := storage.NewSupabase(storage.Config{
		URL:        cfg.Storage.SupabaseURL,
		ServiceKey: cfg.Storage.SupabaseServiceKey,
		Bucket:     cfg.Storage.BackupBucket,
		Timeout:    10 * time.Minute,
	})
	return backups.NewService(db, backups.Options{DatabaseURL: cfg.DatabaseURL, Dir: cfg.Storage.BackupDir, Remote: remote, Logger: logger})
}

// Run serves until ctx is cancelled, then drains in-flight requests.
func Run(ctx context.Context, cfg config.Config, logger *slog.Logger, handler http.Handler) error {
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("api listening", "addr", cfg.HTTPAddr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down", "timeout", cfg.ShutdownTimeout)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
