package server

import (
	"context"
	"net/http"
	"testing"
)

func TestProductCostVisibilityAndPriceRules(t *testing.T) {
	env := newTestEnv(t)
	if _, err := env.pool.Exec(context.Background(), `
		ALTER TABLE inventory_movements DISABLE TRIGGER inventory_movements_immutable;
		DELETE FROM inventory_movements;
		ALTER TABLE inventory_movements ENABLE TRIGGER inventory_movements_immutable;
		DELETE FROM product_presentations; DELETE FROM products; DELETE FROM categories; DELETE FROM brands;`); err != nil {
		t.Fatal(err)
	}
	admin := env.createAdmin()
	cashier := env.createCashier(admin)

	category := admin.do(http.MethodPost, "/categories", map[string]string{"name": "Ron"})
	expectStatus(t, category, http.StatusCreated, "create category")
	var unitID string
	_ = env.pool.QueryRow(context.Background(), `SELECT id FROM units WHERE abbreviation = 'bot'`).Scan(&unitID)

	product := map[string]any{
		"sku": "fdc-7-750", "barcode": "7430011001073", "name": "Flor de Caña 7 años 750 ml",
		"categoryId": category.body["id"], "unitId": unitID, "price": "421.75", "averageCost": "300",
		"presentations": []map[string]any{{"name": "Caja 12", "factor": 12, "barcode": "7430011001080", "price": "4800"}},
	}
	created := admin.do(http.MethodPost, "/products", product)
	expectStatus(t, created, http.StatusCreated, "create product")
	id := created.body["id"].(string)
	if created.body["sku"] != "FDC-7-750" || created.body["averageCost"] == nil {
		t.Errorf("admin should get a normalized SKU and the cost: %v", created.body)
	}

	seen := cashier.do(http.MethodGet, "/products/"+id, nil)
	expectStatus(t, seen, http.StatusOK, "cashier reads product")
	if _, hasCost := seen.body["averageCost"]; hasCost {
		t.Error("cashiers must never see costs")
	}

	lookup := cashier.do(http.MethodGet, "/products/lookup?code=7430011001080", nil)
	expectStatus(t, lookup, http.StatusOK, "lookup by presentation barcode")
	if lookup.body["presentationId"] == nil {
		t.Errorf("presentation barcode must resolve to the presentation: %v", lookup.body)
	}

	duplicate := admin.do(http.MethodPost, "/products", map[string]any{
		"sku": "OTRO", "barcode": "7430011001080", "name": "Otro", "categoryId": category.body["id"], "unitId": unitID, "price": "10",
	})
	expectStatus(t, duplicate, http.StatusUnprocessableEntity, "barcode already used by a presentation")

	res := admin.do(http.MethodPost, "/users", map[string]string{
		"fullName": "Pedro Bodega", "email": "pedro@elbarril.ni", "username": "pedro", "roleId": env.roleID("warehouse"),
	})
	expectStatus(t, res, http.StatusCreated, "create warehouse user")
	temp := extract(t, tempPasswordPattern, env.latestEmail("pedro@elbarril.ni"))
	warehouse := env.newClient()
	warehouse.login("pedro", temp)
	warehouse.do(http.MethodPost, "/auth/change-password", map[string]string{"currentPassword": temp, "newPassword": "Bodega2026Turno"})

	product["price"] = "399.00"
	res = warehouse.do(http.MethodPut, "/products/"+id, product)
	expectStatus(t, res, http.StatusForbidden, "warehouse changing the price")
	if res.errorCode() != "price_change_forbidden" {
		t.Errorf("code = %s, want price_change_forbidden", res.errorCode())
	}
}
