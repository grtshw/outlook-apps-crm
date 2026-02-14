package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		// Remove avatar FileField from contacts
		// Avatar files are now uploaded directly to DAM via HMAC proxy
		contacts, err := app.FindCollectionByNameOrId("contacts")
		if err != nil {
			return err
		}

		contacts.Fields.RemoveById("cont_avatar")
		if err := app.Save(contacts); err != nil {
			return err
		}
		log.Println("[Migration] Removed avatar FileField from contacts")

		// Remove logo FileFields from organisations
		// Logos are managed by DAM, not stored locally
		orgs, err := app.FindCollectionByNameOrId("organisations")
		if err != nil {
			return err
		}

		orgs.Fields.RemoveById("org_logo_square")
		orgs.Fields.RemoveById("org_logo_standard")
		orgs.Fields.RemoveById("org_logo_inverted")
		if err := app.Save(orgs); err != nil {
			return err
		}
		log.Println("[Migration] Removed logo FileFields from organisations")

		return nil
	}, func(app core.App) error {
		return nil
	})
}
