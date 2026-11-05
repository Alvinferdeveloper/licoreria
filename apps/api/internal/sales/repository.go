// Package sales records POS sales: prices come from the catalog, stock leaves through the
// inventory ledger, the invoice number is taken from the register inside the same
// transaction, and every amount is stored in córdobas with the rate used.
package sales

import (
	"context"
	"encoding/json"
	"errors"
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

type Item struct {
	ID             uuid.UUID  `json:"id"`
	ProductID      uuid.UUID  `json:"productId"`
	PresentationID *uuid.UUID `json:"presentationId"`
	Name           string     `json:"name"`
	SKU            string     `json:"sku"`
	Quantity       int        `json:"quantity"`
	UnitsPerItem   int        `json:"unitsPerItem"`
	UnitPrice      string     `json:"unitPrice"`
	Discount       string     `json:"discount"`
	TaxPercent     string     `json:"taxPercent"`
	Tax            string     `json:"tax"`
	Subtotal       string     `json:"subtotal"`
	Total          string     `json:"total"`
}

type Payment struct {
	Method       string  `json:"method"`
	Currency     string  `json:"currency"`
	Amount       string  `json:"amount"`
	ExchangeRate string  `json:"exchangeRate"`
	AmountNIO    string  `json:"amountNio"`
	Reference    *string `json:"reference"`
}

type Customer struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	TaxID *string   `json:"taxId"`
}

type Sale struct {
	ID           uuid.UUID  `json:"id"`
	Invoice      string     `json:"invoice"`
	Series       string     `json:"series"`
	Number       int        `json:"number"`
	Status       string     `json:"status"`
	Register     Ref        `json:"register"`
	Cashier      Ref        `json:"cashier"`
	Customer     Customer   `json:"customer"`
	ShiftID      uuid.UUID  `json:"shiftId"`
	Subtotal     string     `json:"subtotal"`
	Discount     string     `json:"discount"`
	Tax          string     `json:"tax"`
	Total        string     `json:"total"`
	ExchangeRate string     `json:"exchangeRate"`
	ChangeNIO    string     `json:"changeNio"`
	ItemCount    int        `json:"itemCount"`
	Notes        string     `json:"notes"`
	VoidReason   *string    `json:"voidReason"`
	VoidedAt     *time.Time `json:"voidedAt"`
	CreatedAt    time.Time  `json:"createdAt"`
	Items        []Item     `json:"items,omitempty"`
	Payments     []Payment  `json:"payments,omitempty"`
}

type Filter struct {
	UserID   *uuid.UUID
	ShiftID  *uuid.UUID
	Status   string
	Search   string
	From, To string
}

// catalogLine is what the sale needs from a product (and presentation) at selling time.
type catalogLine struct {
	Name, SKU      string
	Active         bool
	UnitPrice      string
	AverageCost    string
	TaxPercent     string
	UnitsPerItem   int
	PresentationOK bool
}

type repository struct{}

const selectSale = `
	SELECT s.id, s.invoice_series, s.invoice_number, s.status, r.id, r.name, u.id, u.full_name,
	       s.customer_id, s.customer_name, s.customer_tax_id, s.shift_id,
	       s.subtotal::text, s.discount::text, s.tax::text, s.total::text, s.exchange_rate::text, s.change_nio::text,
	       (SELECT COALESCE(sum(quantity * units_per_item), 0) FROM sale_items WHERE sale_id = s.id),
	       s.notes, s.void_reason, s.voided_at, s.created_at
	FROM sales s
	JOIN registers r ON r.id = s.register_id
	JOIN users u ON u.id = s.user_id`

func scanSale(row pgx.Row) (Sale, error) {
	var s Sale
	err := row.Scan(&s.ID, &s.Series, &s.Number, &s.Status, &s.Register.ID, &s.Register.Name, &s.Cashier.ID, &s.Cashier.Name,
		&s.Customer.ID, &s.Customer.Name, &s.Customer.TaxID, &s.ShiftID,
		&s.Subtotal, &s.Discount, &s.Tax, &s.Total, &s.ExchangeRate, &s.ChangeNIO, &s.ItemCount,
		&s.Notes, &s.VoidReason, &s.VoidedAt, &s.CreatedAt)
	s.Invoice = fmt.Sprintf("%s-%06d", s.Series, s.Number)
	return s, err
}

func (repository) list(ctx context.Context, db database.DBTX, f Filter, page httpx.Page) ([]Sale, int64, error) {
	where, args := []string{"true"}, []any{}
	add := func(cond string, value any) {
		args = append(args, value)
		where = append(where, strings.ReplaceAll(cond, "?", fmt.Sprintf("$%d", len(args))))
	}
	if f.UserID != nil {
		add("s.user_id = ?", *f.UserID)
	}
	if f.ShiftID != nil {
		add("s.shift_id = ?", *f.ShiftID)
	}
	if f.Status != "" {
		add("s.status = ?", f.Status)
	}
	if q := strings.TrimSpace(f.Search); q != "" {
		add(`(s.invoice_series || '-' || lpad(s.invoice_number::text, 6, '0') ILIKE '%' || ? || '%'
		      OR immutable_unaccent(lower(s.customer_name)) LIKE '%' || immutable_unaccent(lower(?)) || '%')`, q)
	}
	if f.From != "" {
		add("s.created_at >= (?::date)::timestamp AT TIME ZONE 'America/Managua'", f.From)
	}
	if f.To != "" {
		add("s.created_at < (?::date + 1)::timestamp AT TIME ZONE 'America/Managua'", f.To)
	}
	condition := strings.Join(where, " AND ")
	var total int64
	if err := db.QueryRow(ctx, "SELECT count(*) FROM sales s WHERE "+condition, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count sales: %w", err)
	}
	args = append(args, page.Size, page.Offset())
	rows, err := db.Query(ctx, fmt.Sprintf("%s WHERE %s ORDER BY s.created_at DESC LIMIT $%d OFFSET $%d", selectSale, condition, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list sales: %w", err)
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Sale, error) { return scanSale(row) })
	return items, total, err
}

// get loads a sale with its items and payments; lock takes a row lock for voiding.
func (repository) get(ctx context.Context, db database.DBTX, id uuid.UUID, lock bool) (Sale, error) {
	query := selectSale + " WHERE s.id = $1"
	if lock {
		query += " FOR UPDATE OF s"
	}
	s, err := scanSale(db.QueryRow(ctx, query, id))
	if err != nil {
		return Sale{}, database.MapError(err, nil)
	}
	var items, payments []byte
	err = db.QueryRow(ctx, `
		SELECT
		  (SELECT COALESCE(json_agg(json_build_object('id', i.id, 'productId', i.product_id, 'presentationId', i.presentation_id,
		     'name', i.product_name, 'sku', i.sku, 'quantity', i.quantity, 'unitsPerItem', i.units_per_item,
		     'unitPrice', i.unit_price::text, 'discount', i.discount::text, 'taxPercent', i.tax_percent::text,
		     'tax', i.tax::text, 'subtotal', i.subtotal::text, 'total', i.total::text) ORDER BY i.product_name), '[]')
		   FROM sale_items i WHERE i.sale_id = $1),
		  (SELECT COALESCE(json_agg(json_build_object('method', p.method, 'currency', p.currency_code, 'amount', p.amount::text,
		     'exchangeRate', p.exchange_rate::text, 'amountNio', p.amount_nio::text, 'reference', p.reference) ORDER BY p.created_at), '[]')
		   FROM sale_payments p WHERE p.sale_id = $1)`, id).Scan(&items, &payments)
	if err != nil {
		return Sale{}, fmt.Errorf("load sale detail: %w", err)
	}
	if err := json.Unmarshal(items, &s.Items); err != nil {
		return Sale{}, fmt.Errorf("decode sale items: %w", err)
	}
	if err := json.Unmarshal(payments, &s.Payments); err != nil {
		return Sale{}, fmt.Errorf("decode sale payments: %w", err)
	}
	return s, nil
}

func (repository) byIdempotencyKey(ctx context.Context, db database.DBTX, key uuid.UUID) (uuid.UUID, uuid.UUID, bool, error) {
	var id, userID uuid.UUID
	err := db.QueryRow(ctx, `SELECT id, user_id FROM sales WHERE idempotency_key = $1`, key).Scan(&id, &userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, uuid.Nil, false, nil
	}
	return id, userID, err == nil, err
}

func (repository) catalogLine(ctx context.Context, db database.DBTX, productID uuid.UUID, presentationID *uuid.UUID) (catalogLine, error) {
	var c catalogLine
	err := db.QueryRow(ctx, `
		SELECT p.name, p.sku::text, p.active AND COALESCE(pp.active, true),
		       COALESCE(pp.price, p.price)::text, p.average_cost::text, p.tax_percent::text,
		       COALESCE(pp.factor, 1), ($2::uuid IS NULL OR pp.id IS NOT NULL)
		FROM products p
		LEFT JOIN product_presentations pp ON pp.id = $2 AND pp.product_id = p.id
		WHERE p.id = $1`, productID, presentationID).
		Scan(&c.Name, &c.SKU, &c.Active, &c.UnitPrice, &c.AverageCost, &c.TaxPercent, &c.UnitsPerItem, &c.PresentationOK)
	return c, database.MapError(err, nil)
}

type customerRow struct {
	ID    uuid.UUID
	Name  string
	TaxID *string
}

func (repository) customer(ctx context.Context, db database.DBTX, id *uuid.UUID) (customerRow, error) {
	var c customerRow
	query := `SELECT id, full_name, tax_id::text FROM customers WHERE is_walk_in`
	args := []any{}
	if id != nil {
		query, args = `SELECT id, full_name, tax_id::text FROM customers WHERE id = $1 AND active`, []any{*id}
	}
	err := db.QueryRow(ctx, query, args...).Scan(&c.ID, &c.Name, &c.TaxID)
	return c, database.MapError(err, nil)
}

// nextInvoice locks the register and takes its next number.
func (repository) nextInvoice(ctx context.Context, db database.DBTX, registerID uuid.UUID) (string, int, error) {
	var series string
	var number int
	err := db.QueryRow(ctx, `
		UPDATE registers SET next_invoice_number = next_invoice_number + 1 WHERE id = $1
		RETURNING invoice_series, next_invoice_number - 1`, registerID).Scan(&series, &number)
	return series, number, database.MapError(err, nil)
}

type saleRow struct {
	Series              string
	Number              int
	ShiftID, RegisterID uuid.UUID
	UserID              uuid.UUID
	Customer            customerRow
	Totals              totals
	Rate                string
	Change              int64
	AuthorizedBy        *uuid.UUID
	IdempotencyKey      uuid.UUID
	Notes               string
}

func (repository) insertSale(ctx context.Context, db database.DBTX, s saleRow) (uuid.UUID, error) {
	var id uuid.UUID
	err := db.QueryRow(ctx, `
		INSERT INTO sales (invoice_series, invoice_number, shift_id, register_id, user_id, customer_id, customer_name, customer_tax_id,
		                   subtotal, discount, tax, total, exchange_rate, change_nio, age_verified, discount_authorized_by, idempotency_key, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::numeric, $10::numeric, $11::numeric, $12::numeric, $13::numeric, $14::numeric, true, $15, $16, $17)
		RETURNING id`,
		s.Series, s.Number, s.ShiftID, s.RegisterID, s.UserID, s.Customer.ID, s.Customer.Name, s.Customer.TaxID,
		formatCents(s.Totals.Subtotal), formatCents(s.Totals.Discount), formatCents(s.Totals.Tax), formatCents(s.Totals.Total),
		s.Rate, formatCents(s.Change), s.AuthorizedBy, s.IdempotencyKey, s.Notes).Scan(&id)
	return id, database.MapError(err, nil)
}

func (repository) insertItem(ctx context.Context, db database.DBTX, saleID uuid.UUID, l pricedLine, taxPercent string) error {
	_, err := db.Exec(ctx, `
		INSERT INTO sale_items (sale_id, product_id, presentation_id, product_name, sku, quantity, units_per_item,
		                        unit_price, unit_cost, discount, tax_percent, tax, subtotal, total)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8::numeric, $9::numeric, $10::numeric, $11::numeric, $12::numeric, $13::numeric, $14::numeric)`,
		saleID, l.ProductID, l.PresentationID, l.Name, l.SKU, l.Quantity, l.UnitsPerItem,
		formatCents(l.UnitPrice), l.UnitCost, formatCents(l.Discount), taxPercent, formatCents(l.Tax), formatCents(l.Subtotal), formatCents(l.Total))
	return database.MapError(err, nil)
}

func (repository) insertPayment(ctx context.Context, db database.DBTX, saleID uuid.UUID, p settledPayment, rate string) error {
	paymentRate := "1"
	if p.Currency == "USD" {
		paymentRate = rate
	}
	_, err := db.Exec(ctx, `
		INSERT INTO sale_payments (sale_id, method, currency_code, amount, exchange_rate, amount_nio, reference)
		VALUES ($1, $2, $3, $4::numeric, $5::numeric, $6::numeric, $7)`,
		saleID, p.Method, p.Currency, formatCents(p.AmountCents), paymentRate, formatCents(p.AmountNIO), p.Reference)
	return database.MapError(err, nil)
}

func (repository) markVoided(ctx context.Context, db database.DBTX, id, by uuid.UUID, reason string) error {
	_, err := db.Exec(ctx, `UPDATE sales SET status = 'voided', voided_at = now(), voided_by = $2, void_reason = $3 WHERE id = $1`, id, by, reason)
	return database.MapError(err, nil)
}

func (repository) shiftIsOpen(ctx context.Context, db database.DBTX, shiftID uuid.UUID) (bool, error) {
	var open bool
	err := db.QueryRow(ctx, `SELECT status = 'open' FROM cash_shifts WHERE id = $1`, shiftID).Scan(&open)
	return open, database.MapError(err, nil)
}
