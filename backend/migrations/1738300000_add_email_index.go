package migrations

import (
	"log"
	"strings"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("contacts")
		if err != nil {
			return err
		}

		// Check if email_index field already exists
		for _, field := range collection.Fields {
			if field.GetName() == "email_index" {
				log.Println("[Migration] email_index field already exists")
				return nil
			}
		}

		// Add email_index field for blind index lookups on encrypted email
		collection.Fields.Add(&core.TextField{
			Id:       "cont_email_index",
			Name:     "email_index",
			Required: false,
			Max:      64, // SHA-256 hex = 64 characters
		})

		// Update indexes: add email_index unique index
		// Keep existing indexes, add new one for email_index
		hasEmailIndexIndex := false
		for _, idx := range collection.Indexes {
			if strings.Contains(idx, "email_index") {
				hasEmailIndexIndex = true
				break
			}
		}

		if !hasEmailIndexIndex {
			collection.Indexes = append(collection.Indexes,
				"CREATE UNIQUE INDEX idx_contacts_email_index ON contacts (email_index) WHERE email_index != ''",
			)
		}

		if err := app.Save(collection); err != nil {
			return err
		}

		log.Println("[Migration] Added email_index field to contacts collection")
		return nil
	}, func(app core.App) error {
		// Rollback: remove the email_index field
		collection, err := app.FindCollectionByNameOrId("contacts")
		if err != nil {
			return nil
		}

		// Remove field
		for i, field := range collection.Fields {
			if field.GetName() == "email_index" {
				collection.Fields = append(collection.Fields[:i], collection.Fields[i+1:]...)
				break
			}
		}

		// Remove index
		var newIndexes []string
		for _, idx := range collection.Indexes {
			if !strings.Contains(idx, "email_index") {
				newIndexes = append(newIndexes, idx)
			}
		}
		collection.Indexes = newIndexes

		return app.Save(collection)
	})
}
