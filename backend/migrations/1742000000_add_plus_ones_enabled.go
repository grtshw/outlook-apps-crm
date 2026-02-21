package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("guest_lists")
		if err != nil {
			return err
		}

		collection.Fields.Add(
			&core.BoolField{
				Id:   "gl_rsvp_plus_ones_enabled",
				Name: "rsvp_plus_ones_enabled",
			},
		)

		if err := app.Save(collection); err != nil {
			return err
		}

		// Default all existing guest lists to enabled
		records, err := app.FindRecordsByFilter("guest_lists", "1=1", "", 0, 0)
		if err == nil {
			for _, r := range records {
				r.Set("rsvp_plus_ones_enabled", true)
				app.Save(r)
			}
		}

		log.Println("[Migration] Added rsvp_plus_ones_enabled to guest_lists")
		return nil
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("guest_lists")
		if err != nil {
			return nil
		}
		collection.Fields.RemoveById("gl_rsvp_plus_ones_enabled")
		app.Save(collection)
		return nil
	})
}
