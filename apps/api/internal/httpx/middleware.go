package httpx

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/platform/logx"
)

const RequestIDHeader = "X-Request-Id"

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{8,64}$`)

// RequestID reuses a safe incoming X-Request-Id or generates a UUID, echoes it in the
// response and stores the client info and a logger tagged with the id in the context.
func RequestID(base *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get(RequestIDHeader)
			if !validRequestID.MatchString(requestID) {
				requestID = uuid.NewString()
			}
			w.Header().Set(RequestIDHeader, requestID)

			ctx := context.WithValue(withClientInfo(r), middleware.RequestIDKey, requestID)
			ctx = logx.WithLogger(ctx, base.With("request_id", requestID))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AccessLog writes one line per request; the level follows the response status.
func AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		status := ww.Status()
		if status == 0 {
			status = http.StatusOK
		}
		logx.From(r.Context()).Log(r.Context(), levelFor(status), "http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"bytes", ww.BytesWritten(),
			"duration_ms", time.Since(start).Milliseconds(),
			"ip", r.RemoteAddr,
		)
	})
}

func levelFor(status int) slog.Level {
	switch {
	case status >= http.StatusInternalServerError:
		return slog.LevelError
	case status >= http.StatusBadRequest:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}

// Recoverer turns panics into a logged internal_error using the standard envelope.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if rec == http.ErrAbortHandler {
				panic(rec)
			}
			logx.From(r.Context()).Error("panic recovered", "panic", fmt.Sprint(rec), "stack", string(debug.Stack()))
			Fail(w, r, apperr.ErrInternal)
		}()
		next.ServeHTTP(w, r)
	})
}

// SecurityHeaders sets conservative defaults for a JSON API.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}
