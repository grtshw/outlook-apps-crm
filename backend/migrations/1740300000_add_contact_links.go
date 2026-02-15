package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(func(app core.App) error {
		// Look up contacts collection ID for relation fields
		contactsCollection, err := app.FindCollectionByNameOrId("contacts")
		if err != nil {
			return err
		}

		collection := core.NewBaseCollection("contact_links")

		// Both sides of the bidirectional link
		collection.Fields.Add(&core.RelationField{
			Id:            "cl_contact_a",
			Name:          "contact_a",
			Required:      true,
			CollectionId:  contactsCollection.Id,
			CascadeDelete: false,
			MaxSelect:     1,
		})

		collection.Fields.Add(&core.RelationField{
			Id:            "cl_contact_b",
			Name:          "contact_b",
			Required:      true,
			CollectionId:  contactsCollection.Id,
			CascadeDelete: false,
			MaxSelect:     1,
		})

		collection.Fields.Add(&core.BoolField{
			Id:   "cl_verified",
			Name: "verified",
		})

		collection.Fields.Add(&core.SelectField{
			Id:        "cl_source",
			Name:      "source",
			Required:  true,
			MaxSelect: 1,
			Values:    []string{"manual", "attendee", "system"},
		})

		collection.Fields.Add(&core.TextField{
			Id:  "cl_created_by",
			Name: "created_by",
			Max:  200,
		})

		collection.Fields.Add(&core.TextField{
			Id:  "cl_notes",
			Name: "notes",
			Max:  500,
		})

		// Access rules
		collection.ListRule = types.Pointer("@request.auth.id != ''")
		collection.ViewRule = types.Pointer("@request.auth.id != ''")
		collection.CreateRule = types.Pointer("@request.auth.role = 'admin'")
		collection.UpdateRule = types.Pointer("@request.auth.role = 'admin'")
		collection.DeleteRule = types.Pointer("@request.auth.role = 'admin'")

		// Indexes for querying links by either side
		collection.AddIndex("idx_contact_links_a", false, "contact_a", "")
		collection.AddIndex("idx_contact_links_b", false, "contact_b", "")

		if err := app.Save(collection); err != nil {
			return err
		}

		log.Println("[Migration] Created contact_links collection")
		return nil
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("contact_links")
		if err != nil {
			return nil
		}
		return app.Delete(collection)
	})
}
