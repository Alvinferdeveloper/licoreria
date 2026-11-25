package database

import (
	"errors"
	"strings"
	"testing"
)

func TestRedactHidesThePassword(t *testing.T) {
	dsn := "postgres://licoreria:contrase%C3%B1a%40x@db:5432/licoreria"
	err := redact(errors.New(`parse "pgx5://licoreria:contrase%C3%B1a%40x@db:5432/licoreria": failed; raw contraseña@x`), dsn)
	if strings.Contains(err.Error(), "contrase") {
		t.Errorf("password leaked: %v", err)
	}
}
