package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const issuer = "anejo-api"

// TokenIssuer signs and verifies short-lived HS256 access tokens.
type TokenIssuer struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

func NewTokenIssuer(secret []byte, ttl time.Duration, now func() time.Time) *TokenIssuer {
	return &TokenIssuer{secret: secret, ttl: ttl, now: now}
}

type accessClaims struct {
	SessionID string `json:"sid"`
	jwt.RegisteredClaims
}

type AccessClaims struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
}

func (t *TokenIssuer) Issue(userID, sessionID uuid.UUID) (string, time.Time, error) {
	now := t.now()
	expiresAt := now.Add(t.ttl)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims{
		SessionID: sessionID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			ID:        uuid.NewString(),
		},
	})
	signed, err := token.SignedString(t.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return signed, expiresAt, nil
}

var errInvalidToken = errors.New("invalid access token")

// Parse verifies signature, algorithm, issuer and expiry.
func (t *TokenIssuer) Parse(raw string) (AccessClaims, error) {
	var claims accessClaims
	_, err := jwt.ParseWithClaims(raw, &claims,
		func(*jwt.Token) (any, error) { return t.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(t.now),
	)
	if err != nil {
		return AccessClaims{}, errInvalidToken
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return AccessClaims{}, errInvalidToken
	}
	sessionID, err := uuid.Parse(claims.SessionID)
	if err != nil {
		return AccessClaims{}, errInvalidToken
	}
	return AccessClaims{UserID: userID, SessionID: sessionID}, nil
}

// NewOpaqueToken returns a random URL-safe token and the hash to store; the plain value is never persisted.
func NewOpaqueToken() (plain, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate token: %w", err)
	}
	plain = base64.RawURLEncoding.EncodeToString(buf)
	return plain, HashToken(plain), nil
}

func HashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}
