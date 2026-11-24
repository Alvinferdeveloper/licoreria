-- Optional best-before date of what arrived in this line; expiry_alerted_at avoids repeating the email.
ALTER TABLE purchase_items
  ADD COLUMN expires_on date,
  ADD COLUMN expiry_alerted_at timestamptz;

CREATE INDEX purchase_items_expiry_idx ON purchase_items (product_id, expires_on) WHERE expires_on IS NOT NULL;
