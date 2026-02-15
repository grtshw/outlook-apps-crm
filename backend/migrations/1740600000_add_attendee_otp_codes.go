package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		contactsCollection, err := app.FindCollectionByNameOrId("contacts")
		if err != nil {
			return err
		}

		collection := core.NewBaseCollection("attendee_otp_codes")

		collection.Fields.Add(&core.RelationField{
			Id:            "aoc_contact",
			Name:          "contact",
			Required:      true,
			CollectionId:  contactsCollection.Id,
			CascadeDelete: true,
			MaxSelect:     1,
		})

		collection.Fields.Add(&core.TextField{
			Id:       "aoc_code_hash",
			Name:     "code_hash",
			Required: true,
			Max:      200,
		})

		collection.Fields.Add(&core.TextField{
			Id:       "aoc_email",
			Name:     "email",
			Required: true,
			Max:      500,
		})

		collection.Fields.Add(&core.DateField{
			Id:       "aoc_expires_at",
			Name:     "expires_at",
			Required: true,
		})

		collection.Fields.Add(&core.BoolField{
			Id:   "aoc_used",
			Name: "used",
		})

		collection.Fields.Add(&core.NumberField{
			Id:   "aoc_attempts",
			Name: "attempts",
		})

		collection.Fields.Add(&core.TextField{
			Id:  "aoc_ip_address",
			Name: "ip_address",
			Max:  100,
		})

		// No API access — managed entirely through custom handlers
		// (nil rules = no access via PocketBase's default API)

		collection.AddIndex("idx_attendee_otp_contact", false, "contact", "")
		collection.AddIndex("idx_attendee_otp_email", false, "email", "")

		if err := app.Save(collection); err != nil {
			return err
		}

		log.Println("[Migration] Created attendee_otp_codes collection")
		return nil
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("attendee_otp_codes")
		if err != nil {
			return nil
		}
		return app.Delete(collection)
	})
}
