CREATE TABLE categories (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name       text NOT NULL CHECK (length(trim(name)) > 0),
  parent_id  uuid REFERENCES categories (id) ON DELETE RESTRICT,
  active     boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX categories_name_uq ON categories (lower(name), coalesce(parent_id, '00000000-0000-0000-0000-000000000000'));

CREATE TABLE brands (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name       citext NOT NULL UNIQUE CHECK (length(trim(name)) > 0),
  active     boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE units (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name         citext NOT NULL UNIQUE,
  abbreviation citext NOT NULL UNIQUE
);

CREATE TABLE products (
  id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  sku                  citext NOT NULL UNIQUE,
  barcode              text UNIQUE,
  name                 text NOT NULL CHECK (length(trim(name)) > 0),
  description          text NOT NULL DEFAULT '',
  category_id          uuid NOT NULL REFERENCES categories (id) ON DELETE RESTRICT,
  brand_id             uuid REFERENCES brands (id) ON DELETE RESTRICT,
  unit_id              uuid NOT NULL REFERENCES units (id) ON DELETE RESTRICT,
  alcohol_percent      numeric(4, 1) CHECK (alcohol_percent BETWEEN 0 AND 100),
  volume_ml            integer CHECK (volume_ml > 0),
  image_url            text,
  price                numeric(14, 2) NOT NULL CHECK (price >= 0),
  average_cost         numeric(14, 4) NOT NULL DEFAULT 0 CHECK (average_cost >= 0),
  tax_percent          numeric(5, 2) NOT NULL DEFAULT 15 CHECK (tax_percent BETWEEN 0 AND 100),
  stock                integer NOT NULL DEFAULT 0,
  min_stock            integer NOT NULL DEFAULT 0 CHECK (min_stock >= 0),
  reorder_point        integer NOT NULL DEFAULT 0 CHECK (reorder_point >= 0),
  allow_negative_stock boolean NOT NULL DEFAULT false,
  active               boolean NOT NULL DEFAULT true,
  created_at           timestamptz NOT NULL DEFAULT now(),
  updated_at           timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT products_stock_non_negative CHECK (allow_negative_stock OR stock >= 0)
);
CREATE INDEX products_name_trgm_idx ON products USING gin (name gin_trgm_ops);
CREATE INDEX products_category_idx ON products (category_id) WHERE active;

CREATE TABLE product_presentations (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  product_id uuid NOT NULL REFERENCES products (id) ON DELETE CASCADE,
  name       text NOT NULL,
  factor     integer NOT NULL CHECK (factor > 1),
  barcode    text UNIQUE,
  price      numeric(14, 2) NOT NULL CHECK (price >= 0),
  active     boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (product_id, name)
);

CREATE TRIGGER categories_updated_at BEFORE UPDATE ON categories FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER brands_updated_at BEFORE UPDATE ON brands FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER products_updated_at BEFORE UPDATE ON products FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER product_presentations_updated_at BEFORE UPDATE ON product_presentations FOR EACH ROW EXECUTE FUNCTION set_updated_at();
