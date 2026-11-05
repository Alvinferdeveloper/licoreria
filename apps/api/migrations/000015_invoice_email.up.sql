INSERT INTO email_templates (key, subject, html_body) VALUES
('invoice', 'Factura {{.Invoice}} de {{.StoreName}}', $tpl$<!DOCTYPE html>
<html lang="es">
<body style="margin:0;padding:32px 16px;background:#fbf9f5;font-family:Segoe UI,Helvetica,Arial,sans-serif;color:#231b16;">
  <table role="presentation" width="100%" style="max-width:560px;margin:0 auto;background:#fffdfa;border:1px solid #e2dbd4;border-radius:12px;">
    <tr><td style="padding:32px;">
      <p style="margin:0 0 4px;font-size:18px;font-weight:600;">{{.StoreName}}</p>
      {{if .StoreTaxID}}<p style="margin:0 0 24px;font-size:13px;color:#5f564f;">RUC {{.StoreTaxID}}</p>{{end}}
      <p style="margin:0 0 4px;font-size:13px;color:#5f564f;">Factura</p>
      <p style="margin:0 0 4px;font-family:Consolas,monospace;font-size:20px;font-weight:700;">{{.Invoice}}</p>
      <p style="margin:0 0 24px;font-size:13px;color:#5f564f;">{{.Date}} · {{.Customer}}</p>
      <table role="presentation" width="100%" style="border-collapse:collapse;font-size:14px;">
        {{range .Items}}
        <tr>
          <td style="padding:8px 0;border-bottom:1px solid #e2dbd4;">{{.Name}}<br><span style="font-size:12px;color:#5f564f;">{{.Quantity}} × {{.UnitPrice}}</span></td>
          <td style="padding:8px 0;border-bottom:1px solid #e2dbd4;text-align:right;font-family:Consolas,monospace;">{{.Subtotal}}</td>
        </tr>
        {{end}}
        <tr><td style="padding:12px 0 2px;color:#5f564f;">Subtotal</td><td style="padding:12px 0 2px;text-align:right;font-family:Consolas,monospace;">{{.Subtotal}}</td></tr>
        {{if .Discount}}<tr><td style="padding:2px 0;color:#5f564f;">Descuento</td><td style="padding:2px 0;text-align:right;font-family:Consolas,monospace;">-{{.Discount}}</td></tr>{{end}}
        <tr><td style="padding:2px 0;color:#5f564f;">IVA</td><td style="padding:2px 0;text-align:right;font-family:Consolas,monospace;">{{.Tax}}</td></tr>
        <tr><td style="padding:10px 0 0;font-weight:700;font-size:16px;">Total</td><td style="padding:10px 0 0;text-align:right;font-family:Consolas,monospace;font-weight:700;font-size:16px;">{{.Total}}</td></tr>
      </table>
      <p style="margin:8px 0 24px;font-size:12px;color:#5f564f;">Son: {{.TotalWords}}</p>
      <p style="margin:0;font-size:12px;color:#5f564f;line-height:1.5;">Gracias por su compra. Prohibida la venta de bebidas alcohólicas a menores de 18 años. Comprobante interno sin validez fiscal ante la DGI.</p>
    </td></tr>
  </table>
</body>
</html>$tpl$);
