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

		if !fieldExists(collection, "avatar_url") {
			collection.Fields.Add(&core.URLField{
				Id:       "cont_avatar_url",
				Name:     "avatar_url",
				Required: false,
			})
			changed = true
		}

		if !fieldExists(collection, "avatar_thumb_url") {
			collection.Fields.Add(&core.URLField{
				Id:       "cont_avatar_thumb_url",
				Name:     "avatar_thumb_url",
				Required: false,
			})
			changed = true
		}

		if !fieldExists(collection, "avatar_small_url") {
			collection.Fields.Add(&core.URLField{
				Id:       "cont_avatar_small_url",
				Name:     "avatar_small_url",
				Required: false,
			})
			changed = true
		}

		if !fieldExists(collection, "avatar_original_url") {
			collection.Fields.Add(&core.URLField{
				Id:       "cont_avatar_original_url",
				Name:     "avatar_original_url",
				Required: false,
			})
			changed = true
		}

		if !changed {
			log.Println("[Migration] Avatar URL fields already exist on contacts")
			return nil
		}

		if err := app.Save(collection); err != nil {
			return err
		}

		log.Println("[Migration] Added avatar URL fields to contacts collection")
		return nil
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("contacts")
		if err != nil {
			return nil
		}

		collection.Fields.RemoveById("cont_avatar_url")
		collection.Fields.RemoveById("cont_avatar_thumb_url")
		collection.Fields.RemoveById("cont_avatar_small_url")
		collection.Fields.RemoveById("cont_avatar_original_url")

		return app.Save(collection)
	})
}
