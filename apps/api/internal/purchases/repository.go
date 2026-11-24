// Package purchases registers supplier purchases. A draft can be edited freely; receiving it
// adds stock through the inventory ledger and moves the average cost; voiding a received
// purchase reverses those movements.
package purchases

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

const (
	StatusDraft    = "draft"
	StatusReceived = "received"
	StatusVoided   = "voided"
)

type Ref struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type ProductRef struct {
	ID   uuid.UUID `json:"id"`
	SKU  string    `json:"sku"`
	Name string    `json:"name"`
}

// Amounts are decimal strings. PreviousCost is the unit cost of the product's last received
// purchase (other than this one), for comparing supplier prices.
type Item struct {
	ID           uuid.UUID  `json:"id"`
	Product      ProductRef `json:"product"`
	Quantity     int        `json:"quantity"`
	UnitCost     string     `json:"unitCost"`
	Subtotal     string     `json:"subtotal"`
	TaxPercent   string     `json:"taxPercent"`
	PreviousCost *string    `json:"previousCost"`
	SalePrice    string     `json:"salePrice"`
	PriceUpdated bool       `json:"priceUpdated"`
	ExpiresOn    *string    `json:"expiresOn"`
}

type Purchase struct {
	ID                    uuid.UUID  `json:"id"`
	Supplier              Ref        `json:"supplier"`
	SupplierInvoiceNumber string     `json:"supplierInvoiceNumber"`
	Status                string     `json:"status"`
	PurchaseDate          string     `json:"purchaseDate"`
	Subtotal              string     `json:"subtotal"`
	Tax                   string     `json:"tax"`
	Total                 string     `json:"total"`
	Notes                 string     `json:"notes"`
	ItemCount             int        `json:"itemCount"`
	CreatedBy             Ref        `json:"createdBy"`
	ReceivedBy            *Ref       `json:"receivedBy"`
	ReceivedAt            *time.Time `json:"receivedAt"`
	VoidReason            string     `json:"voidReason"`
	VoidedAt              *time.Time `json:"voidedAt"`
	CreatedAt             time.Time  `json:"createdAt"`
	Items                 []Item     `json:"items,omitempty"`
}

type Filter struct {
	Status     string
	SupplierID *uuid.UUID
	Search     string
}

var constraintFields = database.ConstraintFields{
	"purchases_supplier_id_fkey":                "supplierId",
	"purchase_items_product_id_fkey":            "items",
	"purchase_items_purchase_id_product_id_key": "items",
	"purchase_items_quantity_check":             "items",
}

type repository struct{}

const selectPurchase = `
	SELECT pu.id, s.id, s.name, pu.supplier_invoice_number, pu.status, pu.purchase_date::text,
	       pu.subtotal::text, pu.tax::text, pu.total::text, pu.notes,
	       (SELECT count(*) FROM purchase_items pi WHERE pi.purchase_id = pu.id),
	       cu.id, cu.full_name, ru.id, ru.full_name, pu.received_at, pu.void_reason, pu.voided_at, pu.created_at
	FROM purchases pu
	JOIN suppliers s ON s.id = pu.supplier_id
	JOIN users cu ON cu.id = pu.created_by
	LEFT JOIN users ru ON ru.id = pu.received_by`

func scanPurchase(row pgx.Row) (Purchase, error) {
	var p Purchase
	var receiverID *uuid.UUID
	var receiverName *string
	err := row.Scan(&p.ID, &p.Supplier.ID, &p.Supplier.Name, &p.SupplierInvoiceNumber, &p.Status, &p.PurchaseDate,
		&p.Subtotal, &p.Tax, &p.Total, &p.Notes, &p.ItemCount, &p.CreatedBy.ID, &p.CreatedBy.Name,
		&receiverID, &receiverName, &p.ReceivedAt, &p.VoidReason, &p.VoidedAt, &p.CreatedAt)
	if receiverID != nil {
		p.ReceivedBy = &Ref{ID: *receiverID, Name: *receiverName}
	}
	return p, err
}

func (repository) list(ctx context.Context, db database.DBTX, f Filter, page httpx.Page) ([]Purchase, int64, error) {
	where, args := []string{"true"}, []any{}
	add := func(cond string, value any) {
		args = append(args, value)
		where = append(where, strings.ReplaceAll(cond, "?", fmt.Sprintf("$%d", len(args))))
	}
	if f.Status != "" {
		add("pu.status = ?", f.Status)
	}
	if f.SupplierID != nil {
		add("pu.supplier_id = ?", *f.SupplierID)
	}
	if q := strings.TrimSpace(f.Search); q != "" {
		add("(pu.supplier_invoice_number ILIKE '%' || ? || '%' OR immutable_unaccent(lower(s.name)) LIKE '%' || immutable_unaccent(lower(?)) || '%')", q)
	}
	condition := strings.Join(where, " AND ")

	var total int64
	if err := db.QueryRow(ctx, "SELECT count(*) FROM purchases pu JOIN suppliers s ON s.id = pu.supplier_id WHERE "+condition, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count purchases: %w", err)
	}
	args = append(args, page.Size, page.Offset())
	rows, err := db.Query(ctx, fmt.Sprintf("%s WHERE %s ORDER BY pu.purchase_date DESC, pu.created_at DESC LIMIT $%d OFFSET $%d",
		selectPurchase, condition, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list purchases: %w", err)
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Purchase, error) { return scanPurchase(row) })
	if err != nil {
		return nil, 0, fmt.Errorf("read purchases: %w", err)
	}
	return items, total, nil
}

// get loads a purchase with its lines; lock takes a row lock for status changes.
func (repository) get(ctx context.Context, db database.DBTX, id uuid.UUID, lock bool) (Purchase, error) {
	query := selectPurchase + " WHERE pu.id = $1"
	if lock {
		query += " FOR UPDATE OF pu"
	}
	p, err := scanPurchase(db.QueryRow(ctx, query, id))
	if err != nil {
		return Purchase{}, database.MapError(err, nil)
	}
	var raw []byte
	err = db.QueryRow(ctx, `
		SELECT COALESCE(json_agg(json_build_object(
			'id', pi.id, 'product', json_build_object('id', p.id, 'sku', p.sku, 'name', p.name),
			'quantity', pi.quantity, 'unitCost', pi.unit_cost::text, 'subtotal', pi.subtotal::text,
			'taxPercent', p.tax_percent::text, 'expiresOn', pi.expires_on, 'salePrice', p.price::text,
			'priceUpdated', EXISTS (SELECT 1 FROM audit_log a
			                        WHERE a.entity = 'products' AND a.entity_id = p.id AND a.created_at > pu.created_at
			                          AND a.action IN ('products.price_changed', 'products.updated')
			                          AND (a.before_data->>'price')::numeric IS DISTINCT FROM (a.after_data->>'price')::numeric),
			'previousCost', (SELECT prev.unit_cost::text FROM purchase_items prev
			                 JOIN purchases pp ON pp.id = prev.purchase_id AND pp.status = 'received'
			                 WHERE prev.product_id = pi.product_id AND pp.id <> pi.purchase_id
			                 ORDER BY pp.received_at DESC LIMIT 1)
		) ORDER BY p.name), '[]')
		FROM purchase_items pi JOIN products p ON p.id = pi.product_id JOIN purchases pu ON pu.id = pi.purchase_id
		WHERE pi.purchase_id = $1`, id).Scan(&raw)
	if err != nil {
		return Purchase{}, fmt.Errorf("load purchase items: %w", err)
	}
	if err := json.Unmarshal(raw, &p.Items); err != nil {
		return Purchase{}, fmt.Errorf("decode purchase items: %w", err)
	}
	return p, nil
}

func (repository) insert(ctx context.Context, db database.DBTX, in Input, userID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := db.QueryRow(ctx, `
		INSERT INTO purchases (supplier_id, supplier_invoice_number, purchase_date, notes, created_by)
		VALUES ($1, $2, $3::date, $4, $5) RETURNING id`,
		in.SupplierID, in.SupplierInvoiceNumber, in.PurchaseDate, in.Notes, userID).Scan(&id)
	return id, database.MapError(err, constraintFields)
}

func (repository) update(ctx context.Context, db database.DBTX, id uuid.UUID, in Input) error {
	_, err := db.Exec(ctx, `
		UPDATE purchases SET supplier_id = $2, supplier_invoice_number = $3, purchase_date = $4::date, notes = $5
		WHERE id = $1`, id, in.SupplierID, in.SupplierInvoiceNumber, in.PurchaseDate, in.Notes)
	return database.MapError(err, constraintFields)
}

// replaceItems rewrites the lines and recomputes the totals; tax uses each product's rate.
func (repository) replaceItems(ctx context.Context, db database.DBTX, id uuid.UUID, items []ItemInput) error {
	if _, err := db.Exec(ctx, `DELETE FROM purchase_items WHERE purchase_id = $1`, id); err != nil {
		return database.MapError(err, nil)
	}
	for _, item := range items {
		_, err := db.Exec(ctx, `
			INSERT INTO purchase_items (purchase_id, product_id, quantity, unit_cost, subtotal, expires_on)
			VALUES ($1, $2, $3::int, $4::numeric, round($3::int * $4::numeric, 2), $5::date)`, id, item.ProductID, item.Quantity, item.UnitCost, item.ExpiresOn)
		if err != nil {
			return database.MapError(err, constraintFields)
		}
	}
	_, err := db.Exec(ctx, `
		UPDATE purchases pu SET subtotal = t.subtotal, tax = t.tax, total = t.subtotal + t.tax
		FROM (SELECT COALESCE(sum(pi.subtotal), 0) AS subtotal, COALESCE(sum(round(pi.subtotal * p.tax_percent / 100, 2)), 0) AS tax
		      FROM purchase_items pi JOIN products p ON p.id = pi.product_id WHERE pi.purchase_id = $1) t
		WHERE pu.id = $1`, id)
	return database.MapError(err, nil)
}

func (repository) markReceived(ctx context.Context, db database.DBTX, id, userID uuid.UUID) error {
	_, err := db.Exec(ctx, `UPDATE purchases SET status = 'received', received_by = $2, received_at = now() WHERE id = $1`, id, userID)
	return database.MapError(err, nil)
}

func (repository) markVoided(ctx context.Context, db database.DBTX, id, userID uuid.UUID, reason string) error {
	_, err := db.Exec(ctx, `
		UPDATE purchases SET status = 'voided', voided_by = $2, voided_at = now(), void_reason = $3 WHERE id = $1`, id, userID, reason)
	return database.MapError(err, nil)
}
