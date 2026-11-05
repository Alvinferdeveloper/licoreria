package server

import (
	"context"
	"net/http"
	"testing"
)

// A received purchase adds stock and moves the average cost; losses below the minimum
// queue an alert email; voiding the purchase is blocked once its stock is gone.
func TestPurchaseStockAndKardex(t *testing.T) {
	env := newTestEnv(t)
	resetCatalog(env)
	admin := env.createAdmin()
	ctx := context.Background()

	category := admin.do(http.MethodPost, "/categories", map[string]string{"name": "Ron"})
	supplier := admin.do(http.MethodPost, "/suppliers", map[string]any{"name": "Compañía Licorera"})
	expectStatus(t, supplier, http.StatusCreated, "create supplier")
	var unitID string
	_ = env.pool.QueryRow(ctx, `SELECT id FROM units WHERE abbreviation = 'bot'`).Scan(&unitID)
	product := admin.do(http.MethodPost, "/products", map[string]any{
		"sku": "FDC-7", "name": "Flor de Caña 7", "categoryId": category.body["id"], "unitId": unitID,
		"price": "421.75", "averageCost": "0", "minStock": 5,
	})
	expectStatus(t, product, http.StatusCreated, "create product")
	productID := product.body["id"].(string)

	buy := func(qty int, cost string) string {
		res := admin.do(http.MethodPost, "/purchases", map[string]any{
			"supplierId": supplier.body["id"], "supplierInvoiceNumber": "F-1", "purchaseDate": "2026-11-05",
			"items": []map[string]any{{"productId": productID, "quantity": qty, "unitCost": cost}},
		})
		expectStatus(t, res, http.StatusCreated, "create purchase")
		if res.body["status"] != "draft" {
			t.Fatalf("new purchases are drafts: %v", res.body["status"])
		}
		id := res.body["id"].(string)
		expectStatus(t, admin.do(http.MethodPost, "/purchases/"+id+"/receive", nil), http.StatusOK, "receive purchase")
		return id
	}
	first := buy(10, "300")
	buy(10, "320")

	var stock int
	var cost string
	_ = env.pool.QueryRow(ctx, `SELECT stock, average_cost::text FROM products WHERE id = $1`, productID).Scan(&stock, &cost)
	if stock != 20 || cost != "310.0000" {
		t.Fatalf("stock %d cost %s, want 20 at 310.0000", stock, cost)
	}

	loss := admin.do(http.MethodPost, "/inventory/adjustments", map[string]any{"productId": productID, "kind": "shrinkage", "quantity": 16, "reason": "Rotura en bodega"})
	expectStatus(t, loss, http.StatusCreated, "shrinkage")
	var alerts int
	_ = env.pool.QueryRow(ctx, `SELECT count(*) FROM email_queue WHERE template_key = 'low_stock'`).Scan(&alerts)
	if alerts == 0 {
		t.Error("falling to the minimum should queue a low stock email")
	}

	kardex := admin.do(http.MethodGet, "/inventory/movements?productId="+productID, nil)
	expectStatus(t, kardex, http.StatusOK, "kardex")
	if total := kardex.body["meta"].(map[string]any)["total"]; total != 3.0 {
		t.Errorf("kardex should list 2 purchases and 1 loss, got %v", total)
	}

	void := admin.do(http.MethodPost, "/purchases/"+first+"/void", map[string]string{"reason": "Factura duplicada"})
	if void.errorCode() != "insufficient_stock" {
		t.Errorf("voiding a purchase whose stock was used: got %d %s", void.status, void.errorCode())
	}
}
