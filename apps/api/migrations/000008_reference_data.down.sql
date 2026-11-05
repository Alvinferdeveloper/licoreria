DELETE FROM settings;
DELETE FROM registers;
DELETE FROM customers WHERE is_walk_in;
DELETE FROM units;
DELETE FROM role_permissions;
DELETE FROM permissions;
DELETE FROM roles WHERE is_system;
DELETE FROM currencies;
