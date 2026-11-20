DELETE FROM settings WHERE key = 'schedule';
DELETE FROM permissions WHERE module = 'schedule' AND action = 'authorize';
DROP TABLE schedule_extensions;
