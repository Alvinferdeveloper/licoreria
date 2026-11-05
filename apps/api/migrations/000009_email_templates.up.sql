-- Default transactional emails. Bodies use Go html/template syntax and are editable from the admin UI.
INSERT INTO email_templates (key, subject, html_body) VALUES
('welcome', 'Tu acceso a {{.StoreName}}', $tpl$<!DOCTYPE html>
<html lang="es">
<body style="margin:0;padding:32px 16px;background:#fbf9f5;font-family:Segoe UI,Helvetica,Arial,sans-serif;color:#231b16;">
  <table role="presentation" width="100%" style="max-width:520px;margin:0 auto;background:#fffdfa;border:1px solid #e2dbd4;border-radius:12px;">
    <tr><td style="padding:32px;">
      <p style="margin:0 0 24px;font-size:18px;font-weight:600;">{{.StoreName}}</p>
      <p style="margin:0 0 16px;">Hola, {{.Name}}:</p>
      <p style="margin:0 0 16px;line-height:1.5;">Se creó tu cuenta en el sistema. Inicia sesión con estos datos y elige una contraseña nueva al entrar.</p>
      <table role="presentation" style="margin:0 0 24px;background:#efe8e1;border-radius:8px;width:100%;">
        <tr><td style="padding:16px;font-size:14px;line-height:1.8;">
          Usuario: <strong style="font-family:Consolas,monospace;">{{.Username}}</strong><br>
          Contraseña temporal: <strong style="font-family:Consolas,monospace;font-size:16px;">{{.TemporaryPassword}}</strong>
        </td></tr>
      </table>
      <a href="{{.LoginURL}}" style="display:inline-block;padding:12px 20px;background:#ad5907;color:#fffdfa;text-decoration:none;border-radius:8px;font-weight:600;">Iniciar sesión</a>
      <p style="margin:24px 0 0;font-size:13px;color:#5f564f;line-height:1.5;">La contraseña temporal vence en {{.ExpiresInHours}} horas. Si no esperabas este correo, avisa al administrador.</p>
    </td></tr>
  </table>
</body>
</html>$tpl$),
('password_reset', 'Restablece tu contraseña', $tpl$<!DOCTYPE html>
<html lang="es">
<body style="margin:0;padding:32px 16px;background:#fbf9f5;font-family:Segoe UI,Helvetica,Arial,sans-serif;color:#231b16;">
  <table role="presentation" width="100%" style="max-width:520px;margin:0 auto;background:#fffdfa;border:1px solid #e2dbd4;border-radius:12px;">
    <tr><td style="padding:32px;">
      <p style="margin:0 0 24px;font-size:18px;font-weight:600;">{{.StoreName}}</p>
      <p style="margin:0 0 16px;">Hola, {{.Name}}:</p>
      <p style="margin:0 0 24px;line-height:1.5;">Recibimos una solicitud para restablecer tu contraseña. El enlace sirve una sola vez y vence en {{.ExpiresInMinutes}} minutos.</p>
      <a href="{{.ResetURL}}" style="display:inline-block;padding:12px 20px;background:#ad5907;color:#fffdfa;text-decoration:none;border-radius:8px;font-weight:600;">Elegir contraseña nueva</a>
      <p style="margin:24px 0 0;font-size:13px;color:#5f564f;line-height:1.5;">Si no lo pediste, ignora este correo: tu contraseña actual sigue funcionando.</p>
    </td></tr>
  </table>
</body>
</html>$tpl$);
