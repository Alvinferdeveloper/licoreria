package purchases

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/audit"
	"licoreria/api/internal/auth"
	"licoreria/api/internal/httpx"
	"licoreria/api/internal/inventory"
	"licoreria/api/internal/platform/database"
)

var (
	ErrNotDraft = apperr.New(apperr.KindConflict, "purchase_not_draft", "Solo se pueden editar o recibir compras en borrador.")
	ErrVoided   = apperr.New(apperr.KindConflict, "purchase_voided", "La compra ya está anulada.")
	ErrDupItem  = apperr.Validation(apperr.FieldError{Field: "items", Code: "duplicate", Message: "Cada producto debe aparecer una sola vez."})
)

type Input struct {
	SupplierID            uuid.UUID
	SupplierInvoiceNumber string
	PurchaseDate          string
	Notes                 string
	Items                 []ItemInput
}

type ItemInput struct {
	ProductID uuid.UUID
	Quantity  int
	UnitCost  string
	// ExpiresOn is the optional best-before date (YYYY-MM-DD) of this line.
	ExpiresOn *string
}

type Service struct {
	pool   *pgxpool.Pool
	ledger *inventory.Ledger
	repo   repository
}

func NewService(pool *pgxpool.Pool, ledger *inventory.Ledger) *Service {
	return &Service{pool: pool, ledger: ledger}
}

func (s *Service) List(ctx context.Context, f Filter, page httpx.Page) ([]Purchase, int64, error) {
	return s.repo.list(ctx, s.pool, f, page)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (Purchase, error) {
	return s.repo.get(ctx, s.pool, id, false)
}

func normalize(in Input) (Input, error) {
	in.SupplierInvoiceNumber = strings.TrimSpace(in.SupplierInvoiceNumber)
	in.Notes = strings.TrimSpace(in.Notes)
	seen := map[uuid.UUID]bool{}
	for _, item := range in.Items {
		if seen[item.ProductID] {
			return in, ErrDupItem
		}
		seen[item.ProductID] = true
	}
	return in, nil
}

func (s *Service) Create(ctx context.Context, actor auth.Principal, in Input) (Purchase, error) {
	in, err := normalize(in)
	if err != nil {
		return Purchase{}, err
	}
	var created Purchase
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		id, err := s.repo.insert(ctx, tx, in, actor.UserID)
		if err != nil {
			return err
		}
		if err := s.repo.replaceItems(ctx, tx, id, in.Items); err != nil {
			return err
		}
		if created, err = s.repo.get(ctx, tx, id, false); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{UserID: &actor.UserID, Action: "purchases.created", Entity: "purchases", EntityID: &id, After: created})
	})
	return created, err
}

func (s *Service) Update(ctx context.Context, actor auth.Principal, id uuid.UUID, in Input) (Purchase, error) {
	in, err := normalize(in)
	if err != nil {
		return Purchase{}, err
	}
	var updated Purchase
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		before, err := s.repo.get(ctx, tx, id, true)
		if err != nil {
			return err
		}
		if before.Status != StatusDraft {
			return ErrNotDraft
		}
		if err := s.repo.update(ctx, tx, id, in); err != nil {
			return err
		}
		if err := s.repo.replaceItems(ctx, tx, id, in.Items); err != nil {
			return err
		}
		if updated, err = s.repo.get(ctx, tx, id, false); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{UserID: &actor.UserID, Action: "purchases.updated", Entity: "purchases", EntityID: &id, Before: before, After: updated})
	})
	return updated, err
}

// Receive adds every line to stock at its unit cost.
func (s *Service) Receive(ctx context.Context, actor auth.Principal, id uuid.UUID) (Purchase, error) {
	return s.transition(ctx, actor, id, "purchases.received", func(tx pgx.Tx, p Purchase) error {
		if p.Status != StatusDraft {
			return ErrNotDraft
		}
		for _, item := range p.Items {
			_, err := s.ledger.Apply(ctx, tx, inventory.Movement{
				ProductID: item.Product.ID, Type: inventory.TypePurchase, Quantity: item.Quantity, UnitCost: item.UnitCost,
				Reason: purchaseReason(p), SourceType: "purchase", SourceID: &p.ID, UserID: actor.UserID,
			})
			if err != nil {
				return err
			}
		}
		return s.repo.markReceived(ctx, tx, id, actor.UserID)
	})
}

// Void cancels a draft, or reverses a received purchase if its stock is still on hand.
func (s *Service) Void(ctx context.Context, actor auth.Principal, id uuid.UUID, reason string) (Purchase, error) {
	reason = strings.TrimSpace(reason)
	return s.transition(ctx, actor, id, "purchases.voided", func(tx pgx.Tx, p Purchase) error {
		if p.Status == StatusVoided {
			return ErrVoided
		}
		if p.Status == StatusReceived {
			for _, item := range p.Items {
				_, err := s.ledger.Apply(ctx, tx, inventory.Movement{
					ProductID: item.Product.ID, Type: inventory.TypePurchase, Quantity: -item.Quantity, UnitCost: item.UnitCost,
					Reason: "Anulación de " + strings.ToLower(purchaseReason(p)) + ": " + reason, SourceType: "purchase", SourceID: &p.ID, UserID: actor.UserID,
				})
				if err != nil {
					return err
				}
			}
		}
		return s.repo.markVoided(ctx, tx, id, actor.UserID, reason)
	})
}

func (s *Service) transition(ctx context.Context, actor auth.Principal, id uuid.UUID, action string, apply func(pgx.Tx, Purchase) error) (Purchase, error) {
	var result Purchase
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		p, err := s.repo.get(ctx, tx, id, true)
		if err != nil {
			return err
		}
		if err := apply(tx, p); err != nil {
			return err
		}
		if result, err = s.repo.get(ctx, tx, id, false); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{UserID: &actor.UserID, Action: action, Entity: "purchases", EntityID: &id,
			Before: map[string]string{"status": p.Status}, After: map[string]string{"status": result.Status, "reason": result.VoidReason}})
	})
	return result, err
}

func purchaseReason(p Purchase) string {
	if p.SupplierInvoiceNumber == "" {
		return "Compra a " + p.Supplier.Name
	}
	return fmt.Sprintf("Compra a %s, factura %s", p.Supplier.Name, p.SupplierInvoiceNumber)
}
