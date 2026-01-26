package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"os"
	"strings"
	"sync"
)

var (
	encryptionKey    []byte
	keyOnce          sync.Once
	keyInitialized   bool
	ErrNoKey         = errors.New("ENCRYPTION_KEY environment variable not set")
	ErrDecryptFailed = errors.New("decryption failed")
)

// initKey derives a 32-byte key from the environment variable
func initKey() {
	keyStr := os.Getenv("ENCRYPTION_KEY")
	if keyStr == "" {
		log.Printf("[Crypto] Warning: ENCRYPTION_KEY not set, encryption disabled")
		return
	}
	// Derive 32-byte key using SHA-256
	hash := sha256.Sum256([]byte(keyStr))
	encryptionKey = hash[:]
	keyInitialized = true
	log.Printf("[Crypto] Encryption key initialized")
}

// IsEncryptionEnabled returns true if encryption is configured
func IsEncryptionEnabled() bool {
	keyOnce.Do(initKey)
	return keyInitialized
}

// Encrypt encrypts plaintext using AES-256-GCM
// Returns base64-encoded ciphertext with nonce prepended
// Returns empty string for empty input
// Returns original value if encryption is not configured
func Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	keyOnce.Do(initKey)
	if !keyInitialized {
		// Return original if no key configured (allows gradual rollout)
		return plaintext, nil
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	// Prefix with "enc:" to identify encrypted values
	return "enc:" + base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64-encoded AES-256-GCM ciphertext
// Returns original value if decryption fails (handles legacy unencrypted data)
func Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	// Check if this is an encrypted value (prefixed with "enc:")
	if !strings.HasPrefix(ciphertext, "enc:") {
		// Return as-is - this is likely unencrypted legacy data
		return ciphertext, nil
	}

	keyOnce.Do(initKey)
	if !keyInitialized {
		// Can't decrypt without key, return original
		return ciphertext, ErrNoKey
	}

	// Remove "enc:" prefix
	encoded := strings.TrimPrefix(ciphertext, "enc:")

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		// Not valid base64, return original
		return ciphertext, err
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return ciphertext, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return ciphertext, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return ciphertext, ErrDecryptFailed
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return ciphertext, err
	}

	return string(plaintext), nil
}

// BlindIndex creates a deterministic hash for searchable encrypted fields
// Used for email lookups without decrypting all records
// Returns lowercase, trimmed input hashed with HMAC-SHA256
func BlindIndex(value string) string {
	if value == "" {
		return ""
	}

	keyOnce.Do(initKey)
	if !keyInitialized {
		// Without key, return empty (can't create consistent index)
		return ""
	}

	// Normalize: lowercase and trim whitespace
	normalized := strings.ToLower(strings.TrimSpace(value))

	// HMAC-SHA256 with encryption key as secret
	mac := hmac.New(sha256.New, encryptionKey)
	mac.Write([]byte(normalized))
	return hex.EncodeToString(mac.Sum(nil))
}

// DecryptField is a helper that decrypts and returns the value
// Handles errors gracefully by returning original value on failure
func DecryptField(value string) string {
	if value == "" {
		return ""
	}
	decrypted, err := Decrypt(value)
	if err != nil {
		// Log but don't fail - return original value
		// This handles legacy unencrypted data gracefully
		return value
	}
	return decrypted
}

// PIIFields defines which fields need encryption per collection
var PIIFields = map[string][]string{
	"contacts": {"email", "phone", "bio", "location"},
}

// EncryptPIIFields encrypts PII fields on a record
func EncryptPIIFields(collectionName string, data map[string]any) map[string]any {
	fields, ok := PIIFields[collectionName]
	if !ok {
		return data
	}

	for _, field := range fields {
		if val, exists := data[field]; exists {
			if strVal, ok := val.(string); ok && strVal != "" {
				encrypted, err := Encrypt(strVal)
				if err == nil {
					data[field] = encrypted
				}
			}
		}
	}

	// Generate blind index for email if present
	if email, exists := data["email"]; exists {
		if strVal, ok := email.(string); ok && strVal != "" {
			// Get original email for blind index (before encryption)
			originalEmail := strVal
			if strings.HasPrefix(strVal, "enc:") {
				// Already encrypted, decrypt first
				originalEmail = DecryptField(strVal)
			}
			data["email_index"] = BlindIndex(originalEmail)
		}
	}

	return data
}
