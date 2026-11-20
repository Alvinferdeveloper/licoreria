-- When the POS lost its connection, the moment the sale really happened; created_at is when it synced.
ALTER TABLE sales ADD COLUMN offline_at timestamptz;

-- An offline sale already happened, so it is recorded even past the stock on file. The
-- ledger (inventory.Ledger.Apply), the only writer of stock, keeps refusing negative stock
-- everywhere else.
ALTER TABLE products DROP CONSTRAINT products_stock_non_negative;
