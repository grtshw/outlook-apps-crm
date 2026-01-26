package utils

import (
	"log"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// AuditEntry represents an audit log entry
type AuditEntry struct {
	UserID       string
	UserEmail    string
	Action       string // create, read, update, delete, login, logout, login_failed, api_call, webhook_received, webhook_sent
	ResourceType string
	ResourceID   string
	IPAddress    string
	UserAgent    string
	Changes      map[string]any
	Metadata     map[string]any
	Status       string // success, failure, error
	ErrorMessage string
}

// LogAudit creates an audit log entry asynchronously to avoid blocking requests
func LogAudit(app *pocketbase.PocketBase, entry AuditEntry) {
	go func() {
		collection, err := app.FindCollectionByNameOrId("audit_logs")
		if err != nil {
			log.Printf("[Audit] Collection not found: %v", err)
			return
		}

		record := core.NewRecord(collection)
		record.Set("user_id", entry.UserID)
		record.Set("user_email", entry.UserEmail)
		record.Set("action", entry.Action)
		record.Set("resource_type", entry.ResourceType)
		record.Set("resource_id", entry.ResourceID)
		record.Set("ip_address", entry.IPAddress)
		record.Set("user_agent", entry.UserAgent)
		record.Set("changes", entry.Changes)
		record.Set("metadata", entry.Metadata)
		record.Set("status", entry.Status)
		record.Set("error_message", entry.ErrorMessage)

		if err := app.Save(record); err != nil {
			log.Printf("[Audit] Failed to save audit log: %v", err)
		}
	}()
}

// LogFromRequest creates an audit entry from a request event
func LogFromRequest(app *pocketbase.PocketBase, re *core.RequestEvent, action, resourceType, resourceID, status string, changes map[string]any, errorMessage string) {
	entry := AuditEntry{
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		IPAddress:    re.RealIP(),
		UserAgent:    re.Request.UserAgent(),
		Changes:      changes,
		Status:       status,
		ErrorMessage: errorMessage,
	}

	if re.Auth != nil {
		entry.UserID = re.Auth.Id
		entry.UserEmail = re.Auth.GetString("email")
	}

	LogAudit(app, entry)
}

// LogRecordChange logs a record change from PocketBase hooks
func LogRecordChange(app *pocketbase.PocketBase, action, resourceType, resourceID string, changes map[string]any) {
	LogAudit(app, AuditEntry{
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Changes:      changes,
		Status:       "success",
	})
}

// LogAuthEvent logs authentication events
func LogAuthEvent(app *pocketbase.PocketBase, action, userID, userEmail, ipAddress, userAgent, status, errorMessage string) {
	LogAudit(app, AuditEntry{
		UserID:       userID,
		UserEmail:    userEmail,
		Action:       action,
		ResourceType: "users",
		ResourceID:   userID,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		Status:       status,
		ErrorMessage: errorMessage,
	})
}

// LogWebhook logs webhook events (received or sent)
func LogWebhook(app *pocketbase.PocketBase, action, resourceType, resourceID, status string, metadata map[string]any, errorMessage string) {
	LogAudit(app, AuditEntry{
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Metadata:     metadata,
		Status:       status,
		ErrorMessage: errorMessage,
	})
}
