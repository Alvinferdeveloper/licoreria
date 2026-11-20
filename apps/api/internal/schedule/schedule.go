// Package schedule decides whether the store's usage hours allow a user to sign in or
// sell right now, and keeps the administrator's same-day extensions.
package schedule

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/platform/database"
	"licoreria/api/internal/settings"
)

// Zone is the store's clock; Nicaragua has no daylight saving time.
var Zone = time.FixedZone("America/Managua", -6*60*60)

var ErrSalesClosed = apperr.New(apperr.KindForbidden, "outside_hours",
	"El horario de ventas terminó. Puedes cerrar tu caja; para seguir vendiendo, pide al administrador que extienda el horario.")

// LoginClosed explains when the user may sign in again. Clock already ends in "m.",
// so no period is added after it.
func LoginClosed(next, now time.Time) *apperr.Error {
	msg := "Estás fuera del horario de uso. "
	if !next.IsZero() {
		msg += "Podrás entrar " + DescribeNext(next, now) + " "
	}
	return apperr.New(apperr.KindForbidden, "outside_hours", msg+"Si necesitas entrar ahora, pide al administrador que extienda el horario.")
}

// ParseClock reads "HH:MM" as the time since midnight.
func ParseClock(s string) (time.Duration, bool) {
	h, m, ok := strings.Cut(s, ":")
	if !ok || len(h) != 2 || len(m) != 2 {
		return 0, false
	}
	hours, err1 := strconv.Atoi(h)
	minutes, err2 := strconv.Atoi(m)
	if err1 != nil || err2 != nil || hours > 23 || minutes > 59 || hours < 0 || minutes < 0 {
		return 0, false
	}
	return time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute, true
}

func midnight(t time.Time) time.Time {
	t = t.In(Zone)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, Zone)
}

// window returns the usage window, tolerance included, that opens on the given day.
func window(s settings.Schedule, day time.Time) (start, end time.Time, ok bool) {
	d := s.Days[day.Weekday()]
	open, okOpen := ParseClock(d.Open)
	closeAt, okClose := ParseClock(d.Close)
	if d.Closed || !okOpen || !okClose {
		return time.Time{}, time.Time{}, false
	}
	if closeAt <= open {
		closeAt += 24 * time.Hour
	}
	tolerance := time.Duration(s.ToleranceMinutes) * time.Minute
	return day.Add(open - tolerance), day.Add(closeAt + tolerance), true
}

// Status reports whether t is inside the schedule and, when it isn't, when the next
// window opens (zero if every day is closed).
func Status(s settings.Schedule, t time.Time) (open bool, next time.Time) {
	if !s.Enabled {
		return true, time.Time{}
	}
	today := midnight(t)
	for offset := -1; offset <= 7; offset++ {
		start, end, ok := window(s, today.AddDate(0, 0, offset))
		if !ok {
			continue
		}
		if !t.Before(start) && t.Before(end) {
			return true, time.Time{}
		}
		if start.After(t) && (next.IsZero() || start.Before(next)) {
			next = start
		}
	}
	return false, next
}

var weekdays = [7]string{"domingo", "lunes", "martes", "miércoles", "jueves", "viernes", "sábado"}

// Clock formats a time the way the store reads it, e.g. "8:30 p. m.".
func Clock(t time.Time) string {
	t = t.In(Zone)
	suffix := "a. m."
	if t.Hour() >= 12 {
		suffix = "p. m."
	}
	return t.Format("3:04") + " " + suffix
}

// DescribeNext says when next happens relative to now: "hoy a las…", "mañana a las…".
func DescribeNext(next, now time.Time) string {
	days := int(midnight(next).Sub(midnight(now)).Hours() / 24)
	switch days {
	case 0:
		return "hoy a las " + Clock(next)
	case 1:
		return "mañana a las " + Clock(next)
	default:
		return "el " + weekdays[next.In(Zone).Weekday()] + " a las " + Clock(next)
	}
}

// NextOccurrence is the first moment after now whose store clock reads hhmm.
func NextOccurrence(hhmm string, now time.Time) (time.Time, bool) {
	clock, ok := ParseClock(hhmm)
	if !ok {
		return time.Time{}, false
	}
	at := midnight(now).Add(clock)
	if !at.After(now) {
		at = at.AddDate(0, 0, 1)
	}
	return at, true
}

// Decision is the schedule's verdict for one user at one moment.
type Decision struct {
	Outside  bool
	Extended bool
	NextOpen time.Time
}

// Blocked is true when the user is outside the schedule and nothing lets them through.
func (d Decision) Blocked(canBypass bool) bool { return d.Outside && !d.Extended && !canBypass }

func Evaluate(ctx context.Context, db database.DBTX, userID uuid.UUID, now time.Time) (Decision, error) {
	s, err := settings.GetSchedule(ctx, db)
	if err != nil {
		return Decision{}, err
	}
	open, next := Status(s, now)
	if open {
		return Decision{}, nil
	}
	var extended bool
	if err := db.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM schedule_extensions
		               WHERE ends_at > $2 AND (user_id IS NULL OR user_id = $1))`, userID, now).Scan(&extended); err != nil {
		return Decision{}, fmt.Errorf("check schedule extension: %w", err)
	}
	return Decision{Outside: true, Extended: extended, NextOpen: next}, nil
}

// Extension lets one user, or everyone when UserID is nil, work past the schedule until EndsAt.
type Extension struct {
	ID            uuid.UUID  `json:"id"`
	UserID        *uuid.UUID `json:"userId"`
	UserName      *string    `json:"userName"`
	EndsAt        time.Time  `json:"endsAt"`
	CreatedByName string     `json:"createdByName"`
	CreatedAt     time.Time  `json:"createdAt"`
}

const selectExtension = `
	SELECT e.id, e.user_id, u.full_name, e.ends_at, c.full_name, e.created_at
	FROM schedule_extensions e
	LEFT JOIN users u ON u.id = e.user_id
	JOIN users c ON c.id = e.created_by`

func scanExtension(row pgx.Row) (Extension, error) {
	var e Extension
	err := row.Scan(&e.ID, &e.UserID, &e.UserName, &e.EndsAt, &e.CreatedByName, &e.CreatedAt)
	return e, err
}

func ActiveExtensions(ctx context.Context, db database.DBTX, now time.Time) ([]Extension, error) {
	rows, err := db.Query(ctx, selectExtension+" WHERE e.ends_at > $1 ORDER BY e.ends_at", now)
	if err != nil {
		return nil, fmt.Errorf("list schedule extensions: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Extension, error) { return scanExtension(row) })
}

func CreateExtension(ctx context.Context, db database.DBTX, userID *uuid.UUID, endsAt time.Time, by uuid.UUID) (Extension, error) {
	var id uuid.UUID
	err := db.QueryRow(ctx, `
		INSERT INTO schedule_extensions (user_id, ends_at, created_by) VALUES ($1, $2, $3) RETURNING id`,
		userID, endsAt, by).Scan(&id)
	if err != nil {
		return Extension{}, database.MapError(err, map[string]string{"schedule_extensions_user_id_fkey": "userId"})
	}
	return scanExtension(db.QueryRow(ctx, selectExtension+" WHERE e.id = $1", id))
}

// EndExtension stops an active extension now; the row stays as history.
func EndExtension(ctx context.Context, db database.DBTX, id uuid.UUID, now time.Time) (Extension, error) {
	tag, err := db.Exec(ctx, `UPDATE schedule_extensions SET ends_at = $2 WHERE id = $1 AND ends_at > $2`, id, now)
	if err != nil {
		return Extension{}, fmt.Errorf("end schedule extension: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return Extension{}, apperr.ErrNotFound
	}
	return scanExtension(db.QueryRow(ctx, selectExtension+" WHERE e.id = $1", id))
}
