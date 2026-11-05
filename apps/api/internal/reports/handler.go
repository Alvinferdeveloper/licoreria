package reports

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/auth"
	"licoreria/api/internal/httpx"
	"licoreria/api/internal/settings"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Routes(r chi.Router) {
	r.Route("/reports", func(r chi.Router) {
		r.Use(auth.Require("reports", "view"))
		r.Get("/", h.catalog)
		r.Get("/{key}", h.run)
	})
}

func (h *Handler) catalog(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.FromContext(r.Context())
	c, err := h.svc.Catalog(r.Context(), p)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, c)
}

// run returns JSON by default; ?format=xlsx or ?format=pdf downloads the export.
func (h *Handler) run(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	format := r.URL.Query().Get("format")
	if format != "" && format != "json" && format != "xlsx" && format != "pdf" {
		httpx.Fail(w, r, apperr.Validation(apperr.FieldError{Field: "format", Code: "oneof", Message: "Usa json, xlsx o pdf."}))
		return
	}
	values := map[string]string{}
	for k, v := range r.URL.Query() {
		if k != "format" && len(v) > 0 {
			values[k] = v[0]
		}
	}
	limit := PreviewLimit
	if format == "xlsx" || format == "pdf" {
		limit = ExportLimit
	}
	p, _ := auth.FromContext(r.Context())
	res, err := h.svc.Run(r.Context(), p, key, values, limit)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	if format == "" || format == "json" {
		httpx.JSON(w, r, http.StatusOK, res)
		return
	}

	def, _ := find(key)
	business, err := settings.GetBusiness(r.Context(), h.svc.pool)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	var data []byte
	contentType := "application/pdf"
	if format == "xlsx" {
		data, err = Excel(def, res, business.Name)
		contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	} else {
		data, err = PDF(def, res, business.Name)
	}
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+key+`.`+format+`"`)
	_, _ = w.Write(data)
}
