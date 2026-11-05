INSERT INTO email_templates (key, subject, html_body) VALUES
('low_stock', '{{.Product}} llegó al stock mínimo', $tpl$<!DOCTYPE html>
<html lang="es">
<body style="margin:0;padding:32px 16px;background:#fbf9f5;font-family:Segoe UI,Helvetica,Arial,sans-serif;color:#231b16;">
  <table role="presentation" width="100%" style="max-width:520px;margin:0 auto;background:#fffdfa;border:1px solid #e2dbd4;border-radius:12px;">
    <tr><td style="padding:32px;">
      <p style="margin:0 0 24px;font-size:18px;font-weight:600;">{{.StoreName}}</p>
      <p style="margin:0 0 16px;">Hola, {{.Name}}:</p>
      <p style="margin:0 0 16px;line-height:1.5;"><strong>{{.Product}}</strong> bajó a su stock mínimo. Conviene pedirlo pronto.</p>
      <table role="presentation" style="margin:0 0 24px;background:#efe8e1;border-radius:8px;width:100%;">
        <tr><td style="padding:16px;font-size:14px;line-height:1.8;">
          SKU: <strong style="font-family:Consolas,monospace;">{{.SKU}}</strong><br>
          Stock actual: <strong style="font-family:Consolas,monospace;">{{.Stock}}</strong><br>
          Stock mínimo: <strong style="font-family:Consolas,monospace;">{{.MinStock}}</strong>
        </td></tr>
      </table>
      <a href="{{.AlertsURL}}" style="display:inline-block;padding:12px 20px;background:#ad5907;color:#fffdfa;text-decoration:none;border-radius:8px;font-weight:600;">Ver productos por pedir</a>
    </td></tr>
  </table>
</body>
</html>$tpl$);
