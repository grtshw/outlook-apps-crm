package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// ShareSessionClaims holds the data in a share session token.
type ShareSessionClaims struct {
	ShareID string `json:"sid"`
	Token   string `json:"tok"`
	IssuedAt  int64 `json:"iat"`
	ExpiresAt int64 `json:"exp"`
}

// CreateShareSession creates an HMAC-signed session token for share access.
func CreateShareSession(shareID, token string, ttlSeconds int) (string, error) {
	now := time.Now().Unix()
	claims := ShareSessionClaims{
		ShareID:   shareID,
		Token:     token,
		IssuedAt:  now,
		ExpiresAt: now + int64(ttlSeconds),
	}

	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	encoded := base64.RawURLEncoding.EncodeToString(payload)
	sig := signSession(encoded)
	return encoded + "." + sig, nil
}

// ValidateShareSession validates and decodes an HMAC-signed session token.
func ValidateShareSession(sessionToken string) (*ShareSessionClaims, error) {
	parts := strings.SplitN(sessionToken, ".", 2)
	if len(parts) != 2 {
		return nil, errors.New("invalid token format")
	}

	encoded, sig := parts[0], parts[1]

	// Verify signature
	expected := signSession(encoded)
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return nil, errors.New("invalid signature")
	}

	// Decode payload
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.New("invalid encoding")
	}

	var claims ShareSessionClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, errors.New("invalid payload")
	}

	// Check expiry
	if time.Now().Unix() > claims.ExpiresAt {
		return nil, errors.New("session expired")
	}

	return &claims, nil
}

// signSession creates an HMAC-SHA256 signature for a session payload.
// Uses ENCRYPTION_KEY with a domain separator to avoid key reuse.
func signSession(payload string) string {
	key := os.Getenv("ENCRYPTION_KEY")
	if key == "" {
		key = "dev-session-key"
	}

	mac := hmac.New(sha256.New, []byte("guest-list-session:"+key))
	mac.Write([]byte(payload))
	return fmt.Sprintf("%x", mac.Sum(nil))
}
