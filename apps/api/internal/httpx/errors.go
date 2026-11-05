// Package httpx holds shared HTTP helpers: JSON responses, the error envelope, decoding and middleware.
package httpx

import (
	"net/http"

	"licoreria/api/internal/apperr"
)

// ErrorBody is the JSON shape of every error response: {"error": ErrorBody}.
type ErrorBody struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"requestId,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
}

var statusByKind = map[apperr.Kind]int{
	apperr.KindInternal:         http.StatusInternalServerError,
	apperr.KindInvalid:          http.StatusBadRequest,
	apperr.KindValidation:       http.StatusUnprocessableEntity,
	apperr.KindUnauthorized:     http.StatusUnauthorized,
	apperr.KindForbidden:        http.StatusForbidden,
	apperr.KindNotFound:         http.StatusNotFound,
	apperr.KindMethodNotAllowed: http.StatusMethodNotAllowed,
	apperr.KindConflict:         http.StatusConflict,
	apperr.KindTooLarge:         http.StatusRequestEntityTooLarge,
	apperr.KindUnsupportedMedia: http.StatusUnsupportedMediaType,
	apperr.KindUnavailable:      http.StatusServiceUnavailable,
	apperr.KindTooManyRequests:  http.StatusTooManyRequests,
}

// toBody resolves the status and public body for any error; unknown errors become internal_error.
func toBody(err error) (int, ErrorBody) {
	appErr, ok := apperr.As(err)
	if !ok {
		appErr = apperr.ErrInternal
	}

	status, known := statusByKind[appErr.Kind]
	if !known {
		status = http.StatusInternalServerError
	}

	body := ErrorBody{Code: appErr.Code, Message: appErr.Message}
	if len(appErr.Details) > 0 || len(appErr.Fields) > 0 {
		body.Details = make(map[string]any, len(appErr.Details)+1)
		for key, value := range appErr.Details {
			body.Details[key] = value
		}
		if len(appErr.Fields) > 0 {
			body.Details["fields"] = appErr.Fields
		}
	}
	return status, body
}
