package auth

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/audit"
	"licoreria/api/internal/platform/database"
	"licoreria/api/internal/platform/password"
)

const (
	maxPINFailures   = 5
	pinLockoutWindow = "15 minutes"
)

var (
	pinPattern = regexp.MustCompile(`^\d{4,6}$`)

	ErrInvalidPIN = apperr.Validation(apperr.FieldError{Field: "pin", Code: "pin", Message: "El PIN debe tener de 4 a 6 dígitos."})
	// ErrAuthorizationFailed never says whether the user or the PIN was wrong.
	ErrAuthorizationFailed = apperr.New(apperr.KindForbidden, "authorization_failed", "Usuario o PIN incorrecto, o sin permiso para autorizar.")
	ErrAuthorizationLocked = apperr.New(apperr.KindTooManyRequests, "authorization_locked", "Demasiados intentos con ese usuario. Espera 15 minutos.")
)

// ConfirmPassword re-checks a signed-in user's password before a sensitive action.
func ConfirmPassword(ctx context.Context, db database.DBTX, userID uuid.UUID, plain string) error {
	hash, err := Repository{}.passwordHash(ctx, db, userID)
	if err != nil {
		return err
	}
	if ok, err := password.Verify(plain, hash); err != nil {
		return fmt.Errorf("verify password: %w", err)
	} else if !ok {
		return ErrWrongPassword
	}
	return nil
}

// SetPIN stores the caller's authorization PIN after confirming their password.
func (s *Service) SetPIN(ctx context.Context, p Principal, currentPassword, pin string) error {
	if !pinPattern.MatchString(pin) {
		return ErrInvalidPIN
	}
	if err := ConfirmPassword(ctx, s.pool, p.UserID, currentPassword); err != nil {
		return err
	}
	pinHash, err := password.Hash(pin)
	if err != nil {
		return err
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE users SET pin_hash = $2 WHERE id = $1`, p.UserID, pinHash); err != nil {
			return database.MapError(err, nil)
		}
		return audit.Record(ctx, tx, audit.Entry{UserID: &p.UserID, Action: "users.pin_changed", Entity: "users", EntityID: &p.UserID})
	})
}

// VerifyPIN checks that username + PIN belong to an active user allowed to module:action
// and returns that user's id. Failures are audited and lock the user after a few attempts,
// so call it with the pool before opening a transaction that could roll the audit back.
func VerifyPIN(ctx context.Context, db database.DBTX, username, pin, module, action string) (uuid.UUID, error) {
	var id uuid.UUID
	var pinHash *string
	var allowed bool
	err := db.QueryRow(ctx, `
		SELECT u.id, u.pin_hash, EXISTS (
			SELECT 1 FROM role_permissions rp JOIN permissions p ON p.id = rp.permission_id
			WHERE rp.role_id = u.role_id AND p.module = $2 AND p.action = $3)
		FROM users u WHERE u.username = $1 AND u.active`, username, module, action).Scan(&id, &pinHash, &allowed)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrAuthorizationFailed
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("load authorizer: %w", err)
	}

	var failures int
	if err := db.QueryRow(ctx, `
		SELECT count(*) FROM audit_log WHERE action = 'authorization.failed' AND entity_id = $1
		AND created_at > now() - $2::interval`, id, pinLockoutWindow).Scan(&failures); err != nil {
		return uuid.Nil, fmt.Errorf("count pin failures: %w", err)
	}
	if failures >= maxPINFailures {
		return uuid.Nil, ErrAuthorizationLocked
	}

	ok := false
	if pinHash != nil && allowed && pinPattern.MatchString(pin) {
		if ok, err = password.Verify(pin, *pinHash); err != nil {
			return uuid.Nil, fmt.Errorf("verify pin: %w", err)
		}
	}
	if !ok {
		_ = audit.Record(context.WithoutCancel(ctx), db, audit.Entry{Action: "authorization.failed", Entity: "users", EntityID: &id,
			After: map[string]string{"permission": Permission(module, action)}})
		return uuid.Nil, ErrAuthorizationFailed
	}
	return id, nil
}
