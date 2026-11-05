DROP INDEX IF EXISTS products_name_unaccent_trgm_idx;
CREATE INDEX products_name_trgm_idx ON products USING gin (name gin_trgm_ops);
DROP FUNCTION IF EXISTS immutable_unaccent(text);
