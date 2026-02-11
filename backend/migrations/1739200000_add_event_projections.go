package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		existing, _ := app.FindCollectionByNameOrId("event_projections")
		if existing != nil {
			log.Println("[Migration] event_projections collection already exists")
			return nil
		}

		collection := core.NewBaseCollection("event_projections")
		collection.Fields.Add(
			&core.TextField{
				Id:       "ep_event_id",
				Name:     "event_id",
				Required: true,
				Max:      50,
			},
			&core.TextField{
				Id:       "ep_slug",
				Name:     "slug",
				Required: false,
				Max:      300,
			},
			&core.TextField{
				Id:       "ep_name",
				Name:     "name",
				Required: true,
				Max:      300,
			},
			&core.NumberField{
				Id:       "ep_edition_year",
				Name:     "edition_year",
				Required: false,
				OnlyInt:  true,
			},
			&core.TextField{
				Id:       "ep_timezone",
				Name:     "timezone",
				Required: false,
				Max:      100,
			},
			&core.TextField{
				Id:       "ep_date",
				Name:     "date",
				Required: false,
				Max:      20,
			},
			&core.TextField{
				Id:       "ep_start_time",
				Name:     "start_time",
				Required: false,
				Max:      10,
			},
			&core.TextField{
				Id:       "ep_end_time",
				Name:     "end_time",
				Required: false,
				Max:      10,
			},
			&core.TextField{
				Id:       "ep_start_date",
				Name:     "start_date",
				Required: false,
				Max:      20,
			},
			&core.TextField{
				Id:       "ep_end_date",
				Name:     "end_date",
				Required: false,
				Max:      20,
			},
			&core.TextField{
				Id:       "ep_venue",
				Name:     "venue",
				Required: false,
				Max:      300,
			},
			&core.TextField{
				Id:       "ep_venue_city",
				Name:     "venue_city",
				Required: false,
				Max:      200,
			},
			&core.TextField{
				Id:       "ep_venue_country",
				Name:     "venue_country",
				Required: false,
				Max:      200,
			},
			&core.TextField{
				Id:       "ep_format",
				Name:     "format",
				Required: false,
				Max:      50,
			},
			&core.TextField{
				Id:       "ep_event_type",
				Name:     "event_type",
				Required: false,
				Max:      50,
			},
			&core.TextField{
				Id:       "ep_status",
				Name:     "status",
				Required: false,
				Max:      50,
			},
			&core.NumberField{
				Id:       "ep_capacity",
				Name:     "capacity",
				Required: false,
				OnlyInt:  true,
			},
			&core.TextField{
				Id:       "ep_description",
				Name:     "description",
				Required: false,
				Max:      2000,
			},
			&core.DateField{
				Id:       "ep_orphaned_at",
				Name:     "orphaned_at",
				Required: false,
			},
			&core.AutodateField{
				Id:       "ep_last_synced",
				Name:     "last_synced",
				OnCreate: true,
				OnUpdate: true,
			},
			&core.AutodateField{
				Id:       "ep_created",
				Name:     "created",
				OnCreate: true,
			},
			&core.AutodateField{
				Id:       "ep_updated",
				Name:     "updated",
				OnCreate: true,
				OnUpdate: true,
			},
		)

		collection.Indexes = []string{
			"CREATE UNIQUE INDEX idx_ep_event_id ON event_projections (event_id)",
			"CREATE INDEX idx_ep_status ON event_projections (status)",
			"CREATE INDEX idx_ep_event_type ON event_projections (event_type)",
			"CREATE INDEX idx_ep_name ON event_projections (name)",
		}

		// Authenticated users can read, no API write (system-only via webhook)
		authRule := "@request.auth.id != ''"
		collection.ListRule = &authRule
		collection.ViewRule = &authRule
		collection.CreateRule = nil
		collection.UpdateRule = nil
		collection.DeleteRule = nil

		if err := app.Save(collection); err != nil {
			return err
		}

		log.Println("[Migration] Created event_projections collection")
		return nil
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("event_projections")
		if err == nil {
			app.Delete(collection)
		}
		return nil
	})
}
