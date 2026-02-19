package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		contacts, err := app.FindCollectionByNameOrId("contacts")
		if err != nil {
			return err
		}

		for _, f := range contacts.Fields {
			if sf, ok := f.(*core.SelectField); ok && sf.Name == "source" {
				hasValue := false
				for _, v := range sf.Values {
					if v == "rsvp_forward" {
						hasValue = true
						break
					}
				}
				if !hasValue {
					sf.Values = append(sf.Values, "rsvp_forward")
				}
				hasAttendee := false
				for _, v := range sf.Values {
					if v == "attendee" {
						hasAttendee = true
						break
					}
				}
				if !hasAttendee {
					sf.Values = append(sf.Values, "attendee")
				}
				break
			}
		}

		return app.Save(contacts)
	}, func(app core.App) error {
		return nil
	})
}
