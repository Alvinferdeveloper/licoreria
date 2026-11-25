// Package database owns the PostgreSQL connection pool and schema migrations.
package database

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"

	"licoreria/api/migrations"
)

// redact keeps the database password out of error messages, and so out of the logs.
func redact(err error, databaseURL string) error {
	u, parseErr := url.Parse(databaseURL)
	if parseErr != nil || u.User == nil {
		return err
	}
	pass, ok := u.User.Password()
	if !ok || pass == "" {
		return err
	}
	msg := err.Error()
	for _, secret := range []string{pass, url.QueryEscape(pass), url.PathEscape(pass), url.UserPassword("", pass).String()[1:]} {
		if secret != "" {
			msg = strings.ReplaceAll(msg, secret, "xxxxx")
		}
	}
	return errors.New(msg)
}

func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", redact(err, databaseURL))
	}
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 15 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", redact(err, databaseURL))
	}
	return pool, nil
}

// Migrator applies the embedded migrations; golang-migrate needs the pgx5:// scheme.
func Migrator(databaseURL string) (*migrate.Migrate, error) {
	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("load migrations: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", source, toMigrateURL(databaseURL))
	if err != nil {
		return nil, fmt.Errorf("init migrator: %w", redact(err, databaseURL))
	}
	return m, nil
}

// MigrateUp applies all pending migrations.
func MigrateUp(databaseURL string) error {
	m, err := Migrator(databaseURL)
	if err != nil {
		return err
	}
	defer m.Close()
	if err := IgnoreNoChange(m.Up()); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

// IgnoreNoChange treats "already up to date" as success.
func IgnoreNoChange(err error) error {
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	return err
}

func toMigrateURL(url string) string {
	for _, prefix := range []string{"postgres://", "postgresql://"} {
		if rest, ok := strings.CutPrefix(url, prefix); ok {
			return "pgx5://" + rest
		}
	}
	return url
}
