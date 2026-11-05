package server

import (
	"context"
	"testing"

	"licoreria/api/internal/seed"
)

const clearDemo = `
	TRUNCATE inventory_movements, purchases, sales, cash_shifts, exchange_rates, products, categories, brands, suppliers CASCADE;
	DELETE FROM customers WHERE NOT is_walk_in;
	DELETE FROM registers WHERE invoice_series <> 'F001';
	UPDATE registers SET next_invoice_number = 1;`

// The demo history must read like real operation: exact kardex, stock never negative, and
// shift counts that match what the sales say.
func TestDemoHistoryIsConsistent(t *testing.T) {
	env := newTestEnv(t)
	env.createAdmin()
	ctx := context.Background()
	if _, err := env.pool.Exec(ctx, clearDemo); err != nil {
		t.Fatalf("clear: %v", err)
	}
	t.Cleanup(func() {
		if _, err := env.pool.Exec(ctx, clearDemo); err != nil {
			t.Errorf("clear: %v", err)
		}
	})

	result, err := seed.Demo(ctx, env.pool)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if result.Sales < 1500 || result.Purchases == 0 {
		t.Fatalf("history too small: %+v", result)
	}

	checks := map[string]string{
		"stock differs from kardex": `
			SELECT count(*) FROM products p
			WHERE p.stock <> (SELECT COALESCE(sum(quantity), 0) FROM inventory_movements m WHERE m.product_id = p.id)`,
		"negative balance in kardex": `SELECT count(*) FROM inventory_movements WHERE stock_after < 0`,
		"kardex out of date order": `
			SELECT count(*) FROM (
				SELECT stock_after, quantity, lag(stock_after, 1, 0) OVER (PARTITION BY product_id ORDER BY created_at, stock_after) AS previous,
				       count(*) OVER (PARTITION BY product_id, created_at) AS same_moment
				FROM inventory_movements) k
			WHERE same_moment = 1 AND stock_after <> previous + quantity`,
		"shift expected differs from sales": `
			SELECT count(*) FROM cash_shifts s
			WHERE s.expected_nio <> s.opening_nio
			  + COALESCE((SELECT sum(p.amount) FROM sale_payments p JOIN sales v ON v.id = p.sale_id
			              WHERE v.shift_id = s.id AND v.status = 'completed' AND p.method = 'cash' AND p.currency_code = 'NIO'), 0)
			  - COALESCE((SELECT sum(change_nio) FROM sales v WHERE v.shift_id = s.id AND v.status = 'completed'), 0)
			  - COALESCE((SELECT sum(amount) FROM cash_movements m WHERE m.shift_id = s.id AND m.type = 'expense'), 0)`,
		"sale outside its shift": `
			SELECT count(*) FROM sales v JOIN cash_shifts s ON s.id = v.shift_id
			WHERE v.created_at < s.opened_at OR v.created_at > s.closed_at`,
		"invoice counter behind": `
			SELECT count(*) FROM registers r
			WHERE r.next_invoice_number <= (SELECT COALESCE(max(invoice_number), 0) FROM sales v WHERE v.register_id = r.id)`,
	}
	for name, query := range checks {
		var bad int
		if err := env.pool.QueryRow(ctx, query).Scan(&bad); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if bad != 0 {
			t.Errorf("%s: %d rows", name, bad)
		}
	}

	var alerts int
	if err := env.pool.QueryRow(ctx, `SELECT count(*) FROM products WHERE stock <= GREATEST(min_stock, reorder_point)`).Scan(&alerts); err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v, %d alerts", result, alerts)
	if alerts == 0 {
		t.Error("the demo should open with reorder alerts")
	}
}
