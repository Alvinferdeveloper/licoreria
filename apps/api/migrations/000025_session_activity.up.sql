-- Last request seen on each session, to end sessions left idle (security.idleTimeoutMinutes).
ALTER TABLE refresh_sessions ADD COLUMN last_seen_at timestamptz NOT NULL DEFAULT now();
