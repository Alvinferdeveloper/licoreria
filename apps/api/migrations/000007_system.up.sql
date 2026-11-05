CREATE TABLE settings (
  key        text PRIMARY KEY,
  value      jsonb NOT NULL,
  updated_by uuid REFERENCES users (id) ON DELETE SET NULL,
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE email_templates (
  key        text PRIMARY KEY,
  subject    text NOT NULL,
  html_body  text NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE email_queue (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  template_key text REFERENCES email_templates (key) ON DELETE SET NULL,
  to_address   citext NOT NULL,
  subject      text NOT NULL,
  html_body    text NOT NULL,
  status       text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'failed')),
  attempts     integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  last_error   text,
  scheduled_at timestamptz NOT NULL DEFAULT now(),
  sent_at      timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX email_queue_pending_idx ON email_queue (scheduled_at) WHERE status = 'pending';

CREATE TABLE backups (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  file_name       text NOT NULL UNIQUE,
  size_bytes      bigint CHECK (size_bytes >= 0),
  checksum_sha256 char(64),
  kind            text NOT NULL CHECK (kind IN ('manual', 'scheduled', 'pre_restore')),
  status          text NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'completed', 'failed')),
  error           text,
  created_by      uuid REFERENCES users (id) ON DELETE SET NULL,
  created_at      timestamptz NOT NULL DEFAULT now(),
  completed_at    timestamptz
);
CREATE INDEX backups_created_idx ON backups (created_at DESC);

CREATE TRIGGER settings_updated_at BEFORE UPDATE ON settings FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER email_templates_updated_at BEFORE UPDATE ON email_templates FOR EACH ROW EXECUTE FUNCTION set_updated_at();
