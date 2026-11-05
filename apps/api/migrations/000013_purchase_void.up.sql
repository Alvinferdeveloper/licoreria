-- A voided purchase keeps when it was received; voiding is tracked in its own columns.
ALTER TABLE purchases DROP CONSTRAINT purchases_check;
ALTER TABLE purchases
  ADD COLUMN void_reason text NOT NULL DEFAULT '',
  ADD COLUMN voided_by   uuid REFERENCES users (id) ON DELETE RESTRICT,
  ADD COLUMN voided_at   timestamptz,
  ADD CONSTRAINT purchases_received_check CHECK (status <> 'received' OR received_at IS NOT NULL),
  ADD CONSTRAINT purchases_voided_check CHECK ((status = 'voided') = (voided_at IS NOT NULL));
