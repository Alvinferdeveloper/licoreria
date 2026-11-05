// Package mail renders transactional emails from DB templates, queues them and delivers
// them in the background with retries, so a mail failure never blocks a request.
package mail

import (
	"bytes"
	"context"
	"fmt"
	htmltemplate "html/template"
	texttemplate "text/template"

	"github.com/jackc/pgx/v5"

	"licoreria/api/internal/platform/database"
)

const (
	TemplateWelcome        = "welcome"
	TemplatePasswordReset  = "password_reset"
	TemplateLowStock       = "low_stock"
	TemplateCashDifference = "cash_difference"
	TemplateInvoice        = "invoice"
)

type Recipient struct {
	Name  string
	Email string
}

// RecipientsWithPermission lists active users whose role grants module:action.
func RecipientsWithPermission(ctx context.Context, db database.DBTX, module, action string) ([]Recipient, error) {
	rows, err := db.Query(ctx, `
		SELECT DISTINCT u.full_name, u.email::text FROM users u
		JOIN role_permissions rp ON rp.role_id = u.role_id
		JOIN permissions p ON p.id = rp.permission_id
		WHERE u.active AND p.module = $1 AND p.action = $2`, module, action)
	if err != nil {
		return nil, fmt.Errorf("list recipients: %w", err)
	}
	recipients, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Recipient])
	if err != nil {
		return nil, fmt.Errorf("read recipients: %w", err)
	}
	return recipients, nil
}

// Enqueue renders the template with data and stores the message for delivery.
// Pass a transaction as db to queue the email atomically with the change that triggers it.
func Enqueue(ctx context.Context, db database.DBTX, templateKey, to string, data any) error {
	var subjectTpl, bodyTpl string
	err := db.QueryRow(ctx, `SELECT subject, html_body FROM email_templates WHERE key = $1`, templateKey).
		Scan(&subjectTpl, &bodyTpl)
	if err != nil {
		return fmt.Errorf("load email template %q: %w", templateKey, err)
	}

	subject, err := renderText(subjectTpl, data)
	if err != nil {
		return fmt.Errorf("render subject %q: %w", templateKey, err)
	}
	body, err := renderHTML(bodyTpl, data)
	if err != nil {
		return fmt.Errorf("render body %q: %w", templateKey, err)
	}

	_, err = db.Exec(ctx, `
		INSERT INTO email_queue (template_key, to_address, subject, html_body)
		VALUES ($1, $2, $3, $4)`, templateKey, to, subject, body)
	if err != nil {
		return fmt.Errorf("queue email: %w", err)
	}
	return nil
}

func renderText(tpl string, data any) (string, error) {
	t, err := texttemplate.New("subject").Option("missingkey=error").Parse(tpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// renderHTML escapes data values, so user input can't inject markup into emails.
func renderHTML(tpl string, data any) (string, error) {
	t, err := htmltemplate.New("body").Option("missingkey=error").Parse(tpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
