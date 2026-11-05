CREATE EXTENSION IF NOT EXISTS unaccent;

-- unaccent() is not IMMUTABLE, so it can't back an index; this wrapper pins the dictionary.
CREATE FUNCTION immutable_unaccent(text) RETURNS text
  LANGUAGE sql IMMUTABLE PARALLEL SAFE STRICT
  AS $$ SELECT public.unaccent('public.unaccent'::regdictionary, $1) $$;

CREATE INDEX products_name_unaccent_trgm_idx ON products USING gin (immutable_unaccent(lower(name)) gin_trgm_ops);
DROP INDEX IF EXISTS products_name_trgm_idx;
