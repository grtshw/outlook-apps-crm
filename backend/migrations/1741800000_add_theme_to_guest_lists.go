package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("guest_lists")
		if err != nil {
			return err
		}

		// Find the themes collection for the relation
		themesCollection, err := app.FindCollectionByNameOrId("themes")
		if err != nil {
			return err
		}

		collection.Fields.Add(
			&core.RelationField{
				Id:          "gl_theme",
				Name:        "theme",
				CollectionId: themesCollection.Id,
				MaxSelect:   1,
			},
		)

		if err := app.Save(collection); err != nil {
			return err
		}

		// Backfill: set all existing guest lists to "After Dark" theme
		afterDark, err := app.FindFirstRecordByFilter("themes", "slug = 'after-dark'")
		if err == nil {
			records, err := app.FindRecordsByFilter("guest_lists", "theme = ''", "", 0, 0, nil)
			if err == nil {
				for _, r := range records {
					r.Set("theme", afterDark.Id)
					app.Save(r)
				}
				log.Printf("[Migration] Backfilled %d guest lists with After Dark theme", len(records))
			}
		}

		log.Println("[Migration] Added theme relation to guest_lists")
		return nil
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("guest_lists")
		if err != nil {
			return nil
		}
		collection.Fields.RemoveById("gl_theme")
		app.Save(collection)
		return nil
	})
}
