UPDATE products SET allow_negative_stock = true WHERE stock < 0;
ALTER TABLE products ADD CONSTRAINT products_stock_non_negative CHECK (allow_negative_stock OR stock >= 0);
ALTER TABLE sales DROP COLUMN offline_at;
