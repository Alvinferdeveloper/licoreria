package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/audit"
	"licoreria/api/internal/platform/database"
	"licoreria/api/internal/platform/password"
	"licoreria/api/internal/settings"
)

const purposeActivation = "activation"

// Code alphabet without look-alike characters (0/O, 1/I/L) so it can be read aloud and typed.
const (
	codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
	codeLength   = 8
)

var ErrInvalidActivation = apperr.New(apperr.KindInvalid, "invalid_activation",
	"El enlace o el código no es válido o ya venció. Pide al administrador uno nuevo.")

// NewActivationCode returns a code formatted as "ABCD-EFGH" and the hash to store.
func NewActivationCode() (display, hash string, err error) {
	chars := make([]byte, codeLength)
	for i := range chars {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(codeAlphabet))))
		if err != nil {
			return "", "", fmt.Errorf("generate activation code: %w", err)
		}
		chars[i] = codeAlphabet[n.Int64()]
	}
	plain := string(chars)
	return plain[:4] + "-" + plain[4:], HashToken(plain), nil
}

// normalizeCode accepts the code with or without dash, spaces or lowercase.
func normalizeCode(code string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r - 'a' + 'A'
		case (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
			return r
		}
		return -1
	}, code)
}

// IssueActivation stores a link token or counter code hash, replacing earlier unused ones.
func IssueActivation(ctx context.Context, db database.DBTX, userID uuid.UUID, secretHash string, expiresAt time.Time) error {
	return Repository{}.createAuthToken(ctx, db, userID, secretHash, purposeActivation, expiresAt)
}

// ActivationInput carries either an emailed Token, or a Username plus counter Code.
type ActivationInput struct {
	Token       string
	Username    string
	Code        string
	NewPassword string
}

// Activate lets a user choose their password with a one-time link or code. Wrong codes
// count toward the account lockout, like wrong passwords.
func (s *Service) Activate(ctx context.Context, in ActivationInput) error {
	if err := password.CheckPolicy("newPassword", in.NewPassword); err != nil {
		return err
	}

	var owner *uuid.UUID
	secretHash := HashToken(in.Token)
	if in.Token == "" {
		creds, found, err := s.repo.findCredentials(ctx, s.pool, strings.TrimSpace(in.Username))
		if err != nil {
			return err
		}
		if !found || !creds.Active {
			return ErrInvalidActivation
		}
		if now := s.now(); creds.LockedUntil != nil && creds.LockedUntil.After(now) {
			return accountLocked(*creds.LockedUntil, now)
		}
		owner = &creds.UserID
		secretHash = HashToken(normalizeCode(in.Code))
	}

	newHash, err := password.Hash(in.NewPassword)
	if err != nil {
		return err
	}
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		userID, ok, err := s.repo.consumeAuthToken(ctx, tx, secretHash, purposeActivation, owner)
		if err != nil {
			return err
		}
		if !ok {
			return ErrInvalidActivation
		}
		if err := s.checkNewPassword(ctx, userID, in.NewPassword); err != nil {
			return err
		}
		if err := s.repo.setPassword(ctx, tx, userID, newHash); err != nil {
			return err
		}
		if err := s.repo.revokeAllSessions(ctx, tx, userID, uuid.Nil); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{
			UserID: &userID, Action: audit.ActionAccountActivated, Entity: "users", EntityID: &userID,
		})
	})
	if errors.Is(err, ErrInvalidActivation) && owner != nil {
		security, serr := settings.GetSecurity(ctx, s.pool)
		if serr != nil {
			return serr
		}
		lockedUntil, lerr := s.repo.recordFailedLogin(ctx, s.pool, *owner, security.MaxFailedAttempts, security.Lockout())
		if lerr != nil {
			return lerr
		}
		if lockedUntil != nil {
			return accountLocked(*lockedUntil, s.now())
		}
	}
	return err
}
