package users

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/auth"
	"licoreria/api/internal/httpx"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Routes mounts /users; callers must already be authenticated.
func (h *Handler) Routes(r chi.Router) {
	r.Route("/users", func(r chi.Router) {
		r.With(auth.Require("users", "view")).Get("/", h.list)
		r.With(auth.Require("users", "create")).Post("/", h.create)
		r.With(auth.Require("users", "view")).Get("/{id}", h.get)
		r.With(auth.Require("users", "update")).Put("/{id}", h.update)
		r.With(auth.Require("users", "delete")).Patch("/{id}/status", h.setStatus)
		r.With(auth.Require("users", "update")).Post("/{id}/resend-access", h.resendAccess)
	})
}

type userRequest struct {
	FullName string `json:"fullName" validate:"required,min=3,max=120"`
	Email    string `json:"email" validate:"omitempty,email,max=254"`
	Username string `json:"username" validate:"required,username"`
	RoleID   string `json:"roleId" validate:"required,uuid"`
}

// input is called after validation, so RoleID is a well-formed UUID.
func (req userRequest) input() Input {
	return Input{FullName: req.FullName, Email: req.Email, Username: req.Username, RoleID: uuid.MustParse(req.RoleID)}
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	page, err := httpx.ParsePage(r)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	filter, err := parseFilter(r)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	items, total, err := h.svc.List(r.Context(), filter, page)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, httpx.NewList(items, page, total))
}

func parseFilter(r *http.Request) (Filter, error) {
	q := r.URL.Query()
	f := Filter{Search: q.Get("search")}
	if raw := q.Get("roleId"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return Filter{}, apperr.Validation(apperr.FieldError{Field: "roleId", Code: "uuid", Message: "El identificador no es válido."})
		}
		f.RoleID = &id
	}
	if raw := q.Get("active"); raw != "" {
		active, err := strconv.ParseBool(raw)
		if err != nil {
			return Filter{}, apperr.Validation(apperr.FieldError{Field: "active", Code: "boolean", Message: "Usa true o false."})
		}
		f.Active = &active
	}
	return f, nil
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.URLParamUUID(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	user, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, user)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req userRequest
	if err := httpx.Bind(w, r, &req); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	actor, _ := auth.FromContext(r.Context())
	user, access, err := h.svc.Create(r.Context(), actor.UserID, req.input())
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, r, http.StatusCreated, createResponse{User: user, Access: access})
}

type createResponse struct {
	User   User   `json:"user"`
	Access Access `json:"access"`
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.URLParamUUID(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	var req userRequest
	if err := httpx.Bind(w, r, &req); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	actor, _ := auth.FromContext(r.Context())
	user, err := h.svc.Update(r.Context(), actor.UserID, id, req.input())
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, user)
}

type statusRequest struct {
	Active *bool `json:"active" validate:"required"`
}

func (h *Handler) setStatus(w http.ResponseWriter, r *http.Request) {
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
	user, err := h.svc.SetActive(r.Context(), actor.UserID, id, *req.Active)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, user)
}

type resendRequest struct {
	Channel string `json:"channel" validate:"required,oneof=email code"`
}

func (h *Handler) resendAccess(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.URLParamUUID(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	var req resendRequest
	if err := httpx.Bind(w, r, &req); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	actor, _ := auth.FromContext(r.Context())
	access, err := h.svc.ResendAccess(r.Context(), actor.UserID, id, req.Channel)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, r, http.StatusOK, access)
}
