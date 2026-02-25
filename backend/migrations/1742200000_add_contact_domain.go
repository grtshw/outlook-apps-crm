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

		collection.Fields.Add(&core.SelectField{
			Id:        "contact_domain",
			Name:      "domain",
			Required:  false,
			MaxSelect: 3,
			Values:    []string{"design", "product", "leadership"},
		})

		if err := app.Save(collection); err != nil {
			return err
		}

		log.Println("[Migration] Added domain field to contacts")
		return nil
	}, func(app core.App) error {
		return nil
	})
}
