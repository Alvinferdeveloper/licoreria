package config

import (
	"strings"
	"testing"
	"time"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func setRequired(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost/db")
	t.Setenv("JWT_SECRET", testSecret)
}

func TestLoadDefaults(t *testing.T) {
	setRequired(t)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Env != "development" || cfg.HTTPAddr != ":8080" || cfg.LogFormat != "text" || cfg.ShutdownTimeout != 10*time.Second {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
	if cfg.Auth.AccessTokenTTL != 15*time.Minute || cfg.Auth.ResetTokenTTL != 30*time.Minute || cfg.Auth.CookieSecure {
		t.Errorf("unexpected auth defaults: %+v", cfg.Auth)
	}
	if cfg.Mail.SMTPPort != 1025 || cfg.AppBaseURL != "http://localhost:3000" {
		t.Errorf("unexpected mail defaults: %+v %s", cfg.Mail, cfg.AppBaseURL)
	}
}

func TestLoadProductionDefaults(t *testing.T) {
	setRequired(t)
	t.Setenv("APP_ENV", "production")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LogFormat != "json" || !cfg.Auth.CookieSecure {
		t.Errorf("production must default to JSON logs and secure cookies: %+v", cfg)
	}
}

func TestLoadFailsFastWithEveryProblem(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("JWT_SECRET", "short")
	t.Setenv("LOG_FORMAT", "xml")
	t.Setenv("ACCESS_TOKEN_TTL", "soon")
	t.Setenv("APP_BASE_URL", "not a url")

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"DATABASE_URL", "JWT_SECRET", "LOG_FORMAT", "ACCESS_TOKEN_TTL", "APP_BASE_URL"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %s: %v", want, err)
		}
	}
}

func TestInsecureCookiesRejectedInProduction(t *testing.T) {
	setRequired(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("COOKIE_SECURE", "false")

	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "COOKIE_SECURE") {
		t.Errorf("expected COOKIE_SECURE error, got %v", err)
	}
}

func TestAllowedOriginsList(t *testing.T) {
	setRequired(t)
	t.Setenv("CORS_ALLOWED_ORIGINS", " http://localhost:3000 , https://anejo.app ,")

	cfg, err := Load()
	if err != nil || len(cfg.AllowedOrigins) != 2 || cfg.AllowedOrigins[1] != "https://anejo.app" {
		t.Errorf("got %v, %v", cfg.AllowedOrigins, err)
	}
}
