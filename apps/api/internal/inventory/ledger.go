// Package inventory owns the stock ledger (kardex). Every stock change goes through Apply so
// stock checks, average cost and low-stock alerts behave the same for purchases, sales and
// adjustments.
package inventory

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/mail"
	"licoreria/api/internal/platform/database"
	"licoreria/api/internal/settings"
)

const (
	TypePurchase        = "purchase"
	TypeSale            = "sale"
	TypeSaleVoid        = "sale_void"
	TypeCustomerReturn  = "customer_return"
	TypeSupplierReturn  = "supplier_return"
	TypeCountAdjustment = "count_adjustment"
	TypeShrinkage       = "shrinkage"
)

var ErrInsufficientStock = apperr.New(apperr.KindConflict, "insufficient_stock", "No hay stock suficiente.")

// Movement is a signed change in base units. UnitCost (decimal string) is required for
// purchases, where it also moves the weighted average cost.
type Movement struct {
	ProductID  uuid.UUID
	Type       string
	Quantity   int
	UnitCost   string
	Reason     string
	SourceType string
	SourceID   *uuid.UUID
	UserID     uuid.UUID
}

// Ledger applies movements inside the caller's transaction.
type Ledger struct {
	appBaseURL string
}

func NewLedger(appBaseURL string) *Ledger { return &Ledger{appBaseURL: appBaseURL} }

// Apply locks the product row, rejects unauthorized negative stock, records the movement
// (the trigger updates products.stock) and queues a low-stock email when the product
// crosses its minimum. It returns the stock after the movement.
func (l *Ledger) Apply(ctx context.Context, db database.DBTX, m Movement) (int, error) {
	var name, sku string
	var stock, minStock int
	var allowNegative bool
	err := db.QueryRow(ctx, `
		SELECT name, sku::text, stock, min_stock, allow_negative_stock FROM products WHERE id = $1 FOR UPDATE`, m.ProductID).
		Scan(&name, &sku, &stock, &minStock, &allowNegative)
	if err != nil {
		return 0, database.MapError(err, nil)
	}
	after := stock + m.Quantity
	if m.Quantity < 0 && after < 0 && !allowNegative {
		return 0, ErrInsufficientStock.
			WithMessage(fmt.Sprintf("No hay stock suficiente de %s: quedan %d.", name, max(stock, 0))).
			WithDetails(map[string]any{"productId": m.ProductID, "available": stock})
	}

	cost := m.UnitCost
	if cost == "" {
		cost = "0"
	}
	if m.Type == TypePurchase {
		// Weighted average over the stock on hand; negative stock counts as zero.
		_, err = db.Exec(ctx, `
			UPDATE products SET average_cost = CASE
				WHEN GREATEST(stock, 0) + $2 > 0
				THEN GREATEST(round((GREATEST(stock, 0) * average_cost + $2 * $3::numeric) / (GREATEST(stock, 0) + $2), 4), 0)
				ELSE average_cost END
			WHERE id = $1`, m.ProductID, m.Quantity, cost)
		if err != nil {
			return 0, database.MapError(err, nil)
		}
	}

	_, err = db.Exec(ctx, `
		INSERT INTO inventory_movements (product_id, type, quantity, unit_cost, reason, source_type, source_id, created_by)
		VALUES ($1, $2, $3, $4::numeric, $5, NULLIF($6, ''), $7, $8)`,
		m.ProductID, m.Type, m.Quantity, cost, m.Reason, m.SourceType, m.SourceID, m.UserID)
	if err != nil {
		return 0, database.MapError(err, nil)
	}

	if minStock > 0 && stock > minStock && after <= minStock {
		if err := l.notifyLowStock(ctx, db, name, sku, after, minStock); err != nil {
			return 0, err
		}
	}
	return after, nil
}

// notifyLowStock emails every active user who can register purchases.
func (l *Ledger) notifyLowStock(ctx context.Context, db database.DBTX, product, sku string, stock, minStock int) error {
	business, err := settings.GetBusiness(ctx, db)
	if err != nil {
		return err
	}
	recipients, err := mail.RecipientsWithPermission(ctx, db, "purchases", "create")
	if err != nil {
		return err
	}
	for _, r := range recipients {
		err := mail.Enqueue(ctx, db, mail.TemplateLowStock, r.Email, map[string]any{
			"StoreName": business.Name, "Name": r.Name, "Product": product, "SKU": sku,
			"Stock": stock, "MinStock": minStock, "AlertsURL": l.appBaseURL + "/inventario",
		})
		if err != nil {
			return err
		}
	}
	return nil
}
