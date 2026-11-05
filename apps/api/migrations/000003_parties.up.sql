CREATE TABLE customers (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  full_name  text NOT NULL CHECK (length(trim(full_name)) > 0),
  tax_id     citext UNIQUE,
  phone      text,
  email      citext,
  birth_date date CHECK (birth_date > '1900-01-01'),
  address    text NOT NULL DEFAULT '',
  is_walk_in boolean NOT NULL DEFAULT false,
  active     boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX customers_single_walk_in_uq ON customers (is_walk_in) WHERE is_walk_in;
CREATE INDEX customers_name_trgm_idx ON customers USING gin (full_name gin_trgm_ops);

CREATE TABLE suppliers (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name         text NOT NULL CHECK (length(trim(name)) > 0),
  tax_id       citext UNIQUE,
  contact_name text NOT NULL DEFAULT '',
  phone        text,
  email        citext,
  address      text NOT NULL DEFAULT '',
  credit_days  integer NOT NULL DEFAULT 0 CHECK (credit_days >= 0),
  active       boolean NOT NULL DEFAULT true,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER customers_updated_at BEFORE UPDATE ON customers FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER suppliers_updated_at BEFORE UPDATE ON suppliers FOR EACH ROW EXECUTE FUNCTION set_updated_at();
