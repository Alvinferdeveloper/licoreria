package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"licoreria/api/internal/httpx"
	"licoreria/api/internal/mail"
	"licoreria/api/internal/platform/logx"
)

type accountOwner struct {
	Name, Email, Username string
}

func (s *Service) owner(ctx context.Context, userID uuid.UUID) (accountOwner, error) {
	var o accountOwner
	err := s.pool.QueryRow(ctx, `SELECT full_name, COALESCE(email::text, ''), username FROM users WHERE id = $1`, userID).
		Scan(&o.Name, &o.Email, &o.Username)
	if err != nil {
		return o, fmt.Errorf("load account owner: %w", err)
	}
	return o, nil
}

// alert queues a notice for the account's owner and, when admin is set, for the
// administrators. A failure is logged and never changes the response of the request.
func (s *Service) alert(ctx context.Context, userID uuid.UUID, toOwner func(accountOwner) mail.Alert, toAdmins func(accountOwner) mail.Alert) {
	if err := s.sendAlert(ctx, userID, toOwner, toAdmins); err != nil {
		logx.From(ctx).Error("queue security alert", "user_id", userID, "error", err)
	}
}

func (s *Service) sendAlert(ctx context.Context, userID uuid.UUID, toOwner, toAdmins func(accountOwner) mail.Alert) error {
	o, err := s.owner(ctx, userID)
	if err != nil {
		return err
	}
	stamp := func(a mail.Alert) mail.Alert {
		a.When, a.IP = s.now(), httpx.Client(ctx).IP
		return a
	}
	if toOwner != nil && o.Email != "" {
		if err := mail.SendAlert(ctx, s.pool, []mail.Recipient{{Name: o.Name, Email: o.Email}}, stamp(toOwner(o))); err != nil {
			return err
		}
	}
	if toAdmins != nil {
		return mail.AlertAdmins(ctx, s.pool, stamp(toAdmins(o)), o.Email)
	}
	return nil
}

func passwordChangedAlert(accountOwner) mail.Alert {
	return mail.Alert{Subject: "Tu contraseña cambió", Body: "La contraseña de tu cuenta se cambió.", Advice: mail.NotOwnerAdvice}
}

func (s *Service) lockedAlerts(until time.Time) (func(accountOwner) mail.Alert, func(accountOwner) mail.Alert) {
	minutes := max(int(until.Sub(s.now()).Round(time.Minute).Minutes()), 1)
	toOwner := func(accountOwner) mail.Alert {
		return mail.Alert{
			Subject: "Tu cuenta se bloqueó",
			Body:    fmt.Sprintf("Tu cuenta se bloqueó por varios intentos fallidos de inicio de sesión. Podrás intentar de nuevo en %d minutos.", minutes),
			Advice:  mail.NotOwnerAdvice,
		}
	}
	toAdmins := func(o accountOwner) mail.Alert {
		return mail.Alert{
			Subject:     "Cuenta bloqueada: " + o.Name,
			Body:        fmt.Sprintf("La cuenta de %s (%s) se bloqueó por varios intentos fallidos de inicio de sesión. Se desbloquea sola en %d minutos, o al generarle un código de acceso nuevo.", o.Name, o.Username, minutes),
			ActionURL:   s.appBaseURL + "/usuarios",
			ActionLabel: "Ver usuarios",
		}
	}
	return toOwner, toAdmins
}

func (s *Service) outsideAttemptAlert(o accountOwner) mail.Alert {
	return mail.Alert{
		Subject:     "Intento de entrada fuera de horario: " + o.Name,
		Body:        fmt.Sprintf("%s (%s) escribió bien su contraseña, pero intentó entrar fuera del horario de uso y el sistema no lo dejó pasar.", o.Name, o.Username),
		ActionURL:   s.appBaseURL + "/configuracion",
		ActionLabel: "Revisar el horario",
	}
}

func outsideLoginAlert(accountOwner) mail.Alert {
	return mail.Alert{Subject: "Entraste fuera de horario", Body: "Tu cuenta entró al sistema fuera del horario de uso.", Advice: mail.NotOwnerAdvice}
}

// firstOutsideAttempt is true for the first blocked attempt of the user in the last hour,
// so repeated tries raise a single alert.
func (s *Service) firstOutsideAttempt(ctx context.Context, userID uuid.UUID) bool {
	var n int
	err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM audit_log
		WHERE user_id = $1 AND action = 'auth.login_failed' AND after_data->>'reason' = 'outside_hours'
		  AND created_at > now() - interval '1 hour'`, userID).Scan(&n)
	if err != nil {
		logx.From(ctx).Error("count outside-hours attempts", "user_id", userID, "error", err)
		return false
	}
	return n <= 1
}
