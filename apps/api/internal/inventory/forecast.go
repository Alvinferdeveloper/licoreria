package inventory

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"licoreria/api/internal/auth"
	"licoreria/api/internal/platform/database"
)

// HistoryDays is how far back the sales rhythm is read.
const HistoryDays = 28

const (
	UrgencyOut      = "out"
	UrgencyCritical = "critical"
	UrgencySoon     = "soon"
	// UrgencyMin lasts the period at its rhythm but is already under the stock floor.
	UrgencyMin = "min"
)

// Forecast is one product that will run out before the coverage window ends.
type Forecast struct {
	Product   ProductRef `json:"product"`
	Category  string     `json:"category"`
	Stock     int        `json:"stock"`
	MinStock  int        `json:"minStock"`
	DailyRate float64    `json:"dailyRate"`
	// DaysLeft is nil when the product has no recent sales.
	DaysLeft  *float64 `json:"daysLeft"`
	Urgency   string   `json:"urgency"`
	Suggested int      `json:"suggested"`
	PackSize  int      `json:"packSize"`
	PackName  string   `json:"packName"`
	Supplier  *Ref     `json:"supplier"`
	LastCost  *string  `json:"lastCost,omitempty"`
	// History is units sold per day, oldest first, ending today.
	History []int `json:"history"`
}

type forecastRow struct {
	Forecast
	packSize *int
	packName *string
}

// Forecast lists what to buy so stock lasts `coverage` days at the recent sales rhythm.
func (s *Service) Forecast(ctx context.Context, p auth.Principal, coverage int) ([]Forecast, error) {
	rows, err := forecastRows(ctx, s.pool)
	if err != nil {
		return nil, err
	}
	out := []Forecast{}
	for _, row := range rows {
		f := row.Forecast
		if row.packSize != nil {
			f.PackSize, f.PackName = *row.packSize, *row.packName
		}
		if !plan(&f, coverage) {
			continue
		}
		if !canSeeCosts(p) {
			f.LastCost = nil
		}
		out = append(out, f)
	}
	sort.SliceStable(out, func(i, j int) bool { return daysOrMax(out[i]) < daysOrMax(out[j]) })
	return out, nil
}

func daysOrMax(f Forecast) float64 {
	if f.DaysLeft == nil {
		return math.MaxFloat64
	}
	return *f.DaysLeft
}

// plan fills rate, days left, urgency and the suggested order; false means nothing to buy.
// The rate weighs the last week more than the month, so a product that started selling
// faster is caught early without one busy day swinging the whole month.
func plan(f *Forecast, coverage int) bool {
	week, month := 0, 0
	for i, units := range f.History {
		month += units
		if i >= len(f.History)-7 {
			week += units
		}
	}
	f.DailyRate = math.Round((0.6*float64(week)/7+0.4*float64(month)/float64(max(len(f.History), 1)))*100) / 100
	if f.DailyRate > 0 {
		days := math.Round(float64(max(f.Stock, 0))/f.DailyRate*10) / 10
		f.DaysLeft = &days
	}
	runsOut := f.DaysLeft != nil && *f.DaysLeft < float64(coverage)
	if !runsOut && f.Stock > f.MinStock {
		return false
	}

	need := int(math.Ceil(f.DailyRate*float64(coverage))) + f.MinStock - max(f.Stock, 0)
	if need <= 0 {
		return false
	}
	if f.PackSize > 1 {
		need = (need + f.PackSize - 1) / f.PackSize * f.PackSize
	}
	f.Suggested = need

	switch {
	case f.Stock <= 0:
		f.Urgency = UrgencyOut
	case f.DaysLeft != nil && *f.DaysLeft <= 3:
		f.Urgency = UrgencyCritical
	case runsOut:
		f.Urgency = UrgencySoon
	default:
		f.Urgency = UrgencyMin
	}
	return true
}

func forecastRows(ctx context.Context, db database.DBTX) ([]forecastRow, error) {
	rows, err := db.Query(ctx, `
		WITH bounds AS (SELECT (now() AT TIME ZONE 'America/Managua')::date AS today),
		days AS (SELECT generate_series(today - ($1::int - 1), today, interval '1 day')::date AS d FROM bounds),
		sold AS (
			SELECT si.product_id, (coalesce(s.offline_at, s.created_at) AT TIME ZONE 'America/Managua')::date AS d,
			       sum(si.quantity * si.units_per_item)::int AS units
			FROM sale_items si
			JOIN sales s ON s.id = si.sale_id AND s.status = 'completed'
			WHERE coalesce(s.offline_at, s.created_at) >= ((SELECT today FROM bounds) - ($1::int - 1))::timestamp AT TIME ZONE 'America/Managua'
			GROUP BY 1, 2)
		SELECT p.id, p.sku::text, p.name, c.name, p.stock, p.min_stock,
		       (SELECT array_agg(coalesce(sold.units, 0) ORDER BY days.d) FROM days LEFT JOIN sold ON sold.product_id = p.id AND sold.d = days.d),
		       pack.factor, pack.name, last.supplier_id, last.supplier_name, last.unit_cost::text
		FROM products p
		JOIN categories c ON c.id = p.category_id
		LEFT JOIN LATERAL (
			SELECT pp.factor, pp.name FROM product_presentations pp
			WHERE pp.product_id = p.id AND pp.active ORDER BY pp.factor LIMIT 1) pack ON true
		LEFT JOIN LATERAL (
			SELECT s.id AS supplier_id, s.name AS supplier_name, pi.unit_cost
			FROM purchase_items pi
			JOIN purchases pu ON pu.id = pi.purchase_id AND pu.status = 'received'
			JOIN suppliers s ON s.id = pu.supplier_id
			WHERE pi.product_id = p.id
			ORDER BY pu.received_at DESC LIMIT 1) last ON true
		WHERE p.active`, HistoryDays)
	if err != nil {
		return nil, fmt.Errorf("load forecast: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (forecastRow, error) {
		var f forecastRow
		var supplierID *uuid.UUID
		var supplierName *string
		err := row.Scan(&f.Product.ID, &f.Product.SKU, &f.Product.Name, &f.Category, &f.Stock, &f.MinStock, &f.History,
			&f.packSize, &f.packName, &supplierID, &supplierName, &f.LastCost)
		if supplierID != nil {
			f.Supplier = &Ref{ID: *supplierID, Name: *supplierName}
		}
		return f, err
	})
}
