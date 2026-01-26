package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		// Check if collection already exists
		existing, _ := app.FindCollectionByNameOrId("audit_logs")
		if existing != nil {
			log.Println("[Migration] audit_logs collection already exists")
			return nil
		}

		// Create audit_logs collection
		collection := core.NewBaseCollection("audit_logs")
		collection.Fields.Add(
			&core.TextField{
				Id:       "audit_user_id",
				Name:     "user_id",
				Required: false,
				Max:      50,
			},
			&core.TextField{
				Id:       "audit_user_email",
				Name:     "user_email",
				Required: false,
				Max:      200,
			},
			&core.SelectField{
				Id:        "audit_action",
				Name:      "action",
				Required:  true,
				MaxSelect: 1,
				Values: []string{
					"create", "read", "update", "delete",
					"login", "logout", "login_failed",
					"api_call", "webhook_received", "webhook_sent",
				},
			},
			&core.TextField{
				Id:       "audit_resource_type",
				Name:     "resource_type",
				Required: true,
				Max:      50,
			},
			&core.TextField{
				Id:       "audit_resource_id",
				Name:     "resource_id",
				Required: false,
				Max:      50,
			},
			&core.TextField{
				Id:       "audit_ip_address",
				Name:     "ip_address",
				Required: false,
				Max:      45, // IPv6 max length
			},
			&core.TextField{
				Id:       "audit_user_agent",
				Name:     "user_agent",
				Required: false,
				Max:      500,
			},
			&core.JSONField{
				Id:      "audit_changes",
				Name:    "changes",
				MaxSize: 50000,
			},
			&core.JSONField{
				Id:      "audit_metadata",
				Name:    "metadata",
				MaxSize: 10000,
			},
			&core.SelectField{
				Id:        "audit_status",
				Name:      "status",
				Required:  true,
				MaxSelect: 1,
				Values:    []string{"success", "failure", "error"},
			},
			&core.TextField{
				Id:       "audit_error_message",
				Name:     "error_message",
				Required: false,
				Max:      1000,
			},
			&core.AutodateField{
				Id:       "audit_created",
				Name:     "created",
				OnCreate: true,
			},
		)

		// Add indexes for common queries
		collection.Indexes = []string{
			"CREATE INDEX idx_audit_user ON audit_logs (user_id)",
			"CREATE INDEX idx_audit_action ON audit_logs (action)",
			"CREATE INDEX idx_audit_resource ON audit_logs (resource_type, resource_id)",
			"CREATE INDEX idx_audit_created ON audit_logs (created)",
			"CREATE INDEX idx_audit_ip ON audit_logs (ip_address)",
		}

		// Access rules - admin only for viewing, no create/update/delete via API
		adminRule := "@request.auth.role = 'admin'"
		collection.ListRule = &adminRule
		collection.ViewRule = &adminRule
		collection.CreateRule = nil // Only system can create
		collection.UpdateRule = nil // Never update audit logs
		collection.DeleteRule = nil // Never delete audit logs via API

		if err := app.Save(collection); err != nil {
			return err
		}

		log.Println("[Migration] Created audit_logs collection")
		return nil
	}, func(app core.App) error {
		// Rollback: delete the collection
		collection, err := app.FindCollectionByNameOrId("audit_logs")
		if err == nil {
			app.Delete(collection)
		}
		return nil
	})
}
