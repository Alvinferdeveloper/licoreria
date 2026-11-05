// Package roles exposes roles and lets administrators edit each role's permissions.
// Permissions live in tables, so a change applies on the user's next request.
package roles

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/audit"
	"licoreria/api/internal/auth"
	"licoreria/api/internal/httpx"
	"licoreria/api/internal/platform/database"
)

var ErrAdminRoleLocked = apperr.New(apperr.KindConflict, "admin_role_locked", "El rol Administrador siempre tiene todos los permisos.")

type Role struct {
	ID          uuid.UUID `json:"id"`
	Code        *string   `json:"code"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsSystem    bool      `json:"isSystem"`
	UserCount   int       `json:"userCount"`
	Permissions []string  `json:"permissions"`
}

type Permission struct {
	Key         string `json:"key"`
	Module      string `json:"module"`
	Action      string `json:"action"`
	Description string `json:"description"`
}

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

const selectRole = `
	SELECT r.id, r.code, r.name, r.description, r.is_system,
	       (SELECT count(*) FROM users u WHERE u.role_id = r.id AND u.active),
	       COALESCE(array_agg(p.module || ':' || p.action ORDER BY p.module, p.action)
	                FILTER (WHERE p.id IS NOT NULL), '{}')
	FROM roles r
	LEFT JOIN role_permissions rp ON rp.role_id = r.id
	LEFT JOIN permissions p ON p.id = rp.permission_id`

func scanRole(row pgx.Row) (Role, error) {
	var r Role
	err := row.Scan(&r.ID, &r.Code, &r.Name, &r.Description, &r.IsSystem, &r.UserCount, &r.Permissions)
	return r, err
}

func (s *Service) List(ctx context.Context) ([]Role, error) {
	rows, err := s.pool.Query(ctx, selectRole+" GROUP BY r.id ORDER BY r.is_system DESC, r.name")
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Role, error) { return scanRole(row) })
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (Role, error) { return s.get(ctx, s.pool, id) }

func (s *Service) get(ctx context.Context, db database.DBTX, id uuid.UUID) (Role, error) {
	role, err := scanRole(db.QueryRow(ctx, selectRole+" WHERE r.id = $1 GROUP BY r.id", id))
	return role, database.MapError(err, nil)
}

func (s *Service) Permissions(ctx context.Context) ([]Permission, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT module || ':' || action, module, action, description FROM permissions ORDER BY module, action`)
	if err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Permission])
}

// SetPermissions replaces a role's permissions with exactly the given keys.
func (s *Service) SetPermissions(ctx context.Context, actor, roleID uuid.UUID, keys []string) (Role, error) {
	var updated Role
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		before, err := s.get(ctx, tx, roleID)
		if err != nil {
			return err
		}
		if before.Code != nil && *before.Code == "admin" {
			return ErrAdminRoleLocked
		}

		var known []string
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(array_agg(module || ':' || action), '{}') FROM permissions
			WHERE module || ':' || action = ANY($1)`, keys).Scan(&known); err != nil {
			return fmt.Errorf("check permissions: %w", err)
		}
		if unknown := missing(keys, known); len(unknown) > 0 {
			return apperr.Validation(apperr.FieldError{
				Field: "permissions", Code: "unknown_permission", Message: "Hay permisos que no existen: " + strings.Join(unknown, ", ") + ".",
			})
		}

		if _, err := tx.Exec(ctx, `DELETE FROM role_permissions WHERE role_id = $1`, roleID); err != nil {
			return fmt.Errorf("clear permissions: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO role_permissions (role_id, permission_id)
			SELECT $1, id FROM permissions WHERE module || ':' || action = ANY($2)`, roleID, keys); err != nil {
			return fmt.Errorf("set permissions: %w", err)
		}

		if updated, err = s.get(ctx, tx, roleID); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{
			UserID: &actor, Action: audit.ActionRolePermissions, Entity: "roles", EntityID: &roleID,
			Before: before.Permissions, After: updated.Permissions,
		})
	})
	return updated, err
}

func missing(wanted, known []string) []string {
	var out []string
	for _, key := range wanted {
		if !slices.Contains(known, key) && !slices.Contains(out, key) {
			out = append(out, key)
		}
	}
	return out
}

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Routes(r chi.Router) {
	r.With(auth.Require("roles", "view")).Get("/permissions", h.permissions)
	r.Route("/roles", func(r chi.Router) {
		r.With(auth.Require("roles", "view")).Get("/", h.list)
		r.With(auth.Require("roles", "view")).Get("/{id}", h.get)
		r.With(auth.Require("roles", "update")).Put("/{id}/permissions", h.setPermissions)
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.List(r.Context())
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, map[string]any{"data": items})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.URLParamUUID(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	role, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, role)
}

func (h *Handler) permissions(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.Permissions(r.Context())
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, map[string]any{"data": items})
}

type permissionsRequest struct {
	Permissions []string `json:"permissions" validate:"required,dive,required,max=64"`
}

func (h *Handler) setPermissions(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.URLParamUUID(r, "id")
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	var req permissionsRequest
	if err := httpx.Bind(w, r, &req); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	actor, _ := auth.FromContext(r.Context())
	role, err := h.svc.SetPermissions(r.Context(), actor.UserID, id, req.Permissions)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, role)
}
