CREATE TABLE schedule_extensions (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    uuid REFERENCES users (id),
  ends_at    timestamptz NOT NULL,
  created_by uuid NOT NULL REFERENCES users (id),
  created_at timestamptz NOT NULL DEFAULT now()
);

COMMENT ON COLUMN schedule_extensions.user_id IS 'NULL extends the schedule for everyone.';

CREATE INDEX schedule_extensions_ends_at_idx ON schedule_extensions (ends_at);

INSERT INTO permissions (module, action, description) VALUES
  ('schedule', 'authorize', 'Entrar y vender fuera del horario de uso');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.code = 'admin' AND p.module = 'schedule' AND p.action = 'authorize';

INSERT INTO settings (key, value) VALUES ('schedule', '{
  "enabled": false,
  "toleranceMinutes": 30,
  "days": [
    {"closed": false, "open": "10:00", "close": "20:00"},
    {"closed": false, "open": "08:00", "close": "22:00"},
    {"closed": false, "open": "08:00", "close": "22:00"},
    {"closed": false, "open": "08:00", "close": "22:00"},
    {"closed": false, "open": "08:00", "close": "22:00"},
    {"closed": false, "open": "08:00", "close": "23:00"},
    {"closed": false, "open": "08:00", "close": "23:00"}
  ]
}')
ON CONFLICT (key) DO NOTHING;
