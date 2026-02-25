package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("activities")
		if err != nil {
			return err
		}

		for _, f := range collection.Fields {
			if sf, ok := f.(*core.SelectField); ok && sf.Name == "source_app" {
				hasHumanitix := false
				for _, v := range sf.Values {
					if v == "humanitix" {
						hasHumanitix = true
						break
					}
				}
				if !hasHumanitix {
					sf.Values = append(sf.Values, "humanitix")
				}
				break
			}
		}

		if err := app.Save(collection); err != nil {
			return err
		}

		log.Println("[Migration] Added humanitix to activities source_app")
		return nil
	}, func(app core.App) error {
		return nil
	})
}
