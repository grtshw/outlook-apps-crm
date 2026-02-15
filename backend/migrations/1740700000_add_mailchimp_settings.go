package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection := core.NewBaseCollection("mailchimp_settings")

		collection.Fields.Add(&core.TextField{
			Id:   "mc_list_id",
			Name: "list_id",
			Max:  200,
		})

		collection.Fields.Add(&core.TextField{
			Id:   "mc_list_name",
			Name: "list_name",
			Max:  200,
		})

		collection.Fields.Add(&core.JSONField{
			Id:   "mc_merge_field_mappings",
			Name: "merge_field_mappings",
		})

		// Admin-only access
		collection.ListRule = nil
		collection.ViewRule = nil
		collection.CreateRule = nil
		collection.UpdateRule = nil
		collection.DeleteRule = nil

		if err := app.Save(collection); err != nil {
			return err
		}

		log.Println("[Migration] Created mailchimp_settings collection")
		return nil
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("mailchimp_settings")
		if err != nil {
			return nil
		}
		return app.Delete(collection)
	})
}
