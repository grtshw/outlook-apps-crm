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

	// Verify signature — try current key first, then previous key during rotation
	expected := signSessionWithKey(encoded, os.Getenv("ENCRYPTION_KEY"))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		prevKey := os.Getenv("ENCRYPTION_KEY_PREV")
		if prevKey == "" || !hmac.Equal([]byte(sig), []byte(signSessionWithKey(encoded, prevKey))) {
			return nil, errors.New("invalid signature")
		}
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

// AttendeeSessionClaims holds the data in an attendee session token.
type AttendeeSessionClaims struct {
	ContactID string `json:"cid"`
	Email     string `json:"email"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

// CreateAttendeeSession creates an HMAC-signed session token for attendee access.
func CreateAttendeeSession(contactID, email string, ttlSeconds int) (string, error) {
	now := time.Now().Unix()
	claims := AttendeeSessionClaims{
		ContactID: contactID,
		Email:     email,
		IssuedAt:  now,
		ExpiresAt: now + int64(ttlSeconds),
	}

	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	encoded := base64.RawURLEncoding.EncodeToString(payload)
	sig := signWithDomain(encoded, "attendee-session")
	return encoded + "." + sig, nil
}

// ValidateAttendeeSession validates and decodes an attendee session token.
func ValidateAttendeeSession(sessionToken string) (*AttendeeSessionClaims, error) {
	parts := strings.SplitN(sessionToken, ".", 2)
	if len(parts) != 2 {
		return nil, errors.New("invalid token format")
	}

	encoded, sig := parts[0], parts[1]

	// Verify signature — try current key first, then previous key during rotation
	expected := signWithDomainAndKey(encoded, "attendee-session", os.Getenv("ENCRYPTION_KEY"))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		prevKey := os.Getenv("ENCRYPTION_KEY_PREV")
		if prevKey == "" || !hmac.Equal([]byte(sig), []byte(signWithDomainAndKey(encoded, "attendee-session", prevKey))) {
			return nil, errors.New("invalid signature")
		}
	}

	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.New("invalid encoding")
	}

	var claims AttendeeSessionClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, errors.New("invalid payload")
	}

	if time.Now().Unix() > claims.ExpiresAt {
		return nil, errors.New("session expired")
	}

	return &claims, nil
}

// signSession creates an HMAC-SHA256 signature using the current key.
func signSession(payload string) string {
	return signSessionWithKey(payload, os.Getenv("ENCRYPTION_KEY"))
}

// signSessionWithKey creates an HMAC-SHA256 signature with the given key.
// Uses a domain separator to avoid key reuse with encryption.
func signSessionWithKey(payload, key string) string {
	return signWithDomainAndKey(payload, "guest-list-session", key)
}

// signWithDomain creates an HMAC-SHA256 signature with a domain separator.
func signWithDomain(payload, domain string) string {
	return signWithDomainAndKey(payload, domain, os.Getenv("ENCRYPTION_KEY"))
}

// signWithDomainAndKey creates an HMAC-SHA256 signature with a given key and domain separator.
func signWithDomainAndKey(payload, domain, key string) string {
	if key == "" {
		key = "dev-session-key"
	}
	mac := hmac.New(sha256.New, []byte(domain+":"+key))
	mac.Write([]byte(payload))
	return fmt.Sprintf("%x", mac.Sum(nil))
}
