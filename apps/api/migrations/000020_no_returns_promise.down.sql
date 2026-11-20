UPDATE permissions SET description = 'Anular ventas y registrar devoluciones'
WHERE module = 'sales' AND action = 'void';

UPDATE settings
SET value = jsonb_set(value, '{footerNote}', '"Cambios y devoluciones dentro de 7 días con esta factura, producto sellado y en buen estado."')
WHERE key = 'invoice' AND value->>'footerNote' = 'Gracias por su compra. Conserve este comprobante.';
