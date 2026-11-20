package backups

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/google/uuid"

	"licoreria/api/internal/mail"
	"licoreria/api/internal/platform/pdfx"
	"licoreria/api/internal/settings"
)

const (
	scheduleHour  = 2
	checkInterval = 15 * time.Minute
	retryAfter    = time.Hour
)

// RunScheduler takes the daily backup and applies retention until ctx ends. Checking on an
// interval, instead of waiting for the exact hour, also covers a server that was off at 02:00.
func (s *Service) RunScheduler(ctx context.Context) {
	if _, err := s.pool.Exec(ctx, `UPDATE backups SET status = $1, error = 'interrupted', completed_at = now() WHERE status = $2`, statusFailed, statusRunning); err != nil {
		s.logger.ErrorContext(ctx, "close interrupted backups", "error", err)
	}
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()
	for {
		if err := s.runDue(ctx); err != nil && ctx.Err() == nil {
			s.logger.ErrorContext(ctx, "scheduled backup", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) runDue(ctx context.Context) error {
	if !s.mu.TryLock() {
		return nil
	}
	defer s.mu.Unlock()

	var lastDone, lastTry *time.Time
	if err := s.pool.QueryRow(ctx, `
		SELECT max(created_at) FILTER (WHERE status = $2), max(created_at)
		FROM backups WHERE kind = $1`, KindScheduled, statusCompleted).Scan(&lastDone, &lastTry); err != nil {
		return fmt.Errorf("load last scheduled backup: %w", err)
	}
	if !isDue(s.now(), lastDone, lastTry) {
		return nil
	}
	if _, err := s.create(ctx, KindScheduled, nil); err != nil {
		// Retries run every hour; only the first failure since the last success alerts.
		if lastTry == nil || (lastDone != nil && !lastTry.After(*lastDone)) {
			s.alertFailure(ctx)
		}
		return err
	}
	return s.prune(ctx)
}

func (s *Service) alertFailure(ctx context.Context) {
	err := mail.AlertAdmins(ctx, s.pool, mail.Alert{
		Subject:     "Falló el respaldo automático",
		Body:        "El respaldo automático de la base de datos no se pudo crear. El sistema lo reintentará cada hora. Revisa el espacio en disco del servidor o crea un respaldo manual.",
		ActionURL:   s.appURL + "/respaldos",
		ActionLabel: "Ver respaldos",
		When:        s.now(),
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "queue backup failure alert", "error", err)
	}
}

// isDue is true once today's slot has passed without a completed backup, spacing out retries.
func isDue(now time.Time, lastDone, lastTry *time.Time) bool {
	local := now.In(pdfx.Store)
	slot := time.Date(local.Year(), local.Month(), local.Day(), scheduleHour, 0, 0, 0, pdfx.Store)
	if local.Before(slot) {
		slot = slot.AddDate(0, 0, -1)
	}
	if lastDone != nil && !lastDone.Before(slot) {
		return false
	}
	return lastTry == nil || now.Sub(*lastTry) >= retryAfter
}

type retained struct {
	id        uuid.UUID
	createdAt time.Time
}

// expired keeps the newest `daily` backups plus the newest of each of the following `weekly`
// ISO weeks, and returns the rest.
func expired(items []retained, daily, weekly int) []uuid.UUID {
	sorted := append([]retained(nil), items...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].createdAt.After(sorted[j].createdAt) })

	weeks := map[[2]int]bool{}
	var drop []uuid.UUID
	for i, item := range sorted {
		if i < daily {
			continue
		}
		year, number := item.createdAt.In(pdfx.Store).ISOWeek()
		week := [2]int{year, number}
		if !weeks[week] && len(weeks) < weekly {
			weeks[week] = true
			continue
		}
		drop = append(drop, item.id)
	}
	return drop
}

func (s *Service) prune(ctx context.Context) error {
	policy, err := settings.GetBackups(ctx, s.pool)
	if err != nil {
		return err
	}
	rows, err := s.pool.Query(ctx, `SELECT id, created_at FROM backups WHERE kind = $1 AND status = $2`, KindScheduled, statusCompleted)
	if err != nil {
		return fmt.Errorf("list scheduled backups: %w", err)
	}
	defer rows.Close()
	var items []retained
	for rows.Next() {
		var item retained
		if err := rows.Scan(&item.id, &item.createdAt); err != nil {
			return err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range expired(items, policy.DailyRetention, policy.WeeklyRetention) {
		if err := s.remove(ctx, id); err != nil {
			return err
		}
	}
	_, err = s.pool.Exec(ctx, `DELETE FROM backups WHERE kind = $1 AND status = $2 AND created_at < now() - interval '7 days'`, KindScheduled, statusFailed)
	return err
}

func (s *Service) remove(ctx context.Context, id uuid.UUID) error {
	b, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := os.Remove(s.path(b.FileName)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove backup file: %w", err)
	}
	if b.remoteKey != nil && s.remoteEnabled() {
		if err := s.remote.Delete(ctx, *b.remoteKey); err != nil {
			s.logger.WarnContext(ctx, "remove offsite copy", "file", b.FileName, "error", err)
		}
	}
	_, err = s.pool.Exec(ctx, `DELETE FROM backups WHERE id = $1`, id)
	return err
}
