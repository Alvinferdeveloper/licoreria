package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"time"
)

const resendEndpoint = "https://api.resend.com/emails"

// ResendSender delivers mail through Resend's HTTP API.
type ResendSender struct {
	apiKey   string
	from     string
	endpoint string
	client   *http.Client
}

func NewResendSender(apiKey, from string) (*ResendSender, error) {
	if _, err := mail.ParseAddress(from); err != nil {
		return nil, fmt.Errorf("parse MAIL_FROM: %w", err)
	}
	return &ResendSender{apiKey: apiKey, from: from, endpoint: resendEndpoint, client: &http.Client{Timeout: 20 * time.Second}}, nil
}

func (s *ResendSender) Send(ctx context.Context, msg Message) error {
	body, err := json.Marshal(map[string]any{"from": s.from, "to": []string{msg.To}, "subject": msg.Subject, "html": msg.HTML})
	if err != nil {
		return fmt.Errorf("encode resend request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build resend request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("resend request: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		// Resend answers {"name", "message"}; it never echoes the API key.
		detail, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("resend: status %d: %s", res.StatusCode, bytes.TrimSpace(detail))
	}
	return nil
}
