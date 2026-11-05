package configuration

import (
	"errors"
	"io"
	"net/http"

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
	r.With(auth.Require("settings", "view")).Get("/settings", h.get)
	r.With(auth.Require("settings", "update")).Put("/settings", h.update)
	r.With(auth.Require("settings", "update")).Post("/settings/logo", h.uploadLogo)
	r.With(auth.Require("settings", "update")).Delete("/settings/logo", h.removeLogo)
}

func actor(r *http.Request) auth.Principal {
	p, _ := auth.FromContext(r.Context())
	return p
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	view, err := h.svc.Get(r.Context())
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, view)
}

type updateRequest struct {
	Business struct {
		Name    string `json:"name" validate:"required,max=120"`
		TaxID   string `json:"taxId" validate:"max=30"`
		Address string `json:"address" validate:"max=200"`
		Phone   string `json:"phone" validate:"max=40"`
		Email   string `json:"email" validate:"omitempty,email,max=120"`
	} `json:"business"`
	Tax struct {
		DefaultPercent float64 `json:"defaultPercent" validate:"gte=0,lte=100"`
	} `json:"tax"`
	POS struct {
		DiscountLimitPercent float64 `json:"discountLimitPercent" validate:"gte=0,lte=100"`
		DefaultPrintFormat   string  `json:"defaultPrintFormat" validate:"required,oneof=ticket invoice"`
	} `json:"pos"`
	Invoice struct {
		FooterNote string `json:"footerNote" validate:"max=160"`
	} `json:"invoice"`
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	var req updateRequest
	if err := httpx.Bind(w, r, &req); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	view, err := h.svc.Update(r.Context(), actor(r), Input{
		Name: req.Business.Name, TaxID: req.Business.TaxID, Address: req.Business.Address, Phone: req.Business.Phone, Email: req.Business.Email,
		TaxPercent: req.Tax.DefaultPercent, DiscountLimitPercent: req.POS.DiscountLimitPercent,
		DefaultPrintFormat: req.POS.DefaultPrintFormat, FooterNote: req.Invoice.FooterNote,
	})
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, view)
}

// uploadLogo accepts multipart/form-data with the file in the "logo" field.
func (h *Handler) uploadLogo(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxLogoBytes+64<<10)
	file, _, err := r.FormFile("logo")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			httpx.Fail(w, r, ErrInvalidLogo)
			return
		}
		httpx.Fail(w, r, apperr.Validation(apperr.FieldError{Field: "logo", Code: "required", Message: "Selecciona una imagen."}))
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxLogoBytes+1))
	if err != nil {
		httpx.Fail(w, r, ErrInvalidLogo)
		return
	}
	view, err := h.svc.SetLogo(r.Context(), actor(r), data)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, view)
}

func (h *Handler) removeLogo(w http.ResponseWriter, r *http.Request) {
	view, err := h.svc.RemoveLogo(r.Context(), actor(r))
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, view)
}
