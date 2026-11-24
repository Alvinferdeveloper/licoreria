INSERT INTO permissions (module, action, description) VALUES ('system', 'view', 'Ver el estado del sistema');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r JOIN permissions p ON p.module = 'system' AND p.action = 'view'
WHERE r.code = 'admin';
