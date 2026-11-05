package reports

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/auth"
)

const (
	PreviewLimit = 1000
	ExportLimit  = 20000
)

var (
	ErrUnknownReport = apperr.New(apperr.KindNotFound, "report_not_found", "Ese reporte no existe o no tienes acceso.")
	datePattern      = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

type Option struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Catalog struct {
	Reports []Definition        `json:"reports"`
	Options map[string][]Option `json:"options"`
}

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// seesAll: supervisors and read-only roles see everyone's sales; cashiers only their own.
func seesAll(p auth.Principal) bool {
	return p.Can("cash_shifts", "authorize") || !p.Can("pos", "create")
}

func (s *Service) Catalog(ctx context.Context, p auth.Principal) (Catalog, error) {
	c := Catalog{Reports: []Definition{}, Options: map[string][]Option{}}
	for _, d := range definitions {
		if d.allowed(p) {
			d.Params = s.visibleParams(p, d.Params)
			c.Reports = append(c.Reports, d)
		}
	}
	queries := map[string]string{
		"categories": `SELECT id::text, name FROM categories WHERE active ORDER BY name`,
		"brands":     `SELECT id::text, name::text FROM brands WHERE active ORDER BY name`,
		"suppliers":  `SELECT id::text, name FROM suppliers ORDER BY name`,
		"registers":  `SELECT id::text, name FROM registers ORDER BY name`,
	}
	if seesAll(p) {
		queries["users"] = `SELECT id::text, full_name FROM users ORDER BY full_name`
	}
	for key, q := range queries {
		rows, err := s.pool.Query(ctx, q)
		if err != nil {
			return c, fmt.Errorf("load report options: %w", err)
		}
		var opts []Option
		for rows.Next() {
			var o Option
			if err := rows.Scan(&o.ID, &o.Name); err != nil {
				rows.Close()
				return c, err
			}
			opts = append(opts, o)
		}
		rows.Close()
		c.Options[key] = opts
	}
	return c, nil
}

// visibleParams hides the user filter from cashiers, who only ever see their own data.
func (s *Service) visibleParams(p auth.Principal, params []Param) []Param {
	if seesAll(p) {
		return params
	}
	out := make([]Param, 0, len(params))
	for _, param := range params {
		if param.Kind != ParamUser {
			out = append(out, param)
		}
	}
	return out
}

// Run validates the filters, runs the report and adds the totals row.
func (s *Service) Run(ctx context.Context, p auth.Principal, key string, values map[string]string, limit int) (Result, error) {
	def, ok := find(key)
	if !ok || !def.allowed(p) {
		return Result{}, ErrUnknownReport
	}
	params, err := validate(def, values)
	if err != nil {
		return Result{}, err
	}
	if !seesAll(p) {
		self := p.UserID.String()
		params.Self = &self
	}
	rows, err := def.query(ctx, s.pool, params, limit)
	if err != nil {
		return Result{}, err
	}
	res := Result{Key: def.Key, Title: def.Title, Columns: def.Columns, Rows: rows}
	if len(rows) > limit {
		res.Rows, res.Truncated = rows[:limit], true
	}
	if res.Rows == nil {
		res.Rows = []Row{}
	}
	res.Totals = totals(def.Columns, res.Rows)
	res.Filters = s.describe(ctx, def, params)
	return res, nil
}

func find(key string) (Definition, bool) {
	for _, d := range definitions {
		if d.Key == key {
			return d, true
		}
	}
	return Definition{}, false
}

func validate(def Definition, values map[string]string) (Params, error) {
	params := Params{Values: map[string]string{}}
	var fields []apperr.FieldError
	fail := func(key, msg string) {
		fields = append(fields, apperr.FieldError{Field: key, Code: "invalid", Message: msg})
	}
	for _, param := range def.Params {
		v := strings.TrimSpace(values[param.Key])
		if v == "" {
			if param.Required {
				fields = append(fields, apperr.FieldError{Field: param.Key, Code: "required", Message: "Este filtro es obligatorio."})
			}
			continue
		}
		switch param.Kind {
		case ParamDate:
			if !datePattern.MatchString(v) {
				fail(param.Key, "Usa el formato AAAA-MM-DD.")
			}
		case ParamUser, ParamCategory, ParamBrand, ParamSupplier, ParamRegister, ParamProduct:
			if _, err := uuid.Parse(v); err != nil {
				fail(param.Key, "El identificador no es válido.")
			}
		case ParamChoice:
			valid := false
			for _, c := range param.Choices {
				valid = valid || c.Value == v
			}
			if !valid {
				fail(param.Key, "La opción no es válida.")
			}
		case ParamBool:
			if v != "true" && v != "false" {
				fail(param.Key, "Usa true o false.")
			}
		case ParamText:
			if len(v) > 80 {
				fail(param.Key, "Debe tener como máximo 80 caracteres.")
			}
		}
		params.Values[param.Key] = v
	}
	if f, t := params.Get("from"), params.Get("to"); f != "" && t != "" && f > t {
		fail("to", "La fecha final no puede ser anterior a la inicial.")
	}
	if len(fields) > 0 {
		return Params{}, apperr.Validation(fields...)
	}
	return params, nil
}

// totals adds up the columns marked Total; money is summed in cents.
func totals(columns []Column, rows []Row) Row {
	out := Row{}
	hasTotals := false
	for _, c := range columns {
		if !c.Total {
			continue
		}
		hasTotals = true
		var sum int64
		for _, r := range rows {
			switch v := r[c.Key].(type) {
			case string:
				sum += cents(v)
			case int32:
				sum += int64(v)
			case int64:
				sum += v
			case int:
				sum += int64(v)
			}
		}
		if c.Kind == ColMoney {
			out[c.Key] = formatCents(sum)
		} else {
			out[c.Key] = sum
		}
	}
	if !hasTotals {
		return nil
	}
	return out
}

func cents(v string) int64 {
	neg := strings.HasPrefix(v, "-")
	whole, frac, _ := strings.Cut(strings.TrimPrefix(v, "-"), ".")
	frac = (frac + "00")[:2]
	n, err := strconv.ParseInt(whole+frac, 10, 64)
	if err != nil {
		return 0
	}
	if neg {
		return -n
	}
	return n
}

func formatCents(c int64) string {
	sign := ""
	if c < 0 {
		sign, c = "-", -c
	}
	return fmt.Sprintf("%s%d.%02d", sign, c/100, c%100)
}

// describe turns the applied filters into readable lines for export headers.
func (s *Service) describe(ctx context.Context, def Definition, p Params) []string {
	names := map[ParamKind]string{
		ParamUser: "users", ParamCategory: "categories", ParamBrand: "brands", ParamSupplier: "suppliers",
		ParamRegister: "registers", ParamProduct: "products",
	}
	var lines []string
	for _, param := range def.Params {
		v := p.Get(param.Key)
		if v == "" {
			continue
		}
		label := v
		switch param.Kind {
		case ParamChoice:
			for _, c := range param.Choices {
				if c.Value == v {
					label = c.Label
				}
			}
		case ParamBool:
			label = map[string]string{"true": "Sí", "false": "No"}[v]
		case ParamDate:
			label = v[8:10] + "/" + v[5:7] + "/" + v[0:4]
		default:
			if table, ok := names[param.Kind]; ok {
				column := map[string]string{"users": "full_name", "products": "name"}[table]
				if column == "" {
					column = "name"
				}
				_ = s.pool.QueryRow(ctx, fmt.Sprintf(`SELECT %s::text FROM %s WHERE id = $1`, column, table), v).Scan(&label)
			}
		}
		lines = append(lines, param.Label+": "+label)
	}
	if p.Self != nil {
		lines = append(lines, "Solo tus ventas")
	}
	return lines
}
