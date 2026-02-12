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

		if f := collection.Fields.GetByName("status"); f != nil {
			if sf, ok := f.(*core.SelectField); ok {
				for _, v := range sf.Values {
					if v == "pending" {
						log.Println("[Migration] contacts status already has 'pending', skipping")
						return nil
					}
				}
				sf.Values = []string{"active", "inactive", "pending", "archived"}
				if err := app.Save(collection); err != nil {
					return err
				}
				log.Println("[Migration] Added 'pending' to contacts status values")
			}
		}

		return nil
	}, func(app core.App) error {
		return nil
	})
}
