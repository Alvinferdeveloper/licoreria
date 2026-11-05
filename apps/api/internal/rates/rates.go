// Package rates keeps the daily NIO per USD exchange rate. Sales copy the rate they used,
// so correcting a day's rate never changes past sales.
package rates

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/audit"
	"licoreria/api/internal/auth"
	"licoreria/api/internal/httpx"
	"licoreria/api/internal/platform/database"
)

// StoreDay is the calendar day in the store's zone, as SQL.
const StoreDay = "(now() AT TIME ZONE 'America/Managua')::date"

var (
	ErrMissing  = apperr.New(apperr.KindNotFound, "exchange_rate_missing", "Todavía no se registró la tasa de cambio de hoy.")
	ErrPastDate = apperr.Validation(apperr.FieldError{Field: "date", Code: "past_date", Message: "Solo se puede registrar la tasa de hoy o de días próximos."})
)

type Ref struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// Rate is córdobas per one US dollar on Date.
type Rate struct {
	Date      string    `json:"date"`
	Rate      string    `json:"rate"`
	CreatedBy Ref       `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
}

const selectRate = `
	SELECT r.effective_date::text, r.rate::text, u.id, u.full_name, r.created_at
	FROM exchange_rates r JOIN users u ON u.id = r.created_by
	WHERE r.currency_code = 'USD'`

func scan(row pgx.Row) (Rate, error) {
	var r Rate
	err := row.Scan(&r.Date, &r.Rate, &r.CreatedBy.ID, &r.CreatedBy.Name, &r.CreatedAt)
	return r, err
}

// Today returns today's rate or ErrMissing; the POS refuses to sell without it.
func Today(ctx context.Context, db database.DBTX) (Rate, error) {
	r, err := scan(db.QueryRow(ctx, selectRate+" AND r.effective_date = "+StoreDay))
	if errors.Is(err, pgx.ErrNoRows) {
		return Rate{}, ErrMissing
	}
	return r, database.MapError(err, nil)
}

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

func (s *Service) List(ctx context.Context, page httpx.Page) ([]Rate, int64, error) {
	var total int64
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM exchange_rates WHERE currency_code = 'USD'`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count rates: %w", err)
	}
	rows, err := s.pool.Query(ctx, selectRate+" ORDER BY r.effective_date DESC LIMIT $1 OFFSET $2", page.Size, page.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("list rates: %w", err)
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Rate, error) { return scan(row) })
	return items, total, err
}

// Set registers or corrects the rate for today or a later day.
func (s *Service) Set(ctx context.Context, actor auth.Principal, date *string, rate string) (Rate, error) {
	var saved Rate
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var day string
		var past bool
		err := tx.QueryRow(ctx, `SELECT COALESCE($1::date, `+StoreDay+`)::text, COALESCE($1::date, `+StoreDay+`) < `+StoreDay, date).Scan(&day, &past)
		if err != nil {
			return database.MapError(err, nil)
		}
		if past {
			return ErrPastDate
		}
		var before *string
		_ = tx.QueryRow(ctx, `SELECT rate::text FROM exchange_rates WHERE currency_code = 'USD' AND effective_date = $1::date`, day).Scan(&before)
		_, err = tx.Exec(ctx, `
			INSERT INTO exchange_rates (currency_code, rate, effective_date, created_by) VALUES ('USD', $1::numeric, $2::date, $3)
			ON CONFLICT (currency_code, effective_date) DO UPDATE SET rate = EXCLUDED.rate, created_by = EXCLUDED.created_by, created_at = now()`,
			rate, day, actor.UserID)
		if err != nil {
			return database.MapError(err, nil)
		}
		if saved, err = scan(tx.QueryRow(ctx, selectRate+" AND r.effective_date = $1::date", day)); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{UserID: &actor.UserID, Action: "exchange_rates.set", Entity: "exchange_rates",
			Before: before, After: map[string]string{"date": day, "rate": saved.Rate}})
	})
	return saved, err
}
