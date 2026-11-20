package sales

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/audit"
	"licoreria/api/internal/auth"
	"licoreria/api/internal/cash"
	"licoreria/api/internal/httpx"
	"licoreria/api/internal/inventory"
	"licoreria/api/internal/mail"
	"licoreria/api/internal/platform/database"
	"licoreria/api/internal/platform/logx"
	"licoreria/api/internal/platform/pdfx"
	"licoreria/api/internal/rates"
	"licoreria/api/internal/schedule"
	"licoreria/api/internal/settings"
)

var (
	ErrAgeNotVerified     = apperr.Validation(apperr.FieldError{Field: "ageVerified", Code: "required", Message: "Confirma que el cliente es mayor de 18 años."})
	ErrProductUnavailable = apperr.New(apperr.KindConflict, "product_unavailable", "Un producto del carrito ya no está a la venta.")
	ErrNeedsAuthorization = apperr.New(apperr.KindForbidden, "authorization_required", "Este descuento supera tu límite y necesita el PIN de un administrador.")
	ErrAlreadyVoided      = apperr.New(apperr.KindConflict, "sale_voided", "La venta ya está anulada.")
	ErrShiftClosed        = apperr.New(apperr.KindConflict, "sale_shift_closed", "El turno de esa venta ya cerró y ya no se puede anular.")
	ErrKeyReused          = apperr.New(apperr.KindConflict, "idempotency_key_reused", "Esa clave de venta ya fue usada por otro usuario.")
)

type LineInput struct {
	ProductID      uuid.UUID
	PresentationID *uuid.UUID
	Quantity       int
}

type Authorization struct {
	Username, PIN string
}

type Input struct {
	CustomerID    *uuid.UUID
	AgeVerified   bool
	Notes         string
	Discount      *Discount
	Authorization *Authorization
	Lines         []LineInput
	Payments      []PaymentInput
}

type Service struct {
	pool   *pgxpool.Pool
	ledger *inventory.Ledger
	repo   repository
	logos  pdfx.ImageCache
}

func NewService(pool *pgxpool.Pool, ledger *inventory.Ledger) *Service {
	return &Service{pool: pool, ledger: ledger}
}

// seesAll: supervisors and read-only roles see every sale; cashiers only their own.
func seesAll(p auth.Principal) bool {
	return p.Can("cash_shifts", "authorize") || !p.Can("pos", "create")
}

func (s *Service) List(ctx context.Context, p auth.Principal, f Filter, page httpx.Page) ([]Sale, int64, error) {
	if !seesAll(p) {
		f.UserID = &p.UserID
	}
	return s.repo.list(ctx, s.pool, f, page)
}

func (s *Service) Get(ctx context.Context, p auth.Principal, id uuid.UUID) (Sale, error) {
	sale, err := s.repo.get(ctx, s.pool, id, false)
	if err == nil && sale.Cashier.ID != p.UserID && !seesAll(p) {
		return Sale{}, apperr.ErrNotFound
	}
	return sale, err
}

// Invoice renders the detailed PDF of a sale the user is allowed to see.
func (s *Service) Invoice(ctx context.Context, p auth.Principal, id uuid.UUID) (Sale, []byte, error) {
	sale, err := s.Get(ctx, p, id)
	if err != nil {
		return Sale{}, nil, err
	}
	business, err := settings.GetBusiness(ctx, s.pool)
	if err != nil {
		return Sale{}, nil, err
	}
	invoice, err := settings.GetInvoice(ctx, s.pool)
	if err != nil {
		return Sale{}, nil, err
	}
	pdf, err := InvoicePDF(sale, business, invoice.FooterNote, s.logo(ctx, business))
	return sale, pdf, err
}

// logo never blocks an invoice: if it can't be loaded, the PDF goes out without it.
func (s *Service) logo(ctx context.Context, b settings.Business) *pdfx.Image {
	if b.LogoURL == nil {
		return nil
	}
	logo, err := s.logos.Get(ctx, *b.LogoURL)
	if err != nil {
		logx.From(ctx).Warn("load invoice logo", "error", err)
		return nil
	}
	return logo
}

// Email queues the invoice to the given address; delivery happens in the background.
func (s *Service) Email(ctx context.Context, p auth.Principal, id uuid.UUID, to string) error {
	sale, err := s.Get(ctx, p, id)
	if err != nil {
		return err
	}
	business, err := settings.GetBusiness(ctx, s.pool)
	if err != nil {
		return err
	}
	type itemData struct {
		Name, UnitPrice, Subtotal string
		Quantity                  int
	}
	items := make([]itemData, len(sale.Items))
	for i, it := range sale.Items {
		items[i] = itemData{Name: it.Name, Quantity: it.Quantity, UnitPrice: money("C$", it.UnitPrice), Subtotal: money("C$", it.Subtotal)}
	}
	discount := ""
	if mustCents(sale.Discount) > 0 {
		discount = money("C$", sale.Discount)
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		err := mail.Enqueue(ctx, tx, mail.TemplateInvoice, to, map[string]any{
			"StoreName": business.Name, "StoreTaxID": business.TaxID, "Invoice": sale.Invoice,
			"Date": pdfx.DateTime(sale.CreatedAt), "Customer": sale.Customer.Name,
			"Items": items, "Subtotal": money("C$", sale.Subtotal), "Discount": discount, "Tax": money("C$", sale.Tax),
			"Total": money("C$", sale.Total), "TotalWords": AmountInWords(mustCents(sale.Total)),
		})
		if err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{UserID: &p.UserID, Action: "sales.emailed", Entity: "sales", EntityID: &id,
			After: map[string]string{"invoice": sale.Invoice}})
	})
}

// Create records a sale. Repeating a request with the same idempotency key returns the
// sale already recorded instead of charging twice.
func (s *Service) Create(ctx context.Context, actor auth.Principal, key uuid.UUID, in Input) (Sale, bool, error) {
	if id, owner, found, err := s.repo.byIdempotencyKey(ctx, s.pool, key); err != nil {
		return Sale{}, false, err
	} else if found {
		if owner != actor.UserID {
			return Sale{}, false, ErrKeyReused
		}
		sale, err := s.repo.get(ctx, s.pool, id, false)
		return sale, false, err
	}
	if !in.AgeVerified {
		return Sale{}, false, ErrAgeNotVerified
	}
	if hours, err := schedule.Evaluate(ctx, s.pool, actor.UserID, time.Now()); err != nil {
		return Sale{}, false, err
	} else if hours.Blocked(actor.Can("schedule", "authorize")) {
		return Sale{}, false, schedule.ErrSalesClosed
	}
	// Report a missing shift or rate before asking for a PIN; both are rechecked in the transaction.
	if _, _, err := cash.OpenShiftFor(ctx, s.pool, actor.UserID); err != nil {
		return Sale{}, false, err
	}
	if _, err := rates.Today(ctx, s.pool); err != nil {
		return Sale{}, false, err
	}

	authorizedBy, err := s.authorizeDiscount(ctx, actor, in)
	if err != nil {
		return Sale{}, false, err
	}

	var sale Sale
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		shiftID, registerID, err := cash.OpenShiftFor(ctx, tx, actor.UserID)
		if err != nil {
			return err
		}
		rate, err := rates.Today(ctx, tx)
		if err != nil {
			return err
		}
		rateMicros, err := parseFixed(rate.Rate, 6)
		if err != nil {
			return fmt.Errorf("parse rate: %w", err)
		}
		customer, err := s.repo.customer(ctx, tx, in.CustomerID)
		if err != nil {
			return err
		}
		lines, taxPercents, err := s.priceLines(ctx, tx, in.Lines)
		if err != nil {
			return err
		}
		t, err := computeTotals(lines, in.Discount)
		if err != nil {
			return err
		}
		payments, change, err := settle(t.Total, in.Payments, rateMicros)
		if err != nil {
			return err
		}
		series, number, err := s.repo.nextInvoice(ctx, tx, registerID)
		if err != nil {
			return err
		}
		saleID, err := s.repo.insertSale(ctx, tx, saleRow{
			Series: series, Number: number, ShiftID: shiftID, RegisterID: registerID, UserID: actor.UserID, Customer: customer,
			Totals: t, Rate: rate.Rate, Change: change, AuthorizedBy: authorizedBy, IdempotencyKey: key, Notes: strings.TrimSpace(in.Notes),
		})
		if err != nil {
			return err
		}
		for i, l := range lines {
			if err := s.repo.insertItem(ctx, tx, saleID, l, taxPercents[i]); err != nil {
				return err
			}
		}
		for _, p := range payments {
			if err := s.repo.insertPayment(ctx, tx, saleID, p, rate.Rate); err != nil {
				return err
			}
		}
		if err := s.moveStock(ctx, tx, actor.UserID, saleID, lines, -1, inventory.TypeSale, "Venta "+fmt.Sprintf("%s-%06d", series, number)); err != nil {
			return err
		}
		if sale, err = s.repo.get(ctx, tx, saleID, false); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{UserID: &actor.UserID, Action: "sales.created", Entity: "sales", EntityID: &saleID,
			After: map[string]any{"invoice": sale.Invoice, "total": sale.Total, "discount": sale.Discount, "authorizedBy": authorizedBy}})
	})
	return sale, err == nil, err
}

// authorizeDiscount requires an administrator PIN when the discount passes the role limit.
func (s *Service) authorizeDiscount(ctx context.Context, actor auth.Principal, in Input) (*uuid.UUID, error) {
	if in.Discount == nil || actor.Can("discounts", "authorize") {
		return nil, nil
	}
	pos, err := settings.GetPOS(ctx, s.pool)
	if err != nil {
		return nil, err
	}
	lines, _, err := s.priceLines(ctx, s.pool, in.Lines)
	if err != nil {
		return nil, err
	}
	t, err := computeTotals(lines, in.Discount)
	if err != nil {
		return nil, err
	}
	if float64(t.DiscountHundredths) <= pos.DiscountLimitPercent*100 {
		return nil, nil
	}
	if in.Authorization == nil {
		return nil, ErrNeedsAuthorization
	}
	id, err := auth.VerifyPIN(ctx, s.pool, in.Authorization.Username, in.Authorization.PIN, "discounts", "authorize")
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// priceLines takes names, prices, costs and tax from the catalog for each requested line.
func (s *Service) priceLines(ctx context.Context, db database.DBTX, in []LineInput) ([]pricedLine, []string, error) {
	lines := make([]pricedLine, 0, len(in))
	taxPercents := make([]string, 0, len(in))
	for _, l := range in {
		c, err := s.repo.catalogLine(ctx, db, l.ProductID, l.PresentationID)
		if apperr.IsKind(err, apperr.KindNotFound) || (err == nil && (!c.Active || !c.PresentationOK)) {
			return nil, nil, ErrProductUnavailable
		}
		if err != nil {
			return nil, nil, err
		}
		price, err := parseCents(c.UnitPrice)
		if err != nil {
			return nil, nil, err
		}
		tax, err := parseFixed(c.TaxPercent, 2)
		if err != nil {
			return nil, nil, err
		}
		lines = append(lines, pricedLine{ProductID: l.ProductID, PresentationID: l.PresentationID, Name: c.Name, SKU: c.SKU,
			Quantity: l.Quantity, UnitsPerItem: c.UnitsPerItem, UnitPrice: price, UnitCost: c.AverageCost, TaxHundredths: tax})
		taxPercents = append(taxPercents, c.TaxPercent)
	}
	return lines, taxPercents, nil
}

// moveStock applies one ledger movement per product, adding up its unit and pack lines.
func (s *Service) moveStock(ctx context.Context, tx pgx.Tx, userID, saleID uuid.UUID, lines []pricedLine, sign int, kind, reason string) error {
	units := map[uuid.UUID]int{}
	var order []uuid.UUID
	for _, l := range lines {
		if _, seen := units[l.ProductID]; !seen {
			order = append(order, l.ProductID)
		}
		units[l.ProductID] += l.Quantity * l.UnitsPerItem
	}
	for _, productID := range order {
		_, err := s.ledger.Apply(ctx, tx, inventory.Movement{ProductID: productID, Type: kind, Quantity: sign * units[productID],
			Reason: reason, SourceType: "sale", SourceID: &saleID, UserID: userID})
		if err != nil {
			return err
		}
	}
	return nil
}

// Void cancels a sale from a still-open shift and returns its stock. Without sales:void the
// cashier needs the PIN of someone who has it.
func (s *Service) Void(ctx context.Context, actor auth.Principal, id uuid.UUID, reason string, authz *Authorization) (Sale, error) {
	voidedBy := actor.UserID
	if !actor.Can("sales", "void") {
		if authz == nil {
			return Sale{}, apperr.New(apperr.KindForbidden, "authorization_required", "Anular una venta necesita el PIN de un administrador.")
		}
		var err error
		if voidedBy, err = auth.VerifyPIN(ctx, s.pool, authz.Username, authz.PIN, "sales", "void"); err != nil {
			return Sale{}, err
		}
	}
	reason = strings.TrimSpace(reason)
	var result Sale
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		sale, err := s.repo.get(ctx, tx, id, true)
		if err != nil {
			return err
		}
		if sale.Cashier.ID != actor.UserID && !seesAll(actor) {
			return apperr.ErrNotFound
		}
		if sale.Status == "voided" {
			return ErrAlreadyVoided
		}
		if open, err := s.repo.shiftIsOpen(ctx, tx, sale.ShiftID); err != nil {
			return err
		} else if !open {
			return ErrShiftClosed
		}
		lines := make([]pricedLine, len(sale.Items))
		for i, item := range sale.Items {
			lines[i] = pricedLine{ProductID: item.ProductID, Quantity: item.Quantity, UnitsPerItem: item.UnitsPerItem}
		}
		if err := s.moveStock(ctx, tx, actor.UserID, id, lines, 1, inventory.TypeSaleVoid, "Anulación de "+sale.Invoice+": "+reason); err != nil {
			return err
		}
		if err := s.repo.markVoided(ctx, tx, id, voidedBy, reason); err != nil {
			return err
		}
		if result, err = s.repo.get(ctx, tx, id, false); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{UserID: &actor.UserID, Action: "sales.voided", Entity: "sales", EntityID: &id,
			After: map[string]any{"invoice": sale.Invoice, "reason": reason, "authorizedBy": voidedBy}})
	})
	return result, err
}
