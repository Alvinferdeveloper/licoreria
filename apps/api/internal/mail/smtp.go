package mail

import (
	"bytes"
	"context"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"time"

	"github.com/google/uuid"

	"licoreria/api/internal/platform/config"
)

type Message struct {
	To      string
	Subject string
	HTML    string
}

type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// SMTPSender delivers mail through any SMTP server (Mailpit in development, Resend or similar in production).
type SMTPSender struct {
	cfg  config.MailConfig
	from *mail.Address
}

func NewSMTPSender(cfg config.MailConfig) (*SMTPSender, error) {
	from, err := mail.ParseAddress(cfg.From)
	if err != nil {
		return nil, fmt.Errorf("parse MAIL_FROM: %w", err)
	}
	return &SMTPSender{cfg: cfg, from: from}, nil
}

func (s *SMTPSender) Send(ctx context.Context, msg Message) error {
	to, err := mail.ParseAddress(msg.To)
	if err != nil {
		return fmt.Errorf("parse recipient: %w", err)
	}

	addr := net.JoinHostPort(s.cfg.SMTPHost, strconv.Itoa(s.cfg.SMTPPort))
	var auth smtp.Auth
	if s.cfg.SMTPUsername != "" {
		auth = smtp.PlainAuth("", s.cfg.SMTPUsername, s.cfg.SMTPPassword, s.cfg.SMTPHost)
	}

	done := make(chan error, 1)
	go func() {
		done <- smtp.SendMail(addr, auth, s.from.Address, []string{to.Address}, s.build(to, msg))
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *SMTPSender) build(to *mail.Address, msg Message) []byte {
	var buf bytes.Buffer
	headers := [][2]string{
		{"From", s.from.String()},
		{"To", to.String()},
		{"Subject", mime.QEncoding.Encode("utf-8", msg.Subject)},
		{"Date", time.Now().Format(time.RFC1123Z)},
		{"Message-ID", fmt.Sprintf("<%s@anejo>", uuid.NewString())},
		{"MIME-Version", "1.0"},
		{"Content-Type", `text/html; charset="utf-8"`},
		{"Content-Transfer-Encoding", "8bit"},
	}
	for _, h := range headers {
		fmt.Fprintf(&buf, "%s: %s\r\n", h[0], h[1])
	}
	buf.WriteString("\r\n")
	buf.WriteString(msg.HTML)
	return buf.Bytes()
}
