package cash

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/audit"
	"licoreria/api/internal/auth"
	"licoreria/api/internal/httpx"
	"licoreria/api/internal/mail"
	"licoreria/api/internal/platform/database"
	"licoreria/api/internal/settings"
)

var (
	ErrNoOpenShift      = apperr.New(apperr.KindNotFound, "no_open_shift", "No tienes una caja abierta.")
	ErrShiftClosed      = apperr.New(apperr.KindConflict, "shift_closed", "El turno ya está cerrado.")
	ErrRegisterBusy     = apperr.New(apperr.KindConflict, "register_busy", "Esa caja ya tiene un turno abierto.")
	ErrUserHasShift     = apperr.New(apperr.KindConflict, "user_has_shift", "Ya tienes una caja abierta. Ciérrala antes de abrir otra.")
	ErrRegisterOff      = apperr.New(apperr.KindConflict, "register_inactive", "Esa caja está desactivada.")
	ErrInsufficientCash = apperr.Validation(apperr.FieldError{Field: "amount", Code: "insufficient_cash", Message: "No hay tanto efectivo de esa moneda en la caja."})
	ErrSeriesLocked     = apperr.Validation(apperr.FieldError{Field: "invoiceSeries", Code: "series_locked", Message: "La serie ya tiene facturas emitidas y no se puede cambiar."})
)

const (
	MovementIncome  = "income"
	MovementExpense = "expense"
)

type RegisterInput struct {
	Name          string
	InvoiceSeries string
	Active        bool
}

type OpenInput struct {
	RegisterID             uuid.UUID
	OpeningNIO, OpeningUSD string
}

type MovementInput struct {
	Type, Currency, Amount, Reason string
}

type CloseInput struct {
	CountedNIO, CountedUSD, Notes string
}

type Service struct {
	pool       *pgxpool.Pool
	appBaseURL string
	repo       repository
}

func NewService(pool *pgxpool.Pool, appBaseURL string) *Service {
	return &Service{pool: pool, appBaseURL: appBaseURL}
}

// seesAll: supervisors and read-only roles see every shift; cashiers only their own.
func seesAll(p auth.Principal) bool {
	return p.Can("cash_shifts", "authorize") || !p.Can("cash_shifts", "create")
}

// OpenShiftFor returns the user's open shift and its register, locking the shift so it
// cannot close while a sale is being recorded; ErrNoOpenShift when there is none.
func OpenShiftFor(ctx context.Context, db database.DBTX, userID uuid.UUID) (shiftID, registerID uuid.UUID, err error) {
	err = db.QueryRow(ctx, `SELECT id, register_id FROM cash_shifts WHERE user_id = $1 AND status = 'open' FOR UPDATE`, userID).
		Scan(&shiftID, &registerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, uuid.Nil, ErrNoOpenShift
	}
	return shiftID, registerID, database.MapError(err, nil)
}

func (s *Service) Registers(ctx context.Context) ([]Register, error) {
	return s.repo.registers(ctx, s.pool)
}

func (s *Service) SaveRegister(ctx context.Context, actor auth.Principal, id *uuid.UUID, in RegisterInput) error {
	in.Name = strings.Join(strings.Fields(in.Name), " ")
	in.InvoiceSeries = strings.ToUpper(strings.TrimSpace(in.InvoiceSeries))
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if id == nil {
			newID, err := s.repo.insertRegister(ctx, tx, in)
			if err != nil {
				return err
			}
			id = &newID
		} else if locked, err := s.repo.updateRegister(ctx, tx, *id, in); err != nil {
			return err
		} else if locked {
			return ErrSeriesLocked
		}
		return audit.Record(ctx, tx, audit.Entry{UserID: &actor.UserID, Action: "registers.saved", Entity: "registers", EntityID: id, After: in})
	})
}

func (s *Service) Shifts(ctx context.Context, p auth.Principal, f ShiftFilter, page httpx.Page) ([]Shift, int64, error) {
	if !seesAll(p) {
		f.UserID = &p.UserID
	}
	return s.repo.shifts(ctx, s.pool, f, page)
}

// Shift returns a shift with its summary and cash movements.
func (s *Service) Shift(ctx context.Context, p auth.Principal, id uuid.UUID) (Shift, error) {
	return s.detail(ctx, s.pool, p, id)
}

func (s *Service) Current(ctx context.Context, p auth.Principal) (Shift, error) {
	id, err := s.repo.openShiftOf(ctx, s.pool, p.UserID)
	if apperr.IsKind(err, apperr.KindNotFound) {
		return Shift{}, ErrNoOpenShift
	}
	if err != nil {
		return Shift{}, err
	}
	return s.detail(ctx, s.pool, p, id)
}

func (s *Service) detail(ctx context.Context, db database.DBTX, p auth.Principal, id uuid.UUID) (Shift, error) {
	shift, err := s.repo.shift(ctx, db, id, false)
	if err != nil {
		return Shift{}, err
	}
	if shift.User.ID != p.UserID && !seesAll(p) {
		return Shift{}, apperr.ErrNotFound
	}
	sum, err := s.repo.summary(ctx, db, id)
	if err != nil {
		return Shift{}, err
	}
	shift.Summary = &sum
	if shift.Movements, err = s.repo.movements(ctx, db, id); err != nil {
		return Shift{}, err
	}
	return shift, nil
}

// Open starts a shift on a free, active register for a user without another open shift.
func (s *Service) Open(ctx context.Context, actor auth.Principal, in OpenInput) (Shift, error) {
	var opened Shift
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var active, busy bool
		err := tx.QueryRow(ctx, `
			SELECT r.active, EXISTS (SELECT 1 FROM cash_shifts WHERE register_id = r.id AND status = 'open')
			FROM registers r WHERE r.id = $1 FOR UPDATE`, in.RegisterID).Scan(&active, &busy)
		if err != nil {
			return database.MapError(err, nil)
		}
		switch {
		case !active:
			return ErrRegisterOff
		case busy:
			return ErrRegisterBusy
		}
		if _, err := s.repo.openShiftOf(ctx, tx, actor.UserID); err == nil {
			return ErrUserHasShift
		}
		id, err := s.repo.insertShift(ctx, tx, in, actor.UserID)
		if err != nil {
			return err
		}
		if opened, err = s.detail(ctx, tx, actor, id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{UserID: &actor.UserID, Action: "cash_shifts.opened", Entity: "cash_shifts", EntityID: &id,
			After: map[string]string{"register": opened.Register.Name, "openingNio": in.OpeningNIO, "openingUsd": in.OpeningUSD}})
	})
	return opened, err
}

// AddMovement records cash income or an expense; expenses cannot exceed the drawer.
func (s *Service) AddMovement(ctx context.Context, actor auth.Principal, id uuid.UUID, in MovementInput) (Shift, error) {
	in.Reason = strings.TrimSpace(in.Reason)
	return s.withOpenShift(ctx, actor, id, func(tx pgx.Tx, shift Shift) error {
		if in.Type == MovementExpense {
			sum, err := s.repo.summary(ctx, tx, id)
			if err != nil {
				return err
			}
			available := sum.ExpectedNIO
			if in.Currency == "USD" {
				available = sum.ExpectedUSD
			}
			if short, err := lessThan(ctx, tx, available, in.Amount); err != nil {
				return err
			} else if short {
				return ErrInsufficientCash
			}
		}
		if err := s.repo.insertMovement(ctx, tx, id, actor.UserID, in); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{UserID: &actor.UserID, Action: "cash_shifts.movement", Entity: "cash_shifts", EntityID: &id, After: in})
	})
}

// Close stores the count; a difference in either currency emails the supervisors.
func (s *Service) Close(ctx context.Context, actor auth.Principal, id uuid.UUID, in CloseInput) (Shift, error) {
	in.Notes = strings.TrimSpace(in.Notes)
	return s.withOpenShift(ctx, actor, id, func(tx pgx.Tx, shift Shift) error {
		sum, err := s.repo.summary(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.repo.close(ctx, tx, id, sum, in); err != nil {
			return err
		}
		closed, err := s.repo.shift(ctx, tx, id, false)
		if err != nil {
			return err
		}
		if *closed.DifferenceNIO != "0.00" || *closed.DifferenceUSD != "0.00" {
			if err := s.notifyDifference(ctx, tx, closed); err != nil {
				return err
			}
		}
		return audit.Record(ctx, tx, audit.Entry{UserID: &actor.UserID, Action: "cash_shifts.closed", Entity: "cash_shifts", EntityID: &id,
			After: map[string]any{"expectedNio": closed.ExpectedNIO, "countedNio": closed.CountedNIO, "differenceNio": closed.DifferenceNIO,
				"expectedUsd": closed.ExpectedUSD, "countedUsd": closed.CountedUSD, "differenceUsd": closed.DifferenceUSD}})
	})
}

// withOpenShift locks the shift, checks ownership (or supervision) and that it is open,
// runs fn and returns the refreshed detail.
func (s *Service) withOpenShift(ctx context.Context, actor auth.Principal, id uuid.UUID, fn func(pgx.Tx, Shift) error) (Shift, error) {
	var result Shift
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		shift, err := s.repo.shift(ctx, tx, id, true)
		if err != nil {
			return err
		}
		if shift.User.ID != actor.UserID && !actor.Can("cash_shifts", "authorize") {
			return apperr.ErrNotFound
		}
		if shift.Status != "open" {
			return ErrShiftClosed
		}
		if err := fn(tx, shift); err != nil {
			return err
		}
		result, err = s.detail(ctx, tx, actor, id)
		return err
	})
	return result, err
}

func (s *Service) notifyDifference(ctx context.Context, db database.DBTX, shift Shift) error {
	business, err := settings.GetBusiness(ctx, db)
	if err != nil {
		return err
	}
	recipients, err := mail.RecipientsWithPermission(ctx, db, "cash_shifts", "authorize")
	if err != nil {
		return err
	}
	for _, r := range recipients {
		err := mail.Enqueue(ctx, db, mail.TemplateCashDifference, r.Email, map[string]any{
			"StoreName": business.Name, "Name": r.Name, "Cashier": shift.User.Name, "Register": shift.Register.Name,
			"ExpectedNIO": *shift.ExpectedNIO, "CountedNIO": *shift.CountedNIO, "DifferenceNIO": *shift.DifferenceNIO,
			"ExpectedUSD": *shift.ExpectedUSD, "CountedUSD": *shift.CountedUSD, "DifferenceUSD": *shift.DifferenceUSD,
			"Notes": shift.Notes, "ShiftURL": fmt.Sprintf("%s/turnos/%s", s.appBaseURL, shift.ID),
		})
		if err != nil {
			return err
		}
	}
	return nil
}
