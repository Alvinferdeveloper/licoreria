package backups

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/auth"
	"licoreria/api/internal/httpx"
	"licoreria/api/internal/platform/logx"
	"licoreria/api/internal/settings"
)

// ConfirmationPhrase must be typed exactly to restore a backup.
const ConfirmationPhrase = "RESTAURAR"

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Routes(r chi.Router) {
	r.Route("/backups", func(r chi.Router) {
		r.With(auth.Require("backups", "view")).Get("/", h.list)
		r.With(auth.Require("backups", "view")).Get("/status", h.status)
		r.With(auth.Require("backups", "authorize")).Put("/policy", h.setPolicy)
		r.With(auth.Require("backups", "create"), longRunning).Post("/", h.create)
		r.With(auth.Require("backups", "create"), longRunning).Get("/{id}/file", h.download)
		r.With(auth.Require("backups", "create"), longRunning).Post("/{id}/verify", h.verify)
		r.With(auth.Require("backups", "authorize"), longRunning).Post("/{id}/restore", h.restore)
	})
}

// longRunning lifts the server's write timeout for operations that outlast it.
func longRunning(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(operationTimeout)); err != nil {
			logx.From(r.Context()).Warn("extend write deadline", "error", err)
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	page, err := httpx.ParsePage(r)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	items, total, err := h.svc.List(r.Context(), page)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, httpx.NewList(items, page, total))
}

func (h *Handler) status(w http.ResponseWriter, r *http.Request) {
	policy, err := settings.GetBackups(r.Context(), h.svc.pool)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, h.statusBody(policy))
}

func (h *Handler) statusBody(policy settings.Backups) map[string]any {
	return map[string]any{
		"offsite": h.svc.Offsite(), "scheduleHour": policy.Hour,
		"dailyRetention": policy.DailyRetention, "weeklyRetention": policy.WeeklyRetention,
	}
}

type policyRequest struct {
	ScheduleHour    int `json:"scheduleHour" validate:"gte=0,lte=23"`
	DailyRetention  int `json:"dailyRetention" validate:"gte=1,lte=30"`
	WeeklyRetention int `json:"weeklyRetention" validate:"gte=0,lte=12"`
}

func (h *Handler) setPolicy(w http.ResponseWriter, r *http.Request) {
	var req policyRequest
	if err := httpx.Bind(w, r, &req); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	actor, _ := auth.FromContext(r.Context())
	policy, err := h.svc.SetPolicy(r.Context(), actor.UserID, settings.Backups{Hour: req.ScheduleHour, DailyRetention: req.DailyRetention, WeeklyRetention: req.WeeklyRetention})
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, h.statusBody(policy))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.FromContext(r.Context())
	b, err := h.svc.Create(r.Context(), actor.UserID)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusCreated, b)
}

func (h *Handler) download(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.URLParamUUID(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	actor, _ := auth.FromContext(r.Context())
	file, b, err := h.svc.Open(r.Context(), id, actor.UserID)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+b.FileName+`"`)
	http.ServeContent(w, r, b.FileName, b.CreatedAt, file)
}

func (h *Handler) verify(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.URLParamUUID(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	actor, _ := auth.FromContext(r.Context())
	b, err := h.svc.Verify(r.Context(), id, &actor.UserID)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, b)
}

type restoreRequest struct {
	CurrentPassword string `json:"currentPassword" validate:"required"`
	Confirmation    string `json:"confirmation" validate:"required"`
}

func (h *Handler) restore(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.URLParamUUID(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	var req restoreRequest
	if err := httpx.Bind(w, r, &req); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	if req.Confirmation != ConfirmationPhrase {
		httpx.Fail(w, r, apperr.Validation(apperr.FieldError{Field: "confirmation", Code: "confirmation", Message: "Escribe " + ConfirmationPhrase + " para confirmar."}))
		return
	}
	actor, _ := auth.FromContext(r.Context())
	result, err := h.svc.Restore(r.Context(), actor.UserID, id, req.CurrentPassword)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, result)
}
