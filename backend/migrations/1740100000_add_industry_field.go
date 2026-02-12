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

		collection.Fields.Add(&core.SelectField{
			Id:        "org_industry",
			Name:      "industry",
			Required:  false,
			MaxSelect: 1,
			Values: []string{
				"technology",
				"media",
				"finance",
				"healthcare",
				"education",
				"government",
				"retail",
				"manufacturing",
				"hospitality",
				"real_estate",
				"energy",
				"professional_services",
				"non_profit",
				"sports",
				"entertainment",
				"other",
			},
		})

		if err := app.Save(collection); err != nil {
			return err
		}

		log.Println("[Migration] Added industry field to organisations")
		return nil
	}, func(app core.App) error {
		return nil
	})
}
