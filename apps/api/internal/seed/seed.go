// Package seed loads realistic demo data: catalog with stock, customers and suppliers.
package seed

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"licoreria/api/internal/platform/database"
	"licoreria/api/internal/platform/textx"
	"licoreria/api/internal/settings"
)

var demoBusiness = settings.Business{Name: "Bodega Añejo", TaxID: "J0310000123456",
	Address: "Del Colegio La Salle 2 cuadras al sur, Managua", Phone: "2250 1234", Email: "ventas@example.com"}

var ErrNoAdmin = errors.New("create an administrator first (api create-admin)")

type Result struct {
	Products  int
	Customers int
	Suppliers int
	Sales     int
	Shifts    int
	Purchases int
	Skipped   bool
}

// Demo loads the catalog and 90 days of operation once; it does nothing if products already exist.
func Demo(ctx context.Context, pool *pgxpool.Pool) (Result, error) {
	var existing int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM products`).Scan(&existing); err != nil {
		return Result{}, err
	}
	if existing > 0 {
		return Result{Skipped: true}, nil
	}

	var adminID uuid.UUID
	err := pool.QueryRow(ctx, `
		SELECT u.id FROM users u JOIN roles r ON r.id = u.role_id WHERE r.code = 'admin' ORDER BY u.created_at LIMIT 1`).Scan(&adminID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Result{}, ErrNoAdmin
	}
	if err != nil {
		return Result{}, err
	}

	var result Result
	err = database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		s := &seeder{tx: tx, admin: adminID, categories: map[string]uuid.UUID{}, brands: map[string]uuid.UUID{}, units: map[string]uuid.UUID{}}
		if err := s.loadUnits(ctx); err != nil {
			return err
		}
		for _, l := range demoLines() {
			n, err := s.addLine(ctx, l)
			if err != nil {
				return fmt.Errorf("seed %s: %w", l.name, err)
			}
			result.Products += n
		}
		for _, c := range demoCustomers {
			if _, err := tx.Exec(ctx, `INSERT INTO customers (full_name, tax_id, phone, email) VALUES ($1, $2, $3, NULLIF($4, ''))`,
				c.name, c.taxID, c.phone, c.email); err != nil {
				return err
			}
			result.Customers++
		}
		for _, sp := range demoSuppliers {
			if _, err := tx.Exec(ctx, `
				INSERT INTO suppliers (name, tax_id, contact_name, phone, email, credit_days) VALUES ($1, $2, $3, $4, $5, $6)`,
				sp.name, sp.taxID, sp.contact, sp.phone, sp.email, sp.credit); err != nil {
				return err
			}
			result.Suppliers++
		}
		if err := settings.Set(ctx, tx, "business", demoBusiness, adminID); err != nil {
			return err
		}
		return loadHistory(ctx, tx, adminID, &result)
	})
	return result, err
}

// Reset removes the catalog, every operation and the audit log so the demo can be loaded again.
// Users, roles, registers, settings and backups stay.
func Reset(ctx context.Context, pool *pgxpool.Pool) error {
	return database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			TRUNCATE inventory_movements, purchases, returns, sales, cash_shifts, exchange_rates,
			         products, categories, brands, suppliers, audit_log, email_queue CASCADE`); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM customers WHERE NOT is_walk_in`); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE registers SET next_invoice_number = 1`)
		return err
	})
}

type seeder struct {
	tx         pgx.Tx
	admin      uuid.UUID
	categories map[string]uuid.UUID
	brands     map[string]uuid.UUID
	units      map[string]uuid.UUID
	sequence   int
}

func (s *seeder) loadUnits(ctx context.Context) error {
	rows, err := s.tx.Query(ctx, `SELECT abbreviation, id FROM units`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var abbr string
		var id uuid.UUID
		if err := rows.Scan(&abbr, &id); err != nil {
			return err
		}
		s.units[abbr] = id
	}
	return rows.Err()
}

func (s *seeder) idFor(ctx context.Context, cache map[string]uuid.UUID, table, name string) (uuid.UUID, error) {
	if id, ok := cache[name]; ok {
		return id, nil
	}
	var id uuid.UUID
	if err := s.tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s (name) VALUES ($1) RETURNING id`, table), name).Scan(&id); err != nil {
		return uuid.Nil, err
	}
	cache[name] = id
	return id, nil
}

// addLine inserts one product per size, with initial stock recorded as a ledger movement.
func (s *seeder) addLine(ctx context.Context, l line) (int, error) {
	categoryID, err := s.idFor(ctx, s.categories, "categories", l.category)
	if err != nil {
		return 0, err
	}
	brandID, err := s.idFor(ctx, s.brands, "brands", l.brand)
	if err != nil {
		return 0, err
	}

	for _, sz := range l.sizes {
		name := l.name
		if !strings.Contains(l.name, sz.label) {
			name = l.name + " " + sz.label
		}
		price := round2(l.price * sz.scale)
		cost := round2(price * 0.71)
		stock, minStock := stockFor(name, l.category)

		var alcohol any
		if l.alcohol != "" {
			alcohol = l.alcohol
		}
		var volume any
		if sz.ml > 0 {
			volume = sz.ml
		}

		var productID uuid.UUID
		err := s.tx.QueryRow(ctx, `
			INSERT INTO products (sku, barcode, name, category_id, brand_id, unit_id, alcohol_percent, volume_ml,
			                      price, average_cost, min_stock, reorder_point)
			VALUES ($1, $2, $3, $4, $5, $6, $7::numeric, $8, $9, $10, $11, $12) RETURNING id`,
			sku(l.brand, name, s.nextSequence()), s.barcode(), name, categoryID, brandID, s.units[l.unit],
			alcohol, volume, price, cost, minStock, minStock*2).Scan(&productID)
		if err != nil {
			return 0, err
		}

		if l.pack {
			for _, pack := range []struct {
				name   string
				factor int
				off    float64
			}{{"Six-pack", 6, 0.97}, {"Caja 24", 24, 0.94}} {
				if _, err := s.tx.Exec(ctx, `
					INSERT INTO product_presentations (product_id, name, factor, barcode, price) VALUES ($1, $2, $3, $4, $5)`,
					productID, pack.name, pack.factor, s.barcode(), round2(price*float64(pack.factor)*pack.off)); err != nil {
					return 0, err
				}
			}
		}

		if _, err := s.tx.Exec(ctx, `
			INSERT INTO inventory_movements (product_id, type, quantity, unit_cost, reason, created_by, created_at)
			VALUES ($1, 'count_adjustment', $2, $3, 'Inventario inicial', $4, $5)`,
			productID, stock, cost, s.admin, storeToday().AddDate(0, 0, -historyDays-1)); err != nil {
			return 0, err
		}
	}
	return len(l.sizes), nil
}

func (s *seeder) nextSequence() int {
	s.sequence++
	return s.sequence
}

// barcode returns a valid EAN-13 with the Central American 743 prefix.
func (s *seeder) barcode() string {
	s.sequence++
	digits := fmt.Sprintf("743%09d", 100000000+s.sequence*7919)
	sum := 0
	for i, r := range digits {
		d := int(r - '0')
		if i%2 == 1 {
			d *= 3
		}
		sum += d
	}
	return digits + fmt.Sprint((10-sum%10)%10)
}

// stockFor gives each product a stable pseudo-random stock; a few start below the minimum.
func stockFor(name, category string) (stock, minStock int) {
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	n := int(h.Sum32())
	minStock = map[string]int{"Cerveza": 48, "Mezcladores": 24, "Snacks y desechables": 12}[category]
	if minStock == 0 {
		minStock = 4
	}
	stock = minStock + n%(minStock*6)
	if n%9 == 0 {
		stock = n % (minStock + 1)
	}
	return max(stock, 1), minStock
}

func sku(brand, name string, seq int) string {
	code := func(s string, n int) string {
		var out []rune
		for _, r := range strings.ToUpper(textx.StripAccents(s)) {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				out = append(out, r)
			}
			if len(out) == n {
				break
			}
		}
		return string(out)
	}
	return fmt.Sprintf("%s-%s-%03d", code(brand, 3), code(strings.TrimPrefix(name, brand), 3), seq)
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
