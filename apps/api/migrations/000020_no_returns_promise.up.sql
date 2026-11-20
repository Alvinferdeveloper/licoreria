-- Returns are not part of the system: stop promising them on invoices and in permissions.
UPDATE permissions SET description = 'Anular ventas del turno abierto'
WHERE module = 'sales' AND action = 'void';

UPDATE settings
SET value = jsonb_set(value, '{footerNote}', '"Gracias por su compra. Conserve este comprobante."')
WHERE key = 'invoice'
  AND value->>'footerNote' = 'Cambios y devoluciones dentro de 7 días con esta factura, producto sellado y en buen estado.';
