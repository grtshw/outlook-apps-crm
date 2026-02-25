package migrations

import (
	"log"

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
				Id:       "theme_email_logo_url",
				Name:     "email_logo_url",
				Required: false,
				Max:      2000,
			},
		)

		if err := app.Save(collection); err != nil {
			return err
		}

		// Seed After Dark with the existing email PNG
		afterDark, err := app.FindFirstRecordByFilter("themes", "slug = 'after-dark'")
		if err == nil {
			afterDark.Set("email_logo_url", "/images/to-after-dark-white.png")
			app.Save(afterDark)
		}

		// Seed The Outlook with the existing email PNG
		theOutlook, err := app.FindFirstRecordByFilter("themes", "slug = 'the-outlook'")
		if err == nil {
			theOutlook.Set("email_logo_url", "/images/logo-email.png")
			app.Save(theOutlook)
		}

		log.Println("[Migration] Added email_logo_url to themes")
		return nil
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("themes")
		if err != nil {
			return nil
		}
		collection.Fields.RemoveById("theme_email_logo_url")
		return app.Save(collection)
	})
}
