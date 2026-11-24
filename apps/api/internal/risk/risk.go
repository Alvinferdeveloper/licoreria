// Package risk flags cashiers whose voids, discounts, cash shortages or PIN attempts stand out
// from the rest of the store, and lists the events behind those flags.
package risk

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/auth"
	"licoreria/api/internal/httpx"
	"licoreria/api/internal/settings"
)

const (
	FlagVoids     = "voids"
	FlagDiscounts = "discounts"
	FlagShortages = "shortages"
	FlagPIN       = "pin"
	FlagOutside   = "outside_hours"

	LevelHigh   = "high"
	LevelMedium = "medium"
	LevelLow    = "low"

	// ShortageEvent is the cash difference, in córdobas, worth listing on its own.
	ShortageEvent = 100
)

type Person struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// Cashier is one person's activity in the period. Amounts are córdobas as decimal strings.
type Cashier struct {
	User           Person   `json:"user"`
	Sales          int      `json:"sales"`
	Total          string   `json:"total"`
	Voids          int      `json:"voids"`
	VoidedAmount   string   `json:"voidedAmount"`
	Discounts      int      `json:"discounts"`
	DiscountAmount string   `json:"discountAmount"`
	NearLimit      int      `json:"nearLimit"`
	OverLimit      int      `json:"overLimit"`
	Shifts         int      `json:"shifts"`
	ShortShifts    int      `json:"shortShifts"`
	Shortage       string   `json:"shortage"`
	Overage        string   `json:"overage"`
	Expenses       int      `json:"expenses"`
	PINFailures    int      `json:"pinFailures"`
	OutsideHours   int      `json:"outsideHours"`
	Flags          []string `json:"flags"`
	Level          string   `json:"level"`
}

// Event is one fact worth a look: a void, a big shortage, an authorized discount or a PIN failure.
type Event struct {
	Kind   string     `json:"kind"`
	At     time.Time  `json:"at"`
	User   string     `json:"user"`
	Amount *string    `json:"amount"`
	Detail string     `json:"detail"`
	SaleID *uuid.UUID `json:"saleId"`
}

// Rates are the store-wide baselines each cashier is compared with.
type Rates struct {
	Void     float64 `json:"void"`
	Discount float64 `json:"discount"`
	Short    float64 `json:"short"`
}

type Report struct {
	Days                 int       `json:"days"`
	DiscountLimitPercent float64   `json:"discountLimitPercent"`
	Store                Rates     `json:"store"`
	Cashiers             []Cashier `json:"cashiers"`
	Events               []Event   `json:"events"`
}

type Handler struct{ pool *pgxpool.Pool }

func NewHandler(pool *pgxpool.Pool) *Handler { return &Handler{pool: pool} }

func (h *Handler) Routes(r chi.Router) {
	r.With(auth.Require("audit", "view")).Get("/cash-risk", h.report)
}

var periods = map[string]int{"": 30, "7": 7, "30": 30, "90": 90}

func (h *Handler) report(w http.ResponseWriter, r *http.Request) {
	days, ok := periods[r.URL.Query().Get("days")]
	if !ok {
		httpx.Fail(w, r, apperr.Validation(apperr.FieldError{Field: "days", Code: "oneof", Message: "El periodo debe ser de 7, 30 o 90 días."}))
		return
	}
	rep, err := h.build(r.Context(), days)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, rep)
}

func (h *Handler) build(ctx context.Context, days int) (Report, error) {
	pos, err := settings.GetPOS(ctx, h.pool)
	if err != nil {
		return Report{}, err
	}
	cashiers, err := h.cashiers(ctx, days, pos.DiscountLimitPercent)
	if err != nil {
		return Report{}, err
	}
	events, err := h.events(ctx, days)
	if err != nil {
		return Report{}, err
	}
	store := assess(cashiers)
	return Report{Days: days, DiscountLimitPercent: pos.DiscountLimitPercent, Store: store, Cashiers: cashiers, Events: events}, nil
}

func rate(part, whole int) float64 {
	if whole == 0 {
		return 0
	}
	return float64(part) / float64(whole)
}

// assess flags each cashier against the store's own rates, so a busy register isn't punished for
// volume: what counts is doing it noticeably more often than everyone else.
func assess(cashiers []Cashier) Rates {
	var sales, voids, discounted, shifts, short int
	for _, c := range cashiers {
		sales += c.Sales
		voids += c.Voids
		discounted += c.NearLimit + c.OverLimit
		shifts += c.Shifts
		short += c.ShortShifts
	}
	store := Rates{Void: rate(voids, sales), Discount: rate(discounted, sales), Short: rate(short, shifts)}
	for i := range cashiers {
		c := &cashiers[i]
		c.Flags = []string{}
		if c.Voids >= 3 && rate(c.Voids, c.Sales) >= 1.5*store.Void {
			c.Flags = append(c.Flags, FlagVoids)
		}
		if n := c.NearLimit + c.OverLimit; n >= 3 && rate(n, c.Sales) >= 1.5*store.Discount {
			c.Flags = append(c.Flags, FlagDiscounts)
		}
		if c.ShortShifts >= 3 && rate(c.ShortShifts, c.Shifts) >= 1.25*store.Short {
			c.Flags = append(c.Flags, FlagShortages)
		}
		if c.PINFailures >= 3 {
			c.Flags = append(c.Flags, FlagPIN)
		}
		if c.OutsideHours > 0 {
			c.Flags = append(c.Flags, FlagOutside)
		}
		switch {
		case len(c.Flags) >= 2:
			c.Level = LevelHigh
		case len(c.Flags) == 1:
			c.Level = LevelMedium
		default:
			c.Level = LevelLow
		}
	}
	return store
}

func (h *Handler) cashiers(ctx context.Context, days int, limit float64) ([]Cashier, error) {
	rows, err := h.pool.Query(ctx, `
		WITH p AS (SELECT now() - make_interval(days => $1) AS since),
		s AS (
			SELECT user_id, count(*) AS sales,
			       coalesce(sum(total) FILTER (WHERE status = 'completed'), 0) AS total,
			       count(*) FILTER (WHERE status = 'voided') AS voids,
			       coalesce(sum(total) FILTER (WHERE status = 'voided'), 0) AS voided,
			       count(*) FILTER (WHERE status = 'completed' AND discount > 0) AS discounts,
			       coalesce(sum(discount) FILTER (WHERE status = 'completed'), 0) AS discount,
			       count(*) FILTER (WHERE status = 'completed' AND discount_authorized_by IS NULL AND subtotal > 0
			                        AND discount * 100 / subtotal >= $2 * 0.8) AS near,
			       count(*) FILTER (WHERE status = 'completed' AND discount_authorized_by IS NOT NULL) AS over
			FROM sales WHERE created_at >= (SELECT since FROM p) GROUP BY user_id),
		c AS (
			SELECT user_id, count(*) AS shifts, count(*) FILTER (WHERE difference_nio < 0) AS short,
			       coalesce(-sum(difference_nio) FILTER (WHERE difference_nio < 0), 0) AS shortage,
			       coalesce(sum(difference_nio) FILTER (WHERE difference_nio > 0), 0) AS overage
			FROM cash_shifts WHERE status = 'closed' AND closed_at >= (SELECT since FROM p) GROUP BY user_id),
		m AS (
			SELECT created_by AS user_id, count(*) AS expenses
			FROM cash_movements WHERE type = 'expense' AND created_at >= (SELECT since FROM p) GROUP BY 1),
		a AS (
			SELECT user_id, count(*) FILTER (WHERE action = 'authorization.failed') AS pin,
			       count(*) FILTER (WHERE action = 'auth.login_outside_hours') AS outside
			FROM audit_log WHERE user_id IS NOT NULL AND created_at >= (SELECT since FROM p) GROUP BY 1)
		SELECT u.id, u.full_name,
		       coalesce(s.sales, 0), coalesce(s.total, 0)::text, coalesce(s.voids, 0), coalesce(s.voided, 0)::text,
		       coalesce(s.discounts, 0), coalesce(s.discount, 0)::text, coalesce(s.near, 0), coalesce(s.over, 0),
		       coalesce(c.shifts, 0), coalesce(c.short, 0), coalesce(c.shortage, 0)::text, coalesce(c.overage, 0)::text,
		       coalesce(m.expenses, 0), coalesce(a.pin, 0), coalesce(a.outside, 0)
		FROM users u
		LEFT JOIN s ON s.user_id = u.id
		LEFT JOIN c ON c.user_id = u.id
		LEFT JOIN m ON m.user_id = u.id
		LEFT JOIN a ON a.user_id = u.id
		WHERE s.user_id IS NOT NULL OR c.user_id IS NOT NULL OR a.pin > 0 OR a.outside > 0
		ORDER BY u.full_name`, days, limit)
	if err != nil {
		return nil, fmt.Errorf("load cashier risk: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Cashier, error) {
		var c Cashier
		err := row.Scan(&c.User.ID, &c.User.Name, &c.Sales, &c.Total, &c.Voids, &c.VoidedAmount,
			&c.Discounts, &c.DiscountAmount, &c.NearLimit, &c.OverLimit,
			&c.Shifts, &c.ShortShifts, &c.Shortage, &c.Overage, &c.Expenses, &c.PINFailures, &c.OutsideHours)
		return c, err
	})
}

func (h *Handler) events(ctx context.Context, days int) ([]Event, error) {
	rows, err := h.pool.Query(ctx, `
		WITH p AS (SELECT now() - make_interval(days => $1) AS since)
		SELECT * FROM (
			SELECT 'void' AS kind, s.voided_at AS at, u.full_name, s.total::text,
			       s.invoice_series || '-' || lpad(s.invoice_number::text, 6, '0') || ' · ' || s.void_reason, s.id
			FROM sales s JOIN users u ON u.id = s.user_id
			WHERE s.status = 'voided' AND s.voided_at >= (SELECT since FROM p)
			UNION ALL
			SELECT 'shortage', c.closed_at, u.full_name, (-c.difference_nio)::text, r.name, NULL::uuid
			FROM cash_shifts c JOIN users u ON u.id = c.user_id JOIN registers r ON r.id = c.register_id
			WHERE c.status = 'closed' AND c.difference_nio <= -$2::numeric AND c.closed_at >= (SELECT since FROM p)
			UNION ALL
			SELECT 'discount', s.created_at, u.full_name, s.discount::text,
			       s.invoice_series || '-' || lpad(s.invoice_number::text, 6, '0') || ' · ' || a.full_name, s.id
			FROM sales s JOIN users u ON u.id = s.user_id JOIN users a ON a.id = s.discount_authorized_by
			WHERE s.status = 'completed' AND s.created_at >= (SELECT since FROM p)
			UNION ALL
			SELECT CASE l.action WHEN 'authorization.failed' THEN 'pin' ELSE 'outside_hours' END, l.created_at, u.full_name, NULL,
			       coalesce(t.full_name, ''), NULL::uuid
			FROM audit_log l JOIN users u ON u.id = l.user_id LEFT JOIN users t ON t.id = l.entity_id AND l.action = 'authorization.failed'
			WHERE l.action IN ('authorization.failed', 'auth.login_outside_hours') AND l.created_at >= (SELECT since FROM p)
		) e ORDER BY at DESC LIMIT 25`, days, ShortageEvent)
	if err != nil {
		return nil, fmt.Errorf("load risk events: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Event, error) {
		var e Event
		err := row.Scan(&e.Kind, &e.At, &e.User, &e.Amount, &e.Detail, &e.SaleID)
		return e, err
	})
}
