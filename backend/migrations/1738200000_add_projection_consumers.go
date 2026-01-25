package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		// Check if collection already exists
		existing, _ := app.FindCollectionByNameOrId("projection_consumers")
		if existing != nil {
			log.Println("[Migration] projection_consumers collection already exists")
			return nil
		}

		// Create projection_consumers collection
		collection := core.NewBaseCollection("projection_consumers")
		collection.Fields.Add(
			&core.TextField{
				Id:       "proj_cons_name",
				Name:     "name",
				Required: true,
				Max:      100,
			},
			&core.TextField{
				Id:       "proj_cons_app_id",
				Name:     "app_id",
				Required: true,
				Max:      50,
			},
			&core.URLField{
				Id:       "proj_cons_endpoint_url",
				Name:     "endpoint_url",
				Required: true,
			},
			&core.TextField{
				Id:       "proj_cons_webhook_secret",
				Name:     "webhook_secret",
				Required: false,
				Max:      100,
			},
			&core.BoolField{
				Id:   "proj_cons_enabled",
				Name: "enabled",
			},
			&core.TextField{
				Id:       "proj_cons_icon",
				Name:     "icon",
				Required: false,
				Max:      50,
			},
			&core.DateField{
				Id:       "proj_cons_last_consumption",
				Name:     "last_consumption",
				Required: false,
			},
			&core.SelectField{
				Id:        "proj_cons_last_status",
				Name:      "last_status",
				Required:  false,
				MaxSelect: 1,
				Values:    []string{"ok", "error", "pending"},
			},
			&core.TextField{
				Id:       "proj_cons_last_message",
				Name:     "last_message",
				Required: false,
				Max:      1000,
			},
			&core.AutodateField{
				Id:       "proj_cons_created",
				Name:     "created",
				OnCreate: true,
			},
			&core.AutodateField{
				Id:       "proj_cons_updated",
				Name:     "updated",
				OnCreate: true,
				OnUpdate: true,
			},
		)

		// Add unique index on app_id
		collection.Indexes = append(collection.Indexes,
			"CREATE UNIQUE INDEX `idx_proj_cons_app_id` ON `projection_consumers` (`app_id`)",
		)

		// Access rules - admin only
		adminRule := "@request.auth.role = 'admin'"
		collection.ListRule = &adminRule
		collection.ViewRule = &adminRule
		collection.CreateRule = &adminRule
		collection.UpdateRule = &adminRule
		collection.DeleteRule = &adminRule

		if err := app.Save(collection); err != nil {
			return err
		}

		log.Println("[Migration] Created projection_consumers collection")

		// Seed with default consumers
		consumers := []struct {
			Name        string
			AppID       string
			EndpointURL string
			Icon        string
		}{
			{
				Name:        "Presentations",
				AppID:       "presentations",
				EndpointURL: "https://outlook-apps-presentations.fly.dev/api/webhooks/contact-projection",
				Icon:        "easel",
			},
			{
				Name:        "DAM",
				AppID:       "dam",
				EndpointURL: "https://outlook-apps-dam.fly.dev/api/webhooks/contact-projection",
				Icon:        "images",
			},
			{
				Name:        "Website",
				AppID:       "website",
				EndpointURL: "https://theoutlook.com.au/api/webhooks/contact-projection",
				Icon:        "globe",
			},
		}

		for _, c := range consumers {
			record := core.NewRecord(collection)
			record.Set("name", c.Name)
			record.Set("app_id", c.AppID)
			record.Set("endpoint_url", c.EndpointURL)
			record.Set("icon", c.Icon)
			record.Set("enabled", true)
			record.Set("webhook_secret", "") // To be set by admin

			if err := app.Save(record); err != nil {
				log.Printf("[Migration] Failed to seed %s consumer: %v", c.Name, err)
				continue
			}
			log.Printf("[Migration] Seeded %s consumer", c.Name)
		}

		return nil
	}, func(app core.App) error {
		// Rollback: delete the collection
		collection, err := app.FindCollectionByNameOrId("projection_consumers")
		if err == nil {
			app.Delete(collection)
		}
		return nil
	})
}
