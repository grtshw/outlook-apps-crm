package utils

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// --- HTTP Response Helpers ---

// ErrorResponse returns a JSON error response with the given status code and message
func ErrorResponse(re *core.RequestEvent, status int, message string) error {
	return re.JSON(status, map[string]string{"error": message})
}

// NotFoundResponse returns a 404 JSON error response
func NotFoundResponse(re *core.RequestEvent, message string) error {
	return ErrorResponse(re, http.StatusNotFound, message)
}

// BadRequestResponse returns a 400 JSON error response
func BadRequestResponse(re *core.RequestEvent, message string) error {
	return ErrorResponse(re, http.StatusBadRequest, message)
}

// InternalErrorResponse returns a 500 JSON error response
func InternalErrorResponse(re *core.RequestEvent, message string) error {
	return ErrorResponse(re, http.StatusInternalServerError, message)
}

// ForbiddenResponse returns a 403 JSON error response
func ForbiddenResponse(re *core.RequestEvent, message string) error {
	return ErrorResponse(re, http.StatusForbidden, message)
}

// SuccessResponse returns a 200 JSON success response with a message
func SuccessResponse(re *core.RequestEvent, message string) error {
	return re.JSON(http.StatusOK, map[string]string{"message": message})
}

// DataResponse returns a 200 JSON response with arbitrary data
func DataResponse(re *core.RequestEvent, data any) error {
	return re.JSON(http.StatusOK, data)
}

// --- Pointer Helpers ---

// Pointer returns a pointer to the given string value
// Use types.Pointer for PocketBase rule fields
func Pointer(s string) *string {
	return types.Pointer(s)
}

// IntPointer returns a pointer to the given int value
func IntPointer(i int) *int {
	return &i
}

// --- Date Helpers ---

// ParseExpiryDate parses an expiry date string in various formats that PocketBase might use
func ParseExpiryDate(dateStr string) (time.Time, error) {
	// Try to parse with space format first (PocketBase seems to use this)
	if t, err := time.Parse("2006-01-02 15:04:05.000Z", dateStr); err == nil {
		return t, nil
	}

	// Try RFC3339 formats
	if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
		return t, nil
	}

	if t, err := time.Parse(time.RFC3339Nano, dateStr); err == nil {
		return t, nil
	}

	// Try other common formats
	formats := []string{
		"2006-01-02T15:04:05.000Z",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("could not parse date: %s", dateStr)
}

// --- Filter Helpers (SQL injection prevention) ---

// SafeFilterValue validates and escapes a value for use in PocketBase filter expressions.
// It validates that IDs and tokens match safe patterns (alphanumeric, dashes, underscores)
// and escapes single quotes to prevent injection.
func SafeFilterValue(value string) (string, error) {
	// Validate that the value matches a safe pattern for IDs/tokens
	// PocketBase IDs are typically alphanumeric with dashes and underscores
	safePattern := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !safePattern.MatchString(value) {
		return "", fmt.Errorf("invalid filter value: contains unsafe characters")
	}

	// Escape single quotes by doubling them (PocketBase filter syntax)
	escaped := strings.ReplaceAll(value, "'", "''")
	return escaped, nil
}

// BuildFilterEq builds a safe equality filter expression: "field = 'value'"
func BuildFilterEq(field, value string) (string, error) {
	safeValue, err := SafeFilterValue(value)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s = '%s'", field, safeValue), nil
}

// NormalizeEmail normalizes an email address (lowercase, trimmed)
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
