package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		if err := addRSVPFieldsToGuestLists(app); err != nil {
			return err
		}
		if err := addRSVPFieldsToGuestListItems(app); err != nil {
			return err
		}
		log.Println("[Migration] Added RSVP fields to guest list collections")
		return nil
	}, func(app core.App) error {
		// Rollback: remove RSVP fields
		if collection, err := app.FindCollectionByNameOrId("guest_lists"); err == nil {
			collection.Fields.RemoveById("gl_rsvp_enabled")
			collection.Fields.RemoveById("gl_rsvp_generic_token")
			app.Save(collection)
		}
		if collection, err := app.FindCollectionByNameOrId("guest_list_items"); err == nil {
			collection.Fields.RemoveById("gli_rsvp_token")
			collection.Fields.RemoveById("gli_rsvp_status")
			collection.Fields.RemoveById("gli_rsvp_dietary")
			collection.Fields.RemoveById("gli_rsvp_plus_one")
			collection.Fields.RemoveById("gli_rsvp_plus_one_name")
			collection.Fields.RemoveById("gli_rsvp_plus_one_dietary")
			collection.Fields.RemoveById("gli_rsvp_responded_at")
			collection.Fields.RemoveById("gli_rsvp_invited_by")
			app.Save(collection)
		}
		return nil
	})
}

func addRSVPFieldsToGuestLists(app core.App) error {
	collection, err := app.FindCollectionByNameOrId("guest_lists")
	if err != nil {
		return err
	}

	collection.Fields.Add(
		&core.BoolField{
			Id:   "gl_rsvp_enabled",
			Name: "rsvp_enabled",
		},
		&core.TextField{
			Id:       "gl_rsvp_generic_token",
			Name:     "rsvp_generic_token",
			Required: false,
			Max:      64,
		},
	)

	collection.Indexes = append(collection.Indexes,
		"CREATE UNIQUE INDEX idx_gl_rsvp_generic_token ON guest_lists (rsvp_generic_token) WHERE rsvp_generic_token != ''",
	)

	return app.Save(collection)
}

func addRSVPFieldsToGuestListItems(app core.App) error {
	collection, err := app.FindCollectionByNameOrId("guest_list_items")
	if err != nil {
		return err
	}

	collection.Fields.Add(
		&core.TextField{
			Id:       "gli_rsvp_token",
			Name:     "rsvp_token",
			Required: false,
			Max:      64,
		},
		&core.SelectField{
			Id:        "gli_rsvp_status",
			Name:      "rsvp_status",
			Required:  false,
			MaxSelect: 1,
			Values:    []string{"accepted", "declined"},
		},
		&core.TextField{
			Id:       "gli_rsvp_dietary",
			Name:     "rsvp_dietary",
			Required: false,
			Max:      1000,
		},
		&core.BoolField{
			Id:   "gli_rsvp_plus_one",
			Name: "rsvp_plus_one",
		},
		&core.TextField{
			Id:       "gli_rsvp_plus_one_name",
			Name:     "rsvp_plus_one_name",
			Required: false,
			Max:      200,
		},
		&core.TextField{
			Id:       "gli_rsvp_plus_one_dietary",
			Name:     "rsvp_plus_one_dietary",
			Required: false,
			Max:      1000,
		},
		&core.DateField{
			Id:       "gli_rsvp_responded_at",
			Name:     "rsvp_responded_at",
			Required: false,
		},
		&core.TextField{
			Id:       "gli_rsvp_invited_by",
			Name:     "rsvp_invited_by",
			Required: false,
			Max:      300,
		},
	)

	collection.Indexes = append(collection.Indexes,
		"CREATE UNIQUE INDEX idx_gli_rsvp_token ON guest_list_items (rsvp_token) WHERE rsvp_token != ''",
	)

	return app.Save(collection)
}
