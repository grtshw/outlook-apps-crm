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
			&core.TextField{
				Id:       "gl_event_date",
				Name:     "event_date",
				Required: false,
				Max:      100,
			},
			&core.TextField{
				Id:       "gl_event_time",
				Name:     "event_time",
				Required: false,
				Max:      100,
			},
			&core.TextField{
				Id:       "gl_event_location",
				Name:     "event_location",
				Required: false,
				Max:      500,
			},
			&core.TextField{
				Id:       "gl_event_location_address",
				Name:     "event_location_address",
				Required: false,
				Max:      500,
			},
		)

		if err := app.Save(collection); err != nil {
			return err
		}

		log.Println("[Migration] Added event_date, event_time, event_location to guest_lists")
		return nil
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("guest_lists")
		if err != nil {
			return nil
		}
		collection.Fields.RemoveById("gl_event_date")
		collection.Fields.RemoveById("gl_event_time")
		collection.Fields.RemoveById("gl_event_location")
		collection.Fields.RemoveById("gl_event_location_address")
		app.Save(collection)
		return nil
	})
}
