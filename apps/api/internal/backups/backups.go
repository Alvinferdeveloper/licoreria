// Package backups creates, verifies and restores pg_dump archives. Files live in a server
// directory and, when a private bucket is configured, are copied there too.
package backups

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/audit"
	"licoreria/api/internal/httpx"
	"licoreria/api/internal/platform/pdfx"
	"licoreria/api/internal/platform/storage"
)

const (
	KindManual     = "manual"
	KindScheduled  = "scheduled"
	KindPreRestore = "pre_restore"

	statusRunning   = "running"
	statusCompleted = "completed"
	statusFailed    = "failed"

	operationTimeout = 30 * time.Minute
)

var (
	ErrBusy        = apperr.New(apperr.KindConflict, "backup_busy", "Ya hay un respaldo o una restauración en curso. Espera a que termine.")
	ErrNotReady    = apperr.New(apperr.KindConflict, "backup_not_ready", "Este respaldo no terminó correctamente y no se puede usar.")
	ErrFileMissing = apperr.New(apperr.KindConflict, "backup_file_missing", "El archivo de este respaldo ya no está disponible.")
	ErrCorrupted   = apperr.New(apperr.KindConflict, "backup_corrupted", "El archivo no coincide con su suma de verificación. No se usó.")
	ErrFailed      = apperr.New(apperr.KindUnavailable, "backup_failed", "No se pudo crear el respaldo. Intenta de nuevo.")
)

type Person struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type Backup struct {
	ID           uuid.UUID       `json:"id"`
	FileName     string          `json:"fileName"`
	SizeBytes    *int64          `json:"sizeBytes"`
	Checksum     *string         `json:"checksum"`
	Kind         string          `json:"kind"`
	Status       string          `json:"status"`
	CreatedBy    *Person         `json:"createdBy"`
	CreatedAt    time.Time       `json:"createdAt"`
	CompletedAt  *time.Time      `json:"completedAt"`
	Offsite      bool            `json:"offsite"`
	Available    bool            `json:"available"`
	VerifiedAt   *time.Time      `json:"verifiedAt"`
	Verification json.RawMessage `json:"verification"`

	remoteKey *string
}

type Service struct {
	pool   *pgxpool.Pool
	dsn    string
	dir    string
	remote *storage.Supabase
	gate   *Gate
	logger *slog.Logger
	now    func() time.Time
	// mu allows one dump, verification or restore at a time.
	mu sync.Mutex
}

type Options struct {
	DatabaseURL string
	Dir         string
	Remote      *storage.Supabase
	Logger      *slog.Logger
}

func NewService(pool *pgxpool.Pool, opts Options) *Service {
	return &Service{
		pool: pool, dsn: opts.DatabaseURL, dir: opts.Dir, remote: opts.Remote,
		gate: &Gate{}, logger: opts.Logger.With("component", "backups"), now: time.Now,
	}
}

// Gate returns the maintenance gate the router must place in front of the API.
func (s *Service) Gate() *Gate { return s.gate }

const selectBackup = `
	SELECT b.id, b.file_name, b.size_bytes, b.checksum_sha256, b.kind, b.status, b.created_at, b.completed_at,
	       b.remote_key, b.verified_at, b.verification, u.id, u.full_name
	FROM backups b LEFT JOIN users u ON u.id = b.created_by`

func (s *Service) scan(row pgx.Row) (Backup, error) {
	var b Backup
	var userID *uuid.UUID
	var userName *string
	if err := row.Scan(&b.ID, &b.FileName, &b.SizeBytes, &b.Checksum, &b.Kind, &b.Status, &b.CreatedAt, &b.CompletedAt,
		&b.remoteKey, &b.VerifiedAt, &b.Verification, &userID, &userName); err != nil {
		return Backup{}, err
	}
	if userID != nil && userName != nil {
		b.CreatedBy = &Person{ID: *userID, Name: *userName}
	}
	b.Offsite = b.remoteKey != nil
	_, statErr := os.Stat(s.path(b.FileName))
	b.Available = b.Status == statusCompleted && (statErr == nil || (b.Offsite && s.remoteEnabled()))
	return b, nil
}

func (s *Service) List(ctx context.Context, page httpx.Page) ([]Backup, int64, error) {
	var total int64
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM backups`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count backups: %w", err)
	}
	rows, err := s.pool.Query(ctx, selectBackup+` ORDER BY b.created_at DESC LIMIT $1 OFFSET $2`, page.Size, page.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("list backups: %w", err)
	}
	defer rows.Close()
	items := []Backup{}
	for rows.Next() {
		b, err := s.scan(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan backup: %w", err)
		}
		items = append(items, b)
	}
	return items, total, rows.Err()
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (Backup, error) {
	b, err := s.scan(s.pool.QueryRow(ctx, selectBackup+` WHERE b.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Backup{}, apperr.ErrNotFound
	}
	if err != nil {
		return Backup{}, fmt.Errorf("load backup: %w", err)
	}
	return b, nil
}

// Create dumps the database. It keeps running if the client disconnects.
func (s *Service) Create(ctx context.Context, actor uuid.UUID) (Backup, error) {
	if !s.mu.TryLock() {
		return Backup{}, ErrBusy
	}
	defer s.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), operationTimeout)
	defer cancel()
	return s.create(ctx, KindManual, &actor)
}

// create needs s.mu held.
func (s *Service) create(ctx context.Context, kind string, actor *uuid.UUID) (Backup, error) {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return Backup{}, ErrFailed.WithCause(err)
	}
	id := uuid.New()
	name := fmt.Sprintf("anejo-%s-%s-%s.dump", s.now().In(pdfx.Store).Format("20060102-150405"), kind, id.String()[:8])
	if _, err := s.pool.Exec(ctx, `INSERT INTO backups (id, file_name, kind, created_by) VALUES ($1, $2, $3, $4)`, id, name, kind, actor); err != nil {
		return Backup{}, fmt.Errorf("insert backup: %w", err)
	}

	size, checksum, err := s.dumpTo(ctx, s.path(name))
	if err != nil {
		s.logger.ErrorContext(ctx, "backup failed", "backup_id", id, "kind", kind, "error", err)
		if _, dbErr := s.pool.Exec(ctx, `UPDATE backups SET status = $2, error = $3, completed_at = now() WHERE id = $1`, id, statusFailed, err.Error()); dbErr != nil {
			s.logger.ErrorContext(ctx, "mark backup failed", "backup_id", id, "error", dbErr)
		}
		return Backup{}, ErrFailed.WithCause(err)
	}

	remoteKey := s.upload(ctx, name)
	if _, err := s.pool.Exec(ctx, `
		UPDATE backups SET status = $2, size_bytes = $3, checksum_sha256 = $4, remote_key = $5, completed_at = now() WHERE id = $1`,
		id, statusCompleted, size, checksum, remoteKey); err != nil {
		return Backup{}, fmt.Errorf("complete backup: %w", err)
	}
	if err := audit.Record(ctx, s.pool, audit.Entry{UserID: actor, Action: "backups.created", Entity: "backups", EntityID: &id,
		After: map[string]any{"file": name, "kind": kind, "sizeBytes": size}}); err != nil {
		return Backup{}, err
	}
	s.logger.InfoContext(ctx, "backup completed", "backup_id", id, "kind", kind, "size_bytes", size, "offsite", remoteKey != nil)
	return s.Get(ctx, id)
}

func (s *Service) dumpTo(ctx context.Context, target string) (int64, string, error) {
	partial := target + ".partial"
	defer os.Remove(partial)
	if err := s.pgDump(ctx, partial); err != nil {
		return 0, "", err
	}
	if err := os.Chmod(partial, 0o600); err != nil {
		return 0, "", err
	}
	size, checksum, err := fileDigest(partial)
	if err != nil {
		return 0, "", err
	}
	return size, checksum, os.Rename(partial, target)
}

// upload copies the file to the private bucket. A failure leaves the local copy usable.
func (s *Service) upload(ctx context.Context, name string) *string {
	if !s.remoteEnabled() {
		return nil
	}
	data, err := os.ReadFile(s.path(name))
	if err == nil {
		_, err = s.remote.Put(ctx, name, "application/octet-stream", data)
	}
	if err != nil {
		s.logger.WarnContext(ctx, "offsite copy failed", "file", name, "error", err)
		return nil
	}
	return &name
}

// Open returns the verified archive for download.
func (s *Service) Open(ctx context.Context, id uuid.UUID, actor uuid.UUID) (*os.File, Backup, error) {
	b, err := s.Get(ctx, id)
	if err != nil {
		return nil, Backup{}, err
	}
	path, err := s.localCopy(ctx, b)
	if err != nil {
		return nil, Backup{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, Backup{}, fmt.Errorf("open backup: %w", err)
	}
	if err := audit.Record(ctx, s.pool, audit.Entry{UserID: &actor, Action: "backups.downloaded", Entity: "backups", EntityID: &id,
		After: map[string]string{"file": b.FileName}}); err != nil {
		file.Close()
		return nil, Backup{}, err
	}
	return file, b, nil
}

// localCopy returns a path whose content matches the recorded checksum, fetching it from
// the bucket when the local file is gone.
func (s *Service) localCopy(ctx context.Context, b Backup) (string, error) {
	if b.Status != statusCompleted || b.Checksum == nil {
		return "", ErrNotReady
	}
	path := s.path(b.FileName)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := s.fetch(ctx, b, path); err != nil {
			return "", err
		}
	}
	_, checksum, err := fileDigest(path)
	if err != nil {
		return "", fmt.Errorf("hash backup: %w", err)
	}
	if checksum != *b.Checksum {
		return "", ErrCorrupted
	}
	return path, nil
}

func (s *Service) fetch(ctx context.Context, b Backup, path string) error {
	if b.remoteKey == nil || !s.remoteEnabled() {
		return ErrFileMissing
	}
	body, err := s.remote.Get(ctx, *b.remoteKey)
	if err != nil {
		return ErrFileMissing.WithCause(err)
	}
	defer body.Close()
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := io.Copy(file, body); err != nil {
		os.Remove(path)
		return ErrFileMissing.WithCause(err)
	}
	return nil
}

func (s *Service) remoteEnabled() bool { return s.remote != nil && s.remote.Enabled() }

// Offsite reports whether new backups are copied to the private bucket.
func (s *Service) Offsite() bool { return s.remoteEnabled() }

func (s *Service) path(name string) string { return filepath.Join(s.dir, filepath.Base(name)) }

func fileDigest(path string) (int64, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return 0, "", err
	}
	return size, hex.EncodeToString(hash.Sum(nil)), nil
}
