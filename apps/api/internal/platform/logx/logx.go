// Package logx builds the application logger and carries a request-scoped logger in the context.
package logx

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/lmittmann/tint"
)

type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
)

const redacted = "[REDACTED]"

// sensitiveKeys are attribute names whose values must never reach the logs.
var sensitiveKeys = []string{"password", "pin", "token", "secret", "authorization", "cookie", "hash"}

// New returns a JSON logger for production or a colored, human-readable one for development.
func New(w io.Writer, format Format, level slog.Level) *slog.Logger {
	if format == FormatJSON {
		return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level, ReplaceAttr: redact}))
	}
	return slog.New(tint.NewHandler(w, &tint.Options{
		Level:       level,
		TimeFormat:  time.TimeOnly,
		ReplaceAttr: redact,
	}))
}

func ParseLevel(raw string) slog.Level {
	var level slog.Level
	if err := level.UnmarshalText([]byte(strings.ToUpper(raw))); err != nil {
		return slog.LevelInfo
	}
	return level
}

type ctxKey struct{}

func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, logger)
}

// From returns the request-scoped logger, or the default logger outside a request.
func From(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}

// With adds attributes to the context logger for the rest of the request.
func With(ctx context.Context, args ...any) context.Context {
	return WithLogger(ctx, From(ctx).With(args...))
}

func redact(_ []string, attr slog.Attr) slog.Attr {
	if isSensitive(attr.Key) {
		return slog.String(attr.Key, redacted)
	}
	return attr
}

// isSensitive matches whole snake_case segments, so "admin_pin" is redacted but "shipping" is not.
func isSensitive(key string) bool {
	segments := strings.Split(strings.ToLower(key), "_")
	for _, segment := range segments {
		for _, sensitive := range sensitiveKeys {
			if segment == sensitive {
				return true
			}
		}
	}
	return strings.Contains(strings.ToLower(key), "tax_id")
}
