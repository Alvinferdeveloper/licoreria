package configuration

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/audit"
	"licoreria/api/internal/auth"
	"licoreria/api/internal/platform/database"
	"licoreria/api/internal/schedule"
	"licoreria/api/internal/settings"
)

var ErrInvalidUntil = apperr.Validation(apperr.FieldError{Field: "until", Code: "invalid", Message: "Escribe la hora en formato HH:MM, por ejemplo 23:30."})

func checkSchedule(s settings.Schedule) error {
	var fields []apperr.FieldError
	open := 0
	for i, d := range s.Days {
		if d.Closed {
			continue
		}
		open++
		from, okFrom := schedule.ParseClock(d.Open)
		to, okTo := schedule.ParseClock(d.Close)
		if !okFrom || !okTo || from == to {
			fields = append(fields, apperr.FieldError{
				Field: fmt.Sprintf("schedule.days.%d", i), Code: "invalid", Message: "Escribe una hora de apertura y otra de cierre distintas.",
			})
		}
	}
	if s.Enabled && open == 0 {
		fields = append(fields, apperr.FieldError{Field: "schedule.days", Code: "required", Message: "Deja al menos un día abierto o desactiva el horario."})
	}
	if s.ToleranceMinutes < 0 || s.ToleranceMinutes > 120 {
		fields = append(fields, apperr.FieldError{Field: "schedule.toleranceMinutes", Code: "range", Message: "La tolerancia va de 0 a 120 minutos."})
	}
	if len(fields) > 0 {
		return apperr.Validation(fields...)
	}
	return nil
}

func (s *Service) Extensions(ctx context.Context) ([]schedule.Extension, error) {
	return schedule.ActiveExtensions(ctx, s.pool, time.Now())
}

// Extend lets userID (everyone when nil) work until the next time the store clock reads until.
func (s *Service) Extend(ctx context.Context, actor auth.Principal, userID *uuid.UUID, until string) (schedule.Extension, error) {
	endsAt, ok := schedule.NextOccurrence(until, time.Now())
	if !ok {
		return schedule.Extension{}, ErrInvalidUntil
	}
	var ext schedule.Extension
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		if ext, err = schedule.CreateExtension(ctx, tx, userID, endsAt, actor.UserID); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{
			UserID: &actor.UserID, Action: audit.ActionScheduleExtended, Entity: "schedule_extensions", EntityID: &ext.ID, After: ext,
		})
	})
	return ext, err
}

func (s *Service) EndExtension(ctx context.Context, actor auth.Principal, id uuid.UUID) (schedule.Extension, error) {
	var ext schedule.Extension
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		if ext, err = schedule.EndExtension(ctx, tx, id, time.Now()); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{
			UserID: &actor.UserID, Action: audit.ActionExtensionRemoved, Entity: "schedule_extensions", EntityID: &ext.ID, After: ext,
		})
	})
	return ext, err
}
