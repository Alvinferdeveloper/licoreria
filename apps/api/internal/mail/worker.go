package mail

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"licoreria/api/internal/platform/database"
)

const (
	maxAttempts  = 5
	batchSize    = 10
	sendTimeout  = 20 * time.Second
	pollInterval = 5 * time.Second
)

// Worker delivers queued emails until its context is cancelled.
type Worker struct {
	pool   *pgxpool.Pool
	sender Sender
	logger *slog.Logger
}

func NewWorker(pool *pgxpool.Pool, sender Sender, logger *slog.Logger) *Worker {
	return &Worker{pool: pool, sender: sender, logger: logger.With("component", "mail_worker")}
}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		if err := w.ProcessBatch(ctx); err != nil && ctx.Err() == nil {
			w.logger.Error("process email batch", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

type queued struct {
	id       uuid.UUID
	to       string
	subject  string
	html     string
	attempts int
}

// ProcessBatch claims due emails with SKIP LOCKED, so several API instances never send the same one twice.
func (w *Worker) ProcessBatch(ctx context.Context) error {
	return database.WithTx(ctx, w.pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, to_address, subject, html_body, attempts
			FROM email_queue
			WHERE status = 'pending' AND scheduled_at <= now()
			ORDER BY scheduled_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED`, batchSize)
		if err != nil {
			return fmt.Errorf("claim emails: %w", err)
		}
		batch, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (queued, error) {
			var q queued
			err := row.Scan(&q.id, &q.to, &q.subject, &q.html, &q.attempts)
			return q, err
		})
		if err != nil {
			return fmt.Errorf("read emails: %w", err)
		}

		for _, q := range batch {
			if err := w.deliver(ctx, tx, q); err != nil {
				return err
			}
		}
		return nil
	})
}

func (w *Worker) deliver(ctx context.Context, tx pgx.Tx, q queued) error {
	sendCtx, cancel := context.WithTimeout(ctx, sendTimeout)
	sendErr := w.sender.Send(sendCtx, Message{To: q.to, Subject: q.subject, HTML: q.html})
	cancel()

	if sendErr == nil {
		w.logger.Info("email sent", "email_id", q.id, "subject", q.subject)
		// The body is cleared once delivered: it can contain a temporary password or a reset link.
		_, err := tx.Exec(ctx, `
			UPDATE email_queue SET status = 'sent', sent_at = now(), attempts = attempts + 1, html_body = ''
			WHERE id = $1`, q.id)
		return err
	}

	attempts := q.attempts + 1
	status := "pending"
	if attempts >= maxAttempts {
		status = "failed"
	}
	w.logger.Warn("email delivery failed", "email_id", q.id, "attempt", attempts, "status", status, "error", sendErr)
	_, err := tx.Exec(ctx, `
		UPDATE email_queue
		SET attempts = $2, status = $3, last_error = $4, scheduled_at = now() + make_interval(secs => $5)
		WHERE id = $1`, q.id, attempts, status, sendErr.Error(), backoff(attempts).Seconds())
	return err
}

// backoff grows 30s, 1m, 2m, 4m... capped at 30 minutes.
func backoff(attempt int) time.Duration {
	d := 30 * time.Second << (attempt - 1)
	if d > 30*time.Minute || d <= 0 {
		return 30 * time.Minute
	}
	return d
}
