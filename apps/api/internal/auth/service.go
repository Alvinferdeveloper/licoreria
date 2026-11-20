package auth

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/audit"
	"licoreria/api/internal/mail"
	"licoreria/api/internal/platform/config"
	"licoreria/api/internal/platform/database"
	"licoreria/api/internal/platform/logx"
	"licoreria/api/internal/platform/password"
	"licoreria/api/internal/schedule"
	"licoreria/api/internal/settings"
)

// passwordReuseWindow is how many recent passwords (current included) cannot be reused.
const passwordReuseWindow = 3

const purposePasswordReset = "password_reset"

var (
	ErrInvalidCredentials = apperr.New(apperr.KindUnauthorized, "invalid_credentials", "Usuario o contraseña incorrectos.")
	ErrTemporaryExpired   = apperr.New(apperr.KindUnauthorized, "temporary_password_expired", "Tu contraseña temporal venció. Pide al administrador que te reenvíe el acceso.")
	ErrSessionExpired     = apperr.New(apperr.KindUnauthorized, "session_expired", "Tu sesión terminó. Inicia sesión de nuevo.")
	ErrInvalidResetToken  = apperr.New(apperr.KindInvalid, "invalid_reset_token", "El enlace no es válido o ya venció. Solicita uno nuevo.")

	ErrWrongPassword = apperr.Validation(apperr.FieldError{
		Field: "currentPassword", Code: "incorrect", Message: "La contraseña actual no es correcta.",
	})
	ErrPasswordReused = apperr.Validation(apperr.FieldError{
		Field: "newPassword", Code: "reused", Message: "No puedes repetir ninguna de tus últimas 3 contraseñas.",
	})
)

func accountLocked(until time.Time, now time.Time) *apperr.Error {
	minutes := int(math.Ceil(until.Sub(now).Minutes()))
	return apperr.New(apperr.KindTooManyRequests, "account_locked",
		fmt.Sprintf("Cuenta bloqueada por intentos fallidos. Intenta de nuevo en %d minutos.", max(minutes, 1)))
}

type Service struct {
	pool       *pgxpool.Pool
	repo       Repository
	tokens     *TokenIssuer
	cfg        config.AuthConfig
	appBaseURL string
	now        func() time.Time
}

func NewService(pool *pgxpool.Pool, cfg config.AuthConfig, appBaseURL string, now func() time.Time) *Service {
	return &Service{
		pool:       pool,
		tokens:     NewTokenIssuer(cfg.JWTSecret, cfg.AccessTokenTTL, now),
		cfg:        cfg,
		appBaseURL: appBaseURL,
		now:        now,
	}
}

// Session is what a successful login or refresh hands back to the client.
type Session struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
	Principal        Principal
}

func (s *Service) Login(ctx context.Context, identifier, plainPassword string) (Session, error) {
	identifier = strings.TrimSpace(identifier)
	security, err := settings.GetSecurity(ctx, s.pool)
	if err != nil {
		return Session{}, err
	}

	creds, found, err := s.repo.findCredentials(ctx, s.pool, identifier)
	if err != nil {
		return Session{}, err
	}
	if !found {
		// Verify anyway so unknown users take as long as known ones.
		_, _ = password.Verify(plainPassword, password.DummyHash)
		s.auditFailure(ctx, nil, identifier, "unknown_user")
		return Session{}, ErrInvalidCredentials
	}
	if !creds.Activated {
		_, _ = password.Verify(plainPassword, password.DummyHash)
		s.auditFailure(ctx, &creds.UserID, identifier, "pending_activation")
		return Session{}, ErrInvalidCredentials
	}

	now := s.now()
	if creds.LockedUntil != nil && creds.LockedUntil.After(now) {
		return Session{}, accountLocked(*creds.LockedUntil, now)
	}

	ok, err := password.Verify(plainPassword, creds.PasswordHash)
	if err != nil {
		return Session{}, fmt.Errorf("verify password: %w", err)
	}
	if !ok || !creds.Active {
		return Session{}, s.handleFailedLogin(ctx, creds, security, identifier, ok)
	}
	if creds.MustChangePassword && creds.PasswordExpiresAt != nil && creds.PasswordExpiresAt.Before(now) {
		s.auditFailure(ctx, &creds.UserID, identifier, "temporary_password_expired")
		return Session{}, ErrTemporaryExpired
	}

	hours, err := schedule.Evaluate(ctx, s.pool, creds.UserID, now)
	if err != nil {
		return Session{}, err
	}
	outsideBypass := false
	if hours.Outside && !hours.Extended {
		if outsideBypass, err = s.repo.userCan(ctx, s.pool, creds.UserID, "schedule", "authorize"); err != nil {
			return Session{}, err
		}
		if !outsideBypass {
			s.auditFailure(ctx, &creds.UserID, identifier, "outside_hours")
			return Session{}, schedule.LoginClosed(hours.NextOpen, now)
		}
	}

	var session Session
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.repo.recordSuccessfulLogin(ctx, tx, creds.UserID); err != nil {
			return err
		}
		session, err = s.startSession(ctx, tx, creds.UserID)
		if err != nil {
			return err
		}
		if err := audit.Record(ctx, tx, audit.Entry{
			UserID: &creds.UserID, Action: audit.ActionLoginSucceeded, Entity: "users", EntityID: &creds.UserID,
		}); err != nil {
			return err
		}
		if !outsideBypass {
			return nil
		}
		return audit.Record(ctx, tx, audit.Entry{
			UserID: &creds.UserID, Action: audit.ActionLoginOutsideHours, Entity: "users", EntityID: &creds.UserID,
		})
	})
	if err != nil {
		return Session{}, err
	}
	logx.From(ctx).Info("login succeeded", "user_id", creds.UserID)
	return session, nil
}

// handleFailedLogin counts wrong passwords toward the lockout. Inactive accounts get the
// same generic error so the response doesn't reveal the account's state.
func (s *Service) handleFailedLogin(ctx context.Context, creds credentials, security settings.Security, identifier string, passwordOK bool) error {
	if passwordOK {
		s.auditFailure(ctx, &creds.UserID, identifier, "inactive_user")
		return ErrInvalidCredentials
	}

	lockedUntil, err := s.repo.recordFailedLogin(ctx, s.pool, creds.UserID, security.MaxFailedAttempts, security.Lockout())
	if err != nil {
		return err
	}
	s.auditFailure(ctx, &creds.UserID, identifier, "wrong_password")
	if lockedUntil == nil {
		return ErrInvalidCredentials
	}

	logx.From(ctx).Warn("account locked", "user_id", creds.UserID, "until", *lockedUntil)
	if err := audit.Record(ctx, s.pool, audit.Entry{
		UserID: &creds.UserID, Action: audit.ActionAccountLocked, Entity: "users", EntityID: &creds.UserID,
		After: map[string]any{"lockedUntil": lockedUntil},
	}); err != nil {
		logx.From(ctx).Error("audit account lock", "error", err)
	}
	return accountLocked(*lockedUntil, s.now())
}

// auditFailure logs failed logins without letting an audit error mask the login response.
func (s *Service) auditFailure(ctx context.Context, userID *uuid.UUID, identifier, reason string) {
	logx.From(ctx).Warn("login failed", "reason", reason, "user_id", userID)
	if err := audit.Record(ctx, s.pool, audit.Entry{
		UserID: userID, Action: audit.ActionLoginFailed, Entity: "users", EntityID: userID,
		After: map[string]string{"identifier": identifier, "reason": reason},
	}); err != nil {
		logx.From(ctx).Error("audit failed login", "error", err)
	}
}

func (s *Service) startSession(ctx context.Context, db database.DBTX, userID uuid.UUID) (Session, error) {
	refreshPlain, refreshHash, err := NewOpaqueToken()
	if err != nil {
		return Session{}, err
	}
	refreshExpiresAt := s.now().Add(s.cfg.RefreshTokenTTL)
	sessionID, err := s.repo.createSession(ctx, db, userID, refreshHash, refreshExpiresAt)
	if err != nil {
		return Session{}, err
	}

	accessToken, accessExpiresAt, err := s.tokens.Issue(userID, sessionID)
	if err != nil {
		return Session{}, err
	}
	principal, ok, err := s.repo.loadPrincipal(ctx, db, userID, sessionID)
	if err != nil {
		return Session{}, err
	}
	if !ok {
		return Session{}, ErrSessionExpired
	}
	return Session{
		AccessToken: accessToken, AccessExpiresAt: accessExpiresAt,
		RefreshToken: refreshPlain, RefreshExpiresAt: refreshExpiresAt,
		Principal: principal,
	}, nil
}

// Refresh rotates the refresh token. Presenting an already-rotated token means it was
// copied, so every session of that user is revoked.
func (s *Service) Refresh(ctx context.Context, refreshPlain string) (Session, error) {
	if refreshPlain == "" {
		return Session{}, ErrSessionExpired
	}

	var session Session
	var reused *uuid.UUID
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		current, found, err := s.repo.lockSessionByHash(ctx, tx, HashToken(refreshPlain))
		if err != nil {
			return err
		}
		if !found {
			return ErrSessionExpired
		}
		if current.RevokedAt != nil {
			if err := s.repo.revokeAllSessions(ctx, tx, current.UserID, uuid.Nil); err != nil {
				return err
			}
			reused = &current.UserID
			return audit.Record(ctx, tx, audit.Entry{
				UserID: &current.UserID, Action: audit.ActionTokenReuse, Entity: "refresh_sessions", EntityID: &current.ID,
			})
		}
		if !current.ExpiresAt.After(s.now()) {
			return ErrSessionExpired
		}

		if err := s.repo.revokeSession(ctx, tx, current.ID); err != nil {
			return err
		}
		session, err = s.startSession(ctx, tx, current.UserID)
		return err
	})
	if reused != nil {
		logx.From(ctx).Warn("refresh token reuse detected, all sessions revoked", "user_id", *reused)
		return Session{}, ErrSessionExpired
	}
	if err != nil {
		return Session{}, err
	}
	return session, nil
}

// Logout revokes the session behind the refresh token; unknown tokens are ignored.
func (s *Service) Logout(ctx context.Context, refreshPlain string) error {
	if refreshPlain == "" {
		return nil
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		current, found, err := s.repo.lockSessionByHash(ctx, tx, HashToken(refreshPlain))
		if err != nil || !found || current.RevokedAt != nil {
			return err
		}
		if err := s.repo.revokeSession(ctx, tx, current.ID); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{
			UserID: &current.UserID, Action: audit.ActionLogout, Entity: "refresh_sessions", EntityID: &current.ID,
		})
	})
}

// Authenticate resolves an access token to the user behind a still-active session.
func (s *Service) Authenticate(ctx context.Context, accessToken string) (Principal, error) {
	claims, err := s.tokens.Parse(accessToken)
	if err != nil {
		return Principal{}, apperr.ErrUnauthorized
	}
	principal, ok, err := s.repo.loadPrincipal(ctx, s.pool, claims.UserID, claims.SessionID)
	if err != nil {
		return Principal{}, err
	}
	if !ok {
		return Principal{}, ErrSessionExpired
	}
	return principal, nil
}

// ChangePassword keeps the current session and signs out every other device.
func (s *Service) ChangePassword(ctx context.Context, p Principal, current, next string) error {
	hash, err := s.repo.passwordHash(ctx, s.pool, p.UserID)
	if err != nil {
		return err
	}
	ok, err := password.Verify(current, hash)
	if err != nil {
		return fmt.Errorf("verify password: %w", err)
	}
	if !ok {
		return ErrWrongPassword
	}
	if err := s.checkNewPassword(ctx, p.UserID, next); err != nil {
		return err
	}

	newHash, err := password.Hash(next)
	if err != nil {
		return err
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.repo.setPassword(ctx, tx, p.UserID, newHash); err != nil {
			return err
		}
		if err := s.repo.revokeAllSessions(ctx, tx, p.UserID, p.SessionID); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{
			UserID: &p.UserID, Action: audit.ActionPasswordChanged, Entity: "users", EntityID: &p.UserID,
			After: map[string]bool{"wasTemporary": p.MustChangePassword},
		})
	})
}

func (s *Service) checkNewPassword(ctx context.Context, userID uuid.UUID, next string) error {
	if err := password.CheckPolicy("newPassword", next); err != nil {
		return err
	}
	recent, err := s.repo.recentPasswordHashes(ctx, s.pool, userID, passwordReuseWindow-1)
	if err != nil {
		return err
	}
	for _, old := range recent {
		if reused, _ := password.Verify(next, old); reused {
			return ErrPasswordReused
		}
	}
	return nil
}

// RequestPasswordReset emails a one-time link. It succeeds silently for unknown
// emails so the endpoint can't be used to discover accounts.
func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	userID, name, found, err := s.repo.findActiveUserByEmail(ctx, s.pool, strings.TrimSpace(email))
	if err != nil || !found {
		return err
	}

	plain, hash, err := NewOpaqueToken()
	if err != nil {
		return err
	}
	business, err := settings.GetBusiness(ctx, s.pool)
	if err != nil {
		return err
	}

	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.repo.createAuthToken(ctx, tx, userID, hash, purposePasswordReset, s.now().Add(s.cfg.ResetTokenTTL)); err != nil {
			return err
		}
		return mail.Enqueue(ctx, tx, mail.TemplatePasswordReset, email, map[string]any{
			"StoreName":        business.Name,
			"Name":             name,
			"ResetURL":         s.appBaseURL + "/restablecer?token=" + plain,
			"ExpiresInMinutes": int(s.cfg.ResetTokenTTL.Minutes()),
		})
	})
}

// ResetPassword sets a new password from an emailed token and signs out every session.
func (s *Service) ResetPassword(ctx context.Context, token, next string) error {
	if err := password.CheckPolicy("newPassword", next); err != nil {
		return err
	}
	newHash, err := password.Hash(next)
	if err != nil {
		return err
	}

	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		userID, ok, err := s.repo.consumeAuthToken(ctx, tx, HashToken(token), purposePasswordReset, nil)
		if err != nil {
			return err
		}
		if !ok {
			return ErrInvalidResetToken
		}
		if err := s.checkNewPassword(ctx, userID, next); err != nil {
			return err
		}
		if err := s.repo.setPassword(ctx, tx, userID, newHash); err != nil {
			return err
		}
		if err := s.repo.revokeAllSessions(ctx, tx, userID, uuid.Nil); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{
			UserID: &userID, Action: audit.ActionPasswordReset, Entity: "users", EntityID: &userID,
		})
	})
}
