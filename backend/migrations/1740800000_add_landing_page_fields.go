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
				Id:   "gl_landing_enabled",
				Name: "landing_enabled",
			},
			&core.TextField{
				Id:       "gl_landing_headline",
				Name:     "landing_headline",
				Required: false,
				Max:      300,
			},
			&core.TextField{
				Id:       "gl_landing_description",
				Name:     "landing_description",
				Required: false,
				Max:      10000,
			},
			&core.TextField{
				Id:       "gl_landing_image_url",
				Name:     "landing_image_url",
				Required: false,
				Max:      2000,
			},
			&core.JSONField{
				Id:      "gl_landing_program",
				Name:    "landing_program",
				MaxSize: 50000,
			},
			&core.TextField{
				Id:       "gl_landing_content",
				Name:     "landing_content",
				Required: false,
				Max:      10000,
			},
		)

		if err := app.Save(collection); err != nil {
			return err
		}

		log.Println("[Migration] Added landing page fields to guest_lists")
		return nil
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("guest_lists")
		if err != nil {
			return nil
		}
		collection.Fields.RemoveById("gl_landing_enabled")
		collection.Fields.RemoveById("gl_landing_headline")
		collection.Fields.RemoveById("gl_landing_description")
		collection.Fields.RemoveById("gl_landing_image_url")
		collection.Fields.RemoveById("gl_landing_program")
		collection.Fields.RemoveById("gl_landing_content")
		app.Save(collection)
		return nil
	})
}
