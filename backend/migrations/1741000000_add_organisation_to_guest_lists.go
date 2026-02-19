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
				Id:       "gl_organisation",
				Name:     "organisation",
				Required: false,
				Max:      50,
			},
			&core.TextField{
				Id:       "gl_organisation_name",
				Name:     "organisation_name",
				Required: false,
				Max:      300,
			},
			&core.URLField{
				Id:       "gl_organisation_logo_url",
				Name:     "organisation_logo_url",
				Required: false,
			},
		)

		if err := app.Save(collection); err != nil {
			return err
		}

		log.Println("[Migration] Added organisation, organisation_name, organisation_logo_url to guest_lists")
		return nil
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("guest_lists")
		if err != nil {
			return nil
		}
		collection.Fields.RemoveById("gl_organisation")
		collection.Fields.RemoveById("gl_organisation_name")
		collection.Fields.RemoveById("gl_organisation_logo_url")
		app.Save(collection)
		return nil
	})
}
