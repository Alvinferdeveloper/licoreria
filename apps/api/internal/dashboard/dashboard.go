// Package dashboard aggregates the home panel figures in the database: KPIs, daily sales,
// top products, category share, payment mix and peak hours.
package dashboard

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"licoreria/api/internal/auth"
	"licoreria/api/internal/httpx"
)

type KPIs struct {
	SalesToday     string  `json:"salesToday"`
	SalesYesterday string  `json:"salesYesterday"`
	SalesCount     int     `json:"salesCount"`
	AverageTicket  string  `json:"averageTicket"`
	ProfitToday    *string `json:"profitToday,omitempty"`
	LowStock       int     `json:"lowStock"`
}

type DayPoint struct {
	Date     string `json:"date"`
	Total    string `json:"total"`
	Previous string `json:"previous"`
}

type ProductPoint struct {
	Name    string `json:"name"`
	Units   int    `json:"units"`
	Revenue string `json:"revenue"`
}

type CategoryPoint struct {
	Name  string `json:"name"`
	Total string `json:"total"`
}

type MethodPoint struct {
	Date     string `json:"date"`
	CashNIO  string `json:"cashNio"`
	CashUSD  string `json:"cashUsd"`
	Card     string `json:"card"`
	Transfer string `json:"transfer"`
}

type HeatPoint struct {
	Weekday int    `json:"weekday"`
	Hour    int    `json:"hour"`
	Sales   int    `json:"sales"`
	Total   string `json:"total"`
}

type Dashboard struct {
	Scope      string          `json:"scope"`
	KPIs       KPIs            `json:"kpis"`
	Daily      []DayPoint      `json:"daily"`
	Top        []ProductPoint  `json:"top"`
	Categories []CategoryPoint `json:"categories"`
	Methods    []MethodPoint   `json:"methods"`
	Heatmap    []HeatPoint     `json:"heatmap"`
}

type Handler struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Handler { return &Handler{pool: pool} }

func (h *Handler) Routes(r chi.Router) {
	r.With(auth.Require("dashboard", "view")).Get("/dashboard", h.get)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.FromContext(r.Context())
	d, err := Load(r.Context(), h.pool, p)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, d)
}

const day = "(now() AT TIME ZONE 'America/Managua')::date"

// localDay converts a timestamptz column to the store's calendar day.
func localDay(column string) string { return "(" + column + " AT TIME ZONE 'America/Managua')::date" }

// Load computes the panel. Cashiers see only their own sales ("scope": "self").
func Load(ctx context.Context, pool *pgxpool.Pool, p auth.Principal) (Dashboard, error) {
	d := Dashboard{Scope: "all"}
	// $1 is NULL for everyone or the user id for a cashier.
	var self any
	if !p.Can("cash_shifts", "authorize") && p.Can("pos", "create") {
		self, d.Scope = p.UserID, "self"
	}
	sales := "s.status = 'completed' AND ($1::uuid IS NULL OR s.user_id = $1)"

	var profit string
	err := pool.QueryRow(ctx, `
		SELECT COALESCE(sum(total) FILTER (WHERE `+localDay("created_at")+` = `+day+`), 0)::text,
		       COALESCE(sum(total) FILTER (WHERE `+localDay("created_at")+` = `+day+` - 1), 0)::text,
		       count(*) FILTER (WHERE `+localDay("created_at")+` = `+day+`)::int,
		       COALESCE(round(avg(total) FILTER (WHERE `+localDay("created_at")+` = `+day+`), 2), 0)::text,
		       COALESCE((SELECT sum(i.subtotal - i.discount - round(i.quantity * i.units_per_item * i.unit_cost, 2))
		                 FROM sale_items i JOIN sales s2 ON s2.id = i.sale_id
		                 WHERE s2.id IN (SELECT id FROM sales s WHERE `+sales+` AND `+localDay("s.created_at")+` = `+day+`)), 0)::text,
		       (SELECT count(*)::int FROM products WHERE active AND stock <= min_stock)
		FROM sales s WHERE `+sales+` AND s.created_at >= now() - interval '3 days'`, self).
		Scan(&d.KPIs.SalesToday, &d.KPIs.SalesYesterday, &d.KPIs.SalesCount, &d.KPIs.AverageTicket, &profit, &d.KPIs.LowStock)
	if err != nil {
		return d, fmt.Errorf("dashboard kpis: %w", err)
	}
	if p.Can("products", "authorize") {
		d.KPIs.ProfitToday = &profit
	}

	if d.Daily, err = collect(ctx, pool, `
		WITH days AS (SELECT generate_series(`+day+` - 29, `+day+`, interval '1 day')::date AS d),
		     totals AS (SELECT `+localDay("s.created_at")+` AS d, sum(s.total) AS t FROM sales s
		                WHERE `+sales+` AND s.created_at >= now() - interval '62 days' GROUP BY 1)
		SELECT days.d::text, COALESCE(cur.t, 0)::text, COALESCE(prev.t, 0)::text
		FROM days LEFT JOIN totals cur ON cur.d = days.d LEFT JOIN totals prev ON prev.d = days.d - 30
		ORDER BY days.d`, self, func(r pgx.Row) (DayPoint, error) {
		var x DayPoint
		return x, r.Scan(&x.Date, &x.Total, &x.Previous)
	}); err != nil {
		return d, err
	}

	if d.Top, err = collect(ctx, pool, `
		SELECT i.product_name, sum(i.quantity * i.units_per_item)::int, sum(i.subtotal - i.discount)::text
		FROM sales s JOIN sale_items i ON i.sale_id = s.id
		WHERE `+sales+` AND s.created_at >= now() - interval '30 days'
		GROUP BY i.product_id, i.product_name ORDER BY sum(i.subtotal - i.discount) DESC LIMIT 20`, self, func(r pgx.Row) (ProductPoint, error) {
		var x ProductPoint
		return x, r.Scan(&x.Name, &x.Units, &x.Revenue)
	}); err != nil {
		return d, err
	}

	if d.Categories, err = collect(ctx, pool, `
		SELECT c.name, sum(i.subtotal - i.discount)::text
		FROM sales s JOIN sale_items i ON i.sale_id = s.id
		JOIN products pr ON pr.id = i.product_id JOIN categories c ON c.id = pr.category_id
		WHERE `+sales+` AND `+localDay("s.created_at")+` >= date_trunc('month', `+day+`)::date
		GROUP BY c.name ORDER BY sum(i.subtotal - i.discount) DESC`, self, func(r pgx.Row) (CategoryPoint, error) {
		var x CategoryPoint
		return x, r.Scan(&x.Name, &x.Total)
	}); err != nil {
		return d, err
	}

	// How customers paid, net of change: each sale's change comes off its córdoba cash first
	// and the rest off its dollar cash, so every day's bars add up to what was sold.
	if d.Methods, err = collect(ctx, pool, `
		WITH days AS (SELECT generate_series(`+day+` - 13, `+day+`, interval '1 day')::date AS d),
		per_sale AS (
			SELECT `+localDay("s.created_at")+` AS d, s.change_nio AS change,
			       COALESCE(sum(sp.amount_nio) FILTER (WHERE sp.method = 'cash' AND sp.currency_code = 'NIO'), 0) AS nio,
			       COALESCE(sum(sp.amount_nio) FILTER (WHERE sp.method = 'cash' AND sp.currency_code = 'USD'), 0) AS usd,
			       COALESCE(sum(sp.amount_nio) FILTER (WHERE sp.method = 'card'), 0) AS card,
			       COALESCE(sum(sp.amount_nio) FILTER (WHERE sp.method = 'transfer'), 0) AS transfer
			FROM sales s JOIN sale_payments sp ON sp.sale_id = s.id
			WHERE `+sales+` AND s.created_at >= now() - interval '15 days'
			GROUP BY s.id),
		per_day AS (
			SELECT d, sum(GREATEST(nio - change, 0)) AS nio, sum(usd - GREATEST(change - nio, 0)) AS usd,
			       sum(card) AS card, sum(transfer) AS transfer
			FROM per_sale GROUP BY d)
		SELECT days.d::text, COALESCE(p.nio, 0)::text, COALESCE(p.usd, 0)::text, COALESCE(p.card, 0)::text, COALESCE(p.transfer, 0)::text
		FROM days LEFT JOIN per_day p ON p.d = days.d
		ORDER BY days.d`, self, func(r pgx.Row) (MethodPoint, error) {
		var x MethodPoint
		return x, r.Scan(&x.Date, &x.CashNIO, &x.CashUSD, &x.Card, &x.Transfer)
	}); err != nil {
		return d, err
	}

	d.Heatmap, err = collect(ctx, pool, `
		SELECT extract(isodow FROM s.created_at AT TIME ZONE 'America/Managua')::int,
		       extract(hour FROM s.created_at AT TIME ZONE 'America/Managua')::int, count(*)::int, sum(s.total)::text
		FROM sales s WHERE `+sales+` AND s.created_at >= now() - interval '90 days'
		GROUP BY 1, 2 ORDER BY 1, 2`, self, func(r pgx.Row) (HeatPoint, error) {
		var x HeatPoint
		return x, r.Scan(&x.Weekday, &x.Hour, &x.Sales, &x.Total)
	})
	return d, err
}

func collect[T any](ctx context.Context, pool *pgxpool.Pool, query string, self any, scan func(pgx.Row) (T, error)) ([]T, error) {
	rows, err := pool.Query(ctx, query, self)
	if err != nil {
		return nil, fmt.Errorf("dashboard query: %w", err)
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (T, error) { return scan(r) })
	if err != nil {
		return nil, fmt.Errorf("dashboard rows: %w", err)
	}
	if items == nil {
		items = []T{}
	}
	return items, nil
}
