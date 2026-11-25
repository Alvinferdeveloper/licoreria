package server

import (
	"context"
	"net/http"
	"testing"
)

// A session unused for longer than security.idleTimeoutMinutes is rejected and cannot be renewed.
func TestIdleSessionEnds(t *testing.T) {
	env := newTestEnv(t)
	admin := env.createAdmin()
	ctx := context.Background()

	me := admin.do(http.MethodGet, "/auth/me", nil)
	expectStatus(t, me, http.StatusOK, "active session")
	if me.body["idleTimeoutMinutes"] != float64(30) {
		t.Errorf("the screen needs the idle limit, got %v", me.body["idleTimeoutMinutes"])
	}

	if _, err := env.pool.Exec(ctx, `UPDATE refresh_sessions SET last_seen_at = now() - interval '31 minutes' WHERE revoked_at IS NULL`); err != nil {
		t.Fatal(err)
	}
	expectStatus(t, admin.do(http.MethodGet, "/auth/me", nil), http.StatusUnauthorized, "idle access token")
	res := admin.refresh()
	expectStatus(t, res, http.StatusUnauthorized, "idle refresh")
	if code := res.body["error"].(map[string]any)["code"]; code != "session_idle" {
		t.Errorf("want session_idle, got %v", code)
	}
}
