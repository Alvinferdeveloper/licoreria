package reports

import (
	"context"
	"fmt"

	"licoreria/api/internal/platform/database"
)

var (
	from     = Param{Key: "from", Label: "Desde", Kind: ParamDate}
	to       = Param{Key: "to", Label: "Hasta", Kind: ParamDate}
	cashier  = Param{Key: "userId", Label: "Cajero", Kind: ParamUser}
	register = Param{Key: "registerId", Label: "Caja", Kind: ParamRegister}
	category = Param{Key: "categoryId", Label: "Categoría", Kind: ParamCategory}
	brand    = Param{Key: "brandId", Label: "Marca", Kind: ParamBrand}
	supplier = Param{Key: "supplierId", Label: "Proveedor", Kind: ParamSupplier}
	method   = Param{Key: "method", Label: "Método de pago", Kind: ParamChoice, Choices: []Choice{
		{"cash", "Efectivo"}, {"card", "Tarjeta"}, {"transfer", "Transferencia"}}}
	currency = Param{Key: "currency", Label: "Moneda", Kind: ParamChoice, Choices: []Choice{{"NIO", "Córdobas"}, {"USD", "Dólares"}}}
)

// limitSQL fetches one extra row so the caller can tell the result was cut.
func limitSQL(limit int) string { return fmt.Sprintf(" LIMIT %d", limit+1) }

// salesScope applies the cashier restriction and the common sale filters.
func salesScope(w *where, p Params) {
	w.add("s.status = ?", "completed")
	w.dateRange("s.created_at", p)
	if p.Self != nil {
		w.add("s.user_id = ?", *p.Self)
	} else {
		w.addIf("s.user_id = ?::uuid", p.Get("userId"))
	}
}

const (
	labelSales     = "Ventas"
	labelInventory = "Inventario y compras"
	labelControl   = "Control"
)

var definitions = []Definition{
	{
		Key: "sales-detail", Title: "Ventas por período", Group: labelSales,
		Description: "Cada factura con sus líneas. Filtra por fechas, cajero y método de pago.",
		Params:      []Param{from, to, cashier, method},
		GroupBy:     "invoice",
		Columns: []Column{
			{Key: "invoice", Label: "Factura", Kind: ColMono, Width: 3},
			{Key: "created_at", Label: "Fecha", Kind: ColDateTime, Width: 3},
			{Key: "customer", Label: "Cliente", Kind: ColText, Width: 4},
			{Key: "cashier", Label: "Cajero", Kind: ColText, Width: 3},
			{Key: "product", Label: "Producto", Kind: ColText, Width: 6},
			{Key: "quantity", Label: "Cant.", Kind: ColNumber, Width: 2, Total: true},
			{Key: "discount", Label: "Descuento", Kind: ColMoney, Width: 3, Total: true},
			{Key: "tax", Label: "IVA", Kind: ColMoney, Width: 3, Total: true},
			{Key: "total", Label: "Total", Kind: ColMoney, Width: 3, Total: true},
		},
		query: func(ctx context.Context, db database.DBTX, p Params, limit int) ([]Row, error) {
			var w where
			salesScope(&w, p)
			w.addIf("EXISTS (SELECT 1 FROM sale_payments sp WHERE sp.sale_id = s.id AND sp.method = ?)", p.Get("method"))
			return collect(ctx, db, `
				SELECT s.invoice_series || '-' || lpad(s.invoice_number::text, 6, '0') AS invoice, s.created_at,
				       s.customer_name AS customer, u.full_name AS cashier, i.product_name AS product,
				       i.quantity * i.units_per_item AS quantity, i.discount::text AS discount, i.tax::text AS tax, i.total::text AS total
				FROM sales s JOIN users u ON u.id = s.user_id JOIN sale_items i ON i.sale_id = s.id
				WHERE `+w.sql()+` ORDER BY s.created_at DESC, i.product_name`+limitSQL(limit), w.args...)
		},
	},
	{
		Key: "sales-by-product", Title: "Ventas por producto y categoría", Group: labelSales,
		Description: "Unidades e ingresos por producto en el período, sin IVA y con descuentos aplicados.",
		Params:      []Param{from, to, category, brand},
		Columns: []Column{
			{Key: "product", Label: "Producto", Kind: ColText, Width: 7},
			{Key: "sku", Label: "SKU", Kind: ColMono, Width: 3},
			{Key: "category", Label: "Categoría", Kind: ColText, Width: 3},
			{Key: "units", Label: "Unidades", Kind: ColNumber, Width: 2, Total: true},
			{Key: "net", Label: "Venta neta", Kind: ColMoney, Width: 3, Total: true},
			{Key: "tax", Label: "IVA", Kind: ColMoney, Width: 3, Total: true},
			{Key: "total", Label: "Total", Kind: ColMoney, Width: 3, Total: true},
		},
		query: func(ctx context.Context, db database.DBTX, p Params, limit int) ([]Row, error) {
			var w where
			salesScope(&w, p)
			w.addIf("pr.category_id = ?::uuid", p.Get("categoryId"))
			w.addIf("pr.brand_id = ?::uuid", p.Get("brandId"))
			return collect(ctx, db, `
				SELECT pr.name AS product, pr.sku::text AS sku, c.name AS category,
				       sum(i.quantity * i.units_per_item)::int AS units, sum(i.subtotal - i.discount)::text AS net,
				       sum(i.tax)::text AS tax, sum(i.total)::text AS total
				FROM sales s JOIN sale_items i ON i.sale_id = s.id
				JOIN products pr ON pr.id = i.product_id JOIN categories c ON c.id = pr.category_id
				WHERE `+w.sql()+` GROUP BY pr.id, c.name ORDER BY sum(i.total) DESC`+limitSQL(limit), w.args...)
		},
	},
	{
		Key: "shifts", Title: "Ventas por cajero y cierres de caja", Group: labelSales,
		Description: "Cada turno con sus ventas y el arqueo: esperado, contado y diferencia por moneda.",
		Params:      []Param{from, to, cashier, register},
		Columns: []Column{
			{Key: "register", Label: "Caja", Kind: ColText, Width: 2},
			{Key: "cashier", Label: "Cajero", Kind: ColText, Width: 4},
			{Key: "opened_at", Label: "Apertura", Kind: ColDateTime, Width: 3},
			{Key: "closed_at", Label: "Cierre", Kind: ColDateTime, Width: 3},
			{Key: "sales", Label: "Ventas", Kind: ColNumber, Width: 2, Total: true},
			{Key: "total", Label: "Total vendido", Kind: ColMoney, Width: 3, Total: true},
			{Key: "expected_nio", Label: "Esperado C$", Kind: ColMoney, Width: 3},
			{Key: "counted_nio", Label: "Contado C$", Kind: ColMoney, Width: 3},
			{Key: "difference_nio", Label: "Dif. C$", Kind: ColMoney, Width: 3, Total: true},
			{Key: "difference_usd", Label: "Dif. US$", Kind: ColMoney, Width: 3, Total: true},
		},
		query: func(ctx context.Context, db database.DBTX, p Params, limit int) ([]Row, error) {
			var w where
			w.dateRange("cs.opened_at", p)
			if p.Self != nil {
				w.add("cs.user_id = ?", *p.Self)
			} else {
				w.addIf("cs.user_id = ?::uuid", p.Get("userId"))
			}
			w.addIf("cs.register_id = ?::uuid", p.Get("registerId"))
			return collect(ctx, db, `
				SELECT r.name AS register, u.full_name AS cashier, cs.opened_at, cs.closed_at,
				       (SELECT count(*)::int FROM sales s WHERE s.shift_id = cs.id AND s.status = 'completed') AS sales,
				       (SELECT COALESCE(sum(total), 0)::text FROM sales s WHERE s.shift_id = cs.id AND s.status = 'completed') AS total,
				       cs.expected_nio::text AS expected_nio, cs.counted_nio::text AS counted_nio,
				       cs.difference_nio::text AS difference_nio, cs.difference_usd::text AS difference_usd
				FROM cash_shifts cs JOIN registers r ON r.id = cs.register_id JOIN users u ON u.id = cs.user_id
				WHERE `+w.sql()+` ORDER BY cs.opened_at DESC`+limitSQL(limit), w.args...)
		},
	},
	{
		Key: "payments", Title: "Ventas por caja y moneda", Group: labelSales,
		Description: "Conciliación de cobros por caja, método y moneda. El vuelto entregado se resta para cuadrar con lo cobrado.",
		Params:      []Param{from, to, register, currency},
		Columns: []Column{
			{Key: "register", Label: "Caja", Kind: ColText, Width: 3},
			{Key: "method", Label: "Método", Kind: ColText, Width: 3},
			{Key: "currency", Label: "Moneda", Kind: ColMono, Width: 2},
			{Key: "payments", Label: "Cobros", Kind: ColNumber, Width: 2, Total: true},
			{Key: "amount", Label: "Monto recibido", Kind: ColMoney, Width: 3},
			{Key: "amount_nio", Label: "Equivalente C$", Kind: ColMoney, Width: 3, Total: true},
		},
		query: func(ctx context.Context, db database.DBTX, p Params, limit int) ([]Row, error) {
			var w where
			salesScope(&w, p)
			w.addIf("s.register_id = ?::uuid", p.Get("registerId"))
			// Change is always handed back in córdobas, so it only applies without a USD filter.
			change := "false"
			if p.Get("currency") != "USD" {
				change = "s.change_nio > 0"
			}
			sales := w.sql()
			w.addIf("sp.currency_code = ?", p.Get("currency"))
			return collect(ctx, db, `
				SELECT register, method, currency, payments, amount, amount_nio FROM (
					SELECT r.name AS register, sp.method AS sort,
					       CASE sp.method WHEN 'cash' THEN 'Efectivo' WHEN 'card' THEN 'Tarjeta' ELSE 'Transferencia' END AS method,
					       sp.currency_code::text AS currency, count(*)::int AS payments,
					       sum(sp.amount)::text AS amount, sum(sp.amount_nio)::text AS amount_nio
					FROM sales s JOIN sale_payments sp ON sp.sale_id = s.id JOIN registers r ON r.id = s.register_id
					WHERE `+w.sql()+` GROUP BY r.name, sp.method, sp.currency_code
					UNION ALL
					SELECT r.name, 'zz', 'Vuelto entregado', 'NIO', count(*)::int, (-sum(s.change_nio))::text, (-sum(s.change_nio))::text
					FROM sales s JOIN registers r ON r.id = s.register_id
					WHERE `+sales+` AND `+change+` GROUP BY r.name
				) t ORDER BY register, sort, currency`+limitSQL(limit), w.args...)
		},
	},
	{
		Key: "voided", Title: "Ventas anuladas", Group: labelSales,
		Description: "Facturas anuladas con su motivo y quién autorizó la anulación.",
		Params:      []Param{from, to, cashier},
		Columns: []Column{
			{Key: "voided_at", Label: "Anulada", Kind: ColDateTime, Width: 3},
			{Key: "invoice", Label: "Factura", Kind: ColMono, Width: 3},
			{Key: "cashier", Label: "Cajero", Kind: ColText, Width: 3},
			{Key: "voided_by", Label: "Autorizó", Kind: ColText, Width: 3},
			{Key: "reason", Label: "Motivo", Kind: ColText, Width: 6},
			{Key: "total", Label: "Total", Kind: ColMoney, Width: 3, Total: true},
		},
		query: func(ctx context.Context, db database.DBTX, p Params, limit int) ([]Row, error) {
			var w where
			w.add("s.status = ?", "voided")
			w.dateRange("s.voided_at", p)
			if p.Self != nil {
				w.add("s.user_id = ?", *p.Self)
			} else {
				w.addIf("s.user_id = ?::uuid", p.Get("userId"))
			}
			return collect(ctx, db, `
				SELECT s.voided_at, s.invoice_series || '-' || lpad(s.invoice_number::text, 6, '0') AS invoice,
				       u.full_name AS cashier, v.full_name AS voided_by, s.void_reason AS reason, s.total::text AS total
				FROM sales s JOIN users u ON u.id = s.user_id JOIN users v ON v.id = s.voided_by
				WHERE `+w.sql()+` ORDER BY s.voided_at DESC`+limitSQL(limit), w.args...)
		},
	},
	{
		Key: "profit", Title: "Utilidad y margen por producto", Group: labelSales, Permission: "products:authorize",
		Description: "Venta neta contra el costo promedio al momento de vender. Solo para quien ve costos.",
		Params:      []Param{from, to, category},
		Columns: []Column{
			{Key: "product", Label: "Producto", Kind: ColText, Width: 7},
			{Key: "category", Label: "Categoría", Kind: ColText, Width: 3},
			{Key: "units", Label: "Unidades", Kind: ColNumber, Width: 2, Total: true},
			{Key: "net", Label: "Venta neta", Kind: ColMoney, Width: 3, Total: true},
			{Key: "cost", Label: "Costo", Kind: ColMoney, Width: 3, Total: true},
			{Key: "profit", Label: "Utilidad", Kind: ColMoney, Width: 3, Total: true},
			{Key: "margin", Label: "Margen", Kind: ColPercent, Width: 2},
		},
		query: func(ctx context.Context, db database.DBTX, p Params, limit int) ([]Row, error) {
			var w where
			salesScope(&w, p)
			w.addIf("pr.category_id = ?::uuid", p.Get("categoryId"))
			return collect(ctx, db, `
				SELECT product, category, units, net::text, cost::text, (net - cost)::text AS profit,
				       CASE WHEN net > 0 THEN round((net - cost) * 100 / net, 1)::text END AS margin
				FROM (SELECT pr.name AS product, c.name AS category, sum(i.quantity * i.units_per_item)::int AS units,
				             sum(i.subtotal - i.discount) AS net, round(sum(i.quantity * i.units_per_item * i.unit_cost), 2) AS cost
				      FROM sales s JOIN sale_items i ON i.sale_id = s.id
				      JOIN products pr ON pr.id = i.product_id JOIN categories c ON c.id = pr.category_id
				      WHERE `+w.sql()+` GROUP BY pr.id, c.name) t
				ORDER BY net - cost DESC`+limitSQL(limit), w.args...)
		},
	},
	{
		Key: "valuation", Title: "Inventario valorizado", Group: labelInventory, Permission: "products:authorize",
		Description: "Existencias actuales a costo promedio y a precio de venta.",
		Params:      []Param{category, {Key: "inStock", Label: "Solo con stock", Kind: ParamBool}},
		Columns: []Column{
			{Key: "product", Label: "Producto", Kind: ColText, Width: 7},
			{Key: "sku", Label: "SKU", Kind: ColMono, Width: 3},
			{Key: "category", Label: "Categoría", Kind: ColText, Width: 3},
			{Key: "stock", Label: "Stock", Kind: ColNumber, Width: 2, Total: true},
			{Key: "average_cost", Label: "Costo prom.", Kind: ColMoney, Width: 3},
			{Key: "cost_value", Label: "Valor a costo", Kind: ColMoney, Width: 3, Total: true},
			{Key: "sale_value", Label: "Valor a precio", Kind: ColMoney, Width: 3, Total: true},
		},
		query: func(ctx context.Context, db database.DBTX, p Params, limit int) ([]Row, error) {
			var w where
			w.add("p.active = ?", true)
			w.addIf("p.category_id = ?::uuid", p.Get("categoryId"))
			if p.Get("inStock") == "true" {
				w.add("p.stock > ?", 0)
			}
			return collect(ctx, db, `
				SELECT p.name AS product, p.sku::text AS sku, c.name AS category, p.stock,
				       round(p.average_cost, 2)::text AS average_cost,
				       round(GREATEST(p.stock, 0) * p.average_cost, 2)::text AS cost_value,
				       round(GREATEST(p.stock, 0) * p.price, 2)::text AS sale_value
				FROM products p JOIN categories c ON c.id = p.category_id
				WHERE `+w.sql()+` ORDER BY GREATEST(p.stock, 0) * p.average_cost DESC`+limitSQL(limit), w.args...)
		},
	},
	{
		Key: "low-stock", Title: "Productos bajo mínimo", Group: labelInventory,
		Description: "Lista de compra agrupada por el proveedor de la última compra recibida.",
		Params:      []Param{supplier, category},
		GroupBy:     "supplier",
		Columns: []Column{
			{Key: "supplier", Label: "Proveedor", Kind: ColText, Width: 4},
			{Key: "product", Label: "Producto", Kind: ColText, Width: 7},
			{Key: "sku", Label: "SKU", Kind: ColMono, Width: 3},
			{Key: "stock", Label: "Stock", Kind: ColNumber, Width: 2},
			{Key: "min_stock", Label: "Mínimo", Kind: ColNumber, Width: 2},
			{Key: "reorder_point", Label: "Reorden", Kind: ColNumber, Width: 2},
			{Key: "suggested", Label: "Sugerido", Kind: ColNumber, Width: 2, Total: true},
		},
		query: func(ctx context.Context, db database.DBTX, p Params, limit int) ([]Row, error) {
			var w where
			w.add("p.active = ?", true)
			w.conds = append(w.conds, "p.stock <= GREATEST(p.min_stock, p.reorder_point)")
			w.addIf("p.category_id = ?::uuid", p.Get("categoryId"))
			w.addIf("last.supplier_id = ?::uuid", p.Get("supplierId"))
			return collect(ctx, db, `
				SELECT COALESCE(last.supplier_name, 'Sin compras previas') AS supplier, p.name AS product, p.sku::text AS sku,
				       p.stock, p.min_stock, p.reorder_point,
				       GREATEST(GREATEST(p.reorder_point, p.min_stock * 2) - p.stock, 1) AS suggested
				FROM products p
				LEFT JOIN LATERAL (
					SELECT s.id AS supplier_id, s.name AS supplier_name FROM purchase_items pi
					JOIN purchases pu ON pu.id = pi.purchase_id AND pu.status = 'received'
					JOIN suppliers s ON s.id = pu.supplier_id
					WHERE pi.product_id = p.id ORDER BY pu.received_at DESC LIMIT 1) last ON true
				WHERE `+w.sql()+` ORDER BY last.supplier_name NULLS LAST, p.name`+limitSQL(limit), w.args...)
		},
	},
	{
		Key: "kardex", Title: "Kardex por producto", Group: labelInventory,
		Description: "Entradas, salidas y ajustes de un producto con el saldo después de cada movimiento.",
		Params:      []Param{{Key: "productId", Label: "Producto", Kind: ParamProduct, Required: true}, from, to},
		GroupBy:     "product",
		Columns: []Column{
			{Key: "product", Label: "Producto", Kind: ColText, Width: 5},
			{Key: "created_at", Label: "Fecha", Kind: ColDateTime, Width: 3},
			{Key: "type", Label: "Movimiento", Kind: ColText, Width: 3},
			{Key: "quantity", Label: "Cantidad", Kind: ColNumber, Width: 2},
			{Key: "stock_after", Label: "Saldo", Kind: ColNumber, Width: 2},
			{Key: "reason", Label: "Detalle", Kind: ColText, Width: 6},
			{Key: "user", Label: "Usuario", Kind: ColText, Width: 3},
		},
		query: func(ctx context.Context, db database.DBTX, p Params, limit int) ([]Row, error) {
			var w where
			w.add("m.product_id = ?::uuid", p.Get("productId"))
			w.dateRange("m.created_at", p)
			return collect(ctx, db, `
				SELECT p.name AS product, m.created_at,
				       CASE m.type WHEN 'purchase' THEN 'Compra' WHEN 'sale' THEN 'Venta' WHEN 'sale_void' THEN 'Venta anulada'
				            WHEN 'customer_return' THEN 'Devolución de cliente' WHEN 'supplier_return' THEN 'Devolución a proveedor'
				            WHEN 'count_adjustment' THEN 'Conteo físico' ELSE 'Merma' END AS type,
				       m.quantity, m.stock_after, m.reason, u.full_name AS user
				FROM inventory_movements m JOIN products p ON p.id = m.product_id JOIN users u ON u.id = m.created_by
				WHERE `+w.sql()+` ORDER BY m.created_at, m.id`+limitSQL(limit), w.args...)
		},
	},
	{
		Key: "purchases", Title: "Compras por proveedor", Group: labelInventory,
		Description: "Compras no anuladas agrupadas por proveedor, con su estado y totales.",
		Params:      []Param{from, to, supplier},
		GroupBy:     "supplier",
		Columns: []Column{
			{Key: "supplier", Label: "Proveedor", Kind: ColText, Width: 5},
			{Key: "purchase_date", Label: "Fecha", Kind: ColDate, Width: 2},
			{Key: "invoice", Label: "Factura", Kind: ColMono, Width: 3},
			{Key: "status", Label: "Estado", Kind: ColText, Width: 2},
			{Key: "items", Label: "Líneas", Kind: ColNumber, Width: 2},
			{Key: "subtotal", Label: "Subtotal", Kind: ColMoney, Width: 3, Total: true},
			{Key: "tax", Label: "IVA", Kind: ColMoney, Width: 3, Total: true},
			{Key: "total", Label: "Total", Kind: ColMoney, Width: 3, Total: true},
		},
		query: func(ctx context.Context, db database.DBTX, p Params, limit int) ([]Row, error) {
			var w where
			w.add("pu.status <> ?", "voided")
			w.addIf("pu.purchase_date >= ?::date", p.Get("from"))
			w.addIf("pu.purchase_date <= ?::date", p.Get("to"))
			w.addIf("pu.supplier_id = ?::uuid", p.Get("supplierId"))
			return collect(ctx, db, `
				SELECT s.name AS supplier, pu.purchase_date::text AS purchase_date, NULLIF(pu.supplier_invoice_number, '') AS invoice,
				       CASE pu.status WHEN 'draft' THEN 'Borrador' ELSE 'Recibida' END AS status,
				       (SELECT count(*)::int FROM purchase_items pi WHERE pi.purchase_id = pu.id) AS items,
				       pu.subtotal::text AS subtotal, pu.tax::text AS tax, pu.total::text AS total
				FROM purchases pu JOIN suppliers s ON s.id = pu.supplier_id
				WHERE `+w.sql()+` ORDER BY s.name, pu.purchase_date DESC`+limitSQL(limit), w.args...)
		},
	},
	{
		Key: "audit", Title: "Bitácora de auditoría", Group: labelControl, Permission: "audit:view",
		Description: "Quién hizo qué y cuándo: inicios de sesión, cambios de precio, anulaciones, ajustes y permisos.",
		Params:      []Param{from, to, {Key: "userId", Label: "Usuario", Kind: ParamUser}, {Key: "action", Label: "Acción contiene", Kind: ParamText}},
		Columns: []Column{
			{Key: "created_at", Label: "Fecha", Kind: ColDateTime, Width: 3},
			{Key: "user", Label: "Usuario", Kind: ColText, Width: 4},
			{Key: "action", Label: "Acción", Kind: ColMono, Width: 5},
			{Key: "entity", Label: "Entidad", Kind: ColText, Width: 3},
			{Key: "ip", Label: "IP", Kind: ColMono, Width: 3},
		},
		query: func(ctx context.Context, db database.DBTX, p Params, limit int) ([]Row, error) {
			var w where
			w.dateRange("a.created_at", p)
			w.addIf("a.user_id = ?::uuid", p.Get("userId"))
			w.addIf("a.action ILIKE '%' || ? || '%'", p.Get("action"))
			return collect(ctx, db, `
				SELECT a.created_at, COALESCE(u.full_name, 'Sistema') AS user, a.action, a.entity, host(a.ip) AS ip
				FROM audit_log a LEFT JOIN users u ON u.id = a.user_id
				WHERE `+w.sql()+` ORDER BY a.created_at DESC`+limitSQL(limit), w.args...)
		},
	},
}
