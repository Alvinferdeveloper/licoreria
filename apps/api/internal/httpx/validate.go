package httpx

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"

	"licoreria/api/internal/apperr"
)

var validate = newValidator()

func newValidator() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	// Report fields by their JSON name so the frontend can match them to inputs.
	v.RegisterTagNameFunc(func(field reflect.StructField) string {
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name == "-" {
			return ""
		}
		return name
	})
	_ = v.RegisterValidation("username", func(fl validator.FieldLevel) bool {
		return usernamePattern.MatchString(fl.Field().String())
	})
	return v
}

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]{3,32}$`)

// Bind decodes a strict JSON body into dst and validates its `validate` tags.
func Bind(w http.ResponseWriter, r *http.Request, dst any) error {
	if err := Decode(w, r, dst); err != nil {
		return err
	}
	return Validate(dst)
}

func Validate(value any) error {
	err := validate.Struct(value)
	var validationErrs validator.ValidationErrors
	if !errors.As(err, &validationErrs) {
		return err
	}

	fields := make([]apperr.FieldError, 0, len(validationErrs))
	for _, fe := range validationErrs {
		fields = append(fields, apperr.FieldError{
			Field:   fieldPath(fe),
			Code:    fe.Tag(),
			Message: messageFor(fe),
		})
	}
	return apperr.Validation(fields...)
}

// fieldPath drops the root struct name: "CreateUser.email" becomes "email".
func fieldPath(fe validator.FieldError) string {
	_, path, found := strings.Cut(fe.Namespace(), ".")
	if !found {
		return fe.Field()
	}
	return path
}

func messageFor(fe validator.FieldError) string {
	param := fe.Param()
	isText := fe.Kind() == reflect.String

	switch fe.Tag() {
	case "required", "required_if", "required_unless":
		return "Este campo es obligatorio."
	case "email":
		return "Ingresa un correo electrónico válido."
	case "uuid", "uuid4":
		return "El identificador no es válido."
	case "min":
		if isText {
			return fmt.Sprintf("Debe tener al menos %s caracteres.", param)
		}
		return fmt.Sprintf("Debe ser mayor o igual a %s.", param)
	case "max":
		if isText {
			return fmt.Sprintf("Debe tener como máximo %s caracteres.", param)
		}
		return fmt.Sprintf("Debe ser menor o igual a %s.", param)
	case "len":
		return fmt.Sprintf("Debe tener exactamente %s caracteres.", param)
	case "gt":
		return fmt.Sprintf("Debe ser mayor que %s.", param)
	case "gte":
		return fmt.Sprintf("Debe ser mayor o igual a %s.", param)
	case "lt":
		return fmt.Sprintf("Debe ser menor que %s.", param)
	case "lte":
		return fmt.Sprintf("Debe ser menor o igual a %s.", param)
	case "oneof":
		return fmt.Sprintf("Debe ser uno de: %s.", strings.ReplaceAll(param, " ", ", "))
	case "numeric", "number":
		return "Debe ser un número."
	case "datetime":
		return "La fecha no tiene un formato válido."
	case "username":
		return "Usa de 3 a 32 letras, números, punto, guion o guion bajo, sin espacios."
	default:
		return "El valor no es válido."
	}
}
