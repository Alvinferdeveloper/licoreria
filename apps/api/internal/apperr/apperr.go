// Package apperr defines transport-agnostic application errors. Services and repositories
// return them; the HTTP layer maps each Kind to a status code.
package apperr

import (
	"errors"
	"fmt"
)

type Kind int

const (
	KindInternal Kind = iota
	KindInvalid
	KindValidation
	KindUnauthorized
	KindForbidden
	KindNotFound
	KindMethodNotAllowed
	KindConflict
	KindTooLarge
	KindUnsupportedMedia
	KindUnavailable
	KindTooManyRequests
)

// Error carries a stable English code and a Spanish message safe to show to users.
type Error struct {
	Kind    Kind
	Code    string
	Message string
	Fields  []FieldError
	Details map[string]any
	cause   error
}

// FieldError describes why one input field was rejected.
type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func New(kind Kind, code, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message}
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return e.Code + ": " + e.Message
}

func (e *Error) Unwrap() error { return e.cause }

// The With* methods return copies so shared sentinels are never mutated.

func (e *Error) WithMessage(message string) *Error {
	c := *e
	c.Message = message
	return &c
}

func (e *Error) WithDetails(details map[string]any) *Error {
	c := *e
	c.Details = details
	return &c
}

func (e *Error) WithFields(fields ...FieldError) *Error {
	c := *e
	c.Fields = fields
	return &c
}

// WithCause keeps the underlying error for logs; it is never sent to clients.
func (e *Error) WithCause(cause error) *Error {
	c := *e
	c.cause = cause
	return &c
}

func Validation(fields ...FieldError) *Error {
	return ErrValidation.WithFields(fields...)
}

// As extracts an *Error; anything else is treated as internal.
func As(err error) (*Error, bool) {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}

func IsKind(err error, kind Kind) bool {
	appErr, ok := As(err)
	return ok && appErr.Kind == kind
}

var (
	ErrInvalid          = New(KindInvalid, "bad_request", "La solicitud no es válida.")
	ErrValidation       = New(KindValidation, "validation_failed", "Revisa los datos ingresados.")
	ErrUnauthorized     = New(KindUnauthorized, "unauthorized", "Inicia sesión para continuar.")
	ErrForbidden        = New(KindForbidden, "forbidden", "No tienes permiso para realizar esta acción.")
	ErrNotFound         = New(KindNotFound, "not_found", "No encontramos lo que buscas.")
	ErrMethodNotAllowed = New(KindMethodNotAllowed, "method_not_allowed", "Método no permitido.")
	ErrConflict         = New(KindConflict, "conflict", "La operación entra en conflicto con datos existentes.")
	ErrTooLarge         = New(KindTooLarge, "payload_too_large", "La solicitud es demasiado grande.")
	ErrUnsupportedMedia = New(KindUnsupportedMedia, "unsupported_media_type", "Envía los datos en formato JSON.")
	ErrInternal         = New(KindInternal, "internal_error", "Ocurrió un error inesperado. Intenta de nuevo.")
	ErrUnavailable      = New(KindUnavailable, "service_unavailable", "El servicio no está disponible en este momento.")
	ErrTooManyRequests  = New(KindTooManyRequests, "too_many_requests", "Demasiados intentos. Espera un momento e intenta de nuevo.")
)
