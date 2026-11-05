DELETE FROM email_queue WHERE template_key IN ('welcome', 'password_reset');
DELETE FROM email_templates WHERE key IN ('welcome', 'password_reset');
