package logx

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestJSONLoggerRedactsSensitiveAttributes(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, FormatJSON, slog.LevelInfo)

	logger.Info("login attempt", "email", "rosa@example.com", "password", "s3cret", "refresh_token", "abc", "admin_pin", "2580")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("log line is not JSON: %v", err)
	}
	for _, key := range []string{"password", "refresh_token", "admin_pin"} {
		if entry[key] != redacted {
			t.Errorf("%s = %v, want %s", key, entry[key], redacted)
		}
	}
	if entry["email"] != "rosa@example.com" {
		t.Errorf("email should not be redacted, got %v", entry["email"])
	}
}

func TestIsSensitiveMatchesWholeSegments(t *testing.T) {
	for _, key := range []string{"password", "admin_pin", "pin_hash", "refresh_token", "Authorization", "customer_tax_id"} {
		if !isSensitive(key) {
			t.Errorf("%s should be sensitive", key)
		}
	}
	for _, key := range []string{"shipping", "opening_nio", "email", "spinner"} {
		if isSensitive(key) {
			t.Errorf("%s should not be sensitive", key)
		}
	}
}

func TestContextLoggerCarriesAttributes(t *testing.T) {
	var buf bytes.Buffer
	ctx := WithLogger(context.Background(), New(&buf, FormatJSON, slog.LevelInfo))
	ctx = With(ctx, "request_id", "req-123")

	From(ctx).Info("sale completed")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["request_id"] != "req-123" {
		t.Errorf("request_id = %v, want req-123", entry["request_id"])
	}
}

func TestFromFallsBackToDefault(t *testing.T) {
	if From(context.Background()) != slog.Default() {
		t.Error("expected default logger outside a request")
	}
}

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{"debug": slog.LevelDebug, "WARN": slog.LevelWarn, "nonsense": slog.LevelInfo}
	for raw, want := range cases {
		if got := ParseLevel(raw); got != want {
			t.Errorf("ParseLevel(%q) = %v, want %v", raw, got, want)
		}
	}
}
