package server

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"licoreria/api/internal/platform/database"
)

// A restore brings the data back, but neither the audit trail nor the backup history rewind,
// and every session has to sign in again.
func TestBackupRestoreRoundTrip(t *testing.T) {
	requirePostgresTools(t)
	env := newTestEnv(t)
	admin := env.createAdmin()
	cashier := env.createCashier(admin)
	ctx := context.Background()

	expectStatus(t, cashier.do(http.MethodPost, "/backups", nil), http.StatusForbidden, "cashier creates backup")
	created := admin.do(http.MethodPost, "/backups", nil)
	expectStatus(t, created, http.StatusCreated, "create backup")
	id := created.body["id"].(string)

	verified := admin.do(http.MethodPost, "/backups/"+id+"/verify", nil)
	expectStatus(t, verified, http.StatusOK, "verify backup")
	if result := verified.body["verification"].(map[string]any); result["ok"] != true {
		t.Fatalf("verification = %v", result)
	}

	expectStatus(t, admin.do(http.MethodPost, "/exchange-rates", map[string]string{"rate": "36.6243"}), http.StatusOK, "change after backup")

	restore := "/backups/" + id + "/restore"
	good := map[string]string{"currentPassword": adminPassword, "confirmation": "RESTAURAR"}
	expectStatus(t, cashier.do(http.MethodPost, restore, good), http.StatusForbidden, "cashier restores")
	expectStatus(t, admin.do(http.MethodPost, restore, map[string]string{"currentPassword": "wrong", "confirmation": "RESTAURAR"}),
		http.StatusUnprocessableEntity, "wrong password")
	expectStatus(t, admin.do(http.MethodPost, restore, map[string]string{"currentPassword": adminPassword, "confirmation": "si"}),
		http.StatusUnprocessableEntity, "wrong phrase")
	expectStatus(t, admin.do(http.MethodPost, restore, good), http.StatusOK, "restore")

	expectStatus(t, admin.do(http.MethodGet, "/exchange-rates/today", nil), http.StatusUnauthorized, "session after restore")
	expectStatus(t, admin.login("lucia", adminPassword), http.StatusOK, "sign in again")
	expectStatus(t, admin.do(http.MethodGet, "/exchange-rates/today", nil), http.StatusNotFound, "data is back to the backup")

	var survived, restored, denied int
	if err := env.pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE action = 'exchange_rates.set'),
		       count(*) FILTER (WHERE action = 'backups.restored'),
		       count(*) FILTER (WHERE action = 'backups.restore_denied')
		FROM audit_log`).Scan(&survived, &restored, &denied); err != nil {
		t.Fatal(err)
	}
	if survived != 1 || restored != 1 || denied != 1 {
		t.Errorf("audit after restore: change=%d restored=%d denied=%d", survived, restored, denied)
	}
	if list := admin.do(http.MethodGet, "/backups", nil); len(list.body["data"].([]any)) != 2 {
		t.Errorf("backup history = %v", list.body["data"])
	}

	if _, err := env.pool.Exec(ctx, `DELETE FROM audit_log`); err == nil {
		t.Error("audit log accepted a delete")
	}
}

// A damaged archive never touches the data, and one from an older schema is migrated forward.
func TestRestoreBrokenArchiveAndOlderSchema(t *testing.T) {
	requirePostgresTools(t)
	env := newTestEnv(t)
	admin := env.createAdmin()
	ctx := context.Background()
	dir := env.cfg.Storage.BackupDir

	migrator, err := database.Migrator(env.cfg.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer migrator.Close()
	var latest int
	_ = env.pool.QueryRow(ctx, `SELECT version FROM schema_migrations`).Scan(&latest)
	if err := migrator.Steps(-1); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(dir, "old.dump")
	dump := exec.Command("pg_dump", "--format=custom", "--no-owner", "--no-privileges", "--exclude-table-data=public.backups", "--file="+old, "--dbname="+env.cfg.DatabaseURL)
	if out, err := dump.CombinedOutput(); err != nil {
		t.Fatalf("pg_dump: %v: %s", err, out)
	}
	if err := migrator.Up(); err != nil {
		t.Fatal(err)
	}

	register := func(name string) string {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		var id string
		if err := env.pool.QueryRow(ctx, `
			INSERT INTO backups (file_name, size_bytes, checksum_sha256, kind, status)
			VALUES ($1, $2, $3, 'manual', 'completed') RETURNING id`, name, len(data), fmt.Sprintf("%x", sha256.Sum256(data))).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	restore := func(id string) response {
		return admin.do(http.MethodPost, "/backups/"+id+"/restore", map[string]string{"currentPassword": adminPassword, "confirmation": "RESTAURAR"})
	}

	content, _ := os.ReadFile(old)
	if err := os.WriteFile(filepath.Join(dir, "broken.dump"), content[:len(content)/2], 0o600); err != nil {
		t.Fatal(err)
	}
	if res := restore(register("broken.dump")); res.errorCode() != "restore_failed" {
		t.Fatalf("truncated archive: %d %s", res.status, res.errorCode())
	}
	expectStatus(t, admin.do(http.MethodGet, "/backups", nil), http.StatusOK, "data intact and API open after a failed restore")

	id := register("old.dump")
	if err := os.WriteFile(filepath.Join(dir, "old.dump"), append(content, 0), 0o600); err != nil {
		t.Fatal(err)
	}
	if res := restore(id); res.errorCode() != "backup_corrupted" {
		t.Fatalf("altered archive: %d %s", res.status, res.errorCode())
	}
	if err := os.WriteFile(filepath.Join(dir, "old.dump"), content, 0o600); err != nil {
		t.Fatal(err)
	}

	expectStatus(t, restore(id), http.StatusOK, "restore older schema")
	var version int
	if err := env.pool.QueryRow(ctx, `SELECT version FROM schema_migrations`).Scan(&version); err != nil || version != latest {
		t.Errorf("schema after restore = %d (%v), want %d", version, err, latest)
	}
}

func requirePostgresTools(t *testing.T) {
	t.Helper()
	for _, tool := range []string{"pg_dump", "pg_restore", "psql"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not installed", tool)
		}
	}
}
