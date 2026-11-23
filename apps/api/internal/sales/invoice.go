package sales

import (
	"fmt"
	"strings"

	"github.com/johnfercher/maroto/v2/pkg/components/code"
	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/image"
	"github.com/johnfercher/maroto/v2/pkg/components/line"
	"github.com/johnfercher/maroto/v2/pkg/components/row"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/border"
	"github.com/johnfercher/maroto/v2/pkg/core"
	"github.com/johnfercher/maroto/v2/pkg/props"

	"licoreria/api/internal/platform/pdfx"
	"licoreria/api/internal/settings"
)

var (
	methodNames    = map[string]string{"cash": "Efectivo", "card": "Tarjeta", "transfer": "Transferencia"}
	referenceNames = map[string]string{"card": "aut.", "transfer": "ref."}
)

// money formats a decimal string as "C$ 1,250.50" or "US$ 20.00".
func money(symbol, amount string) string {
	cents, err := parseCents(strings.TrimPrefix(amount, "-"))
	if err != nil {
		return symbol + " " + amount
	}
	sign := ""
	if strings.HasPrefix(amount, "-") {
		sign = "-"
	}
	whole := fmt.Sprintf("%d", cents/100)
	var b strings.Builder
	for i, r := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return fmt.Sprintf("%s%s %s.%02d", sign, symbol, b.String(), cents%100)
}

// usdEquivalent converts a córdoba amount with the sale's rate, for the reference line.
func usdEquivalent(totalNIO, rate string) string {
	cents, err1 := parseCents(totalNIO)
	micros, err2 := parseFixed(rate, 6)
	if err1 != nil || err2 != nil || micros == 0 {
		return ""
	}
	return formatCents(divRound(cents*1_000_000, micros))
}

// InvoicePDF renders the detailed letter-size invoice.
func InvoicePDF(s Sale, b settings.Business, note string, logo *pdfx.Image) ([]byte, error) {
	m, err := pdfx.New("Factura " + s.Invoice)
	if err != nil {
		return nil, err
	}
	muted := func(size float64, a align.Type) props.Text { return pdfx.Text(size, false, false, a, pdfx.Muted) }

	// Header: business on the left, invoice number on the right.
	business := []core.Component{text.New(b.Name, pdfx.Text(15, true, false, align.Left, nil))}
	top := 7.0
	for _, line := range []string{prefixed("RUC ", b.TaxID), b.Address, joinNonEmpty(" · ", b.Phone, b.Email)} {
		if line == "" {
			continue
		}
		t := muted(8.5, align.Left)
		t.Top = top
		business = append(business, text.New(line, t))
		top += 4
	}
	right := []core.Component{
		text.New("FACTURA", muted(8, align.Right)),
		text.New(s.Invoice, withTop(pdfx.Text(14, true, true, align.Right, nil), 4)),
		text.New(pdfx.DateTime(s.CreatedAt), withTop(muted(8.5, align.Right), 11)),
	}
	if s.Status == "voided" {
		right = append(right, text.New("ANULADA", withTop(pdfx.Text(10, true, false, align.Right, pdfx.Danger), 15.5)))
	}
	header := row.New(max(top, 20) + 2)
	if logo != nil {
		header.Add(image.NewFromBytesCol(2, logo.Data, logo.Type, props.Rect{Percent: 90}), col.New(6).Add(business...))
	} else {
		header.Add(col.New(8).Add(business...))
	}
	m.AddRows(header.Add(col.New(4).Add(right...)))
	m.AddRows(line.NewRow(3, props.Line{Color: pdfx.Amber, Thickness: 0.6, SizePercent: 100}))

	// Sale data.
	customer := s.Customer.Name
	if s.Customer.TaxID != nil && *s.Customer.TaxID != "" {
		customer += " · Cédula/RUC " + *s.Customer.TaxID
	}
	m.AddRows(
		row.New(5).Add(
			text.NewCol(7, "Cliente", muted(8, align.Left)),
			text.NewCol(5, "Caja y cajero", muted(8, align.Right)),
		),
		row.New(6).Add(
			text.NewCol(7, customer, pdfx.Text(9.5, true, false, align.Left, nil)),
			text.NewCol(5, s.Register.Name+" · "+s.Cashier.Name, pdfx.Text(9.5, false, false, align.Right, nil)),
		),
		row.New(4),
	)

	// Items.
	head := &props.Cell{BackgroundColor: pdfx.Sunken}
	headText := func(a align.Type) props.Text {
		t := pdfx.Text(8, true, false, a, pdfx.Muted)
		t.Top = 1.8
		t.Left = 1.5
		t.Right = 1.5
		return t
	}
	m.AddRows(row.New(7).Add(
		text.NewCol(2, "Código", headText(align.Left)),
		text.NewCol(4, "Descripción", headText(align.Left)),
		text.NewCol(1, "Cant.", headText(align.Right)),
		text.NewCol(2, "P. unitario", headText(align.Right)),
		text.NewCol(1, "Desc.", headText(align.Right)),
		text.NewCol(2, "Subtotal", headText(align.Right)),
	).WithStyle(head))
	cell := func(size float64, mono bool, a align.Type) props.Text {
		t := pdfx.Text(size, false, mono, a, nil)
		t.Top, t.Bottom, t.Left, t.Right = 2.2, 2.2, 1.5, 1.5
		return t
	}
	for _, it := range s.Items {
		desc := it.Name
		if it.UnitsPerItem > 1 {
			desc += fmt.Sprintf(" (× %d u)", it.UnitsPerItem)
		}
		discount := "—"
		if c, _ := parseCents(it.Discount); c > 0 {
			discount = money("", it.Discount)
		}
		m.AddAutoRow(
			text.NewCol(2, it.SKU, cell(8, true, align.Left)),
			text.NewCol(4, desc, cell(8.5, false, align.Left)),
			text.NewCol(1, fmt.Sprintf("%d", it.Quantity), cell(8.5, true, align.Right)),
			text.NewCol(2, money("", it.UnitPrice), cell(8.5, true, align.Right)),
			text.NewCol(1, strings.TrimSpace(discount), cell(8.5, true, align.Right)),
			text.NewCol(2, money("", it.Subtotal), cell(8.5, true, align.Right)),
		).WithStyle(&props.Cell{BorderColor: pdfx.Line, BorderType: border.Bottom})
	}
	m.AddRows(row.New(4))

	// Totals with the QR on the left.
	totals := [][2]string{{"Subtotal", money("C$", s.Subtotal)}}
	if c, _ := parseCents(s.Discount); c > 0 {
		totals = append(totals, [2]string{"Descuento", "-" + money("C$", s.Discount)})
	}
	totals = append(totals, [2]string{"IVA", money("C$", s.Tax)})
	labels := make([]core.Component, 0, len(totals)+2)
	values := make([]core.Component, 0, len(totals)+2)
	for i, t := range totals {
		labels = append(labels, text.New(t[0], withTop(muted(9, align.Right), float64(i)*5)))
		values = append(values, text.New(t[1], withTop(pdfx.Text(9, false, true, align.Right, nil), float64(i)*5)))
	}
	y := float64(len(totals)) * 5
	labels = append(labels, text.New("Total", withTop(pdfx.Text(11, true, false, align.Right, nil), y+1.5)))
	values = append(values, text.New(money("C$", s.Total), withTop(pdfx.Text(12, true, true, align.Right, nil), y+1)))
	if usd := usdEquivalent(s.Total, s.ExchangeRate); usd != "" {
		values = append(values, text.New("≈ "+money("US$", usd), withTop(muted(8.5, align.Right), y+7.5)))
	}
	m.AddRows(row.New(y+13).Add(
		code.NewQrCol(2, s.Invoice, props.Rect{Percent: 100}),
		col.New(4),
		col.New(3).Add(labels...),
		col.New(3).Add(values...),
	))
	m.AddRows(text.NewRow(7, "Son: "+AmountInWords(mustCents(s.Total)), withTop(pdfx.Text(8.5, false, false, align.Left, nil), 2)))

	// Payments.
	m.AddRows(text.NewRow(6, "Pagos", withTop(pdfx.Text(8.5, true, false, align.Left, pdfx.Muted), 2)))
	for _, p := range s.Payments {
		label := methodNames[p.Method]
		if p.Reference != nil && referenceNames[p.Method] != "" {
			label += fmt.Sprintf(" · %s %s", referenceNames[p.Method], *p.Reference)
		}
		symbol := "C$"
		if p.Currency == "USD" {
			symbol = "US$"
			label += fmt.Sprintf(" en dólares · tasa C$ %s · equivale a %s", strings.TrimRight(strings.TrimRight(p.ExchangeRate, "0"), "."), money("C$", p.AmountNIO))
		}
		m.AddRows(row.New(4.5).Add(
			text.NewCol(9, label, muted(8.5, align.Left)),
			text.NewCol(3, money(symbol, p.Amount), pdfx.Text(8.5, false, true, align.Right, nil)),
		))
	}
	if c, _ := parseCents(s.ChangeNIO); c > 0 {
		m.AddRows(row.New(4.5).Add(
			text.NewCol(9, "Vuelto entregado", muted(8.5, align.Left)),
			text.NewCol(3, money("C$", s.ChangeNIO), pdfx.Text(8.5, false, true, align.Right, nil)),
		))
	}
	if s.Status == "voided" && s.VoidReason != nil {
		m.AddRows(text.NewRow(8, "Anulada: "+*s.VoidReason, withTop(pdfx.Text(8.5, true, false, align.Left, pdfx.Danger), 3)))
	}

	footer := []string{
		"Prohibida la venta de bebidas alcohólicas a menores de 18 años. Disfrute con responsabilidad.",
		"Comprobante interno sin validez fiscal ante la DGI.",
	}
	if note != "" {
		footer = append([]string{note}, footer...)
	}
	lines := make([]core.Component, len(footer))
	for i, l := range footer {
		lines[i] = text.New(l, withTop(muted(7.5, align.Center), float64(i)*3.5))
	}
	if err := m.RegisterFooter(row.New(14).Add(col.New(12).Add(lines...))); err != nil {
		return nil, fmt.Errorf("invoice footer: %w", err)
	}

	doc, err := m.Generate()
	if err != nil {
		return nil, fmt.Errorf("generate invoice: %w", err)
	}
	return doc.GetBytes(), nil
}

func withTop(t props.Text, top float64) props.Text {
	t.Top = top
	return t
}

func prefixed(prefix, v string) string {
	if strings.TrimSpace(v) == "" {
		return ""
	}
	return prefix + v
}

func joinNonEmpty(sep string, parts ...string) string {
	var kept []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, sep)
}

func mustCents(v string) int64 {
	c, _ := parseCents(v)
	return c
}
