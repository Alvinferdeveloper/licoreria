package inventory

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/audit"
	"licoreria/api/internal/auth"
	"licoreria/api/internal/httpx"
	"licoreria/api/internal/platform/database"
)

const (
	AdjustCount     = "count"
	AdjustShrinkage = "shrinkage"
)

var ErrNoDifference = apperr.Validation(apperr.FieldError{Field: "quantity", Code: "no_difference",
	Message: "El conteo coincide con el stock actual; no hay nada que ajustar."})

// AdjustInput: for a count, Quantity is the units counted; for shrinkage, the units lost.
type AdjustInput struct {
	ProductID uuid.UUID
	Kind      string
	Quantity  int
	Reason    string
}

type Service struct {
	pool   *pgxpool.Pool
	ledger *Ledger
	repo   repository
}

func NewService(pool *pgxpool.Pool, ledger *Ledger) *Service {
	return &Service{pool: pool, ledger: ledger}
}

func canSeeCosts(p auth.Principal) bool { return p.Can("products", "authorize") }

func (s *Service) Movements(ctx context.Context, p auth.Principal, f MovementFilter, page httpx.Page) ([]MovementEntry, int64, error) {
	items, total, err := s.repo.movements(ctx, s.pool, f, page)
	if !canSeeCosts(p) {
		for i := range items {
			items[i].UnitCost = nil
		}
	}
	return items, total, err
}

func (s *Service) Alerts(ctx context.Context, p auth.Principal) ([]Alert, error) {
	items, err := s.repo.alerts(ctx, s.pool)
	if !canSeeCosts(p) {
		for i := range items {
			items[i].LastCost = nil
		}
	}
	return items, err
}

// Adjust records a physical count (the difference against current stock) or a loss.
func (s *Service) Adjust(ctx context.Context, actor auth.Principal, in AdjustInput) (int, error) {
	in.Reason = strings.TrimSpace(in.Reason)
	var after int
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		m := Movement{ProductID: in.ProductID, Reason: in.Reason, UserID: actor.UserID, SourceType: "adjustment"}
		if in.Kind == AdjustCount {
			current, err := s.repo.stock(ctx, tx, in.ProductID)
			if err != nil {
				return err
			}
			if in.Quantity == current {
				return ErrNoDifference
			}
			m.Type, m.Quantity = TypeCountAdjustment, in.Quantity-current
		} else {
			m.Type, m.Quantity = TypeShrinkage, -in.Quantity
		}
		var err error
		if after, err = s.ledger.Apply(ctx, tx, m); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{UserID: &actor.UserID, Action: "inventory.adjusted", Entity: "products", EntityID: &in.ProductID,
			After: map[string]any{"type": m.Type, "quantity": m.Quantity, "stockAfter": after, "reason": in.Reason}})
	})
	return after, err
}
