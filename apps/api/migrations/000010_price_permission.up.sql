-- Changing prices and costs, and seeing costs, is separate from editing products:
-- warehouse staff edit products but not prices, and cashiers never see costs.
INSERT INTO permissions (module, action, description)
VALUES ('products', 'authorize', 'Ver costos y cambiar precios');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.code = 'admin' AND p.module = 'products' AND p.action = 'authorize';
