// Package parties manages customers and suppliers. Both are deactivated, never deleted,
// because sales and purchases reference them.
package parties

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/audit"
	"licoreria/api/internal/auth"
	"licoreria/api/internal/httpx"
	"licoreria/api/internal/platform/database"
)

type Filter struct {
	Search string
	Active *bool
}

func parseFilter(r *http.Request) (Filter, error) {
	f := Filter{Search: strings.TrimSpace(r.URL.Query().Get("search"))}
	if raw := r.URL.Query().Get("active"); raw != "" {
		active, err := strconv.ParseBool(raw)
		if err != nil {
			return Filter{}, apperr.Validation(apperr.FieldError{Field: "active", Code: "boolean", Message: "Usa true o false."})
		}
		f.Active = &active
	}
	return f, nil
}

// whereClause builds the shared search/active filter over the given searchable columns.
func whereClause(f Filter, columns ...string) (string, []any) {
	where, args := []string{"true"}, []any{}
	if f.Search != "" {
		args = append(args, "%"+f.Search+"%")
		var ors []string
		for _, col := range columns {
			ors = append(ors, fmt.Sprintf("%s ILIKE $1", col))
		}
		where = append(where, "("+strings.Join(ors, " OR ")+")")
	}
	if f.Active != nil {
		args = append(args, *f.Active)
		where = append(where, fmt.Sprintf("active = $%d", len(args)))
	}
	return strings.Join(where, " AND "), args
}

func listPage[T any](ctx context.Context, db database.DBTX, table, selectCols, condition, order string, args []any, page httpx.Page, scan func(pgx.Row) (T, error)) ([]T, int64, error) {
	var total int64
	if err := db.QueryRow(ctx, fmt.Sprintf("SELECT count(*) FROM %s WHERE %s", table, condition), args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count %s: %w", table, err)
	}
	args = append(args, page.Size, page.Offset())
	rows, err := db.Query(ctx, fmt.Sprintf("SELECT %s FROM %s WHERE %s ORDER BY %s LIMIT $%d OFFSET $%d",
		selectCols, table, condition, order, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list %s: %w", table, err)
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (T, error) { return scan(row) })
	if err != nil {
		return nil, 0, fmt.Errorf("read %s: %w", table, err)
	}
	return items, total, nil
}

// setActive toggles a row and records it in the audit log.
func setActive(ctx context.Context, pool *pgxpool.Pool, actor uuid.UUID, table string, id uuid.UUID, active bool) error {
	return database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, fmt.Sprintf("UPDATE %s SET active = $2 WHERE id = $1", table), id, active)
		if err != nil {
			return database.MapError(err, nil)
		}
		if tag.RowsAffected() == 0 {
			return apperr.ErrNotFound
		}
		return audit.Record(ctx, tx, audit.Entry{UserID: &actor, Action: table + ".status_changed", Entity: table, EntityID: &id,
			After: map[string]bool{"active": active}})
	})
}

func clean(v string) string { return strings.Join(strings.Fields(v), " ") }

func cleanPtr(v *string) *string {
	if v == nil {
		return nil
	}
	if trimmed := strings.TrimSpace(*v); trimmed != "" {
		return &trimmed
	}
	return nil
}

type statusRequest struct {
	Active *bool `json:"active" validate:"required"`
}

// mountCRUD wires the common list/get/create/update/status routes for one party type.
func mountCRUD(r chi.Router, path, module string, h crudHandlers) {
	r.Route(path, func(r chi.Router) {
		r.With(auth.Require(module, "view")).Get("/", h.list)
		r.With(auth.Require(module, "create")).Post("/", h.create)
		r.With(auth.Require(module, "view")).Get("/{id}", h.get)
		r.With(auth.Require(module, "update")).Put("/{id}", h.update)
		r.With(auth.Require(module, "delete")).Patch("/{id}/status", h.setStatus)
	})
}

type crudHandlers struct {
	list, create, get, update, setStatus http.HandlerFunc
}

// respond runs fn and writes its result or the error envelope.
func respond[T any](w http.ResponseWriter, r *http.Request, status int, fn func() (T, error)) {
	result, err := fn()
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, status, result)
}

func actorID(r *http.Request) uuid.UUID {
	p, _ := auth.FromContext(r.Context())
	return p.UserID
}
