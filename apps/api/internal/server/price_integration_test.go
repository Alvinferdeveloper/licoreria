package server

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// A price fixed from the purchase screen shows up on that purchase as already updated.
func TestPriceChangeAfterPurchase(t *testing.T) {
	env := newTestEnv(t)
	resetCatalog(env)
	admin := env.createAdmin()

	category := admin.do(http.MethodPost, "/categories", map[string]string{"name": "Ron"})
	supplier := admin.do(http.MethodPost, "/suppliers", map[string]any{"name": "Distribuidora"})
	var unitID string
	_ = env.pool.QueryRow(context.Background(), `SELECT id FROM units LIMIT 1`).Scan(&unitID)
	product := admin.do(http.MethodPost, "/products", map[string]any{
		"sku": "FDC-7", "name": "Flor de Caña 7", "categoryId": category.body["id"], "unitId": unitID, "price": "450", "averageCost": "0",
	})
	expectStatus(t, product, http.StatusCreated, "create product")
	productID := product.body["id"].(string)

	purchase := admin.do(http.MethodPost, "/purchases", map[string]any{
		"supplierId": supplier.body["id"], "purchaseDate": time.Now().Format(time.DateOnly),
		"items": []map[string]any{{"productId": productID, "quantity": 6, "unitCost": "336"}},
	})
	expectStatus(t, purchase, http.StatusCreated, "create purchase")
	expectStatus(t, admin.do(http.MethodPatch, "/products/"+productID+"/price", map[string]string{"price": "504"}), http.StatusOK, "set price")

	res := admin.do(http.MethodGet, "/purchases/"+purchase.body["id"].(string), nil)
	expectStatus(t, res, http.StatusOK, "get purchase")
	item := res.body["items"].([]any)[0].(map[string]any)
	if item["salePrice"] != "504.00" || item["priceUpdated"] != true {
		t.Errorf("want the new price flagged as updated, got %v / %v", item["salePrice"], item["priceUpdated"])
	}
}
