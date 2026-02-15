package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		contacts, err := app.FindCollectionByNameOrId("contacts")
		if err != nil {
			return err
		}

		contacts.Fields.Add(&core.TextField{
			Id:  "contact_mailchimp_id",
			Name: "mailchimp_id",
			Max:  200,
		})

		contacts.Fields.Add(&core.SelectField{
			Id:        "contact_mailchimp_status",
			Name:      "mailchimp_status",
			MaxSelect: 1,
			Values:    []string{"subscribed", "unsubscribed", "cleaned", "pending", "transactional"},
		})

		// Add "mailchimp" to source select values
		for _, f := range contacts.Fields {
			if sf, ok := f.(*core.SelectField); ok && sf.Name == "source" {
				hasMailchimp := false
				for _, v := range sf.Values {
					if v == "mailchimp" {
						hasMailchimp = true
						break
					}
				}
				if !hasMailchimp {
					sf.Values = append(sf.Values, "mailchimp")
				}
				break
			}
		}

		if err := app.Save(contacts); err != nil {
			return err
		}

		log.Println("[Migration] Added mailchimp fields to contacts")
		return nil
	}, func(app core.App) error {
		return nil
	})
}
