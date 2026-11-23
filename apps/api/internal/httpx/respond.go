package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/platform/logx"
)

// StatusClientClosed is nginx's 499: the client closed the request before the response.
const StatusClientClosed = 499

func JSON(w http.ResponseWriter, r *http.Request, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		logx.From(r.Context()).Error("encode response", "error", err)
	}
}

func NoContent(w http.ResponseWriter) { w.WriteHeader(http.StatusNoContent) }

// Fail writes the standard error envelope. Server errors are logged with their real cause;
// the client only gets internal_error and the request id to trace it.
func Fail(w http.ResponseWriter, r *http.Request, err error) {
	// The client went away mid-request: nothing failed on our side and nobody reads the reply.
	if errors.Is(err, context.Canceled) && r.Context().Err() != nil {
		w.WriteHeader(StatusClientClosed)
		return
	}
	status, body := toBody(err)
	body.RequestID = middleware.GetReqID(r.Context())
	if status >= http.StatusInternalServerError {
		logx.From(r.Context()).Error("request failed", "error", err, "code", body.Code)
	}
	JSON(w, r, status, map[string]ErrorBody{"error": body})
}

func NotFound(w http.ResponseWriter, r *http.Request) { Fail(w, r, apperr.ErrNotFound) }

func MethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	Fail(w, r, apperr.ErrMethodNotAllowed)
}
