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
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
)

// currentKeyVersion is the version tag written by Encrypt().
// Bump this each time you rotate keys.
const currentKeyVersion = 2

var (
	currentKey     []byte // derived from ENCRYPTION_KEY — used for encrypt, blind index
	previousKey    []byte // derived from ENCRYPTION_KEY_PREV — used for decrypting old data
	keyOnce        sync.Once
	keyInitialized bool
	ErrNoKey       = errors.New("ENCRYPTION_KEY environment variable not set")
	ErrDecryptFailed = errors.New("decryption failed")
)

// initKey derives 32-byte keys from environment variables
func initKey() {
	keyStr := os.Getenv("ENCRYPTION_KEY")
	if keyStr == "" {
		log.Printf("[Crypto] Warning: ENCRYPTION_KEY not set, encryption disabled")
		return
	}
	hash := sha256.Sum256([]byte(keyStr))
	currentKey = hash[:]
	keyInitialized = true

	prevStr := os.Getenv("ENCRYPTION_KEY_PREV")
	if prevStr != "" {
		prevHash := sha256.Sum256([]byte(prevStr))
		previousKey = prevHash[:]
		log.Printf("[Crypto] Encryption keys initialized (current v%d + previous)", currentKeyVersion)
	} else {
		log.Printf("[Crypto] Encryption key initialized (v%d)", currentKeyVersion)
	}
}

// IsEncryptionEnabled returns true if encryption is configured
func IsEncryptionEnabled() bool {
	keyOnce.Do(initKey)
	return keyInitialized
}

// CurrentKeyVersion returns the current encryption key version.
func CurrentKeyVersion() int {
	return currentKeyVersion
}

// Encrypt encrypts plaintext using AES-256-GCM with the current key.
// Returns versioned ciphertext: enc:v<N>:<base64>
// Returns empty string for empty input.
// Returns original value if encryption is not configured.
func Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	keyOnce.Do(initKey)
	if !keyInitialized {
		return plaintext, nil
	}

	block, err := aes.NewCipher(currentKey)
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
	return fmt.Sprintf("enc:v%d:%s", currentKeyVersion, base64.StdEncoding.EncodeToString(ciphertext)), nil
}

// Decrypt decrypts versioned AES-256-GCM ciphertext.
// Supports: enc:v<N>:<base64> (versioned) and enc:<base64> (legacy v1).
// Returns original value for non-encrypted data or on failure.
func Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	if !strings.HasPrefix(ciphertext, "enc:") {
		return ciphertext, nil
	}

	keyOnce.Do(initKey)
	if !keyInitialized {
		return ciphertext, ErrNoKey
	}

	key, encoded := parseEncryptedValue(ciphertext)
	if key == nil {
		return ciphertext, ErrDecryptFailed
	}

	return decryptWithKey(key, encoded, ciphertext)
}

// parseEncryptedValue extracts the decryption key and base64 payload.
// Supports versioned (enc:v2:<base64>) and legacy (enc:<base64>) formats.
func parseEncryptedValue(value string) ([]byte, string) {
	after := strings.TrimPrefix(value, "enc:")

	// Versioned format: v<N>:<base64>
	if strings.HasPrefix(after, "v") {
		colonIdx := strings.Index(after, ":")
		if colonIdx == -1 {
			return nil, ""
		}
		version, err := strconv.Atoi(after[1:colonIdx])
		if err != nil {
			return nil, ""
		}
		return keyForVersion(version), after[colonIdx+1:]
	}

	// Legacy unversioned format: enc:<base64> — treated as v1
	return keyForVersion(1), after
}

// keyForVersion returns the decryption key for a given ciphertext version.
func keyForVersion(version int) []byte {
	if version == currentKeyVersion {
		return currentKey
	}
	// Older version — use previous key if available, fall back to current
	if previousKey != nil {
		return previousKey
	}
	return currentKey
}

// decryptWithKey performs AES-256-GCM decryption with the given key.
func decryptWithKey(key []byte, encoded string, originalValue string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return originalValue, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return originalValue, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return originalValue, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return originalValue, ErrDecryptFailed
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return originalValue, err
	}

	return string(plaintext), nil
}

// BlindIndex creates a deterministic hash for searchable encrypted fields.
// Uses the current key — blind indexes must be recomputed after key rotation.
func BlindIndex(value string) string {
	if value == "" {
		return ""
	}

	keyOnce.Do(initKey)
	if !keyInitialized {
		return ""
	}

	normalized := strings.ToLower(strings.TrimSpace(value))

	mac := hmac.New(sha256.New, currentKey)
	mac.Write([]byte(normalized))
	return hex.EncodeToString(mac.Sum(nil))
}

// DecryptField is a helper that decrypts and returns the value.
// Returns empty string on failure to avoid leaking ciphertext.
func DecryptField(value string) string {
	if value == "" {
		return ""
	}
	decrypted, err := Decrypt(value)
	if err != nil {
		log.Printf("[Crypto] DecryptField failed: %v", err)
		return ""
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
			originalEmail := strVal
			if strings.HasPrefix(strVal, "enc:") {
				originalEmail = DecryptField(strVal)
			}
			data["email_index"] = BlindIndex(originalEmail)
		}
	}

	return data
}
