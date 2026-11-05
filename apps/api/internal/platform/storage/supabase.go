// Package storage uploads files to Supabase Storage through its REST API.
package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"licoreria/api/internal/apperr"
)

var ErrNotConfigured = apperr.New(apperr.KindUnavailable, "storage_not_configured",
	"El almacenamiento de imágenes no está configurado. Agrega las credenciales de Supabase.")

type Config struct {
	URL        string
	ServiceKey string
	Bucket     string
}

func (c Config) Enabled() bool { return c.URL != "" && c.ServiceKey != "" && c.Bucket != "" }

// Supabase stores objects in a public bucket and returns their public URL.
type Supabase struct {
	cfg    Config
	client *http.Client
}

func NewSupabase(cfg Config) *Supabase {
	cfg.URL = strings.TrimRight(cfg.URL, "/")
	return &Supabase{cfg: cfg, client: &http.Client{Timeout: 30 * time.Second}}
}

func (s *Supabase) Enabled() bool { return s.cfg.Enabled() }

// Put uploads (or replaces) the object at key and returns its public URL.
func (s *Supabase) Put(ctx context.Context, key, contentType string, data []byte) (string, error) {
	if !s.Enabled() {
		return "", ErrNotConfigured
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.objectURL(key), bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	s.authorize(req)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("x-upsert", "true")
	req.Header.Set("Cache-Control", "max-age=31536000")

	if err := s.do(req); err != nil {
		return "", fmt.Errorf("upload %s: %w", key, err)
	}
	return s.PublicURL(key), nil
}

// Delete removes the object at key; a missing object is not an error.
func (s *Supabase) Delete(ctx context.Context, key string) error {
	if !s.Enabled() {
		return ErrNotConfigured
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, s.objectURL(key), nil)
	if err != nil {
		return err
	}
	s.authorize(req)
	if err := s.do(req); err != nil && !strings.Contains(err.Error(), "status 404") {
		return fmt.Errorf("delete %s: %w", key, err)
	}
	return nil
}

func (s *Supabase) PublicURL(key string) string {
	return fmt.Sprintf("%s/storage/v1/object/public/%s/%s", s.cfg.URL, url.PathEscape(s.cfg.Bucket), escapeKey(key))
}

// KeyFromURL returns the object key of a public URL from this bucket, or "" if it isn't one.
func (s *Supabase) KeyFromURL(publicURL string) string {
	prefix := fmt.Sprintf("%s/storage/v1/object/public/%s/", s.cfg.URL, url.PathEscape(s.cfg.Bucket))
	key, ok := strings.CutPrefix(publicURL, prefix)
	if !ok {
		return ""
	}
	unescaped, err := url.PathUnescape(key)
	if err != nil {
		return ""
	}
	return unescaped
}

func (s *Supabase) objectURL(key string) string {
	return fmt.Sprintf("%s/storage/v1/object/%s/%s", s.cfg.URL, url.PathEscape(s.cfg.Bucket), escapeKey(key))
}

func (s *Supabase) authorize(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+s.cfg.ServiceKey)
	req.Header.Set("apikey", s.cfg.ServiceKey)
}

func (s *Supabase) do(req *http.Request) error {
	res, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("supabase storage status %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func escapeKey(key string) string {
	parts := strings.Split(key, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}
