package cash

import (
	"net/http"

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

func (h *Handler) Routes(r chi.Router) {
	r.Route("/registers", func(r chi.Router) {
		r.With(auth.Require("cash_shifts", "view")).Get("/", h.registers)
		r.With(auth.Require("settings", "update")).Post("/", h.createRegister)
		r.With(auth.Require("settings", "update")).Put("/{id}", h.updateRegister)
	})
	r.Route("/cash-shifts", func(r chi.Router) {
		r.With(auth.Require("cash_shifts", "view")).Get("/", h.shifts)
		r.With(auth.Require("cash_shifts", "view")).Get("/current", h.current)
		r.With(auth.Require("cash_shifts", "create")).Post("/", h.open)
		r.With(auth.Require("cash_shifts", "view")).Get("/{id}", h.shift)
		r.With(auth.Require("cash_shifts", "update")).Post("/{id}/movements", h.addMovement)
		r.With(auth.Require("cash_shifts", "update")).Post("/{id}/close", h.close)
	})
}

func (h *Handler) registers(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.Registers(r.Context())
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, map[string]any{"data": items})
}

type registerRequest struct {
	Name          string `json:"name" validate:"required,min=2,max=60"`
	InvoiceSeries string `json:"invoiceSeries" validate:"required,min=2,max=8,alphanum"`
	Active        *bool  `json:"active" validate:"required"`
}

func (h *Handler) saveRegister(w http.ResponseWriter, r *http.Request, id *uuid.UUID) {
	var req registerRequest
	if err := httpx.Bind(w, r, &req); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	p, _ := auth.FromContext(r.Context())
	if err := h.svc.SaveRegister(r.Context(), p, id, RegisterInput{Name: req.Name, InvoiceSeries: req.InvoiceSeries, Active: *req.Active}); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	h.registers(w, r)
}

func (h *Handler) createRegister(w http.ResponseWriter, r *http.Request) { h.saveRegister(w, r, nil) }

func (h *Handler) updateRegister(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.URLParamUUID(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	h.saveRegister(w, r, &id)
}

func (h *Handler) shifts(w http.ResponseWriter, r *http.Request) {
	page, err := httpx.ParsePage(r)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	q := r.URL.Query()
	f := ShiftFilter{Status: q.Get("status")}
	if f.Status != "" && f.Status != "open" && f.Status != "closed" {
		httpx.Fail(w, r, apperr.Validation(apperr.FieldError{Field: "status", Code: "oneof", Message: "El estado no es válido."}))
		return
	}
	if raw := q.Get("registerId"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httpx.Fail(w, r, apperr.Validation(apperr.FieldError{Field: "registerId", Code: "uuid", Message: "El identificador no es válido."}))
			return
		}
		f.RegisterID = &id
	}
	p, _ := auth.FromContext(r.Context())
	items, total, err := h.svc.Shifts(r.Context(), p, f, page)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, httpx.NewList(items, page, total))
}

func (h *Handler) current(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.FromContext(r.Context())
	shift, err := h.svc.Current(r.Context(), p)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, shift)
}

func (h *Handler) shift(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.URLParamUUID(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	p, _ := auth.FromContext(r.Context())
	shift, err := h.svc.Shift(r.Context(), p, id)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, shift)
}

type openRequest struct {
	RegisterID uuid.UUID `json:"registerId" validate:"required"`
	OpeningNIO string    `json:"openingNio" validate:"required,money"`
	OpeningUSD string    `json:"openingUsd" validate:"required,money"`
}

func (h *Handler) open(w http.ResponseWriter, r *http.Request) {
	var req openRequest
	if err := httpx.Bind(w, r, &req); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	p, _ := auth.FromContext(r.Context())
	shift, err := h.svc.Open(r.Context(), p, OpenInput{RegisterID: req.RegisterID, OpeningNIO: req.OpeningNIO, OpeningUSD: req.OpeningUSD})
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusCreated, shift)
}

type movementRequest struct {
	Type     string `json:"type" validate:"required,oneof=income expense"`
	Currency string `json:"currency" validate:"required,oneof=NIO USD"`
	Amount   string `json:"amount" validate:"required,money"`
	Reason   string `json:"reason" validate:"required,min=3,max=300"`
}

func (h *Handler) addMovement(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.URLParamUUID(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	var req movementRequest
	if err := httpx.Bind(w, r, &req); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	if zeroAmount(req.Amount) {
		httpx.Fail(w, r, apperr.Validation(apperr.FieldError{Field: "amount", Code: "gt", Message: "Debe ser mayor que 0."}))
		return
	}
	p, _ := auth.FromContext(r.Context())
	shift, err := h.svc.AddMovement(r.Context(), p, id, MovementInput{Type: req.Type, Currency: req.Currency, Amount: req.Amount, Reason: req.Reason})
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusCreated, shift)
}

type closeRequest struct {
	CountedNIO string `json:"countedNio" validate:"required,money"`
	CountedUSD string `json:"countedUsd" validate:"required,money"`
	Notes      string `json:"notes" validate:"max=500"`
}

func (h *Handler) close(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.URLParamUUID(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	var req closeRequest
	if err := httpx.Bind(w, r, &req); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	p, _ := auth.FromContext(r.Context())
	shift, err := h.svc.Close(r.Context(), p, id, CloseInput{CountedNIO: req.CountedNIO, CountedUSD: req.CountedUSD, Notes: req.Notes})
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, shift)
}

func zeroAmount(v string) bool {
	for _, c := range v {
		if c != '0' && c != '.' {
			return false
		}
	}
	return true
}
