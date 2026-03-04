package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("organisations")
		if err != nil {
			return err
		}

		collection.Fields.Add(&core.JSONField{
			Id:       "org_logo_urls",
			Name:     "logo_urls",
			Required: false,
			MaxSize:  10000,
		})

		if err := app.Save(collection); err != nil {
			return err
		}

		log.Println("[Migration] Added logo_urls field to organisations")
		return nil
	}, func(app core.App) error {
		if collection, err := app.FindCollectionByNameOrId("organisations"); err == nil {
			collection.Fields.RemoveById("org_logo_urls")
			app.Save(collection)
		}
		return nil
	})
}
