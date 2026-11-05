// Package health reports whether the API and its database are reachable.
package health

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"licoreria/api/internal/httpx"
	"licoreria/api/internal/platform/logx"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

type Handler struct {
	db Pinger
}

func NewHandler(db Pinger) *Handler { return &Handler{db: db} }

func (h *Handler) Routes(r chi.Router) {
	r.Get("/health", h.check)
}

type Status struct {
	Status   string `json:"status"`
	Database string `json:"database"`
}

func (h *Handler) check(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		logx.From(r.Context()).Warn("database ping failed", "error", err)
		httpx.JSON(w, r, http.StatusServiceUnavailable, Status{Status: "degraded", Database: "unreachable"})
		return
	}
	httpx.JSON(w, r, http.StatusOK, Status{Status: "ok", Database: "ok"})
}
