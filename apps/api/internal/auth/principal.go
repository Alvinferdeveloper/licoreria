// Package auth handles login, sessions, password changes and recovery, and the
// middleware that authenticates requests and enforces permissions.
package auth

import (
	"context"
	"slices"

	"github.com/google/uuid"
)

// Principal is the authenticated user of a request, with permissions loaded from the database.
type Principal struct {
	UserID             uuid.UUID `json:"id"`
	SessionID          uuid.UUID `json:"-"`
	FullName           string    `json:"fullName"`
	Email              string    `json:"email"`
	Username           string    `json:"username"`
	Role               Role      `json:"role"`
	Permissions        []string  `json:"permissions"`
	MustChangePassword bool      `json:"mustChangePassword"`
}

type Role struct {
	ID   uuid.UUID `json:"id"`
	Code *string   `json:"code"`
	Name string    `json:"name"`
}

// Permission formats a permission key, e.g. Permission("products", "update") is "products:update".
func Permission(module, action string) string { return module + ":" + action }

func (p Principal) Can(module, action string) bool {
	return slices.Contains(p.Permissions, Permission(module, action))
}

type principalKey struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// FromContext returns the authenticated user; ok is false on public routes.
func FromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}
