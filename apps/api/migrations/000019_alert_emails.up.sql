-- One layout for security and operations alerts; the text comes from the code that raises them.
INSERT INTO email_templates (key, subject, html_body) VALUES
('alert', '{{.Subject}}', $tpl$<!DOCTYPE html>
<html lang="es">
<body style="margin:0;padding:32px 16px;background:#fbf9f5;font-family:Segoe UI,Helvetica,Arial,sans-serif;color:#231b16;">
  <table role="presentation" width="100%" style="max-width:520px;margin:0 auto;background:#fffdfa;border:1px solid #e2dbd4;border-radius:12px;">
    <tr><td style="padding:32px;">
      <p style="margin:0 0 24px;font-size:18px;font-weight:600;">{{.StoreName}}</p>
      <p style="margin:0 0 16px;">Hola, {{.Name}}:</p>
      <p style="margin:0 0 16px;line-height:1.5;">{{.Body}}</p>
      <table role="presentation" style="margin:0 0 24px;background:#efe8e1;border-radius:8px;width:100%;">
        <tr><td style="padding:16px;font-size:14px;line-height:1.8;">
          Fecha y hora: <strong>{{.When}}</strong>{{if .IP}}<br>
          Dirección IP: <strong style="font-family:Consolas,monospace;">{{.IP}}</strong>{{end}}
        </td></tr>
      </table>
      {{if .Advice}}<p style="margin:0 0 24px;line-height:1.5;"><strong>{{.Advice}}</strong></p>{{end}}
      {{if .ActionURL}}<a href="{{.ActionURL}}" style="display:inline-block;padding:12px 20px;background:#ad5907;color:#fffdfa;text-decoration:none;border-radius:8px;font-weight:600;">{{.ActionLabel}}</a>{{end}}
    </td></tr>
  </table>
</body>
</html>$tpl$);
