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
			&core.FileField{
				Id:        "gl_landing_image",
				Name:      "landing_image",
				MaxSelect: 1,
				MaxSize:   10 * 1024 * 1024, // 10MB
				MimeTypes: []string{
					"image/jpeg",
					"image/png",
					"image/webp",
				},
			},
			&core.TextField{
				Id:       "gl_program_title",
				Name:     "program_title",
				Required: false,
				Max:      200,
			},
		)

		if err := app.Save(collection); err != nil {
			return err
		}

		log.Println("[Migration] Added landing_image file field and program_title to guest_lists")
		return nil
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("guest_lists")
		if err != nil {
			return nil
		}
		collection.Fields.RemoveById("gl_landing_image")
		collection.Fields.RemoveById("gl_program_title")
		app.Save(collection)
		return nil
	})
}
