package products

import (
	"errors"
	"io"
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

func (h *Handler) Routes(r chi.Router) {
	r.Route("/products", func(r chi.Router) {
		r.With(auth.Require("products", "view")).Get("/", h.list)
		r.With(auth.Require("products", "view")).Get("/lookup", h.lookup)
		r.With(auth.Require("products", "create")).Post("/", h.create)
		r.With(auth.Require("products", "create")).Get("/sku-suggestion", h.suggestSKU)
		r.With(auth.Require("products", "create")).Get("/import/template", h.importTemplate)
		r.With(auth.Require("products", "create")).Post("/import", h.importFile)
		r.With(auth.Require("products", "view")).Get("/{id}", h.get)
		r.With(auth.Require("products", "update")).Put("/{id}", h.update)
		r.With(auth.Require("products", "delete")).Patch("/{id}/status", h.setStatus)
		r.With(auth.Require("products", "update")).Patch("/{id}/price", h.setPrice)
		r.With(auth.Require("products", "update")).Post("/{id}/image", h.uploadImage)
		r.With(auth.Require("products", "update")).Delete("/{id}/image", h.removeImage)
	})
}

type presentationRequest struct {
	ID      *uuid.UUID `json:"id"`
	Name    string     `json:"name" validate:"required,min=2,max=60"`
	Factor  int        `json:"factor" validate:"required,gt=1,lte=1000"`
	Barcode *string    `json:"barcode" validate:"omitempty,max=32"`
	Price   string     `json:"price" validate:"required,money"`
}

type productRequest struct {
	SKU                string                `json:"sku" validate:"required,min=2,max=40"`
	Barcode            *string               `json:"barcode" validate:"omitempty,max=32"`
	Name               string                `json:"name" validate:"required,min=2,max=160"`
	Description        string                `json:"description" validate:"max=1000"`
	CategoryID         uuid.UUID             `json:"categoryId" validate:"required"`
	BrandID            *uuid.UUID            `json:"brandId"`
	UnitID             uuid.UUID             `json:"unitId" validate:"required"`
	AlcoholPercent     *string               `json:"alcoholPercent" validate:"omitempty,percent"`
	VolumeML           *int                  `json:"volumeMl" validate:"omitempty,gt=0,lte=100000"`
	Price              string                `json:"price" validate:"required,money"`
	AverageCost        string                `json:"averageCost" validate:"omitempty,money"`
	TaxPercent         string                `json:"taxPercent" validate:"omitempty,percent"`
	MinStock           int                   `json:"minStock" validate:"gte=0"`
	ReorderPoint       int                   `json:"reorderPoint" validate:"gte=0"`
	AllowNegativeStock bool                  `json:"allowNegativeStock"`
	Presentations      []presentationRequest `json:"presentations" validate:"max=10,dive"`
}

func (req productRequest) input() Input {
	in := Input{
		SKU: req.SKU, Barcode: req.Barcode, Name: req.Name, Description: req.Description,
		CategoryID: req.CategoryID, BrandID: req.BrandID, UnitID: req.UnitID,
		AlcoholPercent: req.AlcoholPercent, VolumeML: req.VolumeML,
		Price: req.Price, AverageCost: req.AverageCost, TaxPercent: req.TaxPercent,
		MinStock: req.MinStock, ReorderPoint: req.ReorderPoint, AllowNegativeStock: req.AllowNegativeStock,
	}
	for _, p := range req.Presentations {
		in.Presentations = append(in.Presentations, PresentationInput{ID: p.ID, Name: p.Name, Factor: p.Factor, Barcode: p.Barcode, Price: p.Price})
	}
	return in
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
	p, _ := auth.FromContext(r.Context())
	items, total, err := h.svc.List(r.Context(), p, filter, page)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, httpx.NewList(items, page, total))
}

func parseFilter(r *http.Request) (Filter, error) {
	q := r.URL.Query()
	f := Filter{Search: q.Get("search"), LowStock: q.Get("lowStock") == "true"}
	for _, param := range []struct {
		name string
		dst  **uuid.UUID
	}{{"categoryId", &f.CategoryID}, {"brandId", &f.BrandID}} {
		if raw := q.Get(param.name); raw != "" {
			id, err := uuid.Parse(raw)
			if err != nil {
				return Filter{}, apperr.Validation(apperr.FieldError{Field: param.name, Code: "uuid", Message: "El identificador no es válido."})
			}
			*param.dst = &id
		}
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

func (h *Handler) suggestSKU(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var brandID *uuid.UUID
	if raw := q.Get("brandId"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httpx.Fail(w, r, apperr.Validation(apperr.FieldError{Field: "brandId", Code: "uuid", Message: "El identificador no es válido."}))
			return
		}
		brandID = &id
	}
	sku, err := h.svc.SuggestSKU(r.Context(), brandID, q.Get("name"))
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, map[string]string{"sku": sku})
}

func (h *Handler) lookup(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		httpx.Fail(w, r, apperr.Validation(apperr.FieldError{Field: "code", Code: "required", Message: "Este campo es obligatorio."}))
		return
	}
	p, _ := auth.FromContext(r.Context())
	result, err := h.svc.Lookup(r.Context(), p, code)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, result)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.URLParamUUID(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	p, _ := auth.FromContext(r.Context())
	product, err := h.svc.Get(r.Context(), p, id)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, product)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req productRequest
	if err := httpx.Bind(w, r, &req); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	p, _ := auth.FromContext(r.Context())
	product, err := h.svc.Create(r.Context(), p, req.input())
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusCreated, product)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.URLParamUUID(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	var req productRequest
	if err := httpx.Bind(w, r, &req); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	p, _ := auth.FromContext(r.Context())
	product, err := h.svc.Update(r.Context(), p, id, req.input())
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, product)
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
	p, _ := auth.FromContext(r.Context())
	product, err := h.svc.SetActive(r.Context(), p, id, *req.Active)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, product)
}

type priceRequest struct {
	Price string `json:"price" validate:"required,money"`
}

func (h *Handler) setPrice(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.URLParamUUID(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	var req priceRequest
	if err := httpx.Bind(w, r, &req); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	p, _ := auth.FromContext(r.Context())
	product, err := h.svc.SetPrice(r.Context(), p, id, req.Price)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, product)
}

// uploadImage accepts multipart/form-data with the file in the "image" field.
func (h *Handler) uploadImage(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.URLParamUUID(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxImageBytes+64<<10)
	file, _, err := r.FormFile("image")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			httpx.Fail(w, r, ErrInvalidImage)
			return
		}
		httpx.Fail(w, r, apperr.Validation(apperr.FieldError{Field: "image", Code: "required", Message: "Selecciona una imagen."}))
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, MaxImageBytes+1))
	if err != nil {
		httpx.Fail(w, r, ErrInvalidImage)
		return
	}
	p, _ := auth.FromContext(r.Context())
	product, err := h.svc.SetImage(r.Context(), p, id, data)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, product)
}

// importFile accepts multipart/form-data with the sheet in the "file" field. With
// ?dryRun=true it only returns the row-by-row preview.
func (h *Handler) importFile(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxImportBytes+64<<10)
	file, _, err := r.FormFile("file")
	if err != nil {
		httpx.Fail(w, r, ErrImportFile)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, MaxImportBytes+1))
	if err != nil || len(data) == 0 || len(data) > MaxImportBytes {
		httpx.Fail(w, r, ErrImportFile)
		return
	}
	p, _ := auth.FromContext(r.Context())
	report, err := h.svc.Import(r.Context(), p, data, r.URL.Query().Get("dryRun") == "true")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, report)
}

func (h *Handler) importTemplate(w http.ResponseWriter, r *http.Request) {
	data, err := h.svc.ImportTemplate(r.Context())
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="plantilla-productos.xlsx"`)
	_, _ = w.Write(data)
}

func (h *Handler) removeImage(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.URLParamUUID(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	p, _ := auth.FromContext(r.Context())
	product, err := h.svc.RemoveImage(r.Context(), p, id)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, product)
}
