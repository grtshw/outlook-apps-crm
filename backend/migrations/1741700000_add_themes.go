package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection := core.NewBaseCollection("themes")

		collection.Fields.Add(
			&core.TextField{
				Id:       "theme_name",
				Name:     "name",
				Required: true,
				Max:      100,
			},
			&core.TextField{
				Id:       "theme_slug",
				Name:     "slug",
				Required: true,
				Max:      50,
			},
			&core.TextField{
				Id:       "theme_color_primary",
				Name:     "color_primary",
				Required: true,
				Max:      9,
			},
			&core.TextField{
				Id:       "theme_color_secondary",
				Name:     "color_secondary",
				Required: false,
				Max:      9,
			},
			&core.TextField{
				Id:       "theme_color_background",
				Name:     "color_background",
				Required: true,
				Max:      9,
			},
			&core.TextField{
				Id:       "theme_color_surface",
				Name:     "color_surface",
				Required: true,
				Max:      9,
			},
			&core.TextField{
				Id:       "theme_color_text",
				Name:     "color_text",
				Required: true,
				Max:      9,
			},
			&core.TextField{
				Id:       "theme_color_text_muted",
				Name:     "color_text_muted",
				Required: true,
				Max:      9,
			},
			&core.TextField{
				Id:       "theme_color_border",
				Name:     "color_border",
				Required: true,
				Max:      9,
			},
			&core.TextField{
				Id:       "theme_logo_url",
				Name:     "logo_url",
				Required: false,
				Max:      2000,
			},
			&core.TextField{
				Id:       "theme_logo_light_url",
				Name:     "logo_light_url",
				Required: false,
				Max:      2000,
			},
			&core.TextField{
				Id:       "theme_hero_image_url",
				Name:     "hero_image_url",
				Required: false,
				Max:      2000,
			},
			&core.BoolField{
				Id:   "theme_is_dark",
				Name: "is_dark",
			},
			&core.NumberField{
				Id:   "theme_sort_order",
				Name: "sort_order",
			},
		)

		// Add unique index on slug
		collection.Indexes = []string{
			"CREATE UNIQUE INDEX idx_themes_slug ON themes (slug)",
		}

		if err := app.Save(collection); err != nil {
			return err
		}

		// Seed "After Dark" theme
		afterDark := core.NewRecord(collection)
		afterDark.Set("name", "After Dark")
		afterDark.Set("slug", "after-dark")
		afterDark.Set("color_primary", "#E95139")
		afterDark.Set("color_secondary", "#645C49")
		afterDark.Set("color_background", "#020202")
		afterDark.Set("color_surface", "#1A1917")
		afterDark.Set("color_text", "#ffffff")
		afterDark.Set("color_text_muted", "#A8A9B1")
		afterDark.Set("color_border", "#645C49")
		afterDark.Set("logo_url", "/images/to-after-dark.png")
		afterDark.Set("logo_light_url", "/images/to-after-dark-white.png")
		afterDark.Set("hero_image_url", "/images/rsvp-hero.jpg")
		afterDark.Set("is_dark", true)
		afterDark.Set("sort_order", 1)
		if err := app.Save(afterDark); err != nil {
			return err
		}

		// Seed "The Outlook" theme
		theOutlook := core.NewRecord(collection)
		theOutlook.Set("name", "The Outlook")
		theOutlook.Set("slug", "the-outlook")
		theOutlook.Set("color_primary", "#0D0D0D")
		theOutlook.Set("color_secondary", "#6B7280")
		theOutlook.Set("color_background", "#ffffff")
		theOutlook.Set("color_surface", "#F8F8F8")
		theOutlook.Set("color_text", "#0D0D0D")
		theOutlook.Set("color_text_muted", "#6B7280")
		theOutlook.Set("color_border", "#E5E7EB")
		theOutlook.Set("logo_url", "/images/logo.svg")
		theOutlook.Set("logo_light_url", "/images/logo-white.svg")
		theOutlook.Set("hero_image_url", "")
		theOutlook.Set("is_dark", false)
		theOutlook.Set("sort_order", 2)
		if err := app.Save(theOutlook); err != nil {
			return err
		}

		log.Println("[Migration] Created themes collection with After Dark and The Outlook defaults")
		return nil
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("themes")
		if err != nil {
			return nil
		}
		app.Delete(collection)
		return nil
	})
}
