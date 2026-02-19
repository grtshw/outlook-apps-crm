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
			&core.JSONField{
				Id:      "gl_rsvp_bcc_contacts",
				Name:    "rsvp_bcc_contacts",
				MaxSize: 10000,
			},
		)

		if err := app.Save(collection); err != nil {
			return err
		}

		log.Println("[Migration] Added rsvp_bcc_contacts to guest_lists")
		return nil
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("guest_lists")
		if err != nil {
			return nil
		}
		collection.Fields.RemoveById("gl_rsvp_bcc_contacts")
		app.Save(collection)
		return nil
	})
}
