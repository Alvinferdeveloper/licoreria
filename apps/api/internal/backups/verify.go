package backups

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/audit"
)

// keyTables are compared after loading an archive; their names are fixed, never user input.
var keyTables = []string{"users", "products", "customers", "sales", "inventory_movements", "audit_log"}

type TableCount struct {
	Table string `json:"table"`
	Rows  int64  `json:"rows"`
}

type Verification struct {
	OK            bool         `json:"ok"`
	SchemaVersion int64        `json:"schemaVersion"`
	Tables        []TableCount `json:"tables"`
}

// Verify is the restore drill: it loads the archive into a throwaway database and counts
// key tables, without touching live data.
func (s *Service) Verify(ctx context.Context, id uuid.UUID, actor *uuid.UUID) (Backup, error) {
	if !s.mu.TryLock() {
		return Backup{}, ErrBusy
	}
	defer s.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), operationTimeout)
	defer cancel()

	b, err := s.Get(ctx, id)
	if err != nil {
		return Backup{}, err
	}
	file, err := s.localCopy(ctx, b)
	if err != nil {
		return Backup{}, err
	}

	result, drillErr := s.drill(ctx, file)
	if drillErr != nil {
		s.logger.ErrorContext(ctx, "backup verification failed", "backup_id", id, "error", drillErr)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return Backup{}, err
	}
	if _, err := s.pool.Exec(ctx, `UPDATE backups SET verified_at = now(), verification = $2 WHERE id = $1`, id, encoded); err != nil {
		return Backup{}, fmt.Errorf("save verification: %w", err)
	}
	if err := audit.Record(ctx, s.pool, audit.Entry{UserID: actor, Action: "backups.verified", Entity: "backups", EntityID: &id, After: result}); err != nil {
		return Backup{}, err
	}
	return s.Get(ctx, id)
}

// VerifyLatest drills the newest completed backup, for use from the command line.
func (s *Service) VerifyLatest(ctx context.Context) (Backup, Verification, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT id FROM backups WHERE status = $1 ORDER BY created_at DESC LIMIT 1`, statusCompleted).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Backup{}, Verification{}, apperr.ErrNotFound.WithMessage("Todavía no hay respaldos.")
	}
	if err != nil {
		return Backup{}, Verification{}, fmt.Errorf("find latest backup: %w", err)
	}
	b, err := s.Verify(ctx, id, nil)
	if err != nil {
		return Backup{}, Verification{}, err
	}
	var result Verification
	return b, result, json.Unmarshal(b.Verification, &result)
}

func (s *Service) drill(ctx context.Context, file string) (Verification, error) {
	name := "anejo_verify_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := s.pool.Exec(ctx, `CREATE DATABASE `+name); err != nil {
		return Verification{}, fmt.Errorf("create drill database: %w", err)
	}
	defer func() {
		if _, err := s.pool.Exec(context.WithoutCancel(ctx), `DROP DATABASE IF EXISTS `+name+` WITH (FORCE)`); err != nil {
			s.logger.ErrorContext(ctx, "drop drill database", "database", name, "error", err)
		}
	}()

	if err := s.load(ctx, file, name, ""); err != nil {
		return Verification{}, err
	}
	dsn, err := s.dsnFor(name)
	if err != nil {
		return Verification{}, err
	}
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return Verification{}, fmt.Errorf("connect drill database: %w", err)
	}
	defer conn.Close(context.WithoutCancel(ctx))

	result := Verification{Tables: make([]TableCount, 0, len(keyTables))}
	if err := conn.QueryRow(ctx, `SELECT version FROM schema_migrations`).Scan(&result.SchemaVersion); err != nil {
		return result, fmt.Errorf("read schema version: %w", err)
	}
	for _, table := range keyTables {
		count := TableCount{Table: table}
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM `+table).Scan(&count.Rows); err != nil {
			return result, fmt.Errorf("count %s: %w", table, err)
		}
		result.Tables = append(result.Tables, count)
	}
	result.OK = result.Tables[0].Rows > 0
	return result, nil
}

func (s *Service) dsnFor(database string) (string, error) {
	u, err := url.Parse(s.dsn)
	if err != nil {
		return "", errors.New("invalid database url")
	}
	u.Path = "/" + database
	return u.String(), nil
}
