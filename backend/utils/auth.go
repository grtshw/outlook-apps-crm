package utils

import (
	"log"
	"net/http"

	"github.com/pocketbase/pocketbase/core"
)

// RequireAuth is middleware that requires authentication
func RequireAuth(e *core.RequestEvent) error {
	if e.Auth == nil {
		log.Printf("[Auth] Unauthorized request to %s from %s", e.Request.URL.Path, e.RealIP())
		return e.JSON(http.StatusUnauthorized, map[string]string{
			"error": "Unauthorized",
		})
	}
	return e.Next()
}

// RequireAdmin is middleware that requires admin role
func RequireAdmin(e *core.RequestEvent) error {
	if e.Auth == nil {
		log.Printf("[Auth] Unauthorized request to %s from %s", e.Request.URL.Path, e.RealIP())
		return e.JSON(http.StatusUnauthorized, map[string]string{
			"error": "Unauthorized",
		})
	}

	if !IsAdmin(e.Auth) {
		log.Printf("[Auth] Forbidden request to %s from user %s", e.Request.URL.Path, e.Auth.Id)
		return e.JSON(http.StatusForbidden, map[string]string{
			"error": "Forbidden",
		})
	}

	return e.Next()
}

// GetUserRole extracts the user role from a record
func GetUserRole(record *core.Record) string {
	if record == nil {
		return ""
	}
	return record.GetString(FieldRole)
}

// IsAdmin checks if a record has admin role or is a superuser
func IsAdmin(record *core.Record) bool {
	if record == nil {
		return false
	}
	// Superusers always have admin access
	if record.Collection().Name == "_superusers" {
		return true
	}
	return GetUserRole(record) == "admin"
}

// IsViewer checks if a record has viewer role
func IsViewer(record *core.Record) bool {
	if record == nil {
		return false
	}
	role := GetUserRole(record)
	return role == "viewer" || role == "admin"
}
