package mail

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResendSender(t *testing.T) {
	var got map[string]any
	var auth string
	fail := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&got)
		if fail {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"name":"validation_error","message":"domain not verified"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"abc"}`))
	}))
	defer srv.Close()

	s, err := NewResendSender("re_test", "Añejo <no-reply@anejo.ni>")
	if err != nil {
		t.Fatal(err)
	}
	s.endpoint = srv.URL
	if err := s.Send(context.Background(), Message{To: "ana@anejo.ni", Subject: "Hola", HTML: "<p>Hola</p>"}); err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer re_test" || got["subject"] != "Hola" || got["to"].([]any)[0] != "ana@anejo.ni" {
		t.Fatalf("unexpected request: auth=%q body=%v", auth, got)
	}

	fail = true
	err = s.Send(context.Background(), Message{To: "ana@anejo.ni", Subject: "Hola", HTML: "x"})
	if err == nil || !strings.Contains(err.Error(), "domain not verified") {
		t.Fatalf("a rejected send must fail with Resend's reason, got %v", err)
	}
}
