package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("projection_consumers")
		if err != nil {
			log.Println("[Migration] projection_consumers collection not found, skipping")
			return nil
		}

		// Check if awards consumer already exists
		existing, _ := app.FindFirstRecordByFilter("projection_consumers", "app_id = 'awards'", nil)
		if existing != nil {
			log.Println("[Migration] Awards projection consumer already exists")
			return nil
		}

		record := core.NewRecord(collection)
		record.Set("name", "Awards")
		record.Set("app_id", "awards")
		record.Set("endpoint_url", "https://outlook-apps-awards.fly.dev/api/webhooks/contact-projection")
		record.Set("icon", "trophy")
		record.Set("enabled", true)
		record.Set("webhook_secret", "")

		if err := app.Save(record); err != nil {
			return err
		}

		log.Println("[Migration] Seeded Awards projection consumer")
		return nil
	}, func(app core.App) error {
		existing, _ := app.FindFirstRecordByFilter("projection_consumers", "app_id = 'awards'", nil)
		if existing != nil {
			return app.Delete(existing)
		}
		return nil
	})
}
