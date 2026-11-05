package sales

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Past is an already dated sale, used to load demo history. The caller owns the shift,
// the invoice number and the stock movements.
type Past struct {
	ShiftID, RegisterID, UserID uuid.UUID
	Series                      string
	Number                      int
	CustomerID                  *uuid.UUID
	Lines                       []LineInput
	Discount                    *Discount
	AuthorizedBy                *uuid.UUID
	Rate                        string
	At                          time.Time
	// Pay decides the payments once the total is known.
	Pay func(totalCents int64) []PaymentInput
}

type PastResult struct {
	ID                       uuid.UUID
	CashNIO, CashUSD, Change int64
}

// RecordPast stores a sale with the same totals, VAT and change rules as Create.
func RecordPast(ctx context.Context, tx pgx.Tx, p Past) (PastResult, error) {
	var s Service
	rateMicros, err := parseFixed(p.Rate, 6)
	if err != nil {
		return PastResult{}, fmt.Errorf("parse rate: %w", err)
	}
	customer, err := s.repo.customer(ctx, tx, p.CustomerID)
	if err != nil {
		return PastResult{}, err
	}
	lines, taxPercents, err := s.priceLines(ctx, tx, p.Lines)
	if err != nil {
		return PastResult{}, err
	}
	t, err := computeTotals(lines, p.Discount)
	if err != nil {
		return PastResult{}, err
	}
	payments, change, err := settle(t.Total, p.Pay(t.Total), rateMicros)
	if err != nil {
		return PastResult{}, err
	}
	id, err := s.repo.insertSale(ctx, tx, saleRow{
		Series: p.Series, Number: p.Number, ShiftID: p.ShiftID, RegisterID: p.RegisterID, UserID: p.UserID, Customer: customer,
		Totals: t, Rate: p.Rate, Change: change, AuthorizedBy: p.AuthorizedBy, IdempotencyKey: uuid.New(),
	})
	if err != nil {
		return PastResult{}, err
	}
	for i, l := range lines {
		if err := s.repo.insertItem(ctx, tx, id, l, taxPercents[i]); err != nil {
			return PastResult{}, err
		}
	}
	result := PastResult{ID: id, Change: change}
	for _, pay := range payments {
		if err := s.repo.insertPayment(ctx, tx, id, pay, p.Rate); err != nil {
			return PastResult{}, err
		}
		switch {
		case pay.Method == "cash" && pay.Currency == "USD":
			result.CashUSD += pay.AmountCents
		case pay.Method == "cash":
			result.CashNIO += pay.AmountCents
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE sales SET created_at = $2 WHERE id = $1`, id, p.At); err != nil {
		return PastResult{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE sale_payments SET created_at = $2 WHERE sale_id = $1`, id, p.At); err != nil {
		return PastResult{}, err
	}
	return result, nil
}
