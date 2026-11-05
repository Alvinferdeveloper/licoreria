CREATE TABLE currencies (
  code    char(3) PRIMARY KEY CHECK (code = upper(code)),
  symbol  text NOT NULL,
  name    text NOT NULL,
  is_base boolean NOT NULL DEFAULT false
);
CREATE UNIQUE INDEX currencies_single_base_uq ON currencies (is_base) WHERE is_base;

-- NIO per one unit of the foreign currency, one rate per currency per day.
CREATE TABLE exchange_rates (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  currency_code  char(3) NOT NULL REFERENCES currencies (code) ON DELETE RESTRICT,
  rate           numeric(12, 6) NOT NULL CHECK (rate > 0),
  effective_date date NOT NULL,
  created_by     uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  created_at     timestamptz NOT NULL DEFAULT now(),
  UNIQUE (currency_code, effective_date)
);

CREATE TABLE registers (
  id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name                text NOT NULL UNIQUE,
  invoice_series      text NOT NULL UNIQUE CHECK (invoice_series ~ '^[A-Z0-9]{2,8}$'),
  next_invoice_number integer NOT NULL DEFAULT 1 CHECK (next_invoice_number > 0),
  active              boolean NOT NULL DEFAULT true,
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE cash_shifts (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  register_id    uuid NOT NULL REFERENCES registers (id) ON DELETE RESTRICT,
  user_id        uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  status         text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'closed')),
  opened_at      timestamptz NOT NULL DEFAULT now(),
  closed_at      timestamptz,
  opening_nio    numeric(14, 2) NOT NULL DEFAULT 0 CHECK (opening_nio >= 0),
  opening_usd    numeric(14, 2) NOT NULL DEFAULT 0 CHECK (opening_usd >= 0),
  expected_nio   numeric(14, 2),
  expected_usd   numeric(14, 2),
  counted_nio    numeric(14, 2) CHECK (counted_nio >= 0),
  counted_usd    numeric(14, 2) CHECK (counted_usd >= 0),
  difference_nio numeric(14, 2),
  difference_usd numeric(14, 2),
  notes          text NOT NULL DEFAULT '',
  CHECK ((status = 'open') = (closed_at IS NULL))
);
CREATE UNIQUE INDEX cash_shifts_open_register_uq ON cash_shifts (register_id) WHERE status = 'open';
CREATE UNIQUE INDEX cash_shifts_open_user_uq ON cash_shifts (user_id) WHERE status = 'open';
CREATE INDEX cash_shifts_opened_idx ON cash_shifts (opened_at DESC);

CREATE TABLE cash_movements (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  shift_id      uuid NOT NULL REFERENCES cash_shifts (id) ON DELETE RESTRICT,
  type          text NOT NULL CHECK (type IN ('income', 'expense')),
  currency_code char(3) NOT NULL REFERENCES currencies (code) ON DELETE RESTRICT,
  amount        numeric(14, 2) NOT NULL CHECK (amount > 0),
  reason        text NOT NULL CHECK (length(trim(reason)) > 0),
  created_by    uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX cash_movements_shift_idx ON cash_movements (shift_id);

CREATE TRIGGER registers_updated_at BEFORE UPDATE ON registers FOR EACH ROW EXECUTE FUNCTION set_updated_at();
