-- Supervisors see and close every register's shifts; cashiers only their own.
INSERT INTO permissions (module, action, description) VALUES
  ('cash_shifts', 'authorize', 'Supervisar y cerrar turnos de todas las cajas');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r JOIN permissions p ON p.module = 'cash_shifts' AND p.action = 'authorize'
WHERE r.code = 'admin';

INSERT INTO email_templates (key, subject, html_body) VALUES
('cash_difference', 'Cierre de {{.Register}} con diferencia', $tpl$<!DOCTYPE html>
<html lang="es">
<body style="margin:0;padding:32px 16px;background:#fbf9f5;font-family:Segoe UI,Helvetica,Arial,sans-serif;color:#231b16;">
  <table role="presentation" width="100%" style="max-width:520px;margin:0 auto;background:#fffdfa;border:1px solid #e2dbd4;border-radius:12px;">
    <tr><td style="padding:32px;">
      <p style="margin:0 0 24px;font-size:18px;font-weight:600;">{{.StoreName}}</p>
      <p style="margin:0 0 16px;">Hola, {{.Name}}:</p>
      <p style="margin:0 0 16px;line-height:1.5;"><strong>{{.Cashier}}</strong> cerró <strong>{{.Register}}</strong> y el arqueo no cuadra.</p>
      <table role="presentation" style="margin:0 0 24px;background:#efe8e1;border-radius:8px;width:100%;">
        <tr><td style="padding:16px;font-size:14px;line-height:1.8;">
          Córdobas: esperado <strong style="font-family:Consolas,monospace;">C$ {{.ExpectedNIO}}</strong>, contado <strong style="font-family:Consolas,monospace;">C$ {{.CountedNIO}}</strong>, diferencia <strong style="font-family:Consolas,monospace;">C$ {{.DifferenceNIO}}</strong><br>
          Dólares: esperado <strong style="font-family:Consolas,monospace;">US$ {{.ExpectedUSD}}</strong>, contado <strong style="font-family:Consolas,monospace;">US$ {{.CountedUSD}}</strong>, diferencia <strong style="font-family:Consolas,monospace;">US$ {{.DifferenceUSD}}</strong>
        </td></tr>
      </table>
      {{if .Notes}}<p style="margin:0 0 24px;line-height:1.5;">Nota del cajero: {{.Notes}}</p>{{end}}
      <a href="{{.ShiftURL}}" style="display:inline-block;padding:12px 20px;background:#ad5907;color:#fffdfa;text-decoration:none;border-radius:8px;font-weight:600;">Ver el turno</a>
    </td></tr>
  </table>
</body>
</html>$tpl$);
