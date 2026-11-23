package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSaleFlow(t *testing.T) {
	env := newTestEnv(t)
	resetCatalog(env)
	admin := env.createAdmin()
	cashier := env.createCashier(admin)
	ctx := context.Background()
	if _, err := env.pool.Exec(ctx, `UPDATE registers SET next_invoice_number = 1`); err != nil {
		t.Fatal(err)
	}

	category := admin.do(http.MethodPost, "/categories", map[string]string{"name": "Ron"})
	var unitID, registerID string
	_ = env.pool.QueryRow(ctx, `SELECT id FROM units WHERE abbreviation = 'bot'`).Scan(&unitID)
	_ = env.pool.QueryRow(ctx, `SELECT id FROM registers WHERE invoice_series = 'F001'`).Scan(&registerID)
	product := admin.do(http.MethodPost, "/products", map[string]any{
		"sku": "RON-1", "name": "Ron Uno", "categoryId": category.body["id"], "unitId": unitID, "price": "100", "averageCost": "60",
	})
	productID := product.body["id"].(string)
	expectStatus(t, admin.do(http.MethodPost, "/inventory/adjustments", map[string]any{"productId": productID, "kind": "count", "quantity": 10, "reason": "Conteo inicial"}), http.StatusCreated, "stock")
	expectStatus(t, admin.do(http.MethodPost, "/exchange-rates", map[string]string{"rate": "36.60"}), http.StatusOK, "rate")

	sale := map[string]any{
		"ageVerified": true,
		"discount":    map[string]string{"kind": "percent", "value": "10"},
		"lines":       []map[string]any{{"productId": productID, "quantity": 2}},
		"payments":    []map[string]string{{"method": "cash", "currency": "USD", "amount": "10"}},
	}
	key := uuid.NewString()
	sell := func(body map[string]any) response {
		return cashier.sendJSON(http.MethodPost, "/sales", body, map[string]string{"Idempotency-Key": key})
	}

	if res := sell(sale); res.errorCode() != "no_open_shift" {
		t.Fatalf("selling without a shift: %d %s", res.status, res.errorCode())
	}
	expectStatus(t, cashier.do(http.MethodPost, "/cash-shifts", map[string]string{"registerId": registerID, "openingNio": "500", "openingUsd": "0"}), http.StatusCreated, "open shift")

	if res := sell(sale); res.errorCode() != "authorization_required" {
		t.Fatalf("10%% discount over the 5%% limit: %d %s", res.status, res.errorCode())
	}
	expectStatus(t, admin.do(http.MethodPost, "/auth/pin", map[string]string{"currentPassword": adminPassword, "pin": "4821"}), http.StatusNoContent, "set pin")
	sale["authorization"] = map[string]string{"username": "lucia", "pin": "4821"}

	created := sell(sale)
	expectStatus(t, created, http.StatusCreated, "sale")
	// 2 x 100 = 200, -10% = 180, +15% IVA = 207; US$ 10 at 36.60 = 366 → change 159.
	if created.body["total"] != "207.00" || created.body["changeNio"] != "159.00" || created.body["invoice"] != "F001-000001" {
		t.Fatalf("sale totals: total %v change %v invoice %v", created.body["total"], created.body["changeNio"], created.body["invoice"])
	}
	again := sell(sale)
	expectStatus(t, again, http.StatusOK, "retry with the same key")
	if again.body["id"] != created.body["id"] {
		t.Error("a retried sale must return the original, not charge twice")
	}

	stock := func() (n int) {
		_ = env.pool.QueryRow(ctx, `SELECT stock FROM products WHERE id = $1`, productID).Scan(&n)
		return n
	}
	if stock() != 8 {
		t.Fatalf("stock after selling 2 = %d, want 8", stock())
	}

	saleID := created.body["id"].(string)
	expectStatus(t, cashier.do(http.MethodGet, "/sales/"+saleID+"/invoice.pdf", nil), http.StatusOK, "invoice pdf")
	expectStatus(t, cashier.do(http.MethodPost, "/sales/"+saleID+"/email", map[string]string{"email": "cliente@correo.ni"}), http.StatusNoContent, "email invoice")
	var queued int
	_ = env.pool.QueryRow(ctx, `SELECT count(*) FROM email_queue WHERE template_key = 'invoice'`).Scan(&queued)
	if queued != 1 {
		t.Errorf("invoice email should be queued, got %d", queued)
	}

	expectStatus(t, cashier.do(http.MethodPost, "/sales/"+saleID+"/void", map[string]any{
		"reason": "Cliente se arrepintió", "authorization": map[string]string{"username": "lucia", "pin": "4821"},
	}), http.StatusOK, "void with pin")
	if stock() != 10 {
		t.Errorf("voiding must return the stock, got %d", stock())
	}

	// Card needs the voucher's authorization number, and the shift lists it for the bank's batch close.
	card := map[string]any{
		"ageVerified": true,
		"lines":       []map[string]any{{"productId": productID, "quantity": 1}},
		"payments":    []map[string]string{{"method": "card", "currency": "NIO", "amount": "115"}},
	}
	if res := cashier.sendJSON(http.MethodPost, "/sales", card, map[string]string{"Idempotency-Key": uuid.NewString()}); !strings.Contains(fmt.Sprint(res.body), "reference_required") {
		t.Fatalf("card without authorization: %d %v", res.status, res.body)
	}
	card["payments"] = []map[string]string{{"method": "card", "currency": "NIO", "amount": "115", "reference": " 042817 "}}
	expectStatus(t, cashier.sendJSON(http.MethodPost, "/sales", card, map[string]string{"Idempotency-Key": uuid.NewString()}), http.StatusCreated, "card sale")
	shift := cashier.do(http.MethodGet, "/cash-shifts/current", nil)
	if nonCash, _ := shift.body["nonCash"].([]any); len(nonCash) != 1 || nonCash[0].(map[string]any)["reference"] != "042817" {
		t.Fatalf("shift should list the card authorization, got %v", shift.body["nonCash"])
	}

	// A sale made without connection is recorded even past the stock, and the admins hear about it.
	offline := map[string]any{
		"ageVerified": true,
		"offlineAt":   time.Now().Add(-10 * time.Minute).Format(time.RFC3339),
		"lines":       []map[string]any{{"productId": productID, "quantity": 12}},
		"payments":    []map[string]string{{"method": "cash", "currency": "NIO", "amount": "1400"}},
	}
	synced := cashier.sendJSON(http.MethodPost, "/sales", offline, map[string]string{"Idempotency-Key": uuid.NewString()})
	expectStatus(t, synced, http.StatusCreated, "offline sale")
	if synced.body["offlineAt"] == nil || stock() != -3 {
		t.Fatalf("offline sale: offlineAt %v, stock %d (want -3)", synced.body["offlineAt"], stock())
	}
	if env.countEmails("lucia@elbarril.ni", "Venta sin conexión dejó stock negativo") != 1 {
		t.Error("an offline sale past the stock should alert the admins")
	}
	offline["offlineAt"] = time.Now().Add(-72 * time.Hour).Format(time.RFC3339)
	if res := cashier.sendJSON(http.MethodPost, "/sales", offline, map[string]string{"Idempotency-Key": uuid.NewString()}); res.status != http.StatusUnprocessableEntity {
		t.Errorf("an offline sale older than 48 h must be refused, got %d", res.status)
	}

	// Every report runs against real data; exports and the dashboard render.
	catalog := admin.do(http.MethodGet, "/reports", nil)
	expectStatus(t, catalog, http.StatusOK, "report catalog")
	for _, r := range catalog.body["reports"].([]any) {
		key := r.(map[string]any)["key"].(string)
		expectStatus(t, admin.do(http.MethodGet, "/reports/"+key+"?productId="+productID, nil), http.StatusOK, "report "+key)
	}
	voided := admin.do(http.MethodGet, "/reports/voided", nil)
	if rows := voided.body["rows"].([]any); len(rows) != 1 {
		t.Errorf("voided report should list the voided sale, got %d rows", len(rows))
	}
	expectStatus(t, admin.do(http.MethodGet, "/reports/kardex?productId="+productID+"&format=xlsx", nil), http.StatusOK, "kardex xlsx")
	expectStatus(t, admin.do(http.MethodGet, "/reports/sales-detail?format=pdf", nil), http.StatusOK, "sales pdf")
	expectStatus(t, cashier.do(http.MethodGet, "/dashboard", nil), http.StatusOK, "dashboard")
}
