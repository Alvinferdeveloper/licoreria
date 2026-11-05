package rates

import (
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/auth"
	"licoreria/api/internal/httpx"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Routes(r chi.Router) {
	r.Route("/exchange-rates", func(r chi.Router) {
		r.With(auth.Require("exchange_rates", "view")).Get("/", h.list)
		r.With(auth.Require("exchange_rates", "view")).Get("/today", h.today)
		r.With(auth.Require("exchange_rates", "create")).Post("/", h.set)
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

func (h *Handler) today(w http.ResponseWriter, r *http.Request) {
	rate, err := Today(r.Context(), h.svc.pool)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, rate)
}

var (
	ratePattern = regexp.MustCompile(`^\d{1,4}(\.\d{1,6})?$`)
	zeroPattern = regexp.MustCompile(`^[0.]+$`)
)

type setRequest struct {
	Rate string  `json:"rate" validate:"required"`
	Date *string `json:"date" validate:"omitempty,datetime=2006-01-02"`
}

func (h *Handler) set(w http.ResponseWriter, r *http.Request) {
	var req setRequest
	if err := httpx.Bind(w, r, &req); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	if !ratePattern.MatchString(req.Rate) || zeroPattern.MatchString(req.Rate) {
		httpx.Fail(w, r, apperr.Validation(apperr.FieldError{Field: "rate", Code: "rate", Message: "Escribe una tasa válida, por ejemplo 36.6243."}))
		return
	}
	p, _ := auth.FromContext(r.Context())
	rate, err := h.svc.Set(r.Context(), p, req.Date, req.Rate)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, rate)
}
