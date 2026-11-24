package inventory

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"licoreria/api/internal/auth"
	"licoreria/api/internal/mail"
	"licoreria/api/internal/platform/database"
)

// AlertDays is how far ahead the daily email looks for lots about to expire.
const AlertDays = 30

// ExpiringLot is what is left on the shelf from one received purchase line with a best-before date.
// The shelf is assumed to sell oldest first, so current stock belongs to the newest purchases.
type ExpiringLot struct {
	LineID     uuid.UUID  `json:"lineId"`
	Product    ProductRef `json:"product"`
	Category   string     `json:"category"`
	Supplier   Ref        `json:"supplier"`
	PurchaseID uuid.UUID  `json:"purchaseId"`
	ReceivedAt time.Time  `json:"receivedAt"`
	ExpiresOn  string     `json:"expiresOn"`
	DaysLeft   int        `json:"daysLeft"`
	Remaining  int        `json:"remaining"`
	UnitCost   *string    `json:"unitCost,omitempty"`
}

const expiringQuery = `
	WITH lots AS (
		SELECT pi.id, pi.product_id, pi.quantity, pi.unit_cost, pi.expires_on, pi.expiry_alerted_at,
		       pu.id AS purchase_id, pu.received_at, pu.supplier_id, p.stock,
		       sum(pi.quantity) OVER (PARTITION BY pi.product_id ORDER BY pu.received_at DESC, pi.id) AS newer_and_this
		FROM purchase_items pi
		JOIN purchases pu ON pu.id = pi.purchase_id AND pu.status = 'received'
		JOIN products p ON p.id = pi.product_id AND p.active AND p.stock > 0
	)
	SELECT l.id, p.id, p.sku::text, p.name, c.name, s.id, s.name, l.purchase_id, l.received_at,
	       l.expires_on::text, l.expires_on - (now() AT TIME ZONE 'America/Managua')::date,
	       least(l.quantity, l.stock - (l.newer_and_this - l.quantity))::int, l.unit_cost::text
	FROM lots l
	JOIN products p ON p.id = l.product_id
	JOIN categories c ON c.id = p.category_id
	JOIN suppliers s ON s.id = l.supplier_id
	WHERE l.expires_on IS NOT NULL
	  AND l.stock - (l.newer_and_this - l.quantity) > 0
	  AND l.expires_on <= (now() AT TIME ZONE 'America/Managua')::date + $1::int`

func scanLot(row pgx.CollectableRow) (ExpiringLot, error) {
	var l ExpiringLot
	err := row.Scan(&l.LineID, &l.Product.ID, &l.Product.SKU, &l.Product.Name, &l.Category, &l.Supplier.ID, &l.Supplier.Name,
		&l.PurchaseID, &l.ReceivedAt, &l.ExpiresOn, &l.DaysLeft, &l.Remaining, &l.UnitCost)
	return l, err
}

// Expiring lists lots expired or expiring within `days`, soonest first.
func (s *Service) Expiring(ctx context.Context, p auth.Principal, days int) ([]ExpiringLot, error) {
	rows, err := s.pool.Query(ctx, expiringQuery+` ORDER BY l.expires_on, p.name`, days)
	if err != nil {
		return nil, fmt.Errorf("list expiring lots: %w", err)
	}
	lots, err := pgx.CollectRows(rows, scanLot)
	if err != nil {
		return nil, err
	}
	if !canSeeCosts(p) {
		for i := range lots {
			lots[i].UnitCost = nil
		}
	}
	return lots, nil
}

// AlertExpiring emails administrators once about each lot entering the alert window.
func AlertExpiring(ctx context.Context, db database.DBTX, appURL string, now time.Time) (int, error) {
	rows, err := db.Query(ctx, expiringQuery+` AND l.expiry_alerted_at IS NULL ORDER BY l.expires_on, p.name`, AlertDays)
	if err != nil {
		return 0, fmt.Errorf("find lots to alert: %w", err)
	}
	lots, err := pgx.CollectRows(rows, scanLot)
	if err != nil || len(lots) == 0 {
		return 0, err
	}

	lines := make([]string, 0, len(lots))
	ids := make([]uuid.UUID, 0, len(lots))
	for _, l := range lots {
		ids = append(ids, l.LineID)
		when := fmt.Sprintf("vence en %d días", l.DaysLeft)
		switch {
		case l.DaysLeft < 0:
			when = "ya venció"
		case l.DaysLeft == 0:
			when = "vence hoy"
		case l.DaysLeft == 1:
			when = "vence mañana"
		}
		lines = append(lines, fmt.Sprintf("%s: %d u, %s (%s)", l.Product.Name, l.Remaining, when, l.ExpiresOn))
	}
	err = mail.AlertAdmins(ctx, db, mail.Alert{
		Subject:     fmt.Sprintf("%d %s por vencer", len(lots), plural(len(lots), "producto", "productos")),
		Body:        "Estos lotes vencen en los próximos 30 días o ya vencieron: " + strings.Join(lines, "; ") + ".",
		Advice:      "Ponlos al frente del estante o en promoción. Lo vencido se da de baja como merma desde Inventario.",
		ActionURL:   appURL + "/inventario?vista=vencimientos",
		ActionLabel: "Ver vencimientos",
		When:        now,
	})
	if err != nil {
		return 0, err
	}
	_, err = db.Exec(ctx, `UPDATE purchase_items SET expiry_alerted_at = now() WHERE id = ANY($1)`, ids)
	return len(lots), err
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// RunExpiryAlerts checks for lots to alert every few hours until ctx ends.
func RunExpiryAlerts(ctx context.Context, db database.DBTX, appURL string, logger *slog.Logger) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for {
		if n, err := AlertExpiring(ctx, db, appURL, time.Now()); err != nil && ctx.Err() == nil {
			logger.ErrorContext(ctx, "expiry alert", "error", err)
		} else if n > 0 {
			logger.InfoContext(ctx, "expiry alert queued", "lots", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
