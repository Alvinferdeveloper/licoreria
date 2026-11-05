// Package reports runs the parameterized reports. Each report declares its filters, columns
// and query once; the same definition feeds the on-screen preview, the Excel export and the
// PDF export.
package reports

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"licoreria/api/internal/auth"
	"licoreria/api/internal/platform/database"
)

// ParamKind tells the client which control to render for a filter.
type ParamKind string

const (
	ParamDate     ParamKind = "date"
	ParamUser     ParamKind = "user"
	ParamCategory ParamKind = "category"
	ParamBrand    ParamKind = "brand"
	ParamSupplier ParamKind = "supplier"
	ParamRegister ParamKind = "register"
	ParamProduct  ParamKind = "product"
	ParamChoice   ParamKind = "choice"
	ParamBool     ParamKind = "bool"
	ParamText     ParamKind = "text"
)

type Choice struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type Param struct {
	Key      string    `json:"key"`
	Label    string    `json:"label"`
	Kind     ParamKind `json:"kind"`
	Required bool      `json:"required,omitempty"`
	Choices  []Choice  `json:"choices,omitempty"`
}

// ColumnKind drives formatting: money and number cells are right-aligned and totalled
// when Total is set.
type ColumnKind string

const (
	ColText     ColumnKind = "text"
	ColMono     ColumnKind = "mono"
	ColMoney    ColumnKind = "money"
	ColNumber   ColumnKind = "number"
	ColPercent  ColumnKind = "percent"
	ColDateTime ColumnKind = "datetime"
	ColDate     ColumnKind = "date"
)

type Column struct {
	Key   string     `json:"key"`
	Label string     `json:"label"`
	Kind  ColumnKind `json:"kind"`
	Total bool       `json:"total,omitempty"`
	// Width is a relative weight for PDF columns.
	Width int `json:"-"`
}

// Row values are strings (amounts as decimals, timestamps as RFC 3339) or ints.
type Row map[string]any

type Result struct {
	Key       string   `json:"key"`
	Title     string   `json:"title"`
	Columns   []Column `json:"columns"`
	Rows      []Row    `json:"rows"`
	Totals    Row      `json:"totals,omitempty"`
	Truncated bool     `json:"truncated"`
	// Filters echoes the applied filters in words, for export headers.
	Filters []string `json:"filters"`
}

// Definition is one report. Query receives validated params and the row limit.
type Definition struct {
	Key         string   `json:"key"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Group       string   `json:"group"`
	Params      []Param  `json:"params"`
	Columns     []Column `json:"columns"`
	// GroupBy names the column whose changes start a new group (master-detail reports).
	GroupBy    string `json:"groupBy,omitempty"`
	Permission string `json:"-"`
	query      func(ctx context.Context, db database.DBTX, p Params, limit int) ([]Row, error)
}

func (d Definition) allowed(p auth.Principal) bool {
	module, action, _ := strings.Cut(d.Permission, ":")
	return p.Can("reports", "view") && (d.Permission == "" || p.Can(module, action))
}

// Params are the validated filter values by key; Self restricts sales reports to the
// caller when they may only see their own sales.
type Params struct {
	Values map[string]string
	Self   *string
}

func (p Params) Get(key string) string { return p.Values[key] }

// where builds a WHERE clause with numbered placeholders; "?" in a condition is replaced by
// the next argument (every "?" in one condition refers to the same argument).
type where struct {
	conds []string
	args  []any
}

func (w *where) add(cond string, value any) {
	w.args = append(w.args, value)
	w.conds = append(w.conds, strings.ReplaceAll(cond, "?", fmt.Sprintf("$%d", len(w.args))))
}

// addIf adds the condition only when value is not empty.
func (w *where) addIf(cond, value string) {
	if value != "" {
		w.add(cond, value)
	}
}

// dateRange filters column by the store-local days [from, to].
func (w *where) dateRange(column string, p Params) {
	w.addIf(column+" >= (?::date)::timestamp AT TIME ZONE 'America/Managua'", p.Get("from"))
	w.addIf(column+" < (?::date + 1)::timestamp AT TIME ZONE 'America/Managua'", p.Get("to"))
}

func (w *where) sql() string {
	if len(w.conds) == 0 {
		return "true"
	}
	return strings.Join(w.conds, " AND ")
}

// collect runs the query and maps each row by column name.
func collect(ctx context.Context, db database.DBTX, query string, args ...any) ([]Row, error) {
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("run report: %w", err)
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Row, error) {
		values, err := r.Values()
		if err != nil {
			return nil, err
		}
		out := Row{}
		for i, f := range r.FieldDescriptions() {
			out[f.Name] = values[i]
		}
		return out, nil
	})
}
