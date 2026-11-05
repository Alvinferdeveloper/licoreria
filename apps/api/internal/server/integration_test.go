package server

import (
	"bytes"
	"context"
	"encoding/json"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"licoreria/api/internal/mail"
	"licoreria/api/internal/platform/config"
	"licoreria/api/internal/platform/database"
	"licoreria/api/internal/users"
)

// Integration tests run against a real PostgreSQL set in TEST_DATABASE_URL and are
// skipped without it. They live in one package so they never truncate tables concurrently.

const adminPassword = "Bodega2026Segura"

type testEnv struct {
	t      *testing.T
	pool   *pgxpool.Pool
	server *httptest.Server
	cfg    config.Config
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	if err := database.MigrateUp(url); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	pool, err := database.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		TRUNCATE users, refresh_sessions, auth_tokens, audit_log, password_history, email_queue RESTART IDENTITY CASCADE;
		DELETE FROM role_permissions rp USING roles r WHERE rp.role_id = r.id AND r.code = 'cashier'
		  AND rp.permission_id IN (SELECT id FROM permissions WHERE module = 'users');`); err != nil {
		t.Fatalf("reset database: %v", err)
	}

	cfg := config.Config{
		Env:            "test",
		AllowedOrigins: []string{"http://localhost:3000"},
		AppBaseURL:     "http://localhost:3000",
		Auth: config.AuthConfig{
			JWTSecret:       []byte("integration-test-secret-0123456789abcdef"),
			AccessTokenTTL:  15 * time.Minute,
			RefreshTokenTTL: time.Hour,
			ResetTokenTTL:   30 * time.Minute,
		},
	}
	logger := slog.New(slog.NewTextHandler(testLogWriter{t}, nil))
	server := httptest.NewServer(NewRouter(Deps{Config: cfg, Logger: logger, DB: pool}))
	t.Cleanup(func() {
		server.Close()
		pool.Close()
	})
	return &testEnv{t: t, pool: pool, server: server, cfg: cfg}
}

// testLogWriter routes server logs to the test output, shown only when a test fails.
type testLogWriter struct{ t *testing.T }

func (w testLogWriter) Write(p []byte) (int, error) {
	w.t.Log(string(bytes.TrimRight(p, "\n")))
	return len(p), nil
}

// client keeps cookies (the refresh token) and the current access token, like the web app.
type client struct {
	env   *testEnv
	http  *http.Client
	token string
}

func (e *testEnv) newClient() *client {
	jar, _ := cookiejar.New(nil)
	return &client{env: e, http: &http.Client{Jar: jar}}
}

type response struct {
	status int
	body   map[string]any
}

func (r response) errorCode() string {
	if e, ok := r.body["error"].(map[string]any); ok {
		code, _ := e["code"].(string)
		return code
	}
	return ""
}

func (r response) fieldErrors() []string {
	e, _ := r.body["error"].(map[string]any)
	details, _ := e["details"].(map[string]any)
	fields, _ := details["fields"].([]any)
	var names []string
	for _, f := range fields {
		names = append(names, f.(map[string]any)["field"].(string))
	}
	return names
}

func (c *client) do(method, path string, body any) response {
	c.env.t.Helper()
	var reader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	}
	req, _ := http.NewRequest(method, c.env.server.URL+"/api/v1"+path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.env.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()

	out := response{status: res.StatusCode, body: map[string]any{}}
	raw, _ := io.ReadAll(res.Body)
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out.body)
	}
	return out
}

func (c *client) login(identifier, password string) response {
	c.env.t.Helper()
	res := c.do(http.MethodPost, "/auth/login", map[string]string{"identifier": identifier, "password": password})
	if token, ok := res.body["accessToken"].(string); ok {
		c.token = token
	}
	return res
}

func (c *client) refresh() response {
	res := c.do(http.MethodPost, "/auth/refresh", nil)
	if token, ok := res.body["accessToken"].(string); ok {
		c.token = token
	}
	return res
}

func expectStatus(t *testing.T, res response, want int, context string) {
	t.Helper()
	if res.status != want {
		t.Fatalf("%s: status %d, want %d (body %v)", context, res.status, want, res.body)
	}
}

// createAdmin bootstraps an administrator and completes the forced password change.
func (e *testEnv) createAdmin() *client {
	e.t.Helper()
	_, temp, err := users.NewService(e.pool, e.cfg.AppBaseURL, time.Now).
		CreateInitialAdmin(context.Background(), "Lucía Barberena", "lucia@elbarril.ni", "lucia")
	if err != nil {
		e.t.Fatalf("create admin: %v", err)
	}
	c := e.newClient()
	expectStatus(e.t, c.login("lucia", temp), http.StatusOK, "admin first login")
	expectStatus(e.t, c.do(http.MethodPost, "/auth/change-password", map[string]string{
		"currentPassword": temp, "newPassword": adminPassword,
	}), http.StatusNoContent, "admin password change")
	return c
}

func (e *testEnv) roleID(code string) string {
	e.t.Helper()
	var id string
	if err := e.pool.QueryRow(context.Background(), `SELECT id FROM roles WHERE code = $1`, code).Scan(&id); err != nil {
		e.t.Fatalf("role %s: %v", code, err)
	}
	return id
}

var (
	tempPasswordPattern = regexp.MustCompile(`Contraseña temporal: <strong[^>]*>([^<]+)</strong>`)
	resetTokenPattern   = regexp.MustCompile(`token=([A-Za-z0-9_-]+)`)
)

// latestEmail returns the newest queued email body sent to the address.
func (e *testEnv) latestEmail(to string) string {
	e.t.Helper()
	var body string
	err := e.pool.QueryRow(context.Background(), `
		SELECT html_body FROM email_queue WHERE to_address = $1 ORDER BY created_at DESC LIMIT 1`, to).Scan(&body)
	if err != nil {
		e.t.Fatalf("no email queued for %s: %v", to, err)
	}
	return body
}

func mustURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return u
}

func extract(t *testing.T, pattern *regexp.Regexp, body string) string {
	t.Helper()
	match := pattern.FindStringSubmatch(body)
	if match == nil {
		t.Fatalf("pattern %s not found in email", pattern)
	}
	return html.UnescapeString(match[1])
}

// createCashier creates a cashier through the API and logs them in with a permanent password.
func (e *testEnv) createCashier(admin *client) *client {
	e.t.Helper()
	res := admin.do(http.MethodPost, "/users", map[string]string{
		"fullName": "Marvin Téllez", "email": "marvin@elbarril.ni", "username": "marvin", "roleId": e.roleID("cashier"),
	})
	expectStatus(e.t, res, http.StatusCreated, "create cashier")

	temp := extract(e.t, tempPasswordPattern, e.latestEmail("marvin@elbarril.ni"))
	cashier := e.newClient()
	expectStatus(e.t, cashier.login("marvin", temp), http.StatusOK, "cashier first login")
	expectStatus(e.t, cashier.do(http.MethodPost, "/auth/change-password", map[string]string{
		"currentPassword": temp, "newPassword": "Caja1Turno2026",
	}), http.StatusNoContent, "cashier password change")
	return cashier
}

func TestFirstLoginForcesPasswordChange(t *testing.T) {
	env := newTestEnv(t)
	_, temp, err := users.NewService(env.pool, env.cfg.AppBaseURL, time.Now).
		CreateInitialAdmin(context.Background(), "Lucía Barberena", "lucia@elbarril.ni", "lucia")
	if err != nil {
		t.Fatal(err)
	}

	c := env.newClient()
	res := c.login("LUCIA@elbarril.ni", temp)
	expectStatus(t, res, http.StatusOK, "login by email, case-insensitive")
	user := res.body["user"].(map[string]any)
	if user["mustChangePassword"] != true {
		t.Fatalf("temporary password must require a change: %v", user)
	}

	res = c.do(http.MethodGet, "/users", nil)
	expectStatus(t, res, http.StatusForbidden, "users before password change")
	if res.errorCode() != "password_change_required" {
		t.Errorf("code = %s, want password_change_required", res.errorCode())
	}
	expectStatus(t, c.do(http.MethodGet, "/auth/me", nil), http.StatusOK, "me is allowed before changing")

	res = c.do(http.MethodPost, "/auth/change-password", map[string]string{"currentPassword": temp, "newPassword": "debil"})
	expectStatus(t, res, http.StatusUnprocessableEntity, "weak password")
	if !slices.Contains(res.fieldErrors(), "newPassword") {
		t.Errorf("weak password must point at newPassword: %v", res.body)
	}

	res = c.do(http.MethodPost, "/auth/change-password", map[string]string{"currentPassword": temp, "newPassword": temp})
	expectStatus(t, res, http.StatusUnprocessableEntity, "reusing the temporary password")

	res = c.do(http.MethodPost, "/auth/change-password", map[string]string{"currentPassword": "Equivocada123", "newPassword": adminPassword})
	expectStatus(t, res, http.StatusUnprocessableEntity, "wrong current password")
	if !slices.Contains(res.fieldErrors(), "currentPassword") {
		t.Errorf("wrong current password must point at currentPassword: %v", res.body)
	}

	expectStatus(t, c.do(http.MethodPost, "/auth/change-password", map[string]string{
		"currentPassword": temp, "newPassword": adminPassword,
	}), http.StatusNoContent, "valid change")
	expectStatus(t, c.do(http.MethodGet, "/users", nil), http.StatusOK, "users after password change")
}

func TestAdminCreatesCashierWithLimitedAccess(t *testing.T) {
	env := newTestEnv(t)
	admin := env.createAdmin()
	cashier := env.createCashier(admin)

	me := cashier.do(http.MethodGet, "/auth/me", nil)
	permissions, _ := me.body["permissions"].([]any)
	if !slices.Contains(permissions, any("pos:create")) || slices.Contains(permissions, any("users:view")) {
		t.Errorf("unexpected cashier permissions: %v", permissions)
	}

	res := cashier.do(http.MethodGet, "/users", nil)
	expectStatus(t, res, http.StatusForbidden, "cashier listing users")
	if res.errorCode() != "forbidden" {
		t.Errorf("code = %s, want forbidden", res.errorCode())
	}

	res = admin.do(http.MethodPost, "/users", map[string]string{
		"fullName": "Otro Marvin", "email": "MARVIN@elbarril.ni", "username": "otro", "roleId": env.roleID("cashier"),
	})
	expectStatus(t, res, http.StatusConflict, "duplicate email")
	if res.errorCode() != "already_exists" || !slices.Contains(res.fieldErrors(), "email") {
		t.Errorf("duplicate email must point at the email field: %v", res.body)
	}

	res = admin.do(http.MethodPost, "/users", map[string]string{"fullName": "X", "email": "no-es-correo", "username": "con espacio", "roleId": ""})
	expectStatus(t, res, http.StatusUnprocessableEntity, "invalid user")

	list := admin.do(http.MethodGet, "/users?search=marvin", nil)
	meta := list.body["meta"].(map[string]any)
	if meta["total"].(float64) != 1 {
		t.Errorf("search should find one user: %v", list.body)
	}
}

func TestRolePermissionChangesApplyImmediately(t *testing.T) {
	env := newTestEnv(t)
	admin := env.createAdmin()
	cashier := env.createCashier(admin)
	expectStatus(t, cashier.do(http.MethodGet, "/users", nil), http.StatusForbidden, "before grant")

	role := admin.do(http.MethodGet, "/roles/"+env.roleID("cashier"), nil)
	permissions := append(role.body["permissions"].([]any), "users:view")
	expectStatus(t, admin.do(http.MethodPut, "/roles/"+env.roleID("cashier")+"/permissions",
		map[string]any{"permissions": permissions}), http.StatusOK, "grant users:view")

	expectStatus(t, cashier.do(http.MethodGet, "/users", nil), http.StatusOK, "same session after grant")

	res := admin.do(http.MethodPut, "/roles/"+env.roleID("admin")+"/permissions", map[string]any{"permissions": []string{}})
	expectStatus(t, res, http.StatusConflict, "admin role is locked")

	res = admin.do(http.MethodPut, "/roles/"+env.roleID("cashier")+"/permissions", map[string]any{"permissions": []string{"inventado:ver"}})
	expectStatus(t, res, http.StatusUnprocessableEntity, "unknown permission")
}

func TestRefreshRotatesAndDetectsReuse(t *testing.T) {
	env := newTestEnv(t)
	admin := env.createAdmin()

	stolen := admin.http.Jar.Cookies(mustURL(env.server.URL + "/api/v1/auth/"))
	if len(stolen) == 0 {
		t.Fatal("login must set the refresh cookie")
	}

	expectStatus(t, admin.refresh(), http.StatusOK, "first refresh")
	expectStatus(t, admin.do(http.MethodGet, "/auth/me", nil), http.StatusOK, "new access token works")

	attacker := env.newClient()
	attacker.http.Jar.SetCookies(mustURL(env.server.URL+"/api/v1/auth/"), stolen)
	res := attacker.refresh()
	expectStatus(t, res, http.StatusUnauthorized, "reused refresh token")

	res = admin.do(http.MethodGet, "/auth/me", nil)
	expectStatus(t, res, http.StatusUnauthorized, "reuse revokes every session")
	if res.errorCode() != "session_expired" {
		t.Errorf("code = %s, want session_expired", res.errorCode())
	}
}

func TestLogoutEndsSessionImmediately(t *testing.T) {
	env := newTestEnv(t)
	admin := env.createAdmin()

	expectStatus(t, admin.do(http.MethodPost, "/auth/logout", nil), http.StatusNoContent, "logout")
	expectStatus(t, admin.do(http.MethodGet, "/auth/me", nil), http.StatusUnauthorized, "access token after logout")
	expectStatus(t, admin.refresh(), http.StatusUnauthorized, "refresh after logout")
}

func TestLockoutAfterFailedAttempts(t *testing.T) {
	env := newTestEnv(t)
	env.createAdmin()
	c := env.newClient()

	for attempt := 1; attempt <= 4; attempt++ {
		res := c.login("lucia", "Incorrecta2026")
		expectStatus(t, res, http.StatusUnauthorized, "wrong password")
		if res.errorCode() != "invalid_credentials" {
			t.Fatalf("attempt %d: code = %s", attempt, res.errorCode())
		}
	}
	res := c.login("lucia", "Incorrecta2026")
	expectStatus(t, res, http.StatusTooManyRequests, "fifth failure locks the account")
	if res.errorCode() != "account_locked" {
		t.Errorf("code = %s, want account_locked", res.errorCode())
	}
	expectStatus(t, c.login("lucia", adminPassword), http.StatusTooManyRequests, "correct password while locked")

	res = c.login("nadie", "Incorrecta2026")
	if res.errorCode() != "invalid_credentials" {
		t.Errorf("unknown users must get the same generic error, got %s", res.errorCode())
	}
}

func TestPasswordResetFlow(t *testing.T) {
	env := newTestEnv(t)
	admin := env.createAdmin()
	anon := env.newClient()

	expectStatus(t, anon.do(http.MethodPost, "/auth/forgot-password", map[string]string{"email": "nadie@elbarril.ni"}),
		http.StatusAccepted, "unknown email gets the same answer")
	expectStatus(t, anon.do(http.MethodPost, "/auth/forgot-password", map[string]string{"email": "lucia@elbarril.ni"}),
		http.StatusAccepted, "known email")

	token := extract(t, resetTokenPattern, env.latestEmail("lucia@elbarril.ni"))

	res := anon.do(http.MethodPost, "/auth/reset-password", map[string]string{"token": token, "newPassword": "debil"})
	expectStatus(t, res, http.StatusUnprocessableEntity, "weak new password")

	expectStatus(t, anon.do(http.MethodPost, "/auth/reset-password", map[string]string{
		"token": token, "newPassword": "NuevaClave2026",
	}), http.StatusNoContent, "reset")

	res = anon.do(http.MethodPost, "/auth/reset-password", map[string]string{"token": token, "newPassword": "OtraClave2026"})
	expectStatus(t, res, http.StatusBadRequest, "token is single use")
	if res.errorCode() != "invalid_reset_token" {
		t.Errorf("code = %s, want invalid_reset_token", res.errorCode())
	}

	expectStatus(t, admin.do(http.MethodGet, "/auth/me", nil), http.StatusUnauthorized, "reset signs out other sessions")
	expectStatus(t, env.newClient().login("lucia", "NuevaClave2026"), http.StatusOK, "login with the new password")
}

func TestDeactivationAndAdminSafeguards(t *testing.T) {
	env := newTestEnv(t)
	admin := env.createAdmin()
	cashier := env.createCashier(admin)

	var adminID, cashierID string
	_ = env.pool.QueryRow(context.Background(), `SELECT id FROM users WHERE username = 'lucia'`).Scan(&adminID)
	_ = env.pool.QueryRow(context.Background(), `SELECT id FROM users WHERE username = 'marvin'`).Scan(&cashierID)

	res := admin.do(http.MethodPatch, "/users/"+adminID+"/status", map[string]bool{"active": false})
	expectStatus(t, res, http.StatusConflict, "self deactivation")

	res = admin.do(http.MethodPut, "/users/"+adminID, map[string]string{
		"fullName": "Lucía Barberena", "email": "lucia@elbarril.ni", "username": "lucia", "roleId": env.roleID("viewer"),
	})
	expectStatus(t, res, http.StatusConflict, "self role change")

	expectStatus(t, admin.do(http.MethodPatch, "/users/"+cashierID+"/status", map[string]bool{"active": false}),
		http.StatusOK, "deactivate cashier")
	expectStatus(t, cashier.do(http.MethodGet, "/auth/me", nil), http.StatusUnauthorized, "deactivated user loses access at once")

	res = admin.do(http.MethodPost, "/users/"+cashierID+"/resend-access", nil)
	expectStatus(t, res, http.StatusConflict, "resend access to an inactive user")

	expectStatus(t, admin.do(http.MethodGet, "/users/not-a-uuid", nil), http.StatusNotFound, "malformed id")
}

type fakeSender struct{ sent []mail.Message }

func (f *fakeSender) Send(_ context.Context, msg mail.Message) error {
	f.sent = append(f.sent, msg)
	return nil
}

func TestMailWorkerDeliversAndClearsBody(t *testing.T) {
	env := newTestEnv(t)
	admin := env.createAdmin()
	env.createCashier(admin)

	sender := &fakeSender{}
	worker := mail.NewWorker(env.pool, sender, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := worker.ProcessBatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sender.sent) != 1 || sender.sent[0].To != "marvin@elbarril.ni" || sender.sent[0].Subject != "Tu acceso a Licorería" {
		t.Fatalf("unexpected deliveries: %+v", sender.sent)
	}

	var status, body string
	_ = env.pool.QueryRow(context.Background(), `SELECT status, html_body FROM email_queue LIMIT 1`).Scan(&status, &body)
	if status != "sent" || body != "" {
		t.Errorf("sent email must be marked and its body cleared, got %s / %d bytes", status, len(body))
	}
}
