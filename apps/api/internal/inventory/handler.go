package inventory

import (
	"net/http"
	"regexp"

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
	r.Route("/inventory", func(r chi.Router) {
		r.With(auth.Require("inventory", "view")).Get("/movements", h.movements)
		r.With(auth.Require("inventory", "view")).Get("/alerts", h.alerts)
		r.With(auth.Require("inventory", "view")).Get("/forecast", h.forecast)
		r.With(auth.Require("inventory", "view")).Get("/expiring", h.expiring)
		r.With(auth.Require("inventory", "create")).Post("/adjustments", h.adjust)
	})
}

var (
	datePattern   = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	movementTypes = map[string]bool{TypePurchase: true, TypeSale: true, TypeSaleVoid: true, TypeCustomerReturn: true,
		TypeSupplierReturn: true, TypeCountAdjustment: true, TypeShrinkage: true}
)

func (h *Handler) movements(w http.ResponseWriter, r *http.Request) {
	page, err := httpx.ParsePage(r)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	q := r.URL.Query()
	f := MovementFilter{Type: q.Get("type"), From: q.Get("from"), To: q.Get("to")}
	var fields []apperr.FieldError
	if raw := q.Get("productId"); raw != "" {
		if id, err := uuid.Parse(raw); err == nil {
			f.ProductID = &id
		} else {
			fields = append(fields, apperr.FieldError{Field: "productId", Code: "uuid", Message: "El identificador no es válido."})
		}
	}
	if f.Type != "" && !movementTypes[f.Type] {
		fields = append(fields, apperr.FieldError{Field: "type", Code: "oneof", Message: "El tipo de movimiento no es válido."})
	}
	for name, v := range map[string]string{"from": f.From, "to": f.To} {
		if v != "" && !datePattern.MatchString(v) {
			fields = append(fields, apperr.FieldError{Field: name, Code: "datetime", Message: "Usa el formato AAAA-MM-DD."})
		}
	}
	if len(fields) > 0 {
		httpx.Fail(w, r, apperr.Validation(fields...))
		return
	}
	p, _ := auth.FromContext(r.Context())
	items, total, err := h.svc.Movements(r.Context(), p, f, page)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, httpx.NewList(items, page, total))
}

func (h *Handler) alerts(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.FromContext(r.Context())
	items, err := h.svc.Alerts(r.Context(), p)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, map[string]any{"data": items})
}

var coverages = map[string]int{"": 14, "7": 7, "14": 14, "30": 30}

func (h *Handler) forecast(w http.ResponseWriter, r *http.Request) {
	coverage, ok := coverages[r.URL.Query().Get("coverage")]
	if !ok {
		httpx.Fail(w, r, apperr.Validation(apperr.FieldError{Field: "coverage", Code: "oneof", Message: "La cobertura debe ser de 7, 14 o 30 días."}))
		return
	}
	p, _ := auth.FromContext(r.Context())
	items, err := h.svc.Forecast(r.Context(), p, coverage)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, map[string]any{"data": items, "coverage": coverage, "historyDays": HistoryDays})
}

var expiryWindows = map[string]int{"": 30, "30": 30, "60": 60, "90": 90}

func (h *Handler) expiring(w http.ResponseWriter, r *http.Request) {
	days, ok := expiryWindows[r.URL.Query().Get("days")]
	if !ok {
		httpx.Fail(w, r, apperr.Validation(apperr.FieldError{Field: "days", Code: "oneof", Message: "La ventana debe ser de 30, 60 o 90 días."}))
		return
	}
	p, _ := auth.FromContext(r.Context())
	lots, err := h.svc.Expiring(r.Context(), p, days)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, map[string]any{"data": lots, "days": days})
}

type adjustRequest struct {
	ProductID uuid.UUID `json:"productId" validate:"required"`
	Kind      string    `json:"kind" validate:"required,oneof=count shrinkage"`
	Quantity  int       `json:"quantity" validate:"gte=0,lte=100000"`
	Reason    string    `json:"reason" validate:"required,min=3,max=300"`
}

func (h *Handler) adjust(w http.ResponseWriter, r *http.Request) {
	var req adjustRequest
	if err := httpx.Bind(w, r, &req); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	if req.Kind == AdjustShrinkage && req.Quantity == 0 {
		httpx.Fail(w, r, apperr.Validation(apperr.FieldError{Field: "quantity", Code: "gt", Message: "Debe ser mayor que 0."}))
		return
	}
	p, _ := auth.FromContext(r.Context())
	stock, err := h.svc.Adjust(r.Context(), p, AdjustInput{ProductID: req.ProductID, Kind: req.Kind, Quantity: req.Quantity, Reason: req.Reason})
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusCreated, map[string]int{"stock": stock})
}
