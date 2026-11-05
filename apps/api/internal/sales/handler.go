package sales

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
	r.Route("/sales", func(r chi.Router) {
		r.With(auth.Require("sales", "view")).Get("/", h.list)
		r.With(auth.Require("pos", "create")).Post("/", h.create)
		r.With(auth.Require("sales", "view")).Get("/{id}", h.get)
		r.With(auth.Require("sales", "view")).Post("/{id}/void", h.void)
	})
}

var ErrMissingKey = apperr.Validation(apperr.FieldError{Field: "Idempotency-Key", Code: "required", Message: "Falta la clave de la venta (Idempotency-Key)."})

type lineRequest struct {
	ProductID      uuid.UUID  `json:"productId" validate:"required"`
	PresentationID *uuid.UUID `json:"presentationId"`
	Quantity       int        `json:"quantity" validate:"gt=0,lte=10000"`
}

type paymentRequest struct {
	Method    string  `json:"method" validate:"required,oneof=cash card transfer"`
	Currency  string  `json:"currency" validate:"required,oneof=NIO USD"`
	Amount    string  `json:"amount" validate:"required,money"`
	Reference *string `json:"reference" validate:"omitempty,max=60"`
}

type discountRequest struct {
	Kind  string `json:"kind" validate:"required,oneof=percent amount"`
	Value string `json:"value" validate:"required,money"`
}

type authorizationRequest struct {
	Username string `json:"username" validate:"required,max=32"`
	PIN      string `json:"pin" validate:"required,max=6"`
}

type saleRequest struct {
	CustomerID    *uuid.UUID            `json:"customerId"`
	AgeVerified   bool                  `json:"ageVerified"`
	Notes         string                `json:"notes" validate:"max=300"`
	Discount      *discountRequest      `json:"discount"`
	Authorization *authorizationRequest `json:"authorization"`
	Lines         []lineRequest         `json:"lines" validate:"required,min=1,max=200,dive"`
	Payments      []paymentRequest      `json:"payments" validate:"required,min=1,max=10,dive"`
}

func (req saleRequest) input() Input {
	in := Input{CustomerID: req.CustomerID, AgeVerified: req.AgeVerified, Notes: req.Notes}
	if req.Discount != nil {
		in.Discount = &Discount{Kind: req.Discount.Kind, Value: req.Discount.Value}
	}
	if req.Authorization != nil {
		in.Authorization = &Authorization{Username: req.Authorization.Username, PIN: req.Authorization.PIN}
	}
	for _, l := range req.Lines {
		in.Lines = append(in.Lines, LineInput{ProductID: l.ProductID, PresentationID: l.PresentationID, Quantity: l.Quantity})
	}
	for _, p := range req.Payments {
		in.Payments = append(in.Payments, PaymentInput{Method: p.Method, Currency: p.Currency, Amount: p.Amount, Reference: p.Reference})
	}
	return in
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	key, err := uuid.Parse(r.Header.Get("Idempotency-Key"))
	if err != nil {
		httpx.Fail(w, r, ErrMissingKey)
		return
	}
	var req saleRequest
	if err := httpx.Bind(w, r, &req); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	p, _ := auth.FromContext(r.Context())
	sale, created, err := h.svc.Create(r.Context(), p, key, req.input())
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	httpx.JSON(w, r, status, sale)
}

var datePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	page, err := httpx.ParsePage(r)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	q := r.URL.Query()
	f := Filter{Status: q.Get("status"), Search: q.Get("search"), From: q.Get("from"), To: q.Get("to")}
	var fields []apperr.FieldError
	if f.Status != "" && f.Status != "completed" && f.Status != "voided" {
		fields = append(fields, apperr.FieldError{Field: "status", Code: "oneof", Message: "El estado no es válido."})
	}
	for name, v := range map[string]string{"from": f.From, "to": f.To} {
		if v != "" && !datePattern.MatchString(v) {
			fields = append(fields, apperr.FieldError{Field: name, Code: "datetime", Message: "Usa el formato AAAA-MM-DD."})
		}
	}
	if raw := q.Get("shiftId"); raw != "" {
		if id, err := uuid.Parse(raw); err == nil {
			f.ShiftID = &id
		} else {
			fields = append(fields, apperr.FieldError{Field: "shiftId", Code: "uuid", Message: "El identificador no es válido."})
		}
	}
	if len(fields) > 0 {
		httpx.Fail(w, r, apperr.Validation(fields...))
		return
	}
	p, _ := auth.FromContext(r.Context())
	items, total, err := h.svc.List(r.Context(), p, f, page)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, httpx.NewList(items, page, total))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.URLParamUUID(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	p, _ := auth.FromContext(r.Context())
	sale, err := h.svc.Get(r.Context(), p, id)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, sale)
}

type voidRequest struct {
	Reason        string                `json:"reason" validate:"required,min=3,max=300"`
	Authorization *authorizationRequest `json:"authorization"`
}

func (h *Handler) void(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.URLParamUUID(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	var req voidRequest
	if err := httpx.Bind(w, r, &req); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	var authz *Authorization
	if req.Authorization != nil {
		authz = &Authorization{Username: req.Authorization.Username, PIN: req.Authorization.PIN}
	}
	p, _ := auth.FromContext(r.Context())
	sale, err := h.svc.Void(r.Context(), p, id, req.Reason, authz)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, sale)
}
