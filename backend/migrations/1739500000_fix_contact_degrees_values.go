package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("guest_list_items")
		if err != nil {
			return err
		}

		// Fix contact_degrees: "1st degree" → "1st", etc.
		f := collection.Fields.GetByName("contact_degrees")
		if f == nil {
			log.Println("[Migration] contact_degrees field not found, skipping")
			return nil
		}

		sf, ok := f.(*core.SelectField)
		if !ok {
			return nil
		}

		needsFix := false
		for _, v := range sf.Values {
			if v == "1st degree" {
				needsFix = true
				break
			}
		}

		if !needsFix {
			log.Println("[Migration] contact_degrees already uses short values, skipping")
			return nil
		}

		// Update schema
		sf.Values = []string{"1st", "2nd", "3rd"}
		if err := app.Save(collection); err != nil {
			return err
		}

		// Update existing records
		for _, old := range []string{"1st degree", "2nd degree", "3rd degree"} {
			newVal := old[:3] // "1st", "2nd", "3rd"
			if records, err := app.FindRecordsByFilter("guest_list_items", "contact_degrees = {:val}", "", 0, 0, map[string]any{"val": old}); err == nil {
				for _, r := range records {
					r.Set("contact_degrees", newVal)
					if err := app.Save(r); err != nil {
						log.Printf("[Migration] Failed to update contact_degrees for %s: %v", r.Id, err)
					}
				}
			}
		}

		log.Println("[Migration] Fixed contact_degrees values in guest_list_items")
		return nil
	}, func(app core.App) error {
		return nil
	})
}
