package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		if err := addCalendarFieldsToUsers(app); err != nil {
			return err
		}
		if err := addCalendarFieldsToGuestLists(app); err != nil {
			return err
		}
		log.Println("[Migration] Added calendar integration fields")
		return nil
	}, func(app core.App) error {
		if collection, err := app.FindCollectionByNameOrId("users"); err == nil {
			collection.Fields.RemoveById("u_ms_access_token")
			collection.Fields.RemoveById("u_ms_refresh_token")
			collection.Fields.RemoveById("u_ms_token_expires_at")
			app.Save(collection)
		}
		if collection, err := app.FindCollectionByNameOrId("guest_lists"); err == nil {
			collection.Fields.RemoveById("gl_event_host")
			collection.Fields.RemoveById("gl_ms_calendar_event_id")
			app.Save(collection)
		}
		return nil
	})
}

func addCalendarFieldsToUsers(app core.App) error {
	collection, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}

	collection.Fields.Add(
		&core.TextField{
			Id:   "u_ms_access_token",
			Name: "ms_access_token",
			Max:  4000,
		},
		&core.TextField{
			Id:   "u_ms_refresh_token",
			Name: "ms_refresh_token",
			Max:  4000,
		},
		&core.TextField{
			Id:   "u_ms_token_expires_at",
			Name: "ms_token_expires_at",
			Max:  50,
		},
	)

	return app.Save(collection)
}

func addCalendarFieldsToGuestLists(app core.App) error {
	collection, err := app.FindCollectionByNameOrId("guest_lists")
	if err != nil {
		return err
	}

	usersCollection, _ := app.FindCollectionByNameOrId("users")
	usersCollectionId := ""
	if usersCollection != nil {
		usersCollectionId = usersCollection.Id
	}

	collection.Fields.Add(
		&core.RelationField{
			Id:           "gl_event_host",
			Name:         "event_host",
			Required:     false,
			CollectionId: usersCollectionId,
			MaxSelect:    1,
		},
		&core.TextField{
			Id:   "gl_ms_calendar_event_id",
			Name: "ms_calendar_event_id",
			Max:  500,
		},
	)

	return app.Save(collection)
}
