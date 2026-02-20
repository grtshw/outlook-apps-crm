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

		if fieldExists(collection, "invite_opened") {
			log.Println("[Migration] invite_opened field already exists")
			return nil
		}

		collection.Fields.Add(&core.BoolField{
			Id:   "gli_invite_opened",
			Name: "invite_opened",
		})

		collection.Fields.Add(&core.BoolField{
			Id:   "gli_invite_clicked",
			Name: "invite_clicked",
		})

		if err := app.Save(collection); err != nil {
			return err
		}

		log.Println("[Migration] Added invite_opened and invite_clicked fields to guest_list_items")
		return nil
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("guest_list_items")
		if err != nil {
			return nil
		}

		collection.Fields.RemoveById("gli_invite_opened")
		collection.Fields.RemoveById("gli_invite_clicked")

		return app.Save(collection)
	})
}
