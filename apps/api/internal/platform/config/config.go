// Package config loads runtime settings from environment variables and validates them at startup.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env             string
	HTTPAddr        string
	DatabaseURL     string
	AllowedOrigins  []string
	ShutdownTimeout time.Duration
	LogLevel        string
	LogFormat       string

	// AppBaseURL is the public web URL used in email links.
	AppBaseURL string

	Auth    AuthConfig
	Mail    MailConfig
	Storage StorageConfig
}

// StorageConfig points at Supabase Storage; when empty, image uploads are disabled.
// BackupBucket must be private; when empty, backups stay only in BackupDir.
type StorageConfig struct {
	SupabaseURL        string
	SupabaseServiceKey string
	Bucket             string
	BackupBucket       string
	BackupDir          string
}

type AuthConfig struct {
	JWTSecret       []byte
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	ResetTokenTTL   time.Duration
	CookieSecure    bool
}

type MailConfig struct {
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	From         string
	// ResendAPIKey, when set, sends through Resend's API instead of SMTP.
	ResendAPIKey string
}

func (c Config) IsProduction() bool { return c.Env == "production" }

const minJWTSecretBytes = 32

// escapeDatabasePassword percent-encodes the password of a database URL. Compose builds the
// URL from DB_PASSWORD as typed, so characters like ñ, @, # or spaces would break parsing.
// A password that is already percent-encoded is kept as is.
func escapeDatabasePassword(raw string) string {
	scheme, rest, ok := strings.Cut(raw, "://")
	if !ok {
		return raw
	}
	at := strings.LastIndex(rest, "@")
	if at < 0 {
		return raw
	}
	user, pass, ok := strings.Cut(rest[:at], ":")
	if !ok {
		return raw
	}
	if unescaped, err := url.PathUnescape(pass); err == nil {
		pass = unescaped
	}
	return scheme + "://" + url.UserPassword(user, pass).String() + "@" + rest[at+1:]
}

func Load() (Config, error) {
	var errs []error
	env := getEnv("APP_ENV", "development")
	production := env == "production"
	defaultLogFormat := "text"
	if production {
		defaultLogFormat = "json"
	}

	cfg := Config{
		Env:            env,
		HTTPAddr:       getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL:    escapeDatabasePassword(os.Getenv("DATABASE_URL")),
		AllowedOrigins: splitList(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")),
		LogLevel:       getEnv("LOG_LEVEL", "info"),
		LogFormat:      getEnv("LOG_FORMAT", defaultLogFormat),
		AppBaseURL:     strings.TrimRight(getEnv("APP_BASE_URL", "http://localhost:3000"), "/"),
		Auth: AuthConfig{
			JWTSecret: []byte(os.Getenv("JWT_SECRET")),
		},
		Mail: MailConfig{
			SMTPHost:     getEnv("SMTP_HOST", "localhost"),
			SMTPUsername: os.Getenv("SMTP_USERNAME"),
			SMTPPassword: os.Getenv("SMTP_PASSWORD"),
			From:         getEnv("MAIL_FROM", "Añejo <no-reply@anejo.local>"),
			ResendAPIKey: os.Getenv("RESEND_API_KEY"),
		},
		Storage: StorageConfig{
			SupabaseURL:        os.Getenv("SUPABASE_URL"),
			SupabaseServiceKey: os.Getenv("SUPABASE_SERVICE_KEY"),
			Bucket:             getEnv("SUPABASE_BUCKET", "productos"),
			BackupBucket:       os.Getenv("SUPABASE_BACKUP_BUCKET"),
			BackupDir:          getEnv("BACKUP_DIR", "/var/lib/anejo/backups"),
		},
	}

	cfg.ShutdownTimeout = durationEnv("SHUTDOWN_TIMEOUT", 10*time.Second, &errs)
	cfg.Auth.AccessTokenTTL = durationEnv("ACCESS_TOKEN_TTL", 15*time.Minute, &errs)
	cfg.Auth.RefreshTokenTTL = durationEnv("REFRESH_TOKEN_TTL", 7*24*time.Hour, &errs)
	cfg.Auth.ResetTokenTTL = durationEnv("RESET_TOKEN_TTL", 30*time.Minute, &errs)
	cfg.Auth.CookieSecure = boolEnv("COOKIE_SECURE", production, &errs)
	cfg.Mail.SMTPPort = intEnv("SMTP_PORT", 1025, &errs)

	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	if len(cfg.Auth.JWTSecret) < minJWTSecretBytes {
		errs = append(errs, fmt.Errorf("JWT_SECRET must be at least %d bytes", minJWTSecretBytes))
	}
	if strings.HasPrefix(cfg.Storage.SupabaseServiceKey, "sb_publishable_") {
		errs = append(errs, errors.New("SUPABASE_SERVICE_KEY must be the secret (sb_secret_…) or service_role key, not the publishable key"))
	}
	if cfg.LogFormat != "text" && cfg.LogFormat != "json" {
		errs = append(errs, errors.New("LOG_FORMAT must be text or json"))
	}
	if u, err := url.Parse(cfg.AppBaseURL); err != nil || u.Scheme == "" || u.Host == "" {
		errs = append(errs, errors.New("APP_BASE_URL must be an absolute URL"))
	}
	if cfg.Storage.SupabaseURL != "" {
		if u, err := url.Parse(cfg.Storage.SupabaseURL); err != nil || u.Scheme != "https" {
			errs = append(errs, errors.New("SUPABASE_URL must be an https URL like https://<project>.supabase.co"))
		}
	}
	if cfg.Storage.BackupBucket != "" && cfg.Storage.BackupBucket == cfg.Storage.Bucket {
		errs = append(errs, errors.New("SUPABASE_BACKUP_BUCKET must be a private bucket, not the public SUPABASE_BUCKET"))
	}
	if production && !cfg.Auth.CookieSecure {
		errs = append(errs, errors.New("COOKIE_SECURE cannot be false in production"))
	}

	if len(errs) > 0 {
		return Config{}, errors.Join(errs...)
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration, errs *[]error) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		*errs = append(*errs, fmt.Errorf("%s must be a positive duration like 15m or 168h", key))
		return fallback
	}
	return d
}

func intEnv(key string, fallback int, errs *[]error) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		*errs = append(*errs, fmt.Errorf("%s must be a positive integer", key))
		return fallback
	}
	return n
}

func boolEnv(key string, fallback bool, errs *[]error) bool {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	b, err := strconv.ParseBool(raw)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("%s must be true or false", key))
		return fallback
	}
	return b
}

func splitList(raw string) []string {
	var items []string
	for _, item := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}
