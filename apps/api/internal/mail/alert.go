package mail

import (
	"context"
	"strings"
	"time"

	"licoreria/api/internal/platform/database"
	"licoreria/api/internal/schedule"
	"licoreria/api/internal/settings"
)

const TemplateAlert = "alert"

// Alert is a security or operations notice. Its text is written for the person who receives
// it and must never carry passwords, codes or access links.
type Alert struct {
	Subject     string
	Body        string
	Advice      string
	ActionURL   string
	ActionLabel string
	When        time.Time
	IP          string
}

// NotOwnerAdvice closes alerts sent to an account's owner.
const NotOwnerAdvice = "Si no fuiste tú, avisa al administrador de inmediato."

// SendAlert queues the alert for each recipient, skipping those listed in skip (by email).
func SendAlert(ctx context.Context, db database.DBTX, to []Recipient, a Alert, skip ...string) error {
	business, err := settings.GetBusiness(ctx, db)
	if err != nil {
		return err
	}
	when := a.When.In(schedule.Zone)
	for _, r := range to {
		if r.Email == "" || containsFold(skip, r.Email) {
			continue
		}
		if err := Enqueue(ctx, db, TemplateAlert, r.Email, map[string]any{
			"Subject": a.Subject, "StoreName": business.Name, "Name": r.Name, "Body": a.Body, "Advice": a.Advice,
			"ActionURL": a.ActionURL, "ActionLabel": a.ActionLabel, "IP": a.IP,
			"When": when.Format("02/01/2006") + ", " + schedule.Clock(when),
		}); err != nil {
			return err
		}
	}
	return nil
}

// AlertAdmins sends the alert to every active user who manages users.
func AlertAdmins(ctx context.Context, db database.DBTX, a Alert, skip ...string) error {
	admins, err := RecipientsWithPermission(ctx, db, "users", "update")
	if err != nil {
		return err
	}
	return SendAlert(ctx, db, admins, a, skip...)
}

func containsFold(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}
