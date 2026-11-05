package server

import (
	"context"
	"net/http"
	"testing"
)

// Only admins set the daily rate; a cashier opens one shift, cannot spend more cash than
// the drawer holds, and closing short records the difference and alerts supervisors.
func TestExchangeRateAndCashShift(t *testing.T) {
	env := newTestEnv(t)
	admin := env.createAdmin()
	cashier := env.createCashier(admin)
	ctx := context.Background()

	expectStatus(t, cashier.do(http.MethodGet, "/exchange-rates/today", nil), http.StatusNotFound, "no rate yet")
	expectStatus(t, cashier.do(http.MethodPost, "/exchange-rates", map[string]string{"rate": "36.6243"}), http.StatusForbidden, "cashier sets rate")
	expectStatus(t, admin.do(http.MethodPost, "/exchange-rates", map[string]string{"rate": "36.6243"}), http.StatusOK, "admin sets rate")
	if today := cashier.do(http.MethodGet, "/exchange-rates/today", nil); today.body["rate"] != "36.624300" {
		t.Fatalf("today's rate = %v", today.body["rate"])
	}

	var registerID string
	_ = env.pool.QueryRow(ctx, `SELECT id FROM registers WHERE invoice_series = 'F001'`).Scan(&registerID)
	opened := cashier.do(http.MethodPost, "/cash-shifts", map[string]string{"registerId": registerID, "openingNio": "1000", "openingUsd": "20"})
	expectStatus(t, opened, http.StatusCreated, "open shift")
	shiftID := opened.body["id"].(string)
	if res := admin.do(http.MethodPost, "/cash-shifts", map[string]string{"registerId": registerID, "openingNio": "0", "openingUsd": "0"}); res.errorCode() != "register_busy" {
		t.Errorf("second shift on the same register: %d %s", res.status, res.errorCode())
	}

	expense := map[string]string{"type": "expense", "currency": "USD", "amount": "25", "reason": "Pago de hielo"}
	if res := cashier.do(http.MethodPost, "/cash-shifts/"+shiftID+"/movements", expense); res.status != http.StatusUnprocessableEntity {
		t.Errorf("expense above the drawer: status %d", res.status)
	}
	expense["currency"], expense["amount"] = "NIO", "150.50"
	expectStatus(t, cashier.do(http.MethodPost, "/cash-shifts/"+shiftID+"/movements", expense), http.StatusCreated, "expense")

	closed := cashier.do(http.MethodPost, "/cash-shifts/"+shiftID+"/close", map[string]string{"countedNio": "800.00", "countedUsd": "20"})
	expectStatus(t, closed, http.StatusOK, "close shift")
	if closed.body["expectedNio"] != "849.50" || closed.body["differenceNio"] != "-49.50" || closed.body["differenceUsd"] != "0.00" {
		t.Errorf("arqueo: expected %v difference %v / %v", closed.body["expectedNio"], closed.body["differenceNio"], closed.body["differenceUsd"])
	}
	var alerts int
	_ = env.pool.QueryRow(ctx, `SELECT count(*) FROM email_queue WHERE template_key = 'cash_difference'`).Scan(&alerts)
	if alerts == 0 {
		t.Error("a closing difference should email the supervisors")
	}
}
