package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		// Create projection_logs collection
		logsCollection := core.NewBaseCollection("projection_logs")
		logsCollection.Fields.Add(
			&core.AutodateField{
				Id:       "proj_log_created",
				Name:     "created",
				OnCreate: true,
			},
			&core.NumberField{
				Id:       "proj_log_record_count",
				Name:     "record_count",
				Required: true,
				Min:      func() *float64 { v := 0.0; return &v }(),
			},
			&core.JSONField{
				Id:      "proj_log_consumers",
				Name:    "consumers",
				MaxSize: 10000,
			},
		)

		if err := app.Save(logsCollection); err != nil {
			return err
		}

		// Create projection_callbacks collection
		callbacksCollection := core.NewBaseCollection("projection_callbacks")
		callbacksCollection.Fields.Add(
			&core.TextField{
				Id:       "proj_cb_projection_id",
				Name:     "projection_id",
				Required: true,
				Max:      50,
			},
			&core.TextField{
				Id:       "proj_cb_consumer",
				Name:     "consumer",
				Required: true,
				Max:      100,
			},
			&core.SelectField{
				Id:        "proj_cb_status",
				Name:      "status",
				Required:  true,
				MaxSelect: 1,
				Values:    []string{"ok", "error", "partial"},
			},
			&core.TextField{
				Id:       "proj_cb_message",
				Name:     "message",
				Required: false,
				Max:      1000,
			},
			&core.NumberField{
				Id:       "proj_cb_records_processed",
				Name:     "records_processed",
				Required: false,
			},
			&core.AutodateField{
				Id:       "proj_cb_received_at",
				Name:     "received_at",
				OnCreate: true,
			},
		)

		callbacksCollection.Indexes = append(callbacksCollection.Indexes,
			"CREATE INDEX `idx_callbacks_projection_id` ON `projection_callbacks` (`projection_id`)",
		)

		if err := app.Save(callbacksCollection); err != nil {
			return err
		}

		return nil
	}, func(app core.App) error {
		logsCollection, err := app.FindCollectionByNameOrId("projection_logs")
		if err == nil {
			app.Delete(logsCollection)
		}

		callbacksCollection, err := app.FindCollectionByNameOrId("projection_callbacks")
		if err == nil {
			app.Delete(callbacksCollection)
		}

		return nil
	})
}
