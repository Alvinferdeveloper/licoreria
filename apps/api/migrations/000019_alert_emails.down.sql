DELETE FROM email_queue WHERE template_key = 'alert';
DELETE FROM email_templates WHERE key = 'alert';
