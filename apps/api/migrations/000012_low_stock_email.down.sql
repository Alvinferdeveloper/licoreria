DELETE FROM email_queue WHERE template_key = 'low_stock';
DELETE FROM email_templates WHERE key = 'low_stock';
