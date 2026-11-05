package database

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"licoreria/api/internal/apperr"
)

// ConstraintFields maps constraint names to the JSON field they protect,
// so violations can point at the right input.
type ConstraintFields map[string]string

// PostgreSQL SQLSTATE codes handled explicitly.
const (
	codeUniqueViolation      = "23505"
	codeForeignKeyViolation  = "23503"
	codeCheckViolation       = "23514"
	codeNotNullViolation     = "23502"
	codeInvalidTextRepr      = "22P02"
	codeNumericOutOfRange    = "22003"
	codeSerializationFailure = "40001"
	codeDeadlockDetected     = "40P01"
)

var (
	ErrAlreadyExists = apperr.New(apperr.KindConflict, "already_exists", "Ya existe un registro con ese valor.")
	ErrInUse         = apperr.New(apperr.KindConflict, "in_use", "No se puede eliminar porque tiene registros asociados.")
	ErrConcurrent    = apperr.New(apperr.KindConflict, "concurrent_update", "Otra operación modificó estos datos al mismo tiempo. Intenta de nuevo.")
	ErrInvalidRef    = apperr.New(apperr.KindValidation, "invalid_reference", "Uno de los registros seleccionados no existe.")
)

// MapError translates driver errors into application errors. Unknown errors are
// returned unchanged and end up as internal_error with the cause logged.
func MapError(err error, fields ConstraintFields) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.ErrNotFound.WithCause(err)
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}

	field := fields[pgErr.ConstraintName]
	switch pgErr.Code {
	case codeUniqueViolation:
		return withField(ErrAlreadyExists, field, "already_exists", "Ya está en uso.").WithCause(err)
	case codeForeignKeyViolation:
		if strings.Contains(pgErr.Detail, "is still referenced") {
			return ErrInUse.WithCause(err)
		}
		return withField(ErrInvalidRef, field, "invalid_reference", "El registro seleccionado no existe.").WithCause(err)
	case codeNotNullViolation:
		return apperr.Validation(apperr.FieldError{
			Field: toCamel(pgErr.ColumnName), Code: "required", Message: "Este campo es obligatorio.",
		}).WithCause(err)
	case codeCheckViolation, codeInvalidTextRepr, codeNumericOutOfRange:
		return withField(apperr.ErrValidation, field, "invalid", "El valor no es válido.").WithCause(err)
	case codeSerializationFailure, codeDeadlockDetected:
		return ErrConcurrent.WithCause(err)
	default:
		return err
	}
}

func withField(base *apperr.Error, field, code, message string) *apperr.Error {
	if field == "" {
		return base
	}
	return base.WithFields(apperr.FieldError{Field: field, Code: code, Message: message})
}

// toCamel converts a snake_case column name to the camelCase JSON field.
func toCamel(column string) string {
	parts := strings.Split(column, "_")
	for i := 1; i < len(parts); i++ {
		if parts[i] != "" {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}
