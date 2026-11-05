CREATE TABLE sales (
  id                     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  invoice_series         text NOT NULL,
  invoice_number         integer NOT NULL CHECK (invoice_number > 0),
  shift_id               uuid NOT NULL REFERENCES cash_shifts (id) ON DELETE RESTRICT,
  register_id            uuid NOT NULL REFERENCES registers (id) ON DELETE RESTRICT,
  user_id                uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  customer_id            uuid NOT NULL REFERENCES customers (id) ON DELETE RESTRICT,
  customer_name          text NOT NULL,
  customer_tax_id        text,
  subtotal               numeric(14, 2) NOT NULL CHECK (subtotal >= 0),
  discount               numeric(14, 2) NOT NULL DEFAULT 0 CHECK (discount >= 0 AND discount <= subtotal),
  tax                    numeric(14, 2) NOT NULL CHECK (tax >= 0),
  total                  numeric(14, 2) NOT NULL CHECK (total >= 0),
  exchange_rate          numeric(12, 6) NOT NULL CHECK (exchange_rate > 0),
  change_nio             numeric(14, 2) NOT NULL DEFAULT 0 CHECK (change_nio >= 0),
  change_usd             numeric(14, 2) NOT NULL DEFAULT 0 CHECK (change_usd >= 0),
  age_verified           boolean NOT NULL CHECK (age_verified),
  discount_authorized_by uuid REFERENCES users (id) ON DELETE RESTRICT,
  status                 text NOT NULL DEFAULT 'completed' CHECK (status IN ('completed', 'voided')),
  voided_at              timestamptz,
  voided_by              uuid REFERENCES users (id) ON DELETE RESTRICT,
  void_reason            text,
  idempotency_key        uuid NOT NULL UNIQUE,
  notes                  text NOT NULL DEFAULT '',
  created_at             timestamptz NOT NULL DEFAULT now(),
  UNIQUE (invoice_series, invoice_number),
  CHECK (total = subtotal - discount + tax),
  CHECK ((status = 'voided') = (voided_at IS NOT NULL AND voided_by IS NOT NULL AND void_reason IS NOT NULL))
);
CREATE INDEX sales_created_idx ON sales (created_at DESC);
CREATE INDEX sales_shift_idx ON sales (shift_id);
CREATE INDEX sales_customer_idx ON sales (customer_id, created_at DESC);
CREATE INDEX sales_user_idx ON sales (user_id, created_at DESC);

CREATE TABLE sale_items (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  sale_id         uuid NOT NULL REFERENCES sales (id) ON DELETE CASCADE,
  product_id      uuid NOT NULL REFERENCES products (id) ON DELETE RESTRICT,
  presentation_id uuid REFERENCES product_presentations (id) ON DELETE RESTRICT,
  product_name    text NOT NULL,
  sku             text NOT NULL,
  quantity        integer NOT NULL CHECK (quantity > 0),
  units_per_item  integer NOT NULL DEFAULT 1 CHECK (units_per_item > 0),
  unit_price      numeric(14, 2) NOT NULL CHECK (unit_price >= 0),
  unit_cost       numeric(14, 4) NOT NULL DEFAULT 0 CHECK (unit_cost >= 0),
  discount        numeric(14, 2) NOT NULL DEFAULT 0 CHECK (discount >= 0),
  tax_percent     numeric(5, 2) NOT NULL CHECK (tax_percent BETWEEN 0 AND 100),
  tax             numeric(14, 2) NOT NULL CHECK (tax >= 0),
  subtotal        numeric(14, 2) NOT NULL CHECK (subtotal >= 0),
  total           numeric(14, 2) NOT NULL CHECK (total >= 0)
);
CREATE INDEX sale_items_sale_idx ON sale_items (sale_id);
CREATE INDEX sale_items_product_idx ON sale_items (product_id);

CREATE TABLE sale_payments (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  sale_id       uuid NOT NULL REFERENCES sales (id) ON DELETE CASCADE,
  method        text NOT NULL CHECK (method IN ('cash', 'card', 'transfer')),
  currency_code char(3) NOT NULL REFERENCES currencies (code) ON DELETE RESTRICT,
  amount        numeric(14, 2) NOT NULL CHECK (amount > 0),
  exchange_rate numeric(12, 6) NOT NULL CHECK (exchange_rate > 0),
  amount_nio    numeric(14, 2) NOT NULL CHECK (amount_nio > 0),
  reference     text,
  created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sale_payments_sale_idx ON sale_payments (sale_id);

CREATE TABLE returns (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  sale_id       uuid NOT NULL REFERENCES sales (id) ON DELETE RESTRICT,
  shift_id      uuid NOT NULL REFERENCES cash_shifts (id) ON DELETE RESTRICT,
  reason        text NOT NULL CHECK (length(trim(reason)) > 0),
  refund_total  numeric(14, 2) NOT NULL CHECK (refund_total >= 0),
  authorized_by uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  created_by    uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX returns_sale_idx ON returns (sale_id);

CREATE TABLE return_items (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  return_id    uuid NOT NULL REFERENCES returns (id) ON DELETE CASCADE,
  sale_item_id uuid NOT NULL REFERENCES sale_items (id) ON DELETE RESTRICT,
  quantity     integer NOT NULL CHECK (quantity > 0),
  restock      boolean NOT NULL,
  amount       numeric(14, 2) NOT NULL CHECK (amount >= 0)
);
CREATE INDEX return_items_return_idx ON return_items (return_id);
