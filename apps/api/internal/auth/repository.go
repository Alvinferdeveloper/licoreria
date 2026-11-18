package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"licoreria/api/internal/httpx"
	"licoreria/api/internal/platform/database"
)

// Repository holds the auth SQL. Methods take a DBTX so the service decides the transaction scope.
type Repository struct{}

type credentials struct {
	UserID             uuid.UUID
	FullName           string
	Email              string
	PasswordHash       string
	Active             bool
	MustChangePassword bool
	PasswordExpiresAt  *time.Time
	LockedUntil        *time.Time
	Activated          bool
}

// findCredentials looks a user up by username or email (both case-insensitive).
func (Repository) findCredentials(ctx context.Context, db database.DBTX, identifier string) (credentials, bool, error) {
	var c credentials
	err := db.QueryRow(ctx, `
		SELECT id, full_name, COALESCE(email::text, ''), password_hash, active, must_change_password,
		       password_expires_at, locked_until, activated_at IS NOT NULL
		FROM users
		WHERE username = $1 OR email = $1`, identifier).
		Scan(&c.UserID, &c.FullName, &c.Email, &c.PasswordHash, &c.Active, &c.MustChangePassword,
			&c.PasswordExpiresAt, &c.LockedUntil, &c.Activated)
	if errors.Is(err, pgx.ErrNoRows) {
		return credentials{}, false, nil
	}
	if err != nil {
		return credentials{}, false, fmt.Errorf("find credentials: %w", err)
	}
	return c, true, nil
}

// recordFailedLogin increments the counter and locks the account once it reaches maxAttempts.
// It returns the lock expiry when this attempt triggered a lock.
func (Repository) recordFailedLogin(ctx context.Context, db database.DBTX, userID uuid.UUID, maxAttempts int, lockout time.Duration) (*time.Time, error) {
	var lockedUntil *time.Time
	err := db.QueryRow(ctx, `
		UPDATE users
		SET failed_attempts = CASE WHEN failed_attempts + 1 >= $2 THEN 0 ELSE failed_attempts + 1 END,
		    locked_until    = CASE WHEN failed_attempts + 1 >= $2 THEN now() + make_interval(secs => $3) ELSE locked_until END
		WHERE id = $1
		RETURNING CASE WHEN locked_until > now() THEN locked_until END`,
		userID, maxAttempts, lockout.Seconds()).Scan(&lockedUntil)
	if err != nil {
		return nil, fmt.Errorf("record failed login: %w", err)
	}
	return lockedUntil, nil
}

func (Repository) recordSuccessfulLogin(ctx context.Context, db database.DBTX, userID uuid.UUID) error {
	_, err := db.Exec(ctx, `
		UPDATE users SET failed_attempts = 0, locked_until = NULL, last_login_at = now() WHERE id = $1`, userID)
	if err != nil {
		return fmt.Errorf("record successful login: %w", err)
	}
	return nil
}

func (Repository) createSession(ctx context.Context, db database.DBTX, userID uuid.UUID, tokenHash string, expiresAt time.Time) (uuid.UUID, error) {
	client := httpx.Client(ctx)
	var id uuid.UUID
	err := db.QueryRow(ctx, `
		INSERT INTO refresh_sessions (user_id, token_hash, expires_at, ip, user_agent)
		VALUES ($1, $2, $3, NULLIF($4, '')::inet, NULLIF($5, ''))
		RETURNING id`, userID, tokenHash, expiresAt, client.IP, client.UserAgent).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create session: %w", err)
	}
	return id, nil
}

type session struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	ExpiresAt time.Time
	RevokedAt *time.Time
}

// lockSessionByHash locks the row so two concurrent refreshes can't both rotate the same token.
func (Repository) lockSessionByHash(ctx context.Context, db database.DBTX, tokenHash string) (session, bool, error) {
	var s session
	err := db.QueryRow(ctx, `
		SELECT id, user_id, expires_at, revoked_at FROM refresh_sessions WHERE token_hash = $1 FOR UPDATE`, tokenHash).
		Scan(&s.ID, &s.UserID, &s.ExpiresAt, &s.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return session{}, false, nil
	}
	if err != nil {
		return session{}, false, fmt.Errorf("find session: %w", err)
	}
	return s, true, nil
}

func (Repository) revokeSession(ctx context.Context, db database.DBTX, sessionID uuid.UUID) error {
	_, err := db.Exec(ctx, `UPDATE refresh_sessions SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`, sessionID)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

// revokeAllSessions ends every active session of the user except keep (pass uuid.Nil to end all).
func (Repository) revokeAllSessions(ctx context.Context, db database.DBTX, userID, keep uuid.UUID) error {
	_, err := db.Exec(ctx, `
		UPDATE refresh_sessions SET revoked_at = now()
		WHERE user_id = $1 AND revoked_at IS NULL AND id <> $2`, userID, keep)
	if err != nil {
		return fmt.Errorf("revoke sessions: %w", err)
	}
	return nil
}

// loadPrincipal returns the user behind an active session, or ok=false when the session
// was revoked, expired or the user was deactivated.
func (Repository) loadPrincipal(ctx context.Context, db database.DBTX, userID, sessionID uuid.UUID) (Principal, bool, error) {
	p := Principal{SessionID: sessionID}
	err := db.QueryRow(ctx, `
		SELECT u.id, u.full_name, COALESCE(u.email::text, ''), u.username, u.must_change_password,
		       r.id, r.code, r.name,
		       COALESCE(array_agg(p.module || ':' || p.action ORDER BY p.module, p.action)
		                FILTER (WHERE p.id IS NOT NULL), '{}')
		FROM users u
		JOIN refresh_sessions s ON s.user_id = u.id AND s.id = $2 AND s.revoked_at IS NULL AND s.expires_at > now()
		JOIN roles r ON r.id = u.role_id
		LEFT JOIN role_permissions rp ON rp.role_id = r.id
		LEFT JOIN permissions p ON p.id = rp.permission_id
		WHERE u.id = $1 AND u.active
		GROUP BY u.id, r.id`, userID, sessionID).
		Scan(&p.UserID, &p.FullName, &p.Email, &p.Username, &p.MustChangePassword,
			&p.Role.ID, &p.Role.Code, &p.Role.Name, &p.Permissions)
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, false, nil
	}
	if err != nil {
		return Principal{}, false, fmt.Errorf("load principal: %w", err)
	}
	return p, true, nil
}

func (Repository) passwordHash(ctx context.Context, db database.DBTX, userID uuid.UUID) (string, error) {
	var hash string
	if err := db.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, userID).Scan(&hash); err != nil {
		return "", fmt.Errorf("load password hash: %w", err)
	}
	return hash, nil
}

// recentPasswordHashes returns the current hash plus the latest history entries, newest first.
func (Repository) recentPasswordHashes(ctx context.Context, db database.DBTX, userID uuid.UUID, limit int) ([]string, error) {
	rows, err := db.Query(ctx, `
		SELECT password_hash FROM (
			(SELECT password_hash, now() AS created_at FROM users WHERE id = $1)
			UNION ALL
			(SELECT password_hash, created_at FROM password_history WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2)
		) recent
		ORDER BY created_at DESC`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("load password history: %w", err)
	}
	hashes, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("read password history: %w", err)
	}
	return hashes, nil
}

// setPassword stores the new hash, archives the previous one and clears temporary-password state.
func (Repository) setPassword(ctx context.Context, db database.DBTX, userID uuid.UUID, newHash string) error {
	_, err := db.Exec(ctx, `
		WITH previous AS (
			INSERT INTO password_history (user_id, password_hash)
			SELECT id, password_hash FROM users WHERE id = $1
		)
		UPDATE users
		SET password_hash = $2, must_change_password = false, password_expires_at = NULL,
		    failed_attempts = 0, locked_until = NULL, activated_at = COALESCE(activated_at, now())
		WHERE id = $1`, userID, newHash)
	if err != nil {
		return fmt.Errorf("set password: %w", err)
	}
	return nil
}

func (Repository) findActiveUserByEmail(ctx context.Context, db database.DBTX, email string) (uuid.UUID, string, bool, error) {
	var id uuid.UUID
	var name string
	err := db.QueryRow(ctx, `SELECT id, full_name FROM users WHERE email = $1 AND active`, email).Scan(&id, &name)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, "", false, nil
	}
	if err != nil {
		return uuid.Nil, "", false, fmt.Errorf("find user by email: %w", err)
	}
	return id, name, true, nil
}

// createAuthToken invalidates earlier unused tokens of the same purpose, so only the latest link works.
func (Repository) createAuthToken(ctx context.Context, db database.DBTX, userID uuid.UUID, tokenHash, purpose string, expiresAt time.Time) error {
	if _, err := db.Exec(ctx, `
		UPDATE auth_tokens SET used_at = now()
		WHERE user_id = $1 AND purpose = $2 AND used_at IS NULL`, userID, purpose); err != nil {
		return fmt.Errorf("invalidate previous tokens: %w", err)
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO auth_tokens (user_id, token_hash, purpose, expires_at) VALUES ($1, $2, $3, $4)`,
		userID, tokenHash, purpose, expiresAt); err != nil {
		return fmt.Errorf("create auth token: %w", err)
	}
	return nil
}

// consumeAuthToken marks a valid token as used and returns its user; ok is false if it is unknown,
// used, expired, belongs to an inactive user or, when owner is set, to someone else.
func (Repository) consumeAuthToken(ctx context.Context, db database.DBTX, tokenHash, purpose string, owner *uuid.UUID) (uuid.UUID, bool, error) {
	var userID uuid.UUID
	err := db.QueryRow(ctx, `
		UPDATE auth_tokens t SET used_at = now()
		FROM users u
		WHERE u.id = t.user_id AND u.active
		  AND t.token_hash = $1 AND t.purpose = $2 AND t.used_at IS NULL AND t.expires_at > now()
		  AND ($3::uuid IS NULL OR t.user_id = $3)
		RETURNING t.user_id`, tokenHash, purpose, owner).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("consume auth token: %w", err)
	}
	return userID, true, nil
}
