DROP TRIGGER audit_log_append_only ON audit_log;
DROP FUNCTION reject_audit_change();

ALTER TABLE backups
  DROP COLUMN remote_key,
  DROP COLUMN verified_at,
  DROP COLUMN verification;
