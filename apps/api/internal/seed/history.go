package seed

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"math/rand/v2"
	"slices"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"licoreria/api/internal/platform/password"
	"licoreria/api/internal/platform/pdfx"
	"licoreria/api/internal/sales"
)

const (
	historyDays = 90
	baseRate    = 36_624_300 // córdobas per dollar, in millionths
	openingNIO  = 200_000
	openingUSD  = 5_000
	// weeklyUnits estimates the units sold per week, to size each restock.
	weeklyUnits = 1500
)

var (
	categoryDemand = map[string]float64{"Cerveza": 6, "Ron": 3.5, "Mezcladores": 2.5, "Snacks y desechables": 2.5}
	brandDemand    = map[string]float64{"Flor de Caña": 2.5, "Toña": 2, "Victoria": 1.6}
	hourDemand     = map[int]float64{10: 2, 11: 3, 12: 4, 13: 5, 14: 4, 15: 4, 16: 5, 17: 7, 18: 10, 19: 12, 20: 10, 21: 6}
	expenses       = []string{"Compra de hielo", "Pago de flete", "Bolsas para despacho", "Limpieza del local"}
	shortages      = []string{"Faltante en el arqueo, se revisó con el supervisor.", "Vuelto mal entregado en hora pico."}
	voidReasons    = []string{"Cliente desistió de la compra", "Error al digitar el producto", "Cobro duplicado"}
	demoCashiers   = []struct{ name, username string }{
		{"Ana Lucía Treminio", "ana"}, {"Carlos Darce", "carlos"}, {"Yader Espinoza", "yader"},
	}
)

type presentation struct {
	id     uuid.UUID
	factor int
}

type stockItem struct {
	id                       uuid.UUID
	category, brand          string
	stock, reorder, target   int
	costCents, taxHundredths int64
	packs                    []presentation
}

type register struct {
	id     uuid.UUID
	series string
	next   int
}

type shift struct {
	id               uuid.UUID
	register         *register
	user             uuid.UUID
	closedAt         time.Time
	cashNIO, cashUSD int64 // net of change
	expense          int64
}

type supplier struct {
	id   uuid.UUID
	name string
}

// history simulates daily operation in order, so every kardex balance stays exact.
type history struct {
	tx         pgx.Tx
	rng        *rand.Rand
	admin      uuid.UUID
	items      []*stockItem
	cumulative []float64
	customers  []uuid.UUID
	suppliers  []supplier
	registers  []*register
	cashiers   []uuid.UUID
	today      time.Time
	voids      []pendingVoid
	result     *Result
}

func storeToday() time.Time {
	now := time.Now().In(pdfx.Store)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, pdfx.Store)
}

func cents(v int64) string { return fmt.Sprintf("%d.%02d", v/100, v%100) }

func stableHash(s string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return int(h.Sum32())
}

func loadHistory(ctx context.Context, tx pgx.Tx, admin uuid.UUID, result *Result) error {
	h := &history{tx: tx, admin: admin, rng: rand.New(rand.NewPCG(2026, 11)), today: storeToday(), result: result}
	for _, load := range []func(context.Context) error{h.loadItems, h.loadParties, h.loadRegisters, h.addCashiers} {
		if err := load(ctx); err != nil {
			return err
		}
	}
	return h.run(ctx)
}

func (h *history) loadItems(ctx context.Context) error {
	rows, err := h.tx.Query(ctx, `
		SELECT p.id, c.name, b.name, p.stock, p.reorder_point, (p.average_cost * 100)::bigint, (p.tax_percent * 100)::bigint,
		       (p.price * 100)::bigint,
		       COALESCE((SELECT json_agg(json_build_object('id', pp.id, 'factor', pp.factor) ORDER BY pp.factor)
		                 FROM product_presentations pp WHERE pp.product_id = p.id), '[]')::text
		FROM products p JOIN categories c ON c.id = p.category_id JOIN brands b ON b.id = p.brand_id
		ORDER BY p.sku`)
	if err != nil {
		return err
	}
	defer rows.Close()
	prices := map[*stockItem]int64{}
	for rows.Next() {
		it := &stockItem{}
		var price int64
		var packs string
		if err := rows.Scan(&it.id, &it.category, &it.brand, &it.stock, &it.reorder, &it.costCents, &it.taxHundredths, &price, &packs); err != nil {
			return err
		}
		if it.packs, err = parsePacks(packs); err != nil {
			return err
		}
		prices[it] = price
		h.items = append(h.items, it)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// Popularity falls with rank and price; a few categories and local brands sell far more.
	weights := make([]float64, len(h.items))
	var total float64
	for rank, i := range h.rng.Perm(len(h.items)) {
		it := h.items[i]
		w := 1 / math.Pow(float64(rank+4), 0.7) / math.Sqrt(float64(prices[it])/10000+1)
		if d, ok := categoryDemand[it.category]; ok {
			w *= d
		}
		if d, ok := brandDemand[it.brand]; ok {
			w *= d
		}
		weights[i] = w
		total += w
	}
	h.cumulative = make([]float64, len(weights))
	var sum float64
	for i, w := range weights {
		sum += w
		h.cumulative[i] = sum
		h.items[i].target = h.items[i].reorder*2 + int(w/total*weeklyUnits*1.5)
	}
	return nil
}

func parsePacks(raw string) ([]presentation, error) {
	var rows []struct {
		ID     uuid.UUID `json:"id"`
		Factor int       `json:"factor"`
	}
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		return nil, err
	}
	packs := make([]presentation, len(rows))
	for i, r := range rows {
		packs[i] = presentation{id: r.ID, factor: r.Factor}
	}
	return packs, nil
}

func (h *history) loadParties(ctx context.Context) error {
	rows, err := h.tx.Query(ctx, `SELECT id FROM customers WHERE NOT is_walk_in ORDER BY full_name`)
	if err != nil {
		return err
	}
	if h.customers, err = pgx.CollectRows(rows, pgx.RowTo[uuid.UUID]); err != nil {
		return err
	}
	rows, err = h.tx.Query(ctx, `SELECT id, name FROM suppliers ORDER BY name`)
	if err != nil {
		return err
	}
	h.suppliers, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (supplier, error) {
		var s supplier
		return s, row.Scan(&s.id, &s.name)
	})
	return err
}

// loadRegisters adds the second register used on busy nights.
func (h *history) loadRegisters(ctx context.Context) error {
	if _, err := h.tx.Exec(ctx, `INSERT INTO registers (name, invoice_series) VALUES ('Caja 2', 'F002') ON CONFLICT DO NOTHING`); err != nil {
		return err
	}
	rows, err := h.tx.Query(ctx, `SELECT id, invoice_series, next_invoice_number FROM registers WHERE active ORDER BY name LIMIT 2`)
	if err != nil {
		return err
	}
	h.registers, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (*register, error) {
		r := &register{}
		return r, row.Scan(&r.id, &r.series, &r.next)
	})
	return err
}

// addCashiers creates staff with an unknown temporary password; an administrator resets it to use them.
func (h *history) addCashiers(ctx context.Context) error {
	for _, c := range demoCashiers {
		temporary, err := password.GenerateTemporary()
		if err != nil {
			return err
		}
		hash, err := password.Hash(temporary)
		if err != nil {
			return err
		}
		var id uuid.UUID
		err = h.tx.QueryRow(ctx, `
			INSERT INTO users (full_name, email, username, password_hash, role_id, activated_at)
			SELECT $1, $2, $3, $4, id, now() - interval '90 days' FROM roles WHERE code = 'cashier' RETURNING id`,
			c.name, c.username+"@example.com", c.username, hash).Scan(&id)
		if err != nil {
			return err
		}
		h.cashiers = append(h.cashiers, id)
	}
	return nil
}

func (h *history) run(ctx context.Context) error {
	start := h.today.AddDate(0, 0, -historyDays)
	for n := 0; n < historyDays; n++ {
		day := start.AddDate(0, 0, n)
		rate := int64(baseRate + n*37)
		if err := h.addRate(ctx, day, rate); err != nil {
			return err
		}
		if n == 0 || day.Weekday() == time.Monday {
			if err := h.restock(ctx, day, historyDays-n <= 28); err != nil {
				return err
			}
		}
		if err := h.runDay(ctx, day, n, rate); err != nil {
			return fmt.Errorf("day %s: %w", day.Format(time.DateOnly), err)
		}
	}
	if err := h.addRate(ctx, h.today, baseRate+historyDays*37); err != nil {
		return err
	}
	for _, r := range h.registers {
		if _, err := h.tx.Exec(ctx, `UPDATE registers SET next_invoice_number = $2 WHERE id = $1`, r.id, r.next); err != nil {
			return err
		}
	}
	return nil
}

func (h *history) addRate(ctx context.Context, day time.Time, micros int64) error {
	_, err := h.tx.Exec(ctx, `
		INSERT INTO exchange_rates (currency_code, rate, effective_date, created_by, created_at) VALUES ('USD', $1::numeric, $2, $3, $4)`,
		rateText(micros), day.Format(time.DateOnly), h.admin, day.Add(8*time.Hour))
	return err
}

func rateText(micros int64) string { return fmt.Sprintf("%d.%06d", micros/1_000_000, micros%1_000_000) }

// supplierOf names who distributes a product: local brands first, then by category.
func supplierOf(it *stockItem) string {
	switch it.brand {
	case "Flor de Caña", "Ron Plata":
		return "Compañía Licorera de Nicaragua"
	case "Hielo Polar":
		return "Hielo Polar S.A."
	}
	switch it.category {
	case "Cerveza":
		return "Compañía Cervecera de Nicaragua"
	case "Vino":
		return "Distribuidora Vinos del Sur"
	case "Mezcladores", "Snacks y desechables":
		return "Embotelladora Nacional (Coca-Cola)"
	}
	return "Casa Pellas Importaciones"
}

// restock receives one purchase per supplier. Near the end some products are left out,
// so the demo opens with real reorder alerts.
func (h *history) restock(ctx context.Context, day time.Time, leaveGaps bool) error {
	bySupplier := map[string][]*stockItem{}
	for _, it := range h.items {
		if it.stock*10 >= it.target*6 || (leaveGaps && stableHash(it.brand+it.category)%3 == 0) {
			continue
		}
		name := supplierOf(it)
		bySupplier[name] = append(bySupplier[name], it)
	}
	at := day.Add(8*time.Hour + 30*time.Minute)
	for _, sp := range h.suppliers {
		items := bySupplier[sp.name]
		if len(items) == 0 {
			continue
		}
		var subtotal, tax int64
		for _, it := range items {
			line := it.costCents * int64(it.target-it.stock)
			subtotal += line
			tax += (line*it.taxHundredths + 5000) / 10000
		}
		h.result.Purchases++
		invoice := fmt.Sprintf("FP-%05d", 10000+h.result.Purchases*7)
		var id uuid.UUID
		err := h.tx.QueryRow(ctx, `
			INSERT INTO purchases (supplier_id, supplier_invoice_number, status, purchase_date, subtotal, tax, total,
			                       created_by, received_by, received_at, created_at)
			VALUES ($1, $2, 'received', $3, $4::numeric, $5::numeric, $6::numeric, $7, $7, $8, $8) RETURNING id`,
			sp.id, invoice, day.Format(time.DateOnly), cents(subtotal), cents(tax), cents(subtotal+tax), h.admin, at).Scan(&id)
		if err != nil {
			return err
		}
		reason := fmt.Sprintf("Compra a %s, factura %s", sp.name, invoice)
		for _, it := range items {
			quantity := it.target - it.stock
			if _, err := h.tx.Exec(ctx, `
				INSERT INTO purchase_items (purchase_id, product_id, quantity, unit_cost, subtotal)
				VALUES ($1, $2, $3, $4::numeric, $5::numeric)`, id, it.id, quantity, cents(it.costCents), cents(it.costCents*int64(quantity))); err != nil {
				return err
			}
			if err := h.move(ctx, it, "purchase", quantity, reason, "purchase", id, at); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *history) move(ctx context.Context, it *stockItem, kind string, quantity int, reason, source string, sourceID uuid.UUID, at time.Time) error {
	it.stock += quantity
	_, err := h.tx.Exec(ctx, `
		INSERT INTO inventory_movements (product_id, type, quantity, unit_cost, reason, source_type, source_id, created_by, created_at)
		VALUES ($1, $2, $3, $4::numeric, $5, $6, $7, $8, $9)`,
		it.id, kind, quantity, cents(it.costCents), reason, source, sourceID, h.admin, at)
	return err
}

func isPeak(day time.Time) bool {
	last := day.AddDate(0, 1, -day.Day()).Day()
	return day.Weekday() == time.Friday || day.Weekday() == time.Saturday || day.Day() == 15 || day.Day() == last
}

func (h *history) salesOn(day time.Time) int {
	base := map[time.Weekday]int{time.Sunday: 22, time.Monday: 16, time.Tuesday: 17, time.Wednesday: 19, time.Thursday: 24, time.Friday: 42, time.Saturday: 52}[day.Weekday()]
	if d := day.Day(); d == 15 || d == 16 || d == 30 || d == 31 || d == 1 {
		base = base * 13 / 10
	}
	return base + h.rng.IntN(7) - 3
}

// saleTimes spreads the day's sales over opening hours, in order.
func (h *history) saleTimes(day time.Time) []time.Time {
	var total float64
	for _, w := range hourDemand {
		total += w
	}
	moments := make([]time.Time, 0, 64)
	for range h.salesOn(day) {
		pick := h.rng.Float64() * total
		for hour := 10; hour <= 21; hour++ {
			pick -= hourDemand[hour]
			if pick <= 0 || hour == 21 {
				moments = append(moments, day.Add(time.Duration(hour)*time.Hour+time.Duration(h.rng.IntN(3600))*time.Second))
				break
			}
		}
	}
	slices.SortFunc(moments, func(a, b time.Time) int { return a.Compare(b) })
	return moments
}

func (h *history) openShift(ctx context.Context, r *register, user uuid.UUID, opened, closed time.Time) (*shift, error) {
	s := &shift{register: r, user: user, closedAt: closed}
	err := h.tx.QueryRow(ctx, `
		INSERT INTO cash_shifts (register_id, user_id, status, opened_at, closed_at, opening_nio, opening_usd)
		VALUES ($1, $2, 'closed', $3, $4, $5::numeric, $6::numeric) RETURNING id`,
		r.id, user, opened, closed, cents(openingNIO), cents(openingUSD)).Scan(&s.id)
	if err != nil {
		return nil, err
	}
	h.result.Shifts++
	if h.rng.IntN(6) == 0 {
		s.expense = int64(150+h.rng.IntN(45)*10) * 100
		_, err = h.tx.Exec(ctx, `
			INSERT INTO cash_movements (shift_id, type, currency_code, amount, reason, created_by, created_at)
			VALUES ($1, 'expense', 'NIO', $2::numeric, $3, $4, $5)`,
			s.id, cents(s.expense), expenses[h.rng.IntN(len(expenses))], user, opened.Add(2*time.Hour))
	}
	return s, err
}

// closeShift stores the count; about one shift in ten ends with a difference.
func (h *history) closeShift(ctx context.Context, s *shift) error {
	expectedNIO := openingNIO + s.cashNIO - s.expense
	expectedUSD := openingUSD + s.cashUSD
	var diffNIO, diffUSD int64
	notes := ""
	if h.rng.IntN(10) == 0 {
		diffNIO = -int64(10+h.rng.IntN(12)*10) * 100
		notes = shortages[h.rng.IntN(len(shortages))]
	}
	_, err := h.tx.Exec(ctx, `
		UPDATE cash_shifts SET expected_nio = $2::numeric, expected_usd = $3::numeric, counted_nio = $4::numeric, counted_usd = $5::numeric,
		       difference_nio = $4::numeric - $2::numeric, difference_usd = $5::numeric - $3::numeric, notes = $6
		WHERE id = $1`, s.id, cents(expectedNIO), cents(expectedUSD), cents(expectedNIO+diffNIO), cents(expectedUSD+diffUSD), notes)
	return err
}

func (h *history) runDay(ctx context.Context, day time.Time, n int, rate int64) error {
	first, second := h.cashiers[n%2], h.cashiers[(n+1)%2]
	morning, err := h.openShift(ctx, h.registers[0], first, day.Add(9*time.Hour+45*time.Minute), day.Add(16*time.Hour))
	if err != nil {
		return err
	}
	evening, err := h.openShift(ctx, h.registers[0], second, day.Add(16*time.Hour), day.Add(22*time.Hour+15*time.Minute))
	if err != nil {
		return err
	}
	shifts := []*shift{morning, evening}
	var extra *shift
	if isPeak(day) && len(h.registers) > 1 {
		if extra, err = h.openShift(ctx, h.registers[1], h.cashiers[2], day.Add(16*time.Hour+45*time.Minute), day.Add(22*time.Hour+15*time.Minute)); err != nil {
			return err
		}
		shifts = append(shifts, extra)
	}
	for _, at := range h.saleTimes(day) {
		s := morning
		if at.Hour() >= 16 {
			s = evening
			if extra != nil && at.After(day.Add(17*time.Hour)) && h.rng.IntN(2) == 0 {
				s = extra
			}
		}
		if err := h.addSale(ctx, s, at, rate); err != nil {
			return err
		}
	}
	if err := h.applyVoids(ctx, day.AddDate(0, 0, 1)); err != nil {
		return err
	}
	for _, s := range shifts {
		if err := h.closeShift(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

func (h *history) pickItem() *stockItem {
	pick := h.rng.Float64() * h.cumulative[len(h.cumulative)-1]
	return h.items[min(sort.SearchFloat64s(h.cumulative, pick), len(h.items)-1)]
}

type soldUnits struct {
	item  *stockItem
	units int
}

// pickLines fills a basket with what is on the shelf right now.
func (h *history) pickLines() ([]sales.LineInput, []soldUnits) {
	count := []int{1, 1, 1, 2, 2, 2, 3, 3, 4, 5}[h.rng.IntN(10)]
	var lines []sales.LineInput
	var sold []soldUnits
	seen := map[*stockItem]bool{}
	for range count {
		it := h.pickItem()
		if seen[it] {
			continue
		}
		seen[it] = true
		line := sales.LineInput{ProductID: it.id, Quantity: 1 + h.rng.IntN(2)}
		units := line.Quantity
		if it.category == "Cerveza" {
			line.Quantity = 1 + h.rng.IntN(6)
			units = line.Quantity
			if len(it.packs) > 0 && h.rng.IntN(4) == 0 {
				p := it.packs[0]
				if h.rng.IntN(6) == 0 {
					p = it.packs[len(it.packs)-1]
				}
				line.PresentationID, line.Quantity, units = &p.id, 1, p.factor
			}
		}
		if units > it.stock {
			continue
		}
		it.stock -= units
		lines = append(lines, line)
		sold = append(sold, soldUnits{it, units})
	}
	return lines, sold
}

type pendingVoid struct {
	id      uuid.UUID
	invoice string
	sold    []soldUnits
	at      time.Time
}

func (h *history) addSale(ctx context.Context, s *shift, at time.Time, rate int64) error {
	if err := h.applyVoids(ctx, at); err != nil {
		return err
	}
	lines, sold := h.pickLines()
	if len(lines) == 0 {
		return nil
	}
	past := sales.Past{ShiftID: s.id, RegisterID: s.register.id, UserID: s.user, Series: s.register.series, Number: s.register.next,
		Lines: lines, Rate: rateText(rate), At: at, Pay: func(total int64) []sales.PaymentInput { return h.payments(total, rate) }}
	if h.rng.IntN(5) == 0 {
		past.CustomerID = &h.customers[h.rng.IntN(len(h.customers))]
	}
	if h.rng.IntN(16) == 0 {
		past.Discount = &sales.Discount{Kind: "percent", Value: "5"}
		if h.rng.IntN(3) == 0 {
			past.Discount.Value = "15"
			past.AuthorizedBy = &h.admin
		}
	}
	result, err := sales.RecordPast(ctx, h.tx, past)
	if err != nil {
		return err
	}
	s.register.next++
	h.result.Sales++

	invoice := fmt.Sprintf("%s-%06d", past.Series, past.Number)
	for _, u := range sold {
		u.item.stock += u.units // move applies the change again
		if err := h.move(ctx, u.item, "sale", -u.units, "Venta "+invoice, "sale", result.ID, at); err != nil {
			return err
		}
	}
	voidAt := at.Add(time.Duration(2+h.rng.IntN(10)) * time.Minute)
	if h.rng.IntN(120) == 0 && voidAt.Before(s.closedAt) {
		h.voids = append(h.voids, pendingVoid{result.ID, invoice, sold, voidAt})
		return nil
	}
	s.cashNIO += result.CashNIO - result.Change
	s.cashUSD += result.CashUSD
	return nil
}

// applyVoids runs the voids due by the given moment, keeping the kardex in date order.
func (h *history) applyVoids(ctx context.Context, until time.Time) error {
	var later []pendingVoid
	for _, v := range h.voids {
		if v.at.After(until) {
			later = append(later, v)
			continue
		}
		reason := voidReasons[h.rng.IntN(len(voidReasons))]
		_, err := h.tx.Exec(ctx, `UPDATE sales SET status = 'voided', voided_at = $2, voided_by = $3, void_reason = $4 WHERE id = $1`, v.id, v.at, h.admin, reason)
		if err != nil {
			return err
		}
		for _, u := range v.sold {
			if err := h.move(ctx, u.item, "sale_void", u.units, "Anulación de "+v.invoice+": "+reason, "sale", v.id, v.at); err != nil {
				return err
			}
		}
	}
	h.voids = later
	return nil
}

// payments mixes methods the way a Nicaraguan counter does: mostly córdobas, some dollars.
func (h *history) payments(total, rate int64) []sales.PaymentInput {
	nio := func(amount int64) sales.PaymentInput {
		return sales.PaymentInput{Method: "cash", Currency: "NIO", Amount: cents(amount)}
	}
	usd := func(dollars int64) sales.PaymentInput {
		return sales.PaymentInput{Method: "cash", Currency: "USD", Amount: cents(dollars * 100)}
	}
	roundUp := func(v int64) int64 {
		if h.rng.IntN(3) == 0 {
			return v
		}
		return (v + 4999) / 5000 * 5000
	}
	switch pick := h.rng.IntN(100); {
	case pick < 55:
		return []sales.PaymentInput{nio(roundUp(total))}
	case pick < 67:
		return []sales.PaymentInput{usd((total*10000 + rate - 1) / rate)}
	case pick < 75:
		dollars := []int64{5, 10, 20}[h.rng.IntN(3)]
		covered := dollars * rate / 10000
		if covered >= total {
			return []sales.PaymentInput{usd((total*10000 + rate - 1) / rate)}
		}
		return []sales.PaymentInput{usd(dollars), nio(roundUp(total - covered))}
	case pick < 93:
		ref := fmt.Sprintf("%06d", h.rng.IntN(1_000_000))
		return []sales.PaymentInput{{Method: "card", Currency: "NIO", Amount: cents(total), Reference: &ref}}
	default:
		ref := fmt.Sprintf("%08d", h.rng.IntN(100_000_000))
		return []sales.PaymentInput{{Method: "transfer", Currency: "NIO", Amount: cents(total), Reference: &ref}}
	}
}
