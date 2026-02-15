package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(func(app core.App) error {
		// --- Add humanitix fields to contacts ---
		contacts, err := app.FindCollectionByNameOrId("contacts")
		if err != nil {
			return err
		}

		contacts.Fields.Add(&core.TextField{
			Id:  "contact_humanitix_order_id",
			Name: "humanitix_order_id",
			Max:  200,
		})

		contacts.Fields.Add(&core.TextField{
			Id:  "contact_humanitix_attendee_id",
			Name: "humanitix_attendee_id",
			Max:  200,
		})

		// Add "humanitix" to source select values
		for _, f := range contacts.Fields {
			if sf, ok := f.(*core.SelectField); ok && sf.Name == "source" {
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

		if err := app.Save(contacts); err != nil {
			return err
		}

		// --- Create humanitix_sync_log collection ---
		collection := core.NewBaseCollection("humanitix_sync_log")

		collection.Fields.Add(&core.SelectField{
			Id:        "hsl_sync_type",
			Name:      "sync_type",
			Required:  true,
			MaxSelect: 1,
			Values:    []string{"manual", "scheduled", "webhook"},
		})

		collection.Fields.Add(&core.TextField{
			Id:  "hsl_event_id",
			Name: "event_id",
			Max:  200,
		})

		collection.Fields.Add(&core.TextField{
			Id:  "hsl_event_name",
			Name: "event_name",
			Max:  500,
		})

		collection.Fields.Add(&core.NumberField{
			Id:   "hsl_records_processed",
			Name: "records_processed",
		})

		collection.Fields.Add(&core.NumberField{
			Id:   "hsl_records_created",
			Name: "records_created",
		})

		collection.Fields.Add(&core.NumberField{
			Id:   "hsl_records_updated",
			Name: "records_updated",
		})

		collection.Fields.Add(&core.JSONField{
			Id:      "hsl_errors",
			Name:    "errors",
			MaxSize: 10000,
		})

		collection.Fields.Add(&core.DateField{
			Id:   "hsl_started_at",
			Name: "started_at",
		})

		collection.Fields.Add(&core.DateField{
			Id:   "hsl_completed_at",
			Name: "completed_at",
		})

		collection.Fields.Add(&core.SelectField{
			Id:        "hsl_status",
			Name:      "status",
			Required:  true,
			MaxSelect: 1,
			Values:    []string{"running", "completed", "failed"},
		})

		collection.ListRule = types.Pointer("@request.auth.id != ''")
		collection.ViewRule = types.Pointer("@request.auth.id != ''")
		collection.CreateRule = types.Pointer("@request.auth.role = 'admin'")
		collection.UpdateRule = types.Pointer("@request.auth.role = 'admin'")
		collection.DeleteRule = types.Pointer("@request.auth.role = 'admin'")

		if err := app.Save(collection); err != nil {
			return err
		}

		log.Println("[Migration] Added humanitix integration fields and sync log collection")
		return nil
	}, func(app core.App) error {
		// Rollback: remove sync log collection
		if collection, err := app.FindCollectionByNameOrId("humanitix_sync_log"); err == nil {
			app.Delete(collection)
		}
		return nil
	})
}
