package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		if err := createGuestListsCollection(app); err != nil {
			return err
		}
		if err := createGuestListItemsCollection(app); err != nil {
			return err
		}
		if err := createGuestListSharesCollection(app); err != nil {
			return err
		}
		if err := createGuestListOTPCodesCollection(app); err != nil {
			return err
		}
		log.Println("[Migration] Created guest list collections")
		return nil
	}, func(app core.App) error {
		// Rollback: delete collections in reverse dependency order
		for _, name := range []string{"guest_list_otp_codes", "guest_list_shares", "guest_list_items", "guest_lists"} {
			if c, err := app.FindCollectionByNameOrId(name); err == nil {
				app.Delete(c)
			}
		}
		return nil
	})
}

func createGuestListsCollection(app core.App) error {
	existing, _ := app.FindCollectionByNameOrId("guest_lists")
	if existing != nil {
		return nil
	}

	collection := core.NewBaseCollection("guest_lists")
	collection.Fields.Add(
		&core.TextField{
			Id:       "gl_name",
			Name:     "name",
			Required: true,
			Max:      300,
		},
		&core.TextField{
			Id:       "gl_description",
			Name:     "description",
			Required: false,
			Max:      2000,
		},
		&core.RelationField{
			Id:            "gl_event_projection",
			Name:          "event_projection",
			Required:      false,
			CollectionId:  "", // Will be resolved by PocketBase at runtime
			MaxSelect:     1,
		},
		&core.RelationField{
			Id:            "gl_created_by",
			Name:          "created_by",
			Required:      true,
			MaxSelect:     1,
		},
		&core.SelectField{
			Id:        "gl_status",
			Name:      "status",
			Required:  true,
			MaxSelect: 1,
			Values:    []string{"draft", "active", "archived"},
		},
		&core.AutodateField{
			Id:       "gl_created",
			Name:     "created",
			OnCreate: true,
		},
		&core.AutodateField{
			Id:       "gl_updated",
			Name:     "updated",
			OnCreate: true,
			OnUpdate: true,
		},
	)

	// Set relation collection IDs
	epCollection, _ := app.FindCollectionByNameOrId("event_projections")
	if epCollection != nil {
		if f := collection.Fields.GetById("gl_event_projection"); f != nil {
			f.(*core.RelationField).CollectionId = epCollection.Id
		}
	}
	usersCollection, _ := app.FindCollectionByNameOrId("users")
	if usersCollection != nil {
		if f := collection.Fields.GetById("gl_created_by"); f != nil {
			f.(*core.RelationField).CollectionId = usersCollection.Id
		}
	}

	collection.Indexes = []string{
		"CREATE INDEX idx_gl_status ON guest_lists (status)",
		"CREATE INDEX idx_gl_created_by ON guest_lists (created_by)",
		"CREATE INDEX idx_gl_event_projection ON guest_lists (event_projection)",
	}

	adminRule := "@request.auth.role = 'admin'"
	authRule := "@request.auth.id != ''"
	collection.ListRule = &authRule
	collection.ViewRule = &authRule
	collection.CreateRule = &adminRule
	collection.UpdateRule = &adminRule
	collection.DeleteRule = &adminRule

	return app.Save(collection)
}

func createGuestListItemsCollection(app core.App) error {
	existing, _ := app.FindCollectionByNameOrId("guest_list_items")
	if existing != nil {
		return nil
	}

	collection := core.NewBaseCollection("guest_list_items")

	min0 := 0.0
	max5 := 5.0

	collection.Fields.Add(
		&core.RelationField{
			Id:       "gli_guest_list",
			Name:     "guest_list",
			Required: true,
			MaxSelect: 1,
		},
		&core.RelationField{
			Id:       "gli_contact",
			Name:     "contact",
			Required: true,
			MaxSelect: 1,
		},
		&core.SelectField{
			Id:        "gli_invite_round",
			Name:      "invite_round",
			Required:  false,
			MaxSelect: 1,
			Values:    []string{"1st", "2nd", "3rd", "maybe"},
		},
		&core.SelectField{
			Id:        "gli_invite_status",
			Name:      "invite_status",
			Required:  false,
			MaxSelect: 1,
			Values:    []string{"pending", "accepted", "declined", "no_show"},
		},
		&core.TextField{
			Id:       "gli_notes",
			Name:     "notes",
			Required: false,
			Max:      2000,
		},
		&core.TextField{
			Id:       "gli_client_notes",
			Name:     "client_notes",
			Required: false,
			Max:      2000,
		},
		&core.NumberField{
			Id:       "gli_sort_order",
			Name:     "sort_order",
			Required: false,
			OnlyInt:  true,
		},
		// Denormalized contact snapshots
		&core.TextField{
			Id:       "gli_contact_name",
			Name:     "contact_name",
			Required: false,
			Max:      200,
		},
		&core.TextField{
			Id:       "gli_contact_job_title",
			Name:     "contact_job_title",
			Required: false,
			Max:      200,
		},
		&core.TextField{
			Id:       "gli_contact_org_name",
			Name:     "contact_organisation_name",
			Required: false,
			Max:      300,
		},
		&core.TextField{
			Id:       "gli_contact_linkedin",
			Name:     "contact_linkedin",
			Required: false,
			Max:      500,
		},
		&core.TextField{
			Id:       "gli_contact_location",
			Name:     "contact_location",
			Required: false,
			Max:      200,
		},
		&core.SelectField{
			Id:        "gli_contact_degrees",
			Name:      "contact_degrees",
			Required:  false,
			MaxSelect: 1,
			Values:    []string{"1st", "2nd", "3rd"},
		},
		&core.NumberField{
			Id:       "gli_contact_relationship",
			Name:     "contact_relationship",
			Required: false,
			OnlyInt:  true,
			Min:      &min0,
			Max:      &max5,
		},
		&core.AutodateField{
			Id:       "gli_created",
			Name:     "created",
			OnCreate: true,
		},
		&core.AutodateField{
			Id:       "gli_updated",
			Name:     "updated",
			OnCreate: true,
			OnUpdate: true,
		},
	)

	// Set relation collection IDs
	glCollection, _ := app.FindCollectionByNameOrId("guest_lists")
	if glCollection != nil {
		if f := collection.Fields.GetById("gli_guest_list"); f != nil {
			f.(*core.RelationField).CollectionId = glCollection.Id
		}
	}
	contactsCollection, _ := app.FindCollectionByNameOrId("contacts")
	if contactsCollection != nil {
		if f := collection.Fields.GetById("gli_contact"); f != nil {
			f.(*core.RelationField).CollectionId = contactsCollection.Id
		}
	}

	collection.Indexes = []string{
		"CREATE INDEX idx_gli_guest_list ON guest_list_items (guest_list)",
		"CREATE INDEX idx_gli_contact ON guest_list_items (contact)",
		"CREATE UNIQUE INDEX idx_gli_unique ON guest_list_items (guest_list, contact)",
	}

	adminRule := "@request.auth.role = 'admin'"
	authRule := "@request.auth.id != ''"
	collection.ListRule = &authRule
	collection.ViewRule = &authRule
	collection.CreateRule = &adminRule
	collection.UpdateRule = &adminRule
	collection.DeleteRule = &adminRule

	return app.Save(collection)
}

func createGuestListSharesCollection(app core.App) error {
	existing, _ := app.FindCollectionByNameOrId("guest_list_shares")
	if existing != nil {
		return nil
	}

	collection := core.NewBaseCollection("guest_list_shares")
	collection.Fields.Add(
		&core.RelationField{
			Id:       "gls_guest_list",
			Name:     "guest_list",
			Required: true,
			MaxSelect: 1,
		},
		&core.TextField{
			Id:       "gls_token",
			Name:     "token",
			Required: true,
			Max:      64,
		},
		&core.TextField{
			Id:       "gls_recipient_email",
			Name:     "recipient_email",
			Required: true,
			Max:      300,
		},
		&core.TextField{
			Id:       "gls_recipient_name",
			Name:     "recipient_name",
			Required: false,
			Max:      200,
		},
		&core.DateField{
			Id:       "gls_expires_at",
			Name:     "expires_at",
			Required: true,
		},
		&core.BoolField{
			Id:   "gls_revoked",
			Name: "revoked",
		},
		&core.DateField{
			Id:       "gls_verified_at",
			Name:     "verified_at",
			Required: false,
		},
		&core.DateField{
			Id:       "gls_last_accessed_at",
			Name:     "last_accessed_at",
			Required: false,
		},
		&core.NumberField{
			Id:       "gls_access_count",
			Name:     "access_count",
			Required: false,
			OnlyInt:  true,
		},
		&core.AutodateField{
			Id:       "gls_created",
			Name:     "created",
			OnCreate: true,
		},
		&core.AutodateField{
			Id:       "gls_updated",
			Name:     "updated",
			OnCreate: true,
			OnUpdate: true,
		},
	)

	// Set relation collection ID
	glCollection, _ := app.FindCollectionByNameOrId("guest_lists")
	if glCollection != nil {
		if f := collection.Fields.GetById("gls_guest_list"); f != nil {
			f.(*core.RelationField).CollectionId = glCollection.Id
		}
	}

	collection.Indexes = []string{
		"CREATE UNIQUE INDEX idx_gls_token ON guest_list_shares (token)",
		"CREATE INDEX idx_gls_guest_list ON guest_list_shares (guest_list)",
		"CREATE INDEX idx_gls_email ON guest_list_shares (recipient_email)",
	}

	// No API access — managed entirely through custom handlers
	collection.ListRule = nil
	collection.ViewRule = nil
	collection.CreateRule = nil
	collection.UpdateRule = nil
	collection.DeleteRule = nil

	return app.Save(collection)
}

func createGuestListOTPCodesCollection(app core.App) error {
	existing, _ := app.FindCollectionByNameOrId("guest_list_otp_codes")
	if existing != nil {
		return nil
	}

	collection := core.NewBaseCollection("guest_list_otp_codes")
	collection.Fields.Add(
		&core.RelationField{
			Id:       "glo_share",
			Name:     "share",
			Required: true,
			MaxSelect: 1,
		},
		&core.TextField{
			Id:       "glo_code_hash",
			Name:     "code_hash",
			Required: true,
			Max:      100,
		},
		&core.TextField{
			Id:       "glo_email",
			Name:     "email",
			Required: true,
			Max:      300,
		},
		&core.DateField{
			Id:       "glo_expires_at",
			Name:     "expires_at",
			Required: true,
		},
		&core.BoolField{
			Id:   "glo_used",
			Name: "used",
		},
		&core.NumberField{
			Id:       "glo_attempts",
			Name:     "attempts",
			Required: false,
			OnlyInt:  true,
		},
		&core.TextField{
			Id:       "glo_ip_address",
			Name:     "ip_address",
			Required: false,
			Max:      45,
		},
		&core.AutodateField{
			Id:       "glo_created",
			Name:     "created",
			OnCreate: true,
		},
	)

	// Set relation collection ID
	sharesCollection, _ := app.FindCollectionByNameOrId("guest_list_shares")
	if sharesCollection != nil {
		if f := collection.Fields.GetById("glo_share"); f != nil {
			f.(*core.RelationField).CollectionId = sharesCollection.Id
		}
	}

	collection.Indexes = []string{
		"CREATE INDEX idx_glo_share ON guest_list_otp_codes (share)",
		"CREATE INDEX idx_glo_email ON guest_list_otp_codes (email)",
	}

	// No API access — managed entirely through custom handlers
	collection.ListRule = nil
	collection.ViewRule = nil
	collection.CreateRule = nil
	collection.UpdateRule = nil
	collection.DeleteRule = nil

	return app.Save(collection)
}
