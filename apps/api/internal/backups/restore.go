package backups

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/audit"
	"licoreria/api/internal/auth"
	"licoreria/api/internal/platform/database"
)

const (
	maxRestoreDenials = 5
	drainTimeout      = 10 * time.Second
)

var (
	ErrRestoreFailed = apperr.New(apperr.KindUnavailable, "restore_failed", "No se pudo restaurar el respaldo. Los datos quedaron como estaban.")
	ErrRestoreLocked = apperr.New(apperr.KindTooManyRequests, "restore_locked", "Demasiados intentos. Espera 15 minutos.")
)

type RestoreResult struct {
	Restored   Backup `json:"restored"`
	SafetyCopy Backup `json:"safetyCopy"`
}

// history holds what must survive a restore: the backup list and the audit entries newer
// than the archive, so restoring can never hide what happened after it.
type history struct {
	backups string
	audit   string
}

// Restore replaces the database with a backup. It confirms the password, closes the API,
// saves the current state and only then touches the data.
func (s *Service) Restore(ctx context.Context, actor uuid.UUID, id uuid.UUID, password string) (RestoreResult, error) {
	if err := s.confirm(ctx, actor, id, password); err != nil {
		return RestoreResult{}, err
	}
	if !s.mu.TryLock() {
		return RestoreResult{}, ErrBusy
	}
	defer s.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), operationTimeout)
	defer cancel()

	target, err := s.Get(ctx, id)
	if err != nil {
		return RestoreResult{}, err
	}
	file, err := s.localCopy(ctx, target)
	if err != nil {
		return RestoreResult{}, err
	}

	s.gate.Close(ctx, drainTimeout)
	defer s.gate.Open()

	safety, err := s.create(ctx, KindPreRestore, &actor)
	if err != nil {
		return RestoreResult{}, ErrRestoreFailed.WithCause(err)
	}
	saved, err := s.snapshot(ctx, target.CreatedAt)
	if err != nil {
		return RestoreResult{}, ErrRestoreFailed.WithCause(err)
	}

	if err := s.load(ctx, file, "", resetSchema); err != nil {
		s.recordFailure(ctx, actor, id, err)
		return RestoreResult{}, ErrRestoreFailed.WithCause(err)
	}
	s.pool.Reset()

	if err := database.MigrateUp(s.dsn); err != nil {
		s.rollBack(ctx, safety, saved)
		s.recordFailure(ctx, actor, id, err)
		return RestoreResult{}, ErrRestoreFailed.WithCause(err)
	}
	if err := s.reinstate(ctx, saved, &actor, target, safety); err != nil {
		return RestoreResult{}, fmt.Errorf("reinstate history: %w", err)
	}
	s.logger.InfoContext(ctx, "backup restored", "backup_id", id, "safety_copy_id", safety.ID)

	if target, err = s.Get(ctx, id); err != nil {
		return RestoreResult{}, err
	}
	if safety, err = s.Get(ctx, safety.ID); err != nil {
		return RestoreResult{}, err
	}
	return RestoreResult{Restored: target, SafetyCopy: safety}, nil
}

func (s *Service) confirm(ctx context.Context, actor, id uuid.UUID, password string) error {
	var denials int
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM audit_log WHERE action = 'backups.restore_denied' AND user_id = $1
		AND created_at > now() - interval '15 minutes'`, actor).Scan(&denials); err != nil {
		return fmt.Errorf("count restore denials: %w", err)
	}
	if denials >= maxRestoreDenials {
		return ErrRestoreLocked
	}
	err := auth.ConfirmPassword(ctx, s.pool, actor, password)
	if errors.Is(err, auth.ErrWrongPassword) {
		if auditErr := audit.Record(context.WithoutCancel(ctx), s.pool, audit.Entry{UserID: &actor, Action: "backups.restore_denied", Entity: "backups", EntityID: &id}); auditErr != nil {
			return auditErr
		}
	}
	return err
}

func (s *Service) snapshot(ctx context.Context, since time.Time) (history, error) {
	var h history
	err := s.pool.QueryRow(ctx, `
		SELECT (SELECT COALESCE(json_agg(b), '[]')::text FROM backups b),
		       (SELECT COALESCE(json_agg(a), '[]')::text FROM audit_log a WHERE a.created_at >= $1)`, since).Scan(&h.backups, &h.audit)
	return h, err
}

// reinstate runs on the restored data: it returns the saved history and drops what would
// otherwise come back to life, like revoked sessions, used tokens and already-sent emails.
// Sessions are deleted, not revoked: a browser refreshing with its old cookie is not token theft.
func (s *Service) reinstate(ctx context.Context, saved history, actor *uuid.UUID, target, safety Backup) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := putHistory(ctx, tx, saved); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM refresh_sessions;
			UPDATE auth_tokens SET used_at = now() WHERE used_at IS NULL;
			UPDATE email_queue SET status = 'failed', last_error = 'cancelled by restore' WHERE status = 'pending'`); err != nil {
			return fmt.Errorf("invalidate restored state: %w", err)
		}
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE id = $1)`, actor).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			actor = nil
		}
		return audit.Record(ctx, tx, audit.Entry{UserID: actor, Action: "backups.restored", Entity: "backups", EntityID: &target.ID,
			After: map[string]any{"file": target.FileName, "backupDate": target.CreatedAt, "safetyCopy": safety.FileName}})
	})
}

func putHistory(ctx context.Context, tx pgx.Tx, saved history) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO backups (id, file_name, size_bytes, checksum_sha256, kind, status, error, created_by, created_at,
		                     completed_at, remote_key, verified_at, verification)
		SELECT r.id, r.file_name, r.size_bytes, r.checksum_sha256, r.kind, r.status, r.error, u.id, r.created_at,
		       r.completed_at, r.remote_key, r.verified_at, r.verification
		FROM json_populate_recordset(NULL::backups, $1::json) r
		LEFT JOIN users u ON u.id = r.created_by
		ON CONFLICT (id) DO NOTHING`, saved.backups); err != nil {
		return fmt.Errorf("restore backup history: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO audit_log (id, user_id, action, entity, entity_id, before_data, after_data, ip, user_agent, created_at)
		SELECT r.id, u.id, r.action, r.entity, r.entity_id, r.before_data, r.after_data, r.ip, r.user_agent, r.created_at
		FROM json_populate_recordset(NULL::audit_log, $1::json) r
		LEFT JOIN users u ON u.id = r.user_id
		ON CONFLICT (id) DO NOTHING`, saved.audit); err != nil {
		return fmt.Errorf("restore audit history: %w", err)
	}
	return nil
}

// rollBack returns to the safety copy when the restored schema can't be brought up to date.
func (s *Service) rollBack(ctx context.Context, safety Backup, saved history) {
	err := s.load(ctx, s.path(safety.FileName), "", resetSchema)
	s.pool.Reset()
	if err == nil {
		err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error { return putHistory(ctx, tx, saved) })
	}
	if err != nil {
		s.logger.ErrorContext(ctx, "rollback to safety copy failed", "file", safety.FileName, "error", err)
	}
}

func (s *Service) recordFailure(ctx context.Context, actor, id uuid.UUID, cause error) {
	s.logger.ErrorContext(ctx, "restore failed", "backup_id", id, "error", cause)
	if err := audit.Record(ctx, s.pool, audit.Entry{UserID: &actor, Action: "backups.restore_failed", Entity: "backups", EntityID: &id}); err != nil {
		s.logger.ErrorContext(ctx, "record restore failure", "error", err)
	}
}
