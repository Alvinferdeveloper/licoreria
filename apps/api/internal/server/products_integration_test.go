package server

import (
	"context"
	"net/http"
	"testing"
)

func resetCatalog(env *testEnv) {
	env.t.Helper()
	if _, err := env.pool.Exec(context.Background(), `
		ALTER TABLE inventory_movements DISABLE TRIGGER inventory_movements_immutable;
		DELETE FROM inventory_movements;
		ALTER TABLE inventory_movements ENABLE TRIGGER inventory_movements_immutable;
		DELETE FROM purchases; DELETE FROM product_presentations; DELETE FROM products; DELETE FROM categories; DELETE FROM brands;`); err != nil {
		env.t.Fatal(err)
	}
}

func TestProductImport(t *testing.T) {
	env := newTestEnv(t)
	resetCatalog(env)
	admin := env.createAdmin()
	cashier := env.createCashier(admin)
	ctx := context.Background()

	category := admin.do(http.MethodPost, "/categories", map[string]string{"name": "Ron"})
	expectStatus(t, category, http.StatusCreated, "create category")
	var unitID string
	_ = env.pool.QueryRow(ctx, `SELECT id FROM units WHERE abbreviation = 'bot'`).Scan(&unitID)
	expectStatus(t, admin.do(http.MethodPost, "/products", map[string]any{
		"sku": "EXISTE", "barcode": "7430000000001", "name": "Ron existente", "categoryId": category.body["id"], "unitId": unitID, "price": "100",
	}), http.StatusCreated, "seed product")

	csv := []byte("sku;codigo_barras;nombre;categoria;marca;unidad;precio;costo\n" +
		"r-1;7430000000002;Ron Uno;ron;Casa Nueva;bot;C$ 1,250.50;900\n" +
		"R-2;;Ron Dos;Ron;;Botella;421,75;\n" +
		"r-1;;Repetido;Ron;;bot;10;\n" +
		"EXISTE;;Ya existe;Ron;;bot;10;\n" +
		"R-3;7430000000001;Código usado;Whisky;;bot;diez;\n")

	expectStatus(t, cashier.upload("/products/import?dryRun=true", "file", "productos.csv", csv), http.StatusForbidden, "cashier import")

	preview := admin.upload("/products/import?dryRun=true", "file", "productos.csv", csv)
	expectStatus(t, preview, http.StatusOK, "preview")
	if preview.body["valid"] != 2.0 || preview.body["invalid"] != 3.0 || preview.body["created"] != 0.0 {
		t.Fatalf("unexpected preview summary: %v", preview.body)
	}
	last := preview.body["rows"].([]any)[4].(map[string]any)
	if n := len(last["errors"].([]any)); n != 3 {
		t.Errorf("row 6 should report barcode, category and price errors, got %v", last["errors"])
	}
	var count int
	_ = env.pool.QueryRow(ctx, `SELECT count(*) FROM products`).Scan(&count)
	if count != 1 {
		t.Fatalf("a dry run must not insert, products = %d", count)
	}

	result := admin.upload("/products/import", "file", "productos.csv", csv)
	expectStatus(t, result, http.StatusOK, "import")
	if result.body["created"] != 2.0 {
		t.Fatalf("created = %v, want 2", result.body["created"])
	}
	var price, brand string
	if err := env.pool.QueryRow(ctx, `SELECT p.price::text, b.name FROM products p JOIN brands b ON b.id = p.brand_id WHERE p.sku = 'R-1'`).
		Scan(&price, &brand); err != nil || price != "1250.50" || brand != "Casa Nueva" {
		t.Errorf("R-1 = %s %s (%v), want 1250.50 with the new brand", price, brand, err)
	}
}

func TestProductCostVisibilityAndPriceRules(t *testing.T) {
	env := newTestEnv(t)
	resetCatalog(env)
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
	warehouse := env.activateFromEmail("pedro@elbarril.ni", "pedro", "Bodega2026Turno")

	product["price"] = "399.00"
	res = warehouse.do(http.MethodPut, "/products/"+id, product)
	expectStatus(t, res, http.StatusForbidden, "warehouse changing the price")
	if res.errorCode() != "price_change_forbidden" {
		t.Errorf("code = %s, want price_change_forbidden", res.errorCode())
	}
}
