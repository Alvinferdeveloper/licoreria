package server

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"licoreria/api/internal/schedule"
)

func TestUsageSchedule(t *testing.T) {
	env := newTestEnv(t)
	admin := env.createAdmin()
	cashier := env.createCashier(admin)

	// A window that opens two hours from now: every login and sale below is outside it.
	now := time.Now().In(schedule.Zone)
	day := map[string]any{"closed": false, "open": now.Add(2 * time.Hour).Format("15:04"), "close": now.Add(3 * time.Hour).Format("15:04")}
	body := map[string]any{
		"business": map[string]string{"name": "Licorería"},
		"pos":      map[string]any{"discountLimitPercent": 5, "defaultPrintFormat": "ticket"},
		"schedule": map[string]any{"enabled": true, "toleranceMinutes": 0, "days": []any{day, day, day, day, day, day, day}},
	}
	expectStatus(t, admin.do(http.MethodPut, "/settings", body), http.StatusOK, "enable schedule")

	sale := cashier.sendJSON(http.MethodPost, "/sales", map[string]any{
		"ageVerified": true,
		"lines":       []map[string]any{{"productId": uuid.NewString(), "quantity": 1}},
		"payments":    []map[string]string{{"method": "cash", "currency": "NIO", "amount": "100"}},
	}, map[string]string{"Idempotency-Key": uuid.NewString()})
	if sale.status != http.StatusForbidden || sale.errorCode() != "outside_hours" {
		t.Fatalf("sale outside hours: %d %v", sale.status, sale.body)
	}

	late := env.newClient()
	if res := late.login("marvin", "Caja1Turno2026"); res.errorCode() != "outside_hours" {
		t.Fatalf("cashier login outside hours: %d %v", res.status, res.body)
	}
	expectStatus(t, env.newClient().login("lucia", adminPassword), http.StatusOK, "admin login outside hours")
	var flagged int
	if err := env.pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_log WHERE action = 'auth.login_outside_hours'`).Scan(&flagged); err != nil || flagged != 1 {
		t.Fatalf("admin login outside hours should be audited once, got %d (%v)", flagged, err)
	}

	cashierID := cashier.do(http.MethodGet, "/auth/me", nil).body["id"]
	ext := admin.do(http.MethodPost, "/schedule/extensions", map[string]any{"userId": cashierID, "until": now.Add(time.Hour).Format("15:04")})
	expectStatus(t, ext, http.StatusCreated, "extend schedule")
	expectStatus(t, late.login("marvin", "Caja1Turno2026"), http.StatusOK, "cashier login with extension")

	expectStatus(t, admin.do(http.MethodDelete, "/schedule/extensions/"+ext.body["id"].(string), nil), http.StatusOK, "end extension")
	if res := env.newClient().login("marvin", "Caja1Turno2026"); res.errorCode() != "outside_hours" {
		t.Fatalf("login after ending the extension: %d %v", res.status, res.body)
	}

	if n := env.countEmails("lucia@elbarril.ni", "Intento de entrada fuera de horario"); n != 1 {
		t.Errorf("two blocked attempts within the hour should alert the admin once, got %d", n)
	}
	if n := env.countEmails("lucia@elbarril.ni", "Entraste fuera de horario"); n != 1 {
		t.Errorf("the admin should be told about their own out-of-hours login, got %d", n)
	}
}

func TestSecurityAlerts(t *testing.T) {
	env := newTestEnv(t)
	admin := env.createAdmin()
	env.createCashier(admin)

	for range 5 {
		env.newClient().login("marvin", "Incorrecta2026")
	}
	if env.countEmails("marvin@elbarril.ni", "Tu cuenta se bloqueó") != 1 || env.countEmails("lucia@elbarril.ni", "Cuenta bloqueada: Marvin") != 1 {
		t.Error("a lockout should alert the owner and the administrators")
	}

	expectStatus(t, admin.do(http.MethodPost, "/auth/change-password", map[string]string{
		"currentPassword": adminPassword, "newPassword": "OtraClave2026Segura",
	}), http.StatusNoContent, "change password")
	if n := env.countEmails("lucia@elbarril.ni", "Tu contraseña cambió"); n != 1 {
		t.Errorf("password change alerts = %d, want 1 (the first, forced change does not alert)", n)
	}
}
