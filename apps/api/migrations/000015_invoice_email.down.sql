DELETE FROM email_queue WHERE template_key = 'invoice';
DELETE FROM email_templates WHERE key = 'invoice';
