package server

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
)

// fakeStorage answers like a public Supabase bucket, keeping objects in memory.
func fakeStorage(t *testing.T) *httptest.Server {
	var mu sync.Mutex
	objects := map[string][]byte{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		key := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "/storage/v1/object/public/"), "/storage/v1/object/")
		switch r.Method {
		case http.MethodPost:
			objects[key], _ = io.ReadAll(r.Body)
		case http.MethodDelete:
			delete(objects, key)
		default:
			data, ok := objects[key]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write(data)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestConfigurationFlow(t *testing.T) {
	env := newTestEnv(t)
	resetCatalog(env)
	ctx := context.Background()
	// newTestEnv empties settings: truncating users cascades to it.
	if _, err := env.pool.Exec(ctx, `INSERT INTO settings (key, value) VALUES ('pos', '{"cashRounding": "none"}')`); err != nil {
		t.Fatal(err)
	}

	bucket := fakeStorage(t)
	env.cfg.Storage.SupabaseURL, env.cfg.Storage.SupabaseServiceKey, env.cfg.Storage.Bucket = bucket.URL, "test-key", "productos"
	logger := slog.New(slog.NewTextHandler(testLogWriter{t}, nil))
	env.server = httptest.NewServer(NewRouter(Deps{Config: env.cfg, Logger: logger, DB: env.pool, Backups: NewBackups(env.cfg, logger, env.pool)}))
	t.Cleanup(env.server.Close)

	admin := env.createAdmin()
	cashier := env.createCashier(admin)
	expectStatus(t, cashier.do(http.MethodGet, "/settings", nil), http.StatusForbidden, "cashier reads settings")

	body := map[string]any{
		"business": map[string]string{"name": "Licorería El Roble", "taxId": "J0310000999999", "address": "Managua", "phone": "2222 0000", "email": "hola@elroble.ni"},
		"tax":      map[string]any{"defaultPercent": 10},
		"pos":      map[string]any{"discountLimitPercent": 8, "defaultPrintFormat": "invoice"},
		"invoice":  map[string]string{"footerNote": "Gracias por preferir El Roble."},
	}
	expectStatus(t, admin.do(http.MethodPut, "/settings", body), http.StatusOK, "update settings")

	store := cashier.do(http.MethodGet, "/settings/store", nil)
	if store.body["business"].(map[string]any)["name"] != "Licorería El Roble" || store.body["pos"].(map[string]any)["defaultPrintFormat"] != "invoice" {
		t.Fatalf("store settings: %v", store.body)
	}
	var unknownKeyKept bool
	if err := env.pool.QueryRow(ctx, `SELECT value ? 'cashRounding' FROM settings WHERE key = 'pos'`).Scan(&unknownKeyKept); err != nil || !unknownKeyKept {
		t.Fatalf("saving dropped a stored key: %v", err)
	}

	category := admin.do(http.MethodPost, "/categories", map[string]string{"name": "Ron"})
	var unitID, registerID string
	_ = env.pool.QueryRow(ctx, `SELECT id FROM units WHERE abbreviation = 'bot'`).Scan(&unitID)
	_ = env.pool.QueryRow(ctx, `SELECT id FROM registers WHERE invoice_series = 'F001'`).Scan(&registerID)
	product := admin.do(http.MethodPost, "/products", map[string]any{"sku": "RON-9", "name": "Ron Nueve", "categoryId": category.body["id"], "unitId": unitID, "price": "100", "averageCost": "60"})
	expectStatus(t, product, http.StatusCreated, "product")
	if tax := product.body["taxPercent"]; tax != "10.00" {
		t.Fatalf("new product should take the default VAT, got %v", tax)
	}

	expectStatus(t, admin.upload("/settings/logo", "logo", "nota.txt", []byte("no soy una imagen")), http.StatusUnprocessableEntity, "text as logo")
	var png1 bytes.Buffer
	if err := png.Encode(&png1, image.NewRGBA(image.Rect(0, 0, 64, 64))); err != nil {
		t.Fatal(err)
	}
	logo := admin.upload("/settings/logo", "logo", "logo.png", png1.Bytes())
	expectStatus(t, logo, http.StatusOK, "upload logo")

	productID := product.body["id"].(string)
	expectStatus(t, admin.do(http.MethodPost, "/inventory/adjustments", map[string]any{"productId": productID, "kind": "count", "quantity": 5, "reason": "Conteo inicial"}), http.StatusCreated, "stock")
	expectStatus(t, admin.do(http.MethodPost, "/exchange-rates", map[string]string{"rate": "36.60"}), http.StatusOK, "rate")
	expectStatus(t, cashier.do(http.MethodPost, "/cash-shifts", map[string]string{"registerId": registerID, "openingNio": "500", "openingUsd": "0"}), http.StatusCreated, "open shift")
	sale := cashier.sendJSON(http.MethodPost, "/sales", map[string]any{
		"ageVerified": true,
		"lines":       []map[string]any{{"productId": productID, "quantity": 1}},
		"payments":    []map[string]string{{"method": "cash", "currency": "NIO", "amount": "200"}},
	}, map[string]string{"Idempotency-Key": uuid.NewString()})
	expectStatus(t, sale, http.StatusCreated, "sale")

	saleID := sale.body["id"].(string)
	withLogo := cashier.do(http.MethodGet, "/sales/"+saleID+"/invoice.pdf", nil)
	expectStatus(t, withLogo, http.StatusOK, "invoice with logo")
	if !bytes.Contains(withLogo.raw, []byte("/Subtype /Image")) {
		t.Fatal("the invoice should draw the logo")
	}

	expectStatus(t, admin.do(http.MethodDelete, "/settings/logo", nil), http.StatusOK, "remove logo")
	removed := admin.do(http.MethodGet, "/settings", nil)
	if removed.body["business"].(map[string]any)["logoUrl"] != nil {
		t.Fatalf("logo still set: %v", removed.body["business"])
	}
}
