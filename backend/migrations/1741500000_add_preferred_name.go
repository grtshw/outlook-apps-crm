package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("contacts")
		if err != nil {
			return err
		}

		if fieldExists(collection, "preferred_name") {
			log.Println("[Migration] preferred_name field already exists on contacts")
			return nil
		}

		collection.Fields.Add(&core.TextField{
			Id:   "cont_preferred_name",
			Name: "preferred_name",
			Max:  200,
		})

		if err := app.Save(collection); err != nil {
			return err
		}

		log.Println("[Migration] Added preferred_name field to contacts collection")
		return nil
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("contacts")
		if err != nil {
			return nil
		}
		collection.Fields.RemoveByName("preferred_name")
		return app.Save(collection)
	})
}
