package password

import (
	"strings"
	"testing"

	"licoreria/api/internal/apperr"
)

func TestHashAndVerify(t *testing.T) {
	hash, err := Hash("Flor de Caña 7")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Errorf("unexpected hash format: %s", hash)
	}

	ok, err := Verify("Flor de Caña 7", hash)
	if err != nil || !ok {
		t.Errorf("correct password rejected: %v", err)
	}
	ok, _ = Verify("flor de caña 7", hash)
	if ok {
		t.Error("wrong password accepted")
	}
}

func TestHashUsesUniqueSalts(t *testing.T) {
	a, _ := Hash("same-Password1")
	b, _ := Hash("same-Password1")
	if a == b {
		t.Error("two hashes of the same password must differ")
	}
}

func TestVerifyRejectsMalformedHashes(t *testing.T) {
	for _, bad := range []string{"", "plain", "$bcrypt$x$y$z$w", "$argon2id$v=19$m=x$a$b"} {
		if _, err := Verify("x", bad); err != ErrMalformedHash {
			t.Errorf("Verify(%q) err = %v, want ErrMalformedHash", bad, err)
		}
	}
}

func TestCheckPolicy(t *testing.T) {
	cases := []struct {
		password string
		valid    bool
		contains string
	}{
		{"Bodega2026", true, ""},
		{"corta1A", false, "al menos 10 caracteres"},
		{"sinmayuscula1", false, "una letra mayúscula"},
		{"SinNumeroAqui", false, "un número"},
		{"abc", false, "al menos 10 caracteres, una letra mayúscula y un número"},
	}
	for _, tc := range cases {
		err := CheckPolicy("newPassword", tc.password)
		if tc.valid {
			if err != nil {
				t.Errorf("%q should be valid, got %v", tc.password, err)
			}
			continue
		}
		appErr, ok := apperr.As(err)
		if !ok || len(appErr.Fields) != 1 || appErr.Fields[0].Field != "newPassword" {
			t.Fatalf("%q: expected one field error, got %v", tc.password, err)
		}
		if !strings.Contains(appErr.Fields[0].Message, tc.contains) {
			t.Errorf("%q: message %q should mention %q", tc.password, appErr.Fields[0].Message, tc.contains)
		}
	}
}

func TestGenerateTemporarySatisfiesPolicy(t *testing.T) {
	seen := map[string]bool{}
	for range 50 {
		temp, err := GenerateTemporary()
		if err != nil {
			t.Fatal(err)
		}
		if len(temp) != 14 {
			t.Errorf("length = %d, want 14", len(temp))
		}
		if err := CheckPolicy("password", temp); err != nil {
			t.Errorf("%q breaks the policy: %v", temp, err)
		}
		if strings.ContainsAny(temp, "0O1lI") {
			t.Errorf("%q contains ambiguous characters", temp)
		}
		if seen[temp] {
			t.Errorf("duplicate temporary password %q", temp)
		}
		seen[temp] = true
	}
}
