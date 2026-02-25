package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("humanitix_sync_log")
		if err != nil {
			return err
		}

		for _, f := range collection.Fields {
			if sf, ok := f.(*core.SelectField); ok && sf.Name == "sync_type" {
				hasCSV := false
				for _, v := range sf.Values {
					if v == "csv_import" {
						hasCSV = true
						break
					}
				}
				if !hasCSV {
					sf.Values = append(sf.Values, "csv_import")
				}
				break
			}
		}

		if err := app.Save(collection); err != nil {
			return err
		}

		log.Println("[Migration] Added csv_import to humanitix_sync_log sync_type")
		return nil
	}, func(app core.App) error {
		return nil
	})
}
