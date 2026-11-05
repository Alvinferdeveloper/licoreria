package inventory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"licoreria/api/internal/httpx"
	"licoreria/api/internal/platform/database"
)

type ProductRef struct {
	ID   uuid.UUID `json:"id"`
	SKU  string    `json:"sku"`
	Name string    `json:"name"`
}

type Ref struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// MovementEntry is one kardex line. UnitCost is omitted for roles that cannot see costs.
type MovementEntry struct {
	ID         uuid.UUID  `json:"id"`
	Product    ProductRef `json:"product"`
	Type       string     `json:"type"`
	Quantity   int        `json:"quantity"`
	StockAfter int        `json:"stockAfter"`
	UnitCost   *string    `json:"unitCost,omitempty"`
	Reason     string     `json:"reason"`
	SourceType *string    `json:"sourceType"`
	SourceID   *uuid.UUID `json:"sourceId"`
	User       Ref        `json:"user"`
	CreatedAt  time.Time  `json:"createdAt"`
}

type MovementFilter struct {
	ProductID *uuid.UUID
	Type      string
	From, To  string // YYYY-MM-DD, store local time
}

// Alert is a product at or below its reorder level, with its last supplier for grouping.
type Alert struct {
	Product      ProductRef `json:"product"`
	Category     string     `json:"category"`
	Stock        int        `json:"stock"`
	MinStock     int        `json:"minStock"`
	ReorderPoint int        `json:"reorderPoint"`
	Level        string     `json:"level"`
	Supplier     *Ref       `json:"supplier"`
	LastCost     *string    `json:"lastCost,omitempty"`
}

type repository struct{}

func (repository) movements(ctx context.Context, db database.DBTX, f MovementFilter, page httpx.Page) ([]MovementEntry, int64, error) {
	where, args := []string{"true"}, []any{}
	add := func(cond string, value any) {
		args = append(args, value)
		where = append(where, strings.ReplaceAll(cond, "?", fmt.Sprintf("$%d", len(args))))
	}
	if f.ProductID != nil {
		add("m.product_id = ?", *f.ProductID)
	}
	if f.Type != "" {
		add("m.type = ?", f.Type)
	}
	if f.From != "" {
		add("m.created_at >= (?::date)::timestamp AT TIME ZONE 'America/Managua'", f.From)
	}
	if f.To != "" {
		add("m.created_at < (?::date + 1)::timestamp AT TIME ZONE 'America/Managua'", f.To)
	}
	condition := strings.Join(where, " AND ")

	var total int64
	if err := db.QueryRow(ctx, "SELECT count(*) FROM inventory_movements m WHERE "+condition, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count movements: %w", err)
	}
	args = append(args, page.Size, page.Offset())
	rows, err := db.Query(ctx, fmt.Sprintf(`
		SELECT m.id, p.id, p.sku::text, p.name, m.type, m.quantity, m.stock_after, m.unit_cost::text, m.reason,
		       m.source_type, m.source_id, u.id, u.full_name, m.created_at
		FROM inventory_movements m
		JOIN products p ON p.id = m.product_id
		JOIN users u ON u.id = m.created_by
		WHERE %s ORDER BY m.created_at DESC, m.id DESC LIMIT $%d OFFSET $%d`, condition, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list movements: %w", err)
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (MovementEntry, error) {
		var e MovementEntry
		var cost string
		err := row.Scan(&e.ID, &e.Product.ID, &e.Product.SKU, &e.Product.Name, &e.Type, &e.Quantity, &e.StockAfter, &cost,
			&e.Reason, &e.SourceType, &e.SourceID, &e.User.ID, &e.User.Name, &e.CreatedAt)
		e.UnitCost = &cost
		return e, err
	})
	if err != nil {
		return nil, 0, fmt.Errorf("read movements: %w", err)
	}
	return items, total, nil
}

func (repository) alerts(ctx context.Context, db database.DBTX) ([]Alert, error) {
	rows, err := db.Query(ctx, `
		SELECT p.id, p.sku::text, p.name, c.name, p.stock, p.min_stock, p.reorder_point,
		       CASE WHEN p.stock <= 0 THEN 'out' WHEN p.stock <= p.min_stock THEN 'min' ELSE 'reorder' END,
		       last.supplier_id, last.supplier_name, last.unit_cost::text
		FROM products p
		JOIN categories c ON c.id = p.category_id
		LEFT JOIN LATERAL (
			SELECT s.id AS supplier_id, s.name AS supplier_name, pi.unit_cost
			FROM purchase_items pi
			JOIN purchases pu ON pu.id = pi.purchase_id AND pu.status = 'received'
			JOIN suppliers s ON s.id = pu.supplier_id
			WHERE pi.product_id = p.id
			ORDER BY pu.received_at DESC LIMIT 1) last ON true
		WHERE p.active AND p.stock <= GREATEST(p.min_stock, p.reorder_point)
		ORDER BY last.supplier_name NULLS LAST, (p.stock - p.min_stock), p.name`)
	if err != nil {
		return nil, fmt.Errorf("list stock alerts: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Alert, error) {
		var a Alert
		var supplierID *uuid.UUID
		var supplierName *string
		err := row.Scan(&a.Product.ID, &a.Product.SKU, &a.Product.Name, &a.Category, &a.Stock, &a.MinStock, &a.ReorderPoint,
			&a.Level, &supplierID, &supplierName, &a.LastCost)
		if supplierID != nil {
			a.Supplier = &Ref{ID: *supplierID, Name: *supplierName}
		}
		return a, err
	})
}

func (repository) stock(ctx context.Context, db database.DBTX, productID uuid.UUID) (int, error) {
	var stock int
	err := db.QueryRow(ctx, `SELECT stock FROM products WHERE id = $1 FOR UPDATE`, productID).Scan(&stock)
	return stock, database.MapError(err, nil)
}
