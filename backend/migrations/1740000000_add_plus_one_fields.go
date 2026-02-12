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

		newFields := []struct {
			id   string
			name string
			max  int
		}{
			{"gli_rsvp_plus_one_last_name", "rsvp_plus_one_last_name", 200},
			{"gli_rsvp_plus_one_job_title", "rsvp_plus_one_job_title", 200},
			{"gli_rsvp_plus_one_company", "rsvp_plus_one_company", 200},
			{"gli_rsvp_plus_one_email", "rsvp_plus_one_email", 300},
		}

		added := 0
		for _, f := range newFields {
			if fieldExists(collection, f.name) {
				continue
			}
			collection.Fields.Add(&core.TextField{
				Id:       f.id,
				Name:     f.name,
				Required: false,
				Max:      f.max,
			})
			added++
		}

		if added == 0 {
			log.Println("[Migration] Plus-one fields already exist")
			return nil
		}

		if err := app.Save(collection); err != nil {
			return err
		}

		log.Printf("[Migration] Added %d plus-one fields to guest_list_items", added)
		return nil
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("guest_list_items")
		if err != nil {
			return nil
		}

		collection.Fields.RemoveById("gli_rsvp_plus_one_last_name")
		collection.Fields.RemoveById("gli_rsvp_plus_one_job_title")
		collection.Fields.RemoveById("gli_rsvp_plus_one_company")
		collection.Fields.RemoveById("gli_rsvp_plus_one_email")

		return app.Save(collection)
	})
}
