package httpx

import (
	"net/http"
	"strconv"

	"licoreria/api/internal/apperr"
)

const (
	DefaultPageSize = 25
	MaxPageSize     = 100
)

type Page struct {
	Number int
	Size   int
}

func (p Page) Offset() int { return (p.Number - 1) * p.Size }

// ParsePage reads ?page and ?pageSize, defaulting to the first page of 25 items.
func ParsePage(r *http.Request) (Page, error) {
	page := Page{Number: 1, Size: DefaultPageSize}
	var fields []apperr.FieldError

	if raw := r.URL.Query().Get("page"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			fields = append(fields, apperr.FieldError{Field: "page", Code: "min", Message: "Debe ser un número mayor o igual a 1."})
		}
		page.Number = n
	}
	if raw := r.URL.Query().Get("pageSize"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > MaxPageSize {
			fields = append(fields, apperr.FieldError{Field: "pageSize", Code: "range", Message: "Debe estar entre 1 y 100."})
		}
		page.Size = n
	}

	if len(fields) > 0 {
		return Page{}, apperr.Validation(fields...)
	}
	return page, nil
}

type PageMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
}

// List is the envelope for every paginated collection.
type List[T any] struct {
	Data []T      `json:"data"`
	Meta PageMeta `json:"meta"`
}

func NewList[T any](items []T, page Page, total int64) List[T] {
	if items == nil {
		items = []T{}
	}
	return List[T]{Data: items, Meta: PageMeta{Page: page.Number, PageSize: page.Size, Total: total}}
}
