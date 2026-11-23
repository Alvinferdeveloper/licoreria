// Package cash manages registers and their shifts: opening float per currency, cash
// income and expenses, and the closing count against what the shift should hold.
package cash

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"licoreria/api/internal/httpx"
	"licoreria/api/internal/platform/database"
)

type Ref struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type OpenShift struct {
	ID       uuid.UUID `json:"id"`
	User     Ref       `json:"user"`
	OpenedAt time.Time `json:"openedAt"`
}

type Register struct {
	ID                uuid.UUID  `json:"id"`
	Name              string     `json:"name"`
	InvoiceSeries     string     `json:"invoiceSeries"`
	NextInvoiceNumber int        `json:"nextInvoiceNumber"`
	Active            bool       `json:"active"`
	OpenShift         *OpenShift `json:"openShift"`
}

// Summary adds up what moved through a shift; Expected* is what the drawer should hold
// per currency (opening + cash sales - change given + income - expenses).
type Summary struct {
	SalesCount   int    `json:"salesCount"`
	CashSalesNIO string `json:"cashSalesNio"`
	CashSalesUSD string `json:"cashSalesUsd"`
	ChangeNIO    string `json:"changeNio"`
	ChangeUSD    string `json:"changeUsd"`
	CardNIO      string `json:"cardNio"`
	TransferNIO  string `json:"transferNio"`
	IncomeNIO    string `json:"incomeNio"`
	IncomeUSD    string `json:"incomeUsd"`
	ExpenseNIO   string `json:"expenseNio"`
	ExpenseUSD   string `json:"expenseUsd"`
	ExpectedNIO  string `json:"expectedNio"`
	ExpectedUSD  string `json:"expectedUsd"`
}

type Movement struct {
	ID        uuid.UUID `json:"id"`
	Type      string    `json:"type"`
	Currency  string    `json:"currency"`
	Amount    string    `json:"amount"`
	Reason    string    `json:"reason"`
	User      Ref       `json:"user"`
	CreatedAt time.Time `json:"createdAt"`
}

type Shift struct {
	ID            uuid.UUID  `json:"id"`
	Register      Ref        `json:"register"`
	User          Ref        `json:"user"`
	Status        string     `json:"status"`
	OpenedAt      time.Time  `json:"openedAt"`
	ClosedAt      *time.Time `json:"closedAt"`
	OpeningNIO    string     `json:"openingNio"`
	OpeningUSD    string     `json:"openingUsd"`
	ExpectedNIO   *string    `json:"expectedNio"`
	ExpectedUSD   *string    `json:"expectedUsd"`
	CountedNIO    *string    `json:"countedNio"`
	CountedUSD    *string    `json:"countedUsd"`
	DifferenceNIO *string    `json:"differenceNio"`
	DifferenceUSD *string    `json:"differenceUsd"`
	Notes         string     `json:"notes"`
	Summary       *Summary   `json:"summary,omitempty"`
	Movements     []Movement `json:"movements,omitempty"`
	// NonCash lists card and transfer payments to reconcile with the bank's batch close.
	NonCash []NonCashPayment `json:"nonCash,omitempty"`
}

type NonCashPayment struct {
	SaleID    uuid.UUID `json:"saleId"`
	Invoice   string    `json:"invoice"`
	Method    string    `json:"method"`
	Currency  string    `json:"currency"`
	Amount    string    `json:"amount"`
	AmountNIO string    `json:"amountNio"`
	Reference *string   `json:"reference"`
	CreatedAt time.Time `json:"createdAt"`
}

type ShiftFilter struct {
	UserID     *uuid.UUID
	RegisterID *uuid.UUID
	Status     string
}

type repository struct{}

func (repository) registers(ctx context.Context, db database.DBTX) ([]Register, error) {
	rows, err := db.Query(ctx, `
		SELECT r.id, r.name, r.invoice_series, r.next_invoice_number, r.active, s.id, u.id, u.full_name, s.opened_at
		FROM registers r
		LEFT JOIN cash_shifts s ON s.register_id = r.id AND s.status = 'open'
		LEFT JOIN users u ON u.id = s.user_id
		ORDER BY r.active DESC, r.name`)
	if err != nil {
		return nil, fmt.Errorf("list registers: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Register, error) {
		var r Register
		var shiftID, userID *uuid.UUID
		var userName *string
		var openedAt *time.Time
		err := row.Scan(&r.ID, &r.Name, &r.InvoiceSeries, &r.NextInvoiceNumber, &r.Active, &shiftID, &userID, &userName, &openedAt)
		if shiftID != nil {
			r.OpenShift = &OpenShift{ID: *shiftID, User: Ref{ID: *userID, Name: *userName}, OpenedAt: *openedAt}
		}
		return r, err
	})
}

var registerFields = database.ConstraintFields{
	"registers_name_key":             "name",
	"registers_invoice_series_key":   "invoiceSeries",
	"registers_invoice_series_check": "invoiceSeries",
}

func (repository) insertRegister(ctx context.Context, db database.DBTX, in RegisterInput) (uuid.UUID, error) {
	var id uuid.UUID
	err := db.QueryRow(ctx, `INSERT INTO registers (name, invoice_series, active) VALUES ($1, $2, $3) RETURNING id`,
		in.Name, in.InvoiceSeries, in.Active).Scan(&id)
	return id, database.MapError(err, registerFields)
}

// updateRegister keeps the series once invoices were issued under it.
func (repository) updateRegister(ctx context.Context, db database.DBTX, id uuid.UUID, in RegisterInput) (seriesLocked bool, err error) {
	var current string
	var next int
	if err := db.QueryRow(ctx, `SELECT invoice_series, next_invoice_number FROM registers WHERE id = $1 FOR UPDATE`, id).Scan(&current, &next); err != nil {
		return false, database.MapError(err, nil)
	}
	if current != in.InvoiceSeries && next > 1 {
		return true, nil
	}
	_, err = db.Exec(ctx, `UPDATE registers SET name = $2, invoice_series = $3, active = $4 WHERE id = $1`, id, in.Name, in.InvoiceSeries, in.Active)
	return false, database.MapError(err, registerFields)
}

const selectShift = `
	SELECT s.id, r.id, r.name, u.id, u.full_name, s.status, s.opened_at, s.closed_at,
	       s.opening_nio::text, s.opening_usd::text, s.expected_nio::text, s.expected_usd::text,
	       s.counted_nio::text, s.counted_usd::text, s.difference_nio::text, s.difference_usd::text, s.notes
	FROM cash_shifts s
	JOIN registers r ON r.id = s.register_id
	JOIN users u ON u.id = s.user_id`

func scanShift(row pgx.Row) (Shift, error) {
	var s Shift
	err := row.Scan(&s.ID, &s.Register.ID, &s.Register.Name, &s.User.ID, &s.User.Name, &s.Status, &s.OpenedAt, &s.ClosedAt,
		&s.OpeningNIO, &s.OpeningUSD, &s.ExpectedNIO, &s.ExpectedUSD, &s.CountedNIO, &s.CountedUSD, &s.DifferenceNIO, &s.DifferenceUSD, &s.Notes)
	return s, err
}

func (repository) shifts(ctx context.Context, db database.DBTX, f ShiftFilter, page httpx.Page) ([]Shift, int64, error) {
	where, args := []string{"true"}, []any{}
	add := func(cond string, value any) {
		args = append(args, value)
		where = append(where, strings.ReplaceAll(cond, "?", fmt.Sprintf("$%d", len(args))))
	}
	if f.UserID != nil {
		add("s.user_id = ?", *f.UserID)
	}
	if f.RegisterID != nil {
		add("s.register_id = ?", *f.RegisterID)
	}
	if f.Status != "" {
		add("s.status = ?", f.Status)
	}
	condition := strings.Join(where, " AND ")
	var total int64
	if err := db.QueryRow(ctx, "SELECT count(*) FROM cash_shifts s WHERE "+condition, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count shifts: %w", err)
	}
	args = append(args, page.Size, page.Offset())
	rows, err := db.Query(ctx, fmt.Sprintf("%s WHERE %s ORDER BY s.opened_at DESC LIMIT $%d OFFSET $%d", selectShift, condition, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list shifts: %w", err)
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Shift, error) { return scanShift(row) })
	return items, total, err
}

// shift loads one shift; lock takes a row lock for closing and cash movements.
func (repository) shift(ctx context.Context, db database.DBTX, id uuid.UUID, lock bool) (Shift, error) {
	query := selectShift + " WHERE s.id = $1"
	if lock {
		query += " FOR UPDATE OF s"
	}
	s, err := scanShift(db.QueryRow(ctx, query, id))
	return s, database.MapError(err, nil)
}

func (repository) openShiftOf(ctx context.Context, db database.DBTX, userID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := db.QueryRow(ctx, `SELECT id FROM cash_shifts WHERE user_id = $1 AND status = 'open'`, userID).Scan(&id)
	return id, database.MapError(err, nil)
}

func (repository) summary(ctx context.Context, db database.DBTX, shiftID uuid.UUID) (Summary, error) {
	var s Summary
	err := db.QueryRow(ctx, `
		WITH ok AS (SELECT id, change_nio, change_usd FROM sales WHERE shift_id = $1 AND status = 'completed'),
		pay AS (SELECT sp.method, sp.currency_code, sum(sp.amount) AS amount, sum(sp.amount_nio) AS amount_nio
		        FROM sale_payments sp JOIN ok ON ok.id = sp.sale_id GROUP BY 1, 2),
		mov AS (SELECT type, currency_code, sum(amount) AS amount FROM cash_movements WHERE shift_id = $1 GROUP BY 1, 2),
		t AS (
			SELECT (SELECT count(*) FROM ok) AS sales_count,
			       COALESCE((SELECT amount FROM pay WHERE method = 'cash' AND currency_code = 'NIO'), 0) AS cash_nio,
			       COALESCE((SELECT amount FROM pay WHERE method = 'cash' AND currency_code = 'USD'), 0) AS cash_usd,
			       COALESCE((SELECT sum(change_nio) FROM ok), 0) AS change_nio,
			       COALESCE((SELECT sum(change_usd) FROM ok), 0) AS change_usd,
			       COALESCE((SELECT sum(amount_nio) FROM pay WHERE method = 'card'), 0) AS card_nio,
			       COALESCE((SELECT sum(amount_nio) FROM pay WHERE method = 'transfer'), 0) AS transfer_nio,
			       COALESCE((SELECT amount FROM mov WHERE type = 'income' AND currency_code = 'NIO'), 0) AS income_nio,
			       COALESCE((SELECT amount FROM mov WHERE type = 'income' AND currency_code = 'USD'), 0) AS income_usd,
			       COALESCE((SELECT amount FROM mov WHERE type = 'expense' AND currency_code = 'NIO'), 0) AS expense_nio,
			       COALESCE((SELECT amount FROM mov WHERE type = 'expense' AND currency_code = 'USD'), 0) AS expense_usd)
		SELECT t.sales_count, t.cash_nio::text, t.cash_usd::text, t.change_nio::text, t.change_usd::text,
		       t.card_nio::text, t.transfer_nio::text, t.income_nio::text, t.income_usd::text, t.expense_nio::text, t.expense_usd::text,
		       (s.opening_nio + t.cash_nio - t.change_nio + t.income_nio - t.expense_nio)::text,
		       (s.opening_usd + t.cash_usd - t.change_usd + t.income_usd - t.expense_usd)::text
		FROM t, cash_shifts s WHERE s.id = $1`, shiftID).Scan(
		&s.SalesCount, &s.CashSalesNIO, &s.CashSalesUSD, &s.ChangeNIO, &s.ChangeUSD, &s.CardNIO, &s.TransferNIO,
		&s.IncomeNIO, &s.IncomeUSD, &s.ExpenseNIO, &s.ExpenseUSD, &s.ExpectedNIO, &s.ExpectedUSD)
	if err != nil {
		return Summary{}, fmt.Errorf("shift summary: %w", err)
	}
	return s, nil
}

func (repository) movements(ctx context.Context, db database.DBTX, shiftID uuid.UUID) ([]Movement, error) {
	var raw []byte
	err := db.QueryRow(ctx, `
		SELECT COALESCE(json_agg(json_build_object('id', m.id, 'type', m.type, 'currency', m.currency_code, 'amount', m.amount::text,
		       'reason', m.reason, 'user', json_build_object('id', u.id, 'name', u.full_name), 'createdAt', m.created_at)
		       ORDER BY m.created_at DESC), '[]')
		FROM cash_movements m JOIN users u ON u.id = m.created_by WHERE m.shift_id = $1`, shiftID).Scan(&raw)
	if err != nil {
		return nil, fmt.Errorf("list cash movements: %w", err)
	}
	var items []Movement
	return items, json.Unmarshal(raw, &items)
}

func (repository) nonCash(ctx context.Context, db database.DBTX, shiftID uuid.UUID) ([]NonCashPayment, error) {
	var raw []byte
	err := db.QueryRow(ctx, `
		SELECT COALESCE(json_agg(json_build_object('saleId', s.id, 'invoice', s.invoice_series || '-' || lpad(s.invoice_number::text, 6, '0'), 'method', p.method,
		       'currency', p.currency_code, 'amount', p.amount::text, 'amountNio', p.amount_nio::text,
		       'reference', p.reference, 'createdAt', s.created_at) ORDER BY p.method, s.created_at), '[]')
		FROM sale_payments p JOIN sales s ON s.id = p.sale_id
		WHERE s.shift_id = $1 AND s.status = 'completed' AND p.method <> 'cash'`, shiftID).Scan(&raw)
	if err != nil {
		return nil, fmt.Errorf("list non-cash payments: %w", err)
	}
	var items []NonCashPayment
	return items, json.Unmarshal(raw, &items)
}

func (repository) insertShift(ctx context.Context, db database.DBTX, in OpenInput, userID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := db.QueryRow(ctx, `
		INSERT INTO cash_shifts (register_id, user_id, opening_nio, opening_usd) VALUES ($1, $2, $3::numeric, $4::numeric) RETURNING id`,
		in.RegisterID, userID, in.OpeningNIO, in.OpeningUSD).Scan(&id)
	return id, database.MapError(err, nil)
}

func (repository) insertMovement(ctx context.Context, db database.DBTX, shiftID, userID uuid.UUID, in MovementInput) error {
	_, err := db.Exec(ctx, `
		INSERT INTO cash_movements (shift_id, type, currency_code, amount, reason, created_by) VALUES ($1, $2, $3, $4::numeric, $5, $6)`,
		shiftID, in.Type, in.Currency, in.Amount, in.Reason, userID)
	return database.MapError(err, nil)
}

// close stores the count and the differences against the expected amounts, all in SQL so
// no amount passes through a float.
func (repository) close(ctx context.Context, db database.DBTX, id uuid.UUID, sum Summary, in CloseInput) error {
	_, err := db.Exec(ctx, `
		UPDATE cash_shifts SET status = 'closed', closed_at = now(),
		       expected_nio = $2::numeric, expected_usd = $3::numeric, counted_nio = $4::numeric, counted_usd = $5::numeric,
		       difference_nio = $4::numeric - $2::numeric, difference_usd = $5::numeric - $3::numeric, notes = $6
		WHERE id = $1`, id, sum.ExpectedNIO, sum.ExpectedUSD, in.CountedNIO, in.CountedUSD, in.Notes)
	return database.MapError(err, nil)
}

// lessThan compares two decimal amounts in the database.
func lessThan(ctx context.Context, db database.DBTX, a, b string) (bool, error) {
	var less bool
	err := db.QueryRow(ctx, `SELECT $1::numeric < $2::numeric`, a, b).Scan(&less)
	return less, err
}
