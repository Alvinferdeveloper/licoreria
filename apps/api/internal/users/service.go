package users

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/audit"
	"licoreria/api/internal/httpx"
	"licoreria/api/internal/mail"
	"licoreria/api/internal/platform/database"
	"licoreria/api/internal/platform/password"
	"licoreria/api/internal/settings"
)

var (
	errUnknownRole = apperr.Validation(apperr.FieldError{
		Field: "roleId", Code: "invalid_reference", Message: "El rol seleccionado no existe.",
	})
	ErrSelfDeactivation = apperr.New(apperr.KindConflict, "self_deactivation", "No puedes desactivar tu propia cuenta.")
	ErrSelfRoleChange   = apperr.New(apperr.KindConflict, "self_role_change", "No puedes cambiar tu propio rol.")
	ErrLastAdmin        = apperr.New(apperr.KindConflict, "last_admin", "Debe quedar al menos un administrador activo.")
	ErrInactiveUser     = apperr.New(apperr.KindConflict, "user_inactive", "Activa la cuenta antes de reenviar el acceso.")
)

const adminRoleCode = "admin"

type Service struct {
	pool       *pgxpool.Pool
	repo       Repository
	appBaseURL string
	now        func() time.Time
}

func NewService(pool *pgxpool.Pool, appBaseURL string, now func() time.Time) *Service {
	return &Service{pool: pool, appBaseURL: appBaseURL, now: now}
}

func (s *Service) List(ctx context.Context, f Filter, page httpx.Page) ([]User, int64, error) {
	return s.repo.list(ctx, s.pool, f, page)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (User, error) {
	return s.repo.get(ctx, s.pool, id)
}

func normalize(in Input) Input {
	return Input{
		FullName: strings.Join(strings.Fields(in.FullName), " "),
		Email:    strings.ToLower(strings.TrimSpace(in.Email)),
		Username: strings.ToLower(strings.TrimSpace(in.Username)),
		RoleID:   in.RoleID,
	}
}

// Create adds a user with a temporary password and queues the welcome email in the same transaction.
func (s *Service) Create(ctx context.Context, actor uuid.UUID, in Input) (User, error) {
	in = normalize(in)
	if _, err := s.repo.roleCode(ctx, s.pool, in.RoleID); err != nil {
		return User{}, err
	}

	var created User
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		temp, hash, expiresAt, err := s.temporaryPassword(ctx, tx)
		if err != nil {
			return err
		}
		id, err := s.repo.create(ctx, tx, in, hash, expiresAt)
		if err != nil {
			return err
		}
		if created, err = s.repo.get(ctx, tx, id); err != nil {
			return err
		}
		if err := s.sendAccess(ctx, tx, created, temp, expiresAt); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{
			UserID: &actor, Action: audit.ActionUserCreated, Entity: "users", EntityID: &id, After: created,
		})
	})
	return created, err
}

func (s *Service) Update(ctx context.Context, actor, id uuid.UUID, in Input) (User, error) {
	in = normalize(in)
	newRole, err := s.repo.roleCode(ctx, s.pool, in.RoleID)
	if err != nil {
		return User{}, err
	}

	var updated User
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		before, err := s.repo.get(ctx, tx, id)
		if err != nil {
			return err
		}
		if before.Role.ID != in.RoleID {
			if id == actor {
				return ErrSelfRoleChange
			}
			if isAdmin(before.Role.Code) && !isAdmin(newRole) && before.Active {
				if err := s.ensureAnotherAdmin(ctx, tx, id); err != nil {
					return err
				}
			}
		}
		if err := s.repo.update(ctx, tx, id, in); err != nil {
			return err
		}
		if updated, err = s.repo.get(ctx, tx, id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{
			UserID: &actor, Action: audit.ActionUserUpdated, Entity: "users", EntityID: &id, Before: before, After: updated,
		})
	})
	return updated, err
}

// SetActive enables or disables an account; disabling it also ends all its sessions.
func (s *Service) SetActive(ctx context.Context, actor, id uuid.UUID, active bool) (User, error) {
	if id == actor && !active {
		return User{}, ErrSelfDeactivation
	}

	var updated User
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		before, err := s.repo.get(ctx, tx, id)
		if err != nil {
			return err
		}
		if before.Active == active {
			updated = before
			return nil
		}
		if !active && isAdmin(before.Role.Code) {
			if err := s.ensureAnotherAdmin(ctx, tx, id); err != nil {
				return err
			}
		}
		if err := s.repo.setActive(ctx, tx, id, active); err != nil {
			return err
		}
		if !active {
			if err := s.repo.revokeSessions(ctx, tx, id); err != nil {
				return err
			}
		}
		updated = before
		updated.Active = active
		return audit.Record(ctx, tx, audit.Entry{
			UserID: &actor, Action: audit.ActionUserStatusChanged, Entity: "users", EntityID: &id,
			Before: map[string]bool{"active": before.Active}, After: map[string]bool{"active": active},
		})
	})
	return updated, err
}

// ResendAccess issues a fresh temporary password, for a lost email or an expired temporary password.
func (s *Service) ResendAccess(ctx context.Context, actor, id uuid.UUID) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		user, err := s.repo.get(ctx, tx, id)
		if err != nil {
			return err
		}
		if !user.Active {
			return ErrInactiveUser
		}
		temp, hash, expiresAt, err := s.temporaryPassword(ctx, tx)
		if err != nil {
			return err
		}
		if err := s.repo.setTemporaryPassword(ctx, tx, id, hash, expiresAt); err != nil {
			return err
		}
		if err := s.repo.revokeSessions(ctx, tx, id); err != nil {
			return err
		}
		if err := s.sendAccess(ctx, tx, user, temp, expiresAt); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{
			UserID: &actor, Action: audit.ActionUserAccessResent, Entity: "users", EntityID: &id,
		})
	})
}

// CreateInitialAdmin bootstraps the first administrator from the CLI and returns the
// temporary password so it can be shown once on the terminal.
func (s *Service) CreateInitialAdmin(ctx context.Context, fullName, email, username string) (User, string, error) {
	var adminRoleID uuid.UUID
	if err := s.pool.QueryRow(ctx, `SELECT id FROM roles WHERE code = $1`, adminRoleCode).Scan(&adminRoleID); err != nil {
		return User{}, "", database.MapError(err, nil)
	}
	in := normalize(Input{FullName: fullName, Email: email, Username: username, RoleID: adminRoleID})

	var created User
	var temp string
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var hash string
		var expiresAt time.Time
		var err error
		if temp, hash, expiresAt, err = s.temporaryPassword(ctx, tx); err != nil {
			return err
		}
		id, err := s.repo.create(ctx, tx, in, hash, expiresAt)
		if err != nil {
			return err
		}
		if created, err = s.repo.get(ctx, tx, id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{Action: audit.ActionUserCreated, Entity: "users", EntityID: &id, After: created})
	})
	return created, temp, err
}

func (s *Service) temporaryPassword(ctx context.Context, db database.DBTX) (plain, hash string, expiresAt time.Time, err error) {
	security, err := settings.GetSecurity(ctx, db)
	if err != nil {
		return "", "", time.Time{}, err
	}
	if plain, err = password.GenerateTemporary(); err != nil {
		return "", "", time.Time{}, err
	}
	if hash, err = password.Hash(plain); err != nil {
		return "", "", time.Time{}, err
	}
	return plain, hash, s.now().Add(security.TempPasswordTTL()), nil
}

func (s *Service) sendAccess(ctx context.Context, db database.DBTX, u User, temp string, expiresAt time.Time) error {
	business, err := settings.GetBusiness(ctx, db)
	if err != nil {
		return err
	}
	return mail.Enqueue(ctx, db, mail.TemplateWelcome, u.Email, map[string]any{
		"StoreName":         business.Name,
		"Name":              u.FullName,
		"Username":          u.Username,
		"TemporaryPassword": temp,
		"LoginURL":          s.appBaseURL + "/login",
		"ExpiresInHours":    int(expiresAt.Sub(s.now()).Round(time.Hour).Hours()),
	})
}

func (s *Service) ensureAnotherAdmin(ctx context.Context, db database.DBTX, excluded uuid.UUID) error {
	count, err := s.repo.countOtherActiveAdmins(ctx, db, excluded)
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrLastAdmin
	}
	return nil
}

func isAdmin(code *string) bool { return code != nil && *code == adminRoleCode }
