package purchases

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
	r.Route("/purchases", func(r chi.Router) {
		r.With(auth.Require("purchases", "view")).Get("/", h.list)
		r.With(auth.Require("purchases", "create")).Post("/", h.create)
		r.With(auth.Require("purchases", "view")).Get("/{id}", h.get)
		r.With(auth.Require("purchases", "update")).Put("/{id}", h.update)
		r.With(auth.Require("purchases", "update")).Post("/{id}/receive", h.receive)
		r.With(auth.Require("purchases", "void")).Post("/{id}/void", h.void)
	})
}

type itemRequest struct {
	ProductID uuid.UUID `json:"productId" validate:"required"`
	Quantity  int       `json:"quantity" validate:"gt=0,lte=100000"`
	UnitCost  string    `json:"unitCost" validate:"required,money"`
	ExpiresOn *string   `json:"expiresOn" validate:"omitempty,datetime=2006-01-02"`
}

type purchaseRequest struct {
	SupplierID            uuid.UUID     `json:"supplierId" validate:"required"`
	SupplierInvoiceNumber string        `json:"supplierInvoiceNumber" validate:"max=40"`
	PurchaseDate          string        `json:"purchaseDate" validate:"required,datetime=2006-01-02"`
	Notes                 string        `json:"notes" validate:"max=500"`
	Items                 []itemRequest `json:"items" validate:"required,min=1,max=200,dive"`
}

func (req purchaseRequest) input() Input {
	in := Input{SupplierID: req.SupplierID, SupplierInvoiceNumber: req.SupplierInvoiceNumber, PurchaseDate: req.PurchaseDate, Notes: req.Notes}
	for _, item := range req.Items {
		in.Items = append(in.Items, ItemInput{ProductID: item.ProductID, Quantity: item.Quantity, UnitCost: item.UnitCost, ExpiresOn: item.ExpiresOn})
	}
	return in
}

var statuses = map[string]bool{StatusDraft: true, StatusReceived: true, StatusVoided: true}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	page, err := httpx.ParsePage(r)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	q := r.URL.Query()
	f := Filter{Status: q.Get("status"), Search: q.Get("search")}
	if f.Status != "" && !statuses[f.Status] {
		httpx.Fail(w, r, apperr.Validation(apperr.FieldError{Field: "status", Code: "oneof", Message: "El estado no es válido."}))
		return
	}
	if raw := q.Get("supplierId"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httpx.Fail(w, r, apperr.Validation(apperr.FieldError{Field: "supplierId", Code: "uuid", Message: "El identificador no es válido."}))
			return
		}
		f.SupplierID = &id
	}
	items, total, err := h.svc.List(r.Context(), f, page)
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
	purchase, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, purchase)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req purchaseRequest
	if err := httpx.Bind(w, r, &req); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	p, _ := auth.FromContext(r.Context())
	purchase, err := h.svc.Create(r.Context(), p, req.input())
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusCreated, purchase)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.URLParamUUID(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	var req purchaseRequest
	if err := httpx.Bind(w, r, &req); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	p, _ := auth.FromContext(r.Context())
	purchase, err := h.svc.Update(r.Context(), p, id, req.input())
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, purchase)
}

func (h *Handler) receive(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.URLParamUUID(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	p, _ := auth.FromContext(r.Context())
	purchase, err := h.svc.Receive(r.Context(), p, id)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, purchase)
}

type voidRequest struct {
	Reason string `json:"reason" validate:"required,min=3,max=300"`
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
	p, _ := auth.FromContext(r.Context())
	purchase, err := h.svc.Void(r.Context(), p, id, req.Reason)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, purchase)
}
