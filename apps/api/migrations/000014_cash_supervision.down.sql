DELETE FROM email_queue WHERE template_key = 'cash_difference';
DELETE FROM email_templates WHERE key = 'cash_difference';
DELETE FROM permissions WHERE module = 'cash_shifts' AND action = 'authorize';
