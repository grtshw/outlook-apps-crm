package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("themes")
		if err != nil {
			return err
		}

		collection.Fields.Add(
			&core.TextField{
				Id:       "theme_color_button",
				Name:     "color_button",
				Required: false,
				Max:      9,
			},
		)

		if err := app.Save(collection); err != nil {
			return err
		}

		// Backfill existing themes with a sensible default
		records, err := app.FindAllRecords("themes")
		if err != nil {
			return err
		}
		for _, r := range records {
			// Default button color to the text color
			r.Set("color_button", r.GetString("color_text"))
			if err := app.Save(r); err != nil {
				return err
			}
		}

		return nil
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("themes")
		if err != nil {
			return err
		}
		collection.Fields.RemoveById("theme_color_button")
		return app.Save(collection)
	})
}
