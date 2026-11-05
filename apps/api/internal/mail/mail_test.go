package mail

import (
	netmail "net/mail"
	"strings"
	"testing"
	"time"

	"licoreria/api/internal/platform/config"
)

func testMailConfig() config.MailConfig {
	return config.MailConfig{SMTPHost: "localhost", SMTPPort: 1025, From: "Añejo <no-reply@anejo.local>"}
}

func parseAddress(raw string) (*netmail.Address, error) { return netmail.ParseAddress(raw) }

func TestRenderHTMLEscapesData(t *testing.T) {
	out, err := renderHTML(`<p>Hola, {{.Name}}</p>`, map[string]string{"Name": `<script>alert(1)</script>`})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "<script>") {
		t.Errorf("user data must be escaped: %s", out)
	}
}

func TestRenderFailsOnMissingKeys(t *testing.T) {
	if _, err := renderText(`Hola {{.Missing}}`, map[string]string{}); err == nil {
		t.Error("a template referencing missing data must fail instead of sending a broken email")
	}
}

func TestBackoff(t *testing.T) {
	cases := map[int]time.Duration{1: 30 * time.Second, 2: time.Minute, 3: 2 * time.Minute, 10: 30 * time.Minute, 60: 30 * time.Minute}
	for attempt, want := range cases {
		if got := backoff(attempt); got != want {
			t.Errorf("backoff(%d) = %v, want %v", attempt, got, want)
		}
	}
}

func TestBuildEncodesUnicodeSubject(t *testing.T) {
	sender, err := NewSMTPSender(testMailConfig())
	if err != nil {
		t.Fatal(err)
	}
	to, _ := parseAddress("rosa@example.com")
	raw := string(sender.build(to, Message{Subject: "Tu acceso a Añejo", HTML: "<p>Hola</p>"}))

	if !strings.Contains(raw, "Subject: =?utf-8?q?") {
		t.Errorf("non-ASCII subject must be MIME encoded:\n%s", raw)
	}
	if !strings.Contains(raw, "\r\n\r\n<p>Hola</p>") {
		t.Error("body must follow a blank line after headers")
	}
}
