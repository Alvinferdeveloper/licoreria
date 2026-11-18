-- Email becomes optional: staff without email activate at the counter with a one-time code.
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;

-- NULL means the user has not chosen a password yet (pending invitation).
ALTER TABLE users ADD COLUMN activated_at timestamptz;
UPDATE users SET activated_at = COALESCE(last_login_at, created_at);

UPDATE email_templates
SET subject = 'Te invitaron a {{.StoreName}}',
    html_body = $tpl$<!DOCTYPE html>
<html lang="es">
<body style="margin:0;padding:32px 16px;background:#fbf9f5;font-family:Segoe UI,Helvetica,Arial,sans-serif;color:#231b16;">
  <table role="presentation" width="100%" style="max-width:520px;margin:0 auto;background:#fffdfa;border:1px solid #e2dbd4;border-radius:12px;">
    <tr><td style="padding:32px;">
      <p style="margin:0 0 24px;font-size:18px;font-weight:600;">{{.StoreName}}</p>
      <p style="margin:0 0 16px;">Hola, {{.Name}}:</p>
      <p style="margin:0 0 16px;line-height:1.5;">Te crearon una cuenta en el sistema. Abre el enlace y elige tu contraseña para empezar.</p>
      <table role="presentation" style="margin:0 0 24px;background:#efe8e1;border-radius:8px;width:100%;">
        <tr><td style="padding:16px;font-size:14px;line-height:1.8;">
          Tu usuario: <strong style="font-family:Consolas,monospace;font-size:16px;">{{.Username}}</strong>
        </td></tr>
      </table>
      <a href="{{.ActivateURL}}" style="display:inline-block;padding:12px 20px;background:#ad5907;color:#fffdfa;text-decoration:none;border-radius:8px;font-weight:600;">Elegir mi contraseña</a>
      <p style="margin:24px 0 0;font-size:13px;color:#5f564f;line-height:1.5;">El enlace sirve una sola vez y vence en {{.ExpiresInHours}} horas. Si no esperabas este correo, avisa al administrador.</p>
    </td></tr>
  </table>
</body>
</html>$tpl$
WHERE key = 'welcome';
