package server

import (
	"context"
	"net/http"
	"testing"
	"time"

	"licoreria/api/internal/inventory"
)

// Stock is assigned to the newest lots first, so after selling some units only what remains
// of each lot is reported, and the admins get one email per lot entering the window.
func TestExpiringLotsSellOldestFirst(t *testing.T) {
	env := newTestEnv(t)
	resetCatalog(env)
	admin := env.createAdmin()
	ctx := context.Background()

	category := admin.do(http.MethodPost, "/categories", map[string]string{"name": "Cerveza"})
	supplier := admin.do(http.MethodPost, "/suppliers", map[string]any{"name": "Cervecera"})
	var unitID string
	_ = env.pool.QueryRow(ctx, `SELECT id FROM units WHERE abbreviation = 'lata'`).Scan(&unitID)
	product := admin.do(http.MethodPost, "/products", map[string]any{
		"sku": "TON-LAT", "name": "Toña lata", "categoryId": category.body["id"], "unitId": unitID, "price": "35", "averageCost": "0",
	})
	expectStatus(t, product, http.StatusCreated, "create product")
	productID := product.body["id"].(string)

	day := func(offset int) string { return time.Now().AddDate(0, 0, offset).Format(time.DateOnly) }
	buy := func(qty int, expires string) {
		res := admin.do(http.MethodPost, "/purchases", map[string]any{
			"supplierId": supplier.body["id"], "purchaseDate": day(0),
			"items": []map[string]any{{"productId": productID, "quantity": qty, "unitCost": "20", "expiresOn": expires}},
		})
		expectStatus(t, res, http.StatusCreated, "create purchase")
		expectStatus(t, admin.do(http.MethodPost, "/purchases/"+res.body["id"].(string)+"/receive", nil), http.StatusOK, "receive")
	}
	buy(10, day(10)) // older lot, expires first
	buy(10, day(80))
	loss := admin.do(http.MethodPost, "/inventory/adjustments", map[string]any{"productId": productID, "kind": "shrinkage", "quantity": 6, "reason": "Venta de prueba"})
	expectStatus(t, loss, http.StatusCreated, "take 6 out")

	res := admin.do(http.MethodGet, "/inventory/expiring?days=90", nil)
	expectStatus(t, res, http.StatusOK, "list expiring")
	lots := res.body["data"].([]any)
	if len(lots) != 2 {
		t.Fatalf("want both lots, got %v", lots)
	}
	first, second := lots[0].(map[string]any), lots[1].(map[string]any)
	if first["remaining"] != float64(4) || second["remaining"] != float64(10) {
		t.Errorf("oldest lot should hold 4 and newest 10, got %v and %v", first["remaining"], second["remaining"])
	}

	for range 2 {
		if _, err := inventory.AlertExpiring(ctx, env.pool, "http://localhost", time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	var alerted, emails int
	_ = env.pool.QueryRow(ctx, `SELECT count(*) FROM purchase_items WHERE expiry_alerted_at IS NOT NULL`).Scan(&alerted)
	_ = env.pool.QueryRow(ctx, `SELECT count(*) FROM email_queue WHERE subject LIKE '%por vencer%'`).Scan(&emails)
	if alerted != 1 || emails != 1 {
		t.Errorf("only the lot inside 30 days alerts, once: alerted %d, emails %d", alerted, emails)
	}
}
