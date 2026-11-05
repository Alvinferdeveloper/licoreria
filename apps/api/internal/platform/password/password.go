// Package password hashes passwords with Argon2id, enforces the password policy
// and generates temporary passwords.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"unicode"

	"golang.org/x/crypto/argon2"

	"licoreria/api/internal/apperr"
)

// Argon2id parameters follow the OWASP minimum recommendation.
const (
	memoryKiB   = 19 * 1024
	iterations  = 2
	parallelism = 1
	saltLength  = 16
	keyLength   = 32

	MinLength = 10
	MaxLength = 128
)

var ErrMalformedHash = errors.New("malformed password hash")

// Hash returns a PHC-formatted Argon2id hash: $argon2id$v=19$m=...,t=...,p=...$salt$key.
func Hash(plain string) (string, error) {
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(plain), salt, iterations, memoryKiB, parallelism, keyLength)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, memoryKiB, iterations, parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// Verify compares plain against a hash produced by Hash in constant time.
func Verify(plain, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, ErrMalformedHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, ErrMalformedHash
	}
	var memory, time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false, ErrMalformedHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, ErrMalformedHash
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, ErrMalformedHash
	}

	got := argon2.IDKey([]byte(plain), salt, time, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// DummyHash is verified against when a user doesn't exist, so response time
// doesn't reveal which accounts exist.
var DummyHash = mustHash("timing-equalizer-not-a-real-password")

func mustHash(plain string) string {
	hash, err := Hash(plain)
	if err != nil {
		panic(err)
	}
	return hash
}

// CheckPolicy returns a validation error for field when plain breaks the password policy.
func CheckPolicy(field, plain string) error {
	if len(plain) > MaxLength {
		return apperr.Validation(apperr.FieldError{
			Field: field, Code: "max", Message: fmt.Sprintf("Debe tener como máximo %d caracteres.", MaxLength),
		})
	}

	var problems []string
	if len([]rune(plain)) < MinLength {
		problems = append(problems, fmt.Sprintf("al menos %d caracteres", MinLength))
	}
	if !strings.ContainsFunc(plain, unicode.IsUpper) {
		problems = append(problems, "una letra mayúscula")
	}
	if !strings.ContainsFunc(plain, unicode.IsDigit) {
		problems = append(problems, "un número")
	}
	if len(problems) == 0 {
		return nil
	}
	return apperr.Validation(apperr.FieldError{
		Field:   field,
		Code:    "weak_password",
		Message: "La contraseña debe tener " + joinSpanish(problems) + ".",
	})
}

func joinSpanish(items []string) string {
	if len(items) == 1 {
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " y " + items[len(items)-1]
}

// Characters that are easy to read aloud or copy from an email (no 0/O, 1/l/I).
const (
	upperChars  = "ABCDEFGHJKLMNPQRSTUVWXYZ"
	lowerChars  = "abcdefghijkmnpqrstuvwxyz"
	digitChars  = "23456789"
	symbolChars = "#$%*+-=?@"
)

// GenerateTemporary returns a random 14-character password that satisfies the policy.
func GenerateTemporary() (string, error) {
	const length = 14
	all := upperChars + lowerChars + digitChars + symbolChars
	required := []string{upperChars, lowerChars, digitChars, symbolChars}

	chars := make([]byte, 0, length)
	for _, set := range required {
		c, err := randomChar(set)
		if err != nil {
			return "", err
		}
		chars = append(chars, c)
	}
	for len(chars) < length {
		c, err := randomChar(all)
		if err != nil {
			return "", err
		}
		chars = append(chars, c)
	}

	for i := len(chars) - 1; i > 0; i-- {
		j, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return "", err
		}
		chars[i], chars[j.Int64()] = chars[j.Int64()], chars[i]
	}
	return string(chars), nil
}

func randomChar(set string) (byte, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(set))))
	if err != nil {
		return 0, fmt.Errorf("generate random char: %w", err)
	}
	return set[n.Int64()], nil
}
