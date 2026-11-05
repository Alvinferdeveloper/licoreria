INSERT INTO currencies (code, symbol, name, is_base) VALUES
  ('NIO', 'C$', 'Córdoba', true),
  ('USD', 'US$', 'Dólar estadounidense', false);

INSERT INTO roles (code, name, description, is_system) VALUES
  ('admin', 'Administrador', 'Acceso total al sistema.', true),
  ('cashier', 'Cajero', 'Vende, cobra y gestiona su propia caja.', true),
  ('warehouse', 'Bodeguero', 'Recibe compras y ajusta inventario.', true),
  ('viewer', 'Consulta', 'Solo lectura de reportes y datos.', true);

INSERT INTO permissions (module, action, description) VALUES
  ('pos', 'view', 'Abrir el punto de venta'),
  ('pos', 'create', 'Registrar ventas'),
  ('sales', 'view', 'Ver ventas y facturas'),
  ('sales', 'void', 'Anular ventas y registrar devoluciones'),
  ('discounts', 'authorize', 'Autorizar descuentos sobre el límite'),
  ('cash_shifts', 'view', 'Ver turnos de caja'),
  ('cash_shifts', 'create', 'Abrir caja'),
  ('cash_shifts', 'update', 'Registrar movimientos y cerrar caja'),
  ('exchange_rates', 'view', 'Ver tasa de cambio'),
  ('exchange_rates', 'create', 'Registrar tasa de cambio'),
  ('products', 'view', 'Ver productos'),
  ('products', 'create', 'Crear productos'),
  ('products', 'update', 'Editar productos'),
  ('products', 'delete', 'Desactivar productos'),
  ('categories', 'view', 'Ver categorías y marcas'),
  ('categories', 'create', 'Crear categorías y marcas'),
  ('categories', 'update', 'Editar categorías y marcas'),
  ('categories', 'delete', 'Desactivar categorías y marcas'),
  ('purchases', 'view', 'Ver compras'),
  ('purchases', 'create', 'Registrar compras'),
  ('purchases', 'update', 'Editar y recibir compras'),
  ('purchases', 'void', 'Anular compras'),
  ('inventory', 'view', 'Ver kardex y existencias'),
  ('inventory', 'create', 'Registrar ajustes de inventario'),
  ('customers', 'view', 'Ver clientes'),
  ('customers', 'create', 'Crear clientes'),
  ('customers', 'update', 'Editar clientes'),
  ('customers', 'delete', 'Desactivar clientes'),
  ('suppliers', 'view', 'Ver proveedores'),
  ('suppliers', 'create', 'Crear proveedores'),
  ('suppliers', 'update', 'Editar proveedores'),
  ('suppliers', 'delete', 'Desactivar proveedores'),
  ('reports', 'view', 'Ver y exportar reportes'),
  ('dashboard', 'view', 'Ver el panel principal'),
  ('users', 'view', 'Ver usuarios'),
  ('users', 'create', 'Crear usuarios'),
  ('users', 'update', 'Editar usuarios'),
  ('users', 'delete', 'Desactivar usuarios'),
  ('roles', 'view', 'Ver roles y permisos'),
  ('roles', 'update', 'Editar permisos de roles'),
  ('settings', 'view', 'Ver configuración'),
  ('settings', 'update', 'Editar configuración'),
  ('backups', 'view', 'Ver respaldos'),
  ('backups', 'create', 'Crear y descargar respaldos'),
  ('backups', 'authorize', 'Restaurar respaldos'),
  ('audit', 'view', 'Ver bitácora de auditoría');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p WHERE r.code = 'admin';

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON (p.module, p.action) IN (
  ('pos', 'view'), ('pos', 'create'), ('sales', 'view'),
  ('cash_shifts', 'view'), ('cash_shifts', 'create'), ('cash_shifts', 'update'),
  ('exchange_rates', 'view'), ('products', 'view'), ('categories', 'view'),
  ('customers', 'view'), ('customers', 'create'), ('customers', 'update'),
  ('reports', 'view'), ('dashboard', 'view')
)
WHERE r.code = 'cashier';

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON (p.module, p.action) IN (
  ('products', 'view'), ('products', 'update'), ('categories', 'view'),
  ('purchases', 'view'), ('purchases', 'create'), ('purchases', 'update'), ('purchases', 'void'),
  ('inventory', 'view'), ('inventory', 'create'), ('suppliers', 'view'),
  ('reports', 'view'), ('dashboard', 'view')
)
WHERE r.code = 'warehouse';

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.action = 'view' AND p.module NOT IN ('pos', 'users', 'roles', 'settings', 'backups')
WHERE r.code = 'viewer';

INSERT INTO units (name, abbreviation) VALUES
  ('Unidad', 'u'),
  ('Botella', 'bot'),
  ('Lata', 'lata'),
  ('Caja', 'caja'),
  ('Paquete', 'paq'),
  ('Bolsa', 'bolsa');

INSERT INTO customers (full_name, is_walk_in) VALUES ('Consumidor final', true);

INSERT INTO registers (name, invoice_series) VALUES ('Caja 1', 'F001');

INSERT INTO settings (key, value) VALUES
  ('business', '{"name": "Licorería", "taxId": "", "address": "", "phone": "", "email": "", "logoUrl": null}'),
  ('tax', '{"defaultPercent": 15}'),
  ('pos', '{"discountLimitPercent": 5, "cashRounding": "none", "allowNegativeStock": false, "defaultPrintFormat": "ticket"}'),
  ('security', '{"idleTimeoutMinutes": 30, "maxFailedAttempts": 5, "lockoutMinutes": 15, "tempPasswordHours": 72}'),
  ('backups', '{"dailyRetention": 7, "weeklyRetention": 4}');
