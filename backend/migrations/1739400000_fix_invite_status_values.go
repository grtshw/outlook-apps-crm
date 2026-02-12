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

		// Fix invite_status: "pending" → "invited"
		if f := collection.Fields.GetByName("invite_status"); f != nil {
			if sf, ok := f.(*core.SelectField); ok {
				for _, v := range sf.Values {
					if v == "pending" {
						sf.Values = []string{"invited", "accepted", "declined", "no_show"}
						if err := app.Save(collection); err != nil {
							return err
						}
						// Update existing records
						if records, err := app.FindRecordsByFilter("guest_list_items", "invite_status = 'pending'", "", 0, 0, nil); err == nil {
							for _, r := range records {
								r.Set("invite_status", "invited")
								app.Save(r)
							}
						}
						log.Println("[Migration] Fixed invite_status values")
						return nil
					}
				}
			}
		}

		log.Println("[Migration] invite_status already uses 'invited', skipping")
		return nil
	}, func(app core.App) error {
		return nil
	})
}
