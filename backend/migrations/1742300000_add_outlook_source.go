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

		// Add "outlook" to the source select field
		sourceField := collection.Fields.GetByName("source").(*core.SelectField)
		sourceField.Values = append(sourceField.Values, "outlook-addin")
		collection.Fields.Add(sourceField)

		if err := app.Save(collection); err != nil {
			return err
		}

		log.Println("[Migration] Added 'outlook-addin' to contacts source field")
		return nil
	}, func(app core.App) error {
		return nil
	})
}
