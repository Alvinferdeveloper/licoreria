package catalog

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"licoreria/api/internal/auth"
	"licoreria/api/internal/httpx"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Routes mounts /categories, /brands and /units, all guarded by the categories permissions.
func (h *Handler) Routes(r chi.Router) {
	mount := func(path string, k kind) {
		r.Route(path, func(r chi.Router) {
			r.With(auth.Require("categories", "view")).Get("/", h.list(k))
			r.With(auth.Require("categories", "create")).Post("/", h.create(k))
			r.With(auth.Require("categories", "update")).Put("/{id}", h.update(k))
			if k.hasActive {
				r.With(auth.Require("categories", "delete")).Patch("/{id}/status", h.setStatus(k))
			}
		})
	}
	mount("/categories", categories)
	mount("/brands", brands)
	mount("/units", units)
}

type masterRequest struct {
	Name         string     `json:"name" validate:"required,min=2,max=80"`
	Abbreviation string     `json:"abbreviation" validate:"max=12"`
	ParentID     *uuid.UUID `json:"parentId"`
}

func (req masterRequest) input() MasterInput {
	return MasterInput{Name: req.Name, Abbreviation: req.Abbreviation, ParentID: req.ParentID}
}

func (h *Handler) list(k kind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := h.svc.list(r.Context(), k)
		if err != nil {
			httpx.Fail(w, r, err)
			return
		}
		httpx.JSON(w, r, http.StatusOK, map[string]any{"data": items})
	}
}

func (h *Handler) create(k kind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req masterRequest
		if err := httpx.Bind(w, r, &req); err != nil {
			httpx.Fail(w, r, err)
			return
		}
		actor, _ := auth.FromContext(r.Context())
		item, err := h.svc.create(r.Context(), actor.UserID, k, req.input())
		if err != nil {
			httpx.Fail(w, r, err)
			return
		}
		httpx.JSON(w, r, http.StatusCreated, item)
	}
}

func (h *Handler) update(k kind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httpx.URLParamUUID(r, "id")
		if err != nil {
			httpx.Fail(w, r, err)
			return
		}
		var req masterRequest
		if err := httpx.Bind(w, r, &req); err != nil {
			httpx.Fail(w, r, err)
			return
		}
		actor, _ := auth.FromContext(r.Context())
		item, err := h.svc.update(r.Context(), actor.UserID, k, id, req.input())
		if err != nil {
			httpx.Fail(w, r, err)
			return
		}
		httpx.JSON(w, r, http.StatusOK, item)
	}
}

type statusRequest struct {
	Active *bool `json:"active" validate:"required"`
}

func (h *Handler) setStatus(k kind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httpx.URLParamUUID(r, "id")
		if err != nil {
			httpx.Fail(w, r, err)
			return
		}
		var req statusRequest
		if err := httpx.Bind(w, r, &req); err != nil {
			httpx.Fail(w, r, err)
			return
		}
		actor, _ := auth.FromContext(r.Context())
		item, err := h.svc.setActive(r.Context(), actor.UserID, k, id, *req.Active)
		if err != nil {
			httpx.Fail(w, r, err)
			return
		}
		httpx.JSON(w, r, http.StatusOK, item)
	}
}
