package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost/db")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Env != "development" || cfg.HTTPAddr != ":8080" || cfg.LogFormat != "text" || cfg.ShutdownTimeout != 10*time.Second {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadProductionDefaultsToJSONLogs(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost/db")
	t.Setenv("APP_ENV", "production")

	cfg, err := Load()
	if err != nil || cfg.LogFormat != "json" {
		t.Errorf("got %q, %v", cfg.LogFormat, err)
	}
}

func TestLoadFailsFast(t *testing.T) {
	cases := map[string]map[string]string{
		"missing database url": {"DATABASE_URL": ""},
		"bad shutdown timeout": {"DATABASE_URL": "postgres://x", "SHUTDOWN_TIMEOUT_SECONDS": "-1"},
		"bad log format":       {"DATABASE_URL": "postgres://x", "LOG_FORMAT": "xml"},
	}
	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			for key, value := range env {
				t.Setenv(key, value)
			}
			if _, err := Load(); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

func TestAllowedOriginsList(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("CORS_ALLOWED_ORIGINS", " http://localhost:3000 , https://anejo.app ,")

	cfg, _ := Load()
	if len(cfg.AllowedOrigins) != 2 || cfg.AllowedOrigins[1] != "https://anejo.app" {
		t.Errorf("got %v", cfg.AllowedOrigins)
	}
}
