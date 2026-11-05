package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	"licoreria/api/internal/apperr"
)

const MaxBodyBytes = 1 << 20

// Decode reads a strict JSON body into dst: JSON content type, at most 1 MB,
// a single object and no unknown fields. Failures come back as API errors.
func Decode(w http.ResponseWriter, r *http.Request, dst any) error {
	if mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mediaType != "application/json" {
		return apperr.ErrUnsupportedMedia
	}

	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, MaxBodyBytes))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		return decodeError(err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return apperr.ErrInvalid.WithMessage("El cuerpo debe contener un solo objeto JSON.")
	}
	return nil
}

func decodeError(err error) *apperr.Error {
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	var maxBytesErr *http.MaxBytesError

	switch {
	case errors.Is(err, io.EOF):
		return apperr.ErrInvalid.WithMessage("El cuerpo de la solicitud está vacío.")
	case errors.As(err, &syntaxErr), errors.Is(err, io.ErrUnexpectedEOF):
		return apperr.ErrInvalid.WithMessage("El cuerpo no es un JSON válido.")
	case errors.As(err, &typeErr):
		return apperr.Validation(apperr.FieldError{
			Field:   typeErr.Field,
			Code:    "invalid_type",
			Message: "El valor no tiene el tipo esperado.",
		})
	case errors.As(err, &maxBytesErr):
		return apperr.ErrTooLarge
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		field := strings.Trim(strings.TrimPrefix(err.Error(), "json: unknown field "), `"`)
		return apperr.ErrInvalid.WithMessage(fmt.Sprintf("El campo %q no es reconocido.", field)).
			WithDetails(map[string]any{"field": field})
	default:
		return apperr.ErrInvalid
	}
}
