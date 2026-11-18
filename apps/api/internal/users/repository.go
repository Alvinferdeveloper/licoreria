// Package users manages accounts: creation with an invitation (emailed link or counter
// code), edits, activation status and new access codes. Users are deactivated, never deleted.
package users

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"licoreria/api/internal/httpx"
	"licoreria/api/internal/platform/database"
)

type User struct {
	ID                 uuid.UUID  `json:"id"`
	FullName           string     `json:"fullName"`
	Email              *string    `json:"email"`
	Username           string     `json:"username"`
	Role               RoleRef    `json:"role"`
	Active             bool       `json:"active"`
	MustChangePassword bool       `json:"mustChangePassword"`
	PendingActivation  bool       `json:"pendingActivation"`
	LastLoginAt        *time.Time `json:"lastLoginAt"`
	CreatedAt          time.Time  `json:"createdAt"`
}

type RoleRef struct {
	ID   uuid.UUID `json:"id"`
	Code *string   `json:"code"`
	Name string    `json:"name"`
}

type Filter struct {
	Search string
	RoleID *uuid.UUID
	Active *bool
}

type Input struct {
	FullName string
	Email    string
	Username string
	RoleID   uuid.UUID
}

// constraintFields points unique and foreign-key violations at the request field.
var constraintFields = database.ConstraintFields{
	"users_email_key":    "email",
	"users_username_key": "username",
	"users_role_id_fkey": "roleId",
}

type Repository struct{}

const selectUser = `
	SELECT u.id, u.full_name, u.email::text, u.username, r.id, r.code, r.name,
	       u.active, u.must_change_password, u.activated_at IS NULL, u.last_login_at, u.created_at
	FROM users u
	JOIN roles r ON r.id = u.role_id`

func scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.FullName, &u.Email, &u.Username, &u.Role.ID, &u.Role.Code, &u.Role.Name,
		&u.Active, &u.MustChangePassword, &u.PendingActivation, &u.LastLoginAt, &u.CreatedAt)
	return u, err
}

func (Repository) list(ctx context.Context, db database.DBTX, f Filter, page httpx.Page) ([]User, int64, error) {
	where, args := []string{"true"}, []any{}
	if search := strings.TrimSpace(f.Search); search != "" {
		args = append(args, "%"+search+"%")
		n := len(args)
		where = append(where, fmt.Sprintf("(u.full_name ILIKE $%d OR u.email ILIKE $%d OR u.username ILIKE $%d)", n, n, n))
	}
	if f.RoleID != nil {
		args = append(args, *f.RoleID)
		where = append(where, fmt.Sprintf("u.role_id = $%d", len(args)))
	}
	if f.Active != nil {
		args = append(args, *f.Active)
		where = append(where, fmt.Sprintf("u.active = $%d", len(args)))
	}
	condition := strings.Join(where, " AND ")

	var total int64
	if err := db.QueryRow(ctx, "SELECT count(*) FROM users u WHERE "+condition, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	args = append(args, page.Size, page.Offset())
	rows, err := db.Query(ctx, fmt.Sprintf("%s WHERE %s ORDER BY u.active DESC, u.full_name LIMIT $%d OFFSET $%d",
		selectUser, condition, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (User, error) { return scanUser(row) })
	if err != nil {
		return nil, 0, fmt.Errorf("read users: %w", err)
	}
	return items, total, nil
}

func (Repository) get(ctx context.Context, db database.DBTX, id uuid.UUID) (User, error) {
	u, err := scanUser(db.QueryRow(ctx, selectUser+" WHERE u.id = $1", id))
	return u, database.MapError(err, nil)
}

// create inserts a user. With temporaryUntil the password is a temporary one the user
// must change; without it the account stays pending until activated.
func (Repository) create(ctx context.Context, db database.DBTX, in Input, passwordHash string, temporaryUntil *time.Time) (uuid.UUID, error) {
	var id uuid.UUID
	err := db.QueryRow(ctx, `
		INSERT INTO users (full_name, email, username, password_hash, role_id,
		                   must_change_password, password_expires_at, activated_at)
		VALUES ($1, NULLIF($2, ''), $3, $4, $5,
		        $6::timestamptz IS NOT NULL, $6, CASE WHEN $6::timestamptz IS NOT NULL THEN now() END)
		RETURNING id`, in.FullName, in.Email, in.Username, passwordHash, in.RoleID, temporaryUntil).Scan(&id)
	return id, database.MapError(err, constraintFields)
}

func (Repository) update(ctx context.Context, db database.DBTX, id uuid.UUID, in Input) error {
	tag, err := db.Exec(ctx, `
		UPDATE users SET full_name = $2, email = NULLIF($3, ''), username = $4, role_id = $5 WHERE id = $1`,
		id, in.FullName, in.Email, in.Username, in.RoleID)
	if err != nil {
		return database.MapError(err, constraintFields)
	}
	if tag.RowsAffected() == 0 {
		return database.MapError(pgx.ErrNoRows, nil)
	}
	return nil
}

func (Repository) setActive(ctx context.Context, db database.DBTX, id uuid.UUID, active bool) error {
	_, err := db.Exec(ctx, `UPDATE users SET active = $2 WHERE id = $1`, id, active)
	return database.MapError(err, nil)
}

// clearLockout lifts a lockout so a freshly issued code can be used right away.
func (Repository) clearLockout(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	_, err := db.Exec(ctx, `UPDATE users SET failed_attempts = 0, locked_until = NULL WHERE id = $1`, id)
	return database.MapError(err, nil)
}

func (Repository) revokeSessions(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	_, err := db.Exec(ctx, `UPDATE refresh_sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, id)
	return database.MapError(err, nil)
}

func (Repository) roleCode(ctx context.Context, db database.DBTX, roleID uuid.UUID) (*string, error) {
	var code *string
	err := db.QueryRow(ctx, `SELECT code FROM roles WHERE id = $1`, roleID).Scan(&code)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errUnknownRole
	}
	return code, database.MapError(err, nil)
}

// countOtherActiveAdmins counts active administrators other than excluded, locking them
// so two concurrent changes can't both remove "the other" admin.
func (Repository) countOtherActiveAdmins(ctx context.Context, db database.DBTX, excluded uuid.UUID) (int, error) {
	rows, err := db.Query(ctx, `
		SELECT u.id FROM users u JOIN roles r ON r.id = u.role_id
		WHERE r.code = 'admin' AND u.active AND u.id <> $1
		FOR UPDATE OF u`, excluded)
	if err != nil {
		return 0, fmt.Errorf("count admins: %w", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return 0, fmt.Errorf("read admins: %w", err)
	}
	return len(ids), nil
}
