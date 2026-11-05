package settings

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"licoreria/api/internal/httpx"
)

type Handler struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Handler { return &Handler{pool: pool} }

// Routes exposes the settings every signed-in screen needs (store name, POS rules).
func (h *Handler) Routes(r chi.Router) {
	r.Get("/settings/store", h.store)
}

func (h *Handler) store(w http.ResponseWriter, r *http.Request) {
	business, err := GetBusiness(r.Context(), h.pool)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	pos, err := GetPOS(r.Context(), h.pool)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, map[string]any{"business": business, "pos": pos})
}
