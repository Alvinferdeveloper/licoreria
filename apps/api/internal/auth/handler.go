package auth

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/httpx"
)

const (
	refreshCookieName = "anejo_refresh"
	refreshCookiePath = "/api/v1/auth"
)

type Handler struct {
	svc          *Service
	cookieSecure bool
}

func NewHandler(svc *Service, cookieSecure bool) *Handler {
	return &Handler{svc: svc, cookieSecure: cookieSecure}
}

// Routes mounts /auth. Credential endpoints are rate limited per IP on top of the per-account lockout.
func (h *Handler) Routes(r chi.Router) {
	limit := httprate.Limit(10, time.Minute,
		httprate.WithKeyFuncs(httprate.KeyByIP, httprate.KeyByEndpoint),
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			httpx.Fail(w, r, apperr.ErrTooManyRequests)
		}),
	)
	r.Route("/auth", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(limit)
			r.Post("/login", h.login)
			r.Post("/forgot-password", h.forgotPassword)
			r.Post("/reset-password", h.resetPassword)
			r.Post("/activate", h.activate)
		})
		r.Post("/refresh", h.refresh)
		r.Post("/logout", h.logout)

		r.Group(func(r chi.Router) {
			r.Use(Authenticate(h.svc))
			r.Get("/me", h.me)
			r.Post("/change-password", h.changePassword)
			r.With(limit).Post("/pin", h.setPIN)
		})
	})
}

type loginRequest struct {
	Identifier string `json:"identifier" validate:"required,max=254"`
	Password   string `json:"password" validate:"required,max=128"`
}

type sessionResponse struct {
	AccessToken          string    `json:"accessToken"`
	AccessTokenExpiresAt time.Time `json:"accessTokenExpiresAt"`
	User                 Principal `json:"user"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.Bind(w, r, &req); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	session, err := h.svc.Login(r.Context(), req.Identifier, req.Password)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	h.writeSession(w, r, session)
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	session, err := h.svc.Refresh(r.Context(), readCookie(r))
	if err != nil {
		h.clearRefreshCookie(w)
		httpx.Fail(w, r, err)
		return
	}
	h.writeSession(w, r, session)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Logout(r.Context(), readCookie(r)); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	h.clearRefreshCookie(w)
	httpx.NoContent(w)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	p, _ := FromContext(r.Context())
	httpx.JSON(w, r, http.StatusOK, p)
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" validate:"required,max=128"`
	NewPassword     string `json:"newPassword" validate:"required"`
}

func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) {
	var req changePasswordRequest
	if err := httpx.Bind(w, r, &req); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	p, _ := FromContext(r.Context())
	if err := h.svc.ChangePassword(r.Context(), p, req.CurrentPassword, req.NewPassword); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.NoContent(w)
}

type setPINRequest struct {
	CurrentPassword string `json:"currentPassword" validate:"required,max=128"`
	PIN             string `json:"pin" validate:"required"`
}

func (h *Handler) setPIN(w http.ResponseWriter, r *http.Request) {
	var req setPINRequest
	if err := httpx.Bind(w, r, &req); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	p, _ := FromContext(r.Context())
	if err := h.svc.SetPIN(r.Context(), p, req.CurrentPassword, req.PIN); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.NoContent(w)
}

type forgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email,max=254"`
}

func (h *Handler) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var req forgotPasswordRequest
	if err := httpx.Bind(w, r, &req); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	if err := h.svc.RequestPasswordReset(r.Context(), req.Email); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusAccepted, map[string]string{
		"message": "Si el correo está registrado, te enviamos un enlace para restablecer la contraseña.",
	})
}

type resetPasswordRequest struct {
	Token       string `json:"token" validate:"required,max=128"`
	NewPassword string `json:"newPassword" validate:"required"`
}

func (h *Handler) resetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetPasswordRequest
	if err := httpx.Bind(w, r, &req); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	if err := h.svc.ResetPassword(r.Context(), req.Token, req.NewPassword); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.NoContent(w)
}

// activateRequest takes the emailed token, or the username plus the code shown at the counter.
type activateRequest struct {
	Token       string `json:"token" validate:"required_without=Code,max=128"`
	Username    string `json:"username" validate:"required_with=Code,max=254"`
	Code        string `json:"code" validate:"required_without=Token,max=16"`
	NewPassword string `json:"newPassword" validate:"required"`
}

func (h *Handler) activate(w http.ResponseWriter, r *http.Request) {
	var req activateRequest
	if err := httpx.Bind(w, r, &req); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	err := h.svc.Activate(r.Context(), ActivationInput{
		Token: req.Token, Username: req.Username, Code: req.Code, NewPassword: req.NewPassword,
	})
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.NoContent(w)
}

func (h *Handler) writeSession(w http.ResponseWriter, r *http.Request, s Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    s.RefreshToken,
		Path:     refreshCookiePath,
		Expires:  s.RefreshExpiresAt,
		MaxAge:   int(time.Until(s.RefreshExpiresAt).Seconds()),
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, r, http.StatusOK, sessionResponse{
		AccessToken:          s.AccessToken,
		AccessTokenExpiresAt: s.AccessExpiresAt,
		User:                 s.Principal,
	})
}

func (h *Handler) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     refreshCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

func readCookie(r *http.Request) string {
	cookie, err := r.Cookie(refreshCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}
