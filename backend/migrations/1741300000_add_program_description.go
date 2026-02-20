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

		collection.Fields.Add(&core.TextField{
			Id:  "gl_program_description",
			Name: "program_description",
			Max:  2000,
		})

		if err := app.Save(collection); err != nil {
			return err
		}

		log.Println("[Migration] Added program_description to guest_lists")
		return nil
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("guest_lists")
		if err != nil {
			return nil
		}
		collection.Fields.RemoveById("gl_program_description")
		app.Save(collection)
		return nil
	})
}
