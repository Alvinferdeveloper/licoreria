package database

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"licoreria/api/internal/apperr"
)

func TestMapError(t *testing.T) {
	fields := ConstraintFields{"products_sku_key": "sku", "products_category_id_fkey": "categoryId"}

	cases := []struct {
		name      string
		err       error
		wantCode  string
		wantKind  apperr.Kind
		wantField string
	}{
		{"no rows", pgx.ErrNoRows, "not_found", apperr.KindNotFound, ""},
		{"unique with known field", &pgconn.PgError{Code: "23505", ConstraintName: "products_sku_key"}, "already_exists", apperr.KindConflict, "sku"},
		{"unique unknown constraint", &pgconn.PgError{Code: "23505", ConstraintName: "other"}, "already_exists", apperr.KindConflict, ""},
		{"fk on insert", &pgconn.PgError{Code: "23503", ConstraintName: "products_category_id_fkey", Detail: `Key (category_id)=(x) is not present in table "categories".`}, "invalid_reference", apperr.KindValidation, "categoryId"},
		{"fk on delete", &pgconn.PgError{Code: "23503", Detail: `Key (id)=(x) is still referenced from table "products".`}, "in_use", apperr.KindConflict, ""},
		{"not null", &pgconn.PgError{Code: "23502", ColumnName: "invoice_series"}, "validation_failed", apperr.KindValidation, "invoiceSeries"},
		{"check", &pgconn.PgError{Code: "23514"}, "validation_failed", apperr.KindValidation, ""},
		{"deadlock", &pgconn.PgError{Code: "40P01"}, "concurrent_update", apperr.KindConflict, ""},
		{"wrapped", fmt.Errorf("insert product: %w", &pgconn.PgError{Code: "23505", ConstraintName: "products_sku_key"}), "already_exists", apperr.KindConflict, "sku"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			appErr, ok := apperr.As(MapError(tc.err, fields))
			if !ok {
				t.Fatalf("expected app error for %v", tc.err)
			}
			if appErr.Code != tc.wantCode || appErr.Kind != tc.wantKind {
				t.Errorf("got %s/%d, want %s/%d", appErr.Code, appErr.Kind, tc.wantCode, tc.wantKind)
			}
			gotField := ""
			if len(appErr.Fields) > 0 {
				gotField = appErr.Fields[0].Field
			}
			if gotField != tc.wantField {
				t.Errorf("field = %q, want %q", gotField, tc.wantField)
			}
			if !errors.Is(appErr, tc.err) {
				t.Error("cause must be preserved for logging")
			}
		})
	}
}

func TestMapErrorPassesThroughUnknownErrors(t *testing.T) {
	original := errors.New("connection reset")
	if got := MapError(original, nil); got != original {
		t.Errorf("unknown error should pass through, got %v", got)
	}
	if MapError(nil, nil) != nil {
		t.Error("nil must stay nil")
	}
}

func TestToMigrateURL(t *testing.T) {
	cases := map[string]string{
		"postgres://u:p@localhost:5432/db":   "pgx5://u:p@localhost:5432/db",
		"postgresql://u:p@localhost:5432/db": "pgx5://u:p@localhost:5432/db",
		"pgx5://already":                     "pgx5://already",
	}
	for in, want := range cases {
		if got := toMigrateURL(in); got != want {
			t.Errorf("toMigrateURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestToCamel(t *testing.T) {
	cases := map[string]string{"invoice_series": "invoiceSeries", "email": "email", "opening_nio_amount": "openingNioAmount"}
	for in, want := range cases {
		if got := toCamel(in); got != want {
			t.Errorf("toCamel(%q) = %q, want %q", in, got, want)
		}
	}
}
