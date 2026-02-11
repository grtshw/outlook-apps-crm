package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
)

// generateOTPCode creates a cryptographically secure 6-digit numeric code.
// Ported from onboarding-kit/twofactor/code.go
func generateOTPCode() (string, error) {
	max := big.NewInt(1000000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// hashOTPCode returns the SHA-256 hex digest of a code.
func hashOTPCode(code string) string {
	h := sha256.Sum256([]byte(code))
	return hex.EncodeToString(h[:])
}

// verifyOTPCode checks if a plaintext code matches a hash using constant-time comparison.
func verifyOTPCode(code, hash string) bool {
	return hmac.Equal([]byte(hashOTPCode(code)), []byte(hash))
}

// generateToken creates a cryptographically secure 64-character hex token.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
