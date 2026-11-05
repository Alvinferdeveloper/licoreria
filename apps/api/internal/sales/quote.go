package sales

import (
	"github.com/google/uuid"

	"licoreria/api/internal/apperr"
)

var (
	ErrDiscountTooLarge = apperr.Validation(apperr.FieldError{Field: "discount", Code: "exceeds_subtotal", Message: "El descuento no puede superar el subtotal."})
	ErrNonCashOverpay   = apperr.Validation(apperr.FieldError{Field: "payments", Code: "non_cash_overpay", Message: "Tarjeta y transferencia no pueden cobrar más que el saldo."})
	ErrUnderpaid        = apperr.Validation(apperr.FieldError{Field: "payments", Code: "underpaid", Message: "Lo pagado no cubre el total de la venta."})
)

// pricedLine is a cart line with its price and tax taken from the catalog, never the client.
type pricedLine struct {
	ProductID      uuid.UUID
	PresentationID *uuid.UUID
	Name, SKU      string
	Quantity       int
	UnitsPerItem   int
	UnitPrice      int64
	UnitCost       string
	TaxHundredths  int64
	// Filled by computeTotals.
	Subtotal, Discount, Tax, Total int64
}

type Discount struct {
	Kind  string // "percent" or "amount"
	Value string
}

type totals struct {
	Subtotal, Discount, Tax, Total int64
	// DiscountHundredths is the discount as a share of the subtotal, for the role limit.
	DiscountHundredths int64
}

// computeTotals spreads the sale discount across lines so VAT is charged on the discounted
// base, line by line, exactly like the POS.
func computeTotals(lines []pricedLine, d *Discount) (totals, error) {
	var t totals
	weights := make([]int64, len(lines))
	for i := range lines {
		lines[i].Subtotal = lines[i].UnitPrice * int64(lines[i].Quantity)
		weights[i] = lines[i].Subtotal
		t.Subtotal += lines[i].Subtotal
	}
	if d != nil {
		switch d.Kind {
		case "percent":
			hundredths, err := parseFixed(d.Value, 2)
			if err != nil || hundredths > 10000 {
				return t, ErrDiscountTooLarge
			}
			t.Discount = percentOf(t.Subtotal, hundredths)
		default:
			cents, err := parseCents(d.Value)
			if err != nil || cents > t.Subtotal {
				return t, ErrDiscountTooLarge
			}
			t.Discount = cents
		}
		if t.Subtotal > 0 {
			t.DiscountHundredths = divRound(t.Discount*10000, t.Subtotal)
		}
	}
	shares := allocate(t.Discount, weights)
	for i := range lines {
		lines[i].Discount = shares[i]
		lines[i].Tax = percentOf(lines[i].Subtotal-shares[i], lines[i].TaxHundredths)
		lines[i].Total = lines[i].Subtotal - shares[i] + lines[i].Tax
		t.Tax += lines[i].Tax
	}
	t.Total = t.Subtotal - t.Discount + t.Tax
	return t, nil
}

type PaymentInput struct {
	Method    string
	Currency  string
	Amount    string
	Reference *string
}

type settledPayment struct {
	PaymentInput
	AmountCents int64
	AmountNIO   int64
}

// settle converts each payment to córdobas and returns the change owed in córdobas.
// Card and transfer must not push the paid amount past the total.
func settle(total int64, payments []PaymentInput, rateMicros int64) ([]settledPayment, int64, error) {
	settled := make([]settledPayment, 0, len(payments))
	var paid, nonCash int64
	for _, p := range payments {
		cents, err := parseCents(p.Amount)
		if err != nil || cents == 0 {
			return nil, 0, apperr.Validation(apperr.FieldError{Field: "payments", Code: "money", Message: "Cada pago debe tener un monto mayor que 0."})
		}
		nio := cents
		if p.Currency == "USD" {
			nio = usdToNIO(cents, rateMicros)
		}
		paid += nio
		if p.Method != "cash" {
			nonCash += nio
		}
		settled = append(settled, settledPayment{PaymentInput: p, AmountCents: cents, AmountNIO: nio})
	}
	// A USD card payment may exceed by the rounding of one cent; anything more is an overpay.
	if nonCash > total+usdToNIO(1, rateMicros) {
		return nil, 0, ErrNonCashOverpay
	}
	if paid < total {
		return nil, 0, ErrUnderpaid
	}
	return settled, paid - total, nil
}
