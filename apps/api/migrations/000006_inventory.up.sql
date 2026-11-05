CREATE TABLE purchases (
  id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  supplier_id             uuid NOT NULL REFERENCES suppliers (id) ON DELETE RESTRICT,
  supplier_invoice_number text NOT NULL DEFAULT '',
  status                  text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'received', 'voided')),
  purchase_date           date NOT NULL DEFAULT current_date,
  subtotal                numeric(14, 2) NOT NULL DEFAULT 0 CHECK (subtotal >= 0),
  tax                     numeric(14, 2) NOT NULL DEFAULT 0 CHECK (tax >= 0),
  total                   numeric(14, 2) NOT NULL DEFAULT 0 CHECK (total >= 0),
  notes                   text NOT NULL DEFAULT '',
  created_by              uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  received_by             uuid REFERENCES users (id) ON DELETE RESTRICT,
  received_at             timestamptz,
  created_at              timestamptz NOT NULL DEFAULT now(),
  updated_at              timestamptz NOT NULL DEFAULT now(),
  CHECK ((status = 'received') = (received_at IS NOT NULL))
);
CREATE INDEX purchases_supplier_idx ON purchases (supplier_id, purchase_date DESC);
CREATE INDEX purchases_date_idx ON purchases (purchase_date DESC);

CREATE TABLE purchase_items (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  purchase_id uuid NOT NULL REFERENCES purchases (id) ON DELETE CASCADE,
  product_id  uuid NOT NULL REFERENCES products (id) ON DELETE RESTRICT,
  quantity    integer NOT NULL CHECK (quantity > 0),
  unit_cost   numeric(14, 4) NOT NULL CHECK (unit_cost >= 0),
  subtotal    numeric(14, 2) NOT NULL CHECK (subtotal >= 0),
  UNIQUE (purchase_id, product_id)
);

-- Stock ledger (kardex): signed quantities in base units; products.stock is derived from it.
CREATE TABLE inventory_movements (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  product_id  uuid NOT NULL REFERENCES products (id) ON DELETE RESTRICT,
  type        text NOT NULL CHECK (type IN ('purchase', 'sale', 'sale_void', 'customer_return', 'supplier_return', 'count_adjustment', 'shrinkage')),
  quantity    integer NOT NULL CHECK (quantity <> 0),
  unit_cost   numeric(14, 4) NOT NULL DEFAULT 0 CHECK (unit_cost >= 0),
  stock_after integer NOT NULL,
  reason      text NOT NULL DEFAULT '',
  source_type text,
  source_id   uuid,
  created_by  uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  created_at  timestamptz NOT NULL DEFAULT now(),
  CHECK (type NOT IN ('count_adjustment', 'shrinkage') OR length(trim(reason)) > 0)
);
CREATE INDEX inventory_movements_product_idx ON inventory_movements (product_id, created_at DESC);
CREATE INDEX inventory_movements_source_idx ON inventory_movements (source_type, source_id);

-- Applies each movement to products.stock and records the resulting balance.
CREATE FUNCTION apply_inventory_movement() RETURNS trigger AS $$
BEGIN
  UPDATE products SET stock = stock + NEW.quantity
  WHERE id = NEW.product_id
  RETURNING stock INTO NEW.stock_after;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- The ledger is append-only: corrections are new movements.
CREATE FUNCTION reject_ledger_change() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'inventory_movements is append-only' USING ERRCODE = 'restrict_violation';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER inventory_movements_apply BEFORE INSERT ON inventory_movements FOR EACH ROW EXECUTE FUNCTION apply_inventory_movement();
CREATE TRIGGER inventory_movements_immutable BEFORE UPDATE OR DELETE ON inventory_movements FOR EACH ROW EXECUTE FUNCTION reject_ledger_change();
CREATE TRIGGER purchases_updated_at BEFORE UPDATE ON purchases FOR EACH ROW EXECUTE FUNCTION set_updated_at();
