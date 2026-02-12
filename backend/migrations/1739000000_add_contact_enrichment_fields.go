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

		changed := false

		// degrees - connection degree (1st degree, 2nd degree, 3rd degree)
		if !fieldExists(collection, "degrees") {
			collection.Fields.Add(&core.SelectField{
				Id:        "cont_degrees",
				Name:      "degrees",
				Required:  false,
				MaxSelect: 1,
				Values:    []string{"1st", "2nd", "3rd"},
			})
			changed = true
		}

		// relationship - strength rating 0-5
		if !fieldExists(collection, "relationship") {
			min := 0.0
			max := 5.0
			collection.Fields.Add(&core.NumberField{
				Id:       "cont_relationship",
				Name:     "relationship",
				Required: false,
				OnlyInt:  true,
				Min:      &min,
				Max:      &max,
			})
			changed = true
		}

		// notes - internal notes about the contact
		if !fieldExists(collection, "notes") {
			collection.Fields.Add(&core.TextField{
				Id:       "cont_notes",
				Name:     "notes",
				Required: false,
				Max:      10000,
			})
			changed = true
		}

		if !changed {
			log.Println("[Migration] Contact enrichment fields already exist")
			return nil
		}

		if err := app.Save(collection); err != nil {
			return err
		}

		log.Println("[Migration] Added degrees, relationship, notes fields to contacts collection")
		return nil
	}, func(app core.App) error {
		// Rollback: remove the fields
		collection, err := app.FindCollectionByNameOrId("contacts")
		if err != nil {
			return nil
		}

		collection.Fields.RemoveByName("degrees")
		collection.Fields.RemoveByName("relationship")
		collection.Fields.RemoveByName("notes")

		return app.Save(collection)
	})
}
