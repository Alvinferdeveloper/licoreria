ALTER TABLE purchases
  DROP CONSTRAINT purchases_voided_check,
  DROP CONSTRAINT purchases_received_check,
  DROP COLUMN voided_at,
  DROP COLUMN voided_by,
  DROP COLUMN void_reason;
UPDATE purchases SET received_at = NULL WHERE status <> 'received';
ALTER TABLE purchases ADD CONSTRAINT purchases_check CHECK ((status = 'received') = (received_at IS NOT NULL));
