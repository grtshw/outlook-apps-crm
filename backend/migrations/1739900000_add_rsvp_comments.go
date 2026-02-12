package migrations

import (
	"log"
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("guest_list_items")
		if err != nil {
			return err
		}

		if fieldExists(collection, "rsvp_comments") {
			log.Println("[Migration] rsvp_comments field already exists")
			return nil
		}

		collection.Fields.Add(&core.TextField{
			Id:       "gli_rsvp_comments",
			Name:     "rsvp_comments",
			Required: false,
			Max:      2000,
		})

		if err := app.Save(collection); err != nil {
			return err
		}

		log.Println("[Migration] Added rsvp_comments field to guest_list_items")
		return nil
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("guest_list_items")
		if err != nil {
			return nil
		}

		collection.Fields.RemoveById("gli_rsvp_comments")

		return app.Save(collection)
	})
}
