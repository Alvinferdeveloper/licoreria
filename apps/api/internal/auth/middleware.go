package auth

import (
	"net/http"
	"strings"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/httpx"
	"licoreria/api/internal/platform/logx"
)

var ErrPasswordChangeRequired = apperr.New(apperr.KindForbidden, "password_change_required", "Debes cambiar tu contraseña antes de continuar.")

// Authenticate requires a valid Bearer access token and puts the Principal in the context.
func Authenticate(svc *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if !ok {
				httpx.Fail(w, r, apperr.ErrUnauthorized)
				return
			}
			principal, err := svc.Authenticate(r.Context(), token)
			if err != nil {
				httpx.Fail(w, r, err)
				return
			}
			ctx := logx.With(WithPrincipal(r.Context(), principal), "user_id", principal.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequirePasswordChanged blocks users still on a temporary password from everything
// except the routes that let them change it.
func RequirePasswordChanged(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p, ok := FromContext(r.Context()); ok && p.MustChangePassword {
			httpx.Fail(w, r, ErrPasswordChangeRequired)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Require allows the request only if the user's role grants module:action.
// Authorization is always enforced here, never only by hiding UI.
func Require(module, action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := FromContext(r.Context())
			if !ok {
				httpx.Fail(w, r, apperr.ErrUnauthorized)
				return
			}
			if !p.Can(module, action) {
				logx.From(r.Context()).Warn("permission denied", "permission", Permission(module, action))
				httpx.Fail(w, r, apperr.ErrForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func bearerToken(r *http.Request) (string, bool) {
	scheme, token, found := strings.Cut(r.Header.Get("Authorization"), " ")
	if !found || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return "", false
	}
	return token, true
}
