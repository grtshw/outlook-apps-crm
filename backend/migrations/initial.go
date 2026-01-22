package migrations

import (
	"encoding/json"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(func(app core.App) error {
		// Create users collection (extends default)
		if err := createUsersCollection(app); err != nil {
			return err
		}

		// Create organisations collection FIRST (contacts references it)
		if err := createOrganisationsCollection(app); err != nil {
			return err
		}

		// Create contacts collection (references organisations)
		if err := createContactsCollection(app); err != nil {
			return err
		}

		// Create activities collection (references contacts and organisations)
		if err := createActivitiesCollection(app); err != nil {
			return err
		}

		// Create app_settings collection (required for initAppShell)
		if err := createAppSettingsCollection(app); err != nil {
			return err
		}

		return nil
	}, nil)
}

func createUsersCollection(app core.App) error {
	collection, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		// Users collection should exist by default, just extend it
		return nil
	}

	// Add role field if not exists
	if !fieldExists(collection, "role") {
		collection.Fields.Add(&core.SelectField{
			Id:        "users_role",
			Name:      "role",
			Required:  false,
			MaxSelect: 1,
			Values:    []string{"admin", "viewer"},
		})
	}

	// Add name field if not exists
	if !fieldExists(collection, "name") {
		collection.Fields.Add(&core.TextField{
			Id:       "users_name",
			Name:     "name",
			Required: false,
			Max:      200,
		})
	}

	// Add avatarURL field for OAuth
	if !fieldExists(collection, "avatarURL") {
		collection.Fields.Add(&core.URLField{
			Id:       "users_avatar_url",
			Name:     "avatarURL",
			Required: false,
		})
	}

	return app.Save(collection)
}

func createContactsCollection(app core.App) error {
	existing, _ := app.FindCollectionByNameOrId("contacts")
	if existing != nil {
		return nil // Already exists
	}

	collection := core.NewBaseCollection("contacts")

	// Core identity fields
	collection.Fields.Add(&core.EmailField{
		Id:       "cont_email",
		Name:     "email",
		Required: true,
	})

	collection.Fields.Add(&core.TextField{
		Id:       "cont_name",
		Name:     "name",
		Required: true,
		Max:      200,
	})

	// Profile fields
	collection.Fields.Add(&core.TextField{
		Id:       "cont_phone",
		Name:     "phone",
		Required: false,
		Max:      50,
	})

	collection.Fields.Add(&core.TextField{
		Id:       "cont_pronouns",
		Name:     "pronouns",
		Required: false,
		Max:      50,
	})

	collection.Fields.Add(&core.TextField{
		Id:       "cont_bio",
		Name:     "bio",
		Required: false,
		Max:      10000,
	})

	collection.Fields.Add(&core.TextField{
		Id:       "cont_job_title",
		Name:     "job_title",
		Required: false,
		Max:      200,
	})

	// Social/web links
	collection.Fields.Add(&core.URLField{
		Id:       "cont_linkedin",
		Name:     "linkedin",
		Required: false,
	})

	collection.Fields.Add(&core.TextField{
		Id:       "cont_instagram",
		Name:     "instagram",
		Required: false,
		Max:      100,
	})

	collection.Fields.Add(&core.URLField{
		Id:       "cont_website",
		Name:     "website",
		Required: false,
	})

	// Location
	collection.Fields.Add(&core.TextField{
		Id:       "cont_location",
		Name:     "location",
		Required: false,
		Max:      200,
	})

	collection.Fields.Add(&core.TextField{
		Id:       "cont_do_position",
		Name:     "do_position",
		Required: false,
		Max:      200,
	})

	// Avatar - local file upload
	collection.Fields.Add(&core.FileField{
		Id:        "cont_avatar",
		Name:      "avatar",
		Required:  false,
		MaxSelect: 1,
		MaxSize:   5242880, // 5MB
		MimeTypes: []string{"image/jpeg", "image/png", "image/webp"},
	})

	// Avatar URLs from DAM (for presenters imported from Presentations)
	collection.Fields.Add(&core.URLField{
		Id:       "cont_avatar_url",
		Name:     "avatar_url",
		Required: false,
	})

	collection.Fields.Add(&core.URLField{
		Id:       "cont_avatar_thumb_url",
		Name:     "avatar_thumb_url",
		Required: false,
	})

	collection.Fields.Add(&core.URLField{
		Id:       "cont_avatar_small_url",
		Name:     "avatar_small_url",
		Required: false,
	})

	collection.Fields.Add(&core.URLField{
		Id:       "cont_avatar_original_url",
		Name:     "avatar_original_url",
		Required: false,
	})

	// Organisation relation - look up the collection ID
	orgsCollection, _ := app.FindCollectionByNameOrId("organisations")
	orgCollectionId := ""
	if orgsCollection != nil {
		orgCollectionId = orgsCollection.Id
	}
	collection.Fields.Add(&core.RelationField{
		Id:           "cont_organisation",
		Name:         "organisation",
		Required:     false,
		CollectionId: orgCollectionId,
		MaxSelect:    1,
	})

	// Metadata fields
	collection.Fields.Add(&core.JSONField{
		Id:       "cont_tags",
		Name:     "tags",
		Required: false,
		MaxSize:  10000,
	})

	// Roles - multi-select for contact roles (Presenter, Speaker, Sponsor, etc.)
	collection.Fields.Add(&core.SelectField{
		Id:        "cont_roles",
		Name:      "roles",
		Required:  false,
		MaxSelect: 7,
		Values:    []string{"presenter", "speaker", "sponsor", "judge", "attendee", "staff", "volunteer"},
	})

	collection.Fields.Add(&core.SelectField{
		Id:        "cont_status",
		Name:      "status",
		Required:  true,
		MaxSelect: 1,
		Values:    []string{"active", "inactive", "archived"},
	})

	collection.Fields.Add(&core.SelectField{
		Id:        "cont_source",
		Name:      "source",
		Required:  false,
		MaxSelect: 1,
		Values:    []string{"presentations", "awards", "events", "hubspot", "manual"},
	})

	collection.Fields.Add(&core.JSONField{
		Id:       "cont_source_ids",
		Name:     "source_ids",
		Required: false,
		MaxSize:  5000,
	})

	// HubSpot sync fields
	collection.Fields.Add(&core.TextField{
		Id:       "cont_hubspot_id",
		Name:     "hubspot_contact_id",
		Required: false,
		Max:      100,
	})

	collection.Fields.Add(&core.DateField{
		Id:       "cont_hubspot_synced",
		Name:     "hubspot_synced_at",
		Required: false,
	})

	// Indexes
	collection.Indexes = []string{
		"CREATE UNIQUE INDEX idx_contacts_email ON contacts (email)",
		"CREATE INDEX idx_contacts_status ON contacts (status)",
		"CREATE INDEX idx_contacts_name ON contacts (name)",
	}

	// Access rules - authenticated users can read, admins can write
	collection.ListRule = types.Pointer("@request.auth.id != ''")
	collection.ViewRule = types.Pointer("@request.auth.id != ''")
	collection.CreateRule = types.Pointer("@request.auth.role = 'admin'")
	collection.UpdateRule = types.Pointer("@request.auth.role = 'admin'")
	collection.DeleteRule = types.Pointer("@request.auth.role = 'admin'")

	return app.Save(collection)
}

func createOrganisationsCollection(app core.App) error {
	existing, _ := app.FindCollectionByNameOrId("organisations")
	if existing != nil {
		return nil // Already exists
	}

	collection := core.NewBaseCollection("organisations")

	collection.Fields.Add(&core.TextField{
		Id:       "org_name",
		Name:     "name",
		Required: true,
		Max:      300,
	})

	collection.Fields.Add(&core.URLField{
		Id:       "org_website",
		Name:     "website",
		Required: false,
	})

	collection.Fields.Add(&core.URLField{
		Id:       "org_linkedin",
		Name:     "linkedin",
		Required: false,
	})

	// Description fields (short, medium, long)
	collection.Fields.Add(&core.TextField{
		Id:       "org_desc_short",
		Name:     "description_short",
		Required: false,
		Max:      500,
	})

	collection.Fields.Add(&core.TextField{
		Id:       "org_desc_medium",
		Name:     "description_medium",
		Required: false,
		Max:      2000,
	})

	collection.Fields.Add(&core.TextField{
		Id:       "org_desc_long",
		Name:     "description_long",
		Required: false,
		Max:      10000,
	})

	// Typed logo fields (square, standard, inverted)
	collection.Fields.Add(&core.FileField{
		Id:        "org_logo_square",
		Name:      "logo_square",
		Required:  false,
		MaxSelect: 1,
		MaxSize:   5242880,
		MimeTypes: []string{"image/jpeg", "image/png", "image/svg+xml", "image/webp"},
	})

	collection.Fields.Add(&core.FileField{
		Id:        "org_logo_standard",
		Name:      "logo_standard",
		Required:  false,
		MaxSelect: 1,
		MaxSize:   5242880,
		MimeTypes: []string{"image/jpeg", "image/png", "image/svg+xml", "image/webp"},
	})

	collection.Fields.Add(&core.FileField{
		Id:        "org_logo_inverted",
		Name:      "logo_inverted",
		Required:  false,
		MaxSelect: 1,
		MaxSize:   5242880,
		MimeTypes: []string{"image/jpeg", "image/png", "image/svg+xml", "image/webp"},
	})

	// Contacts JSON (array of {name, linkedin, email})
	collection.Fields.Add(&core.JSONField{
		Id:       "org_contacts",
		Name:     "contacts",
		Required: false,
		MaxSize:  50000,
	})

	// Metadata fields
	collection.Fields.Add(&core.JSONField{
		Id:       "org_tags",
		Name:     "tags",
		Required: false,
		MaxSize:  10000,
	})

	collection.Fields.Add(&core.SelectField{
		Id:        "org_status",
		Name:      "status",
		Required:  true,
		MaxSelect: 1,
		Values:    []string{"active", "archived"},
	})

	collection.Fields.Add(&core.SelectField{
		Id:        "org_source",
		Name:      "source",
		Required:  false,
		MaxSelect: 1,
		Values:    []string{"presentations", "awards", "events", "hubspot", "manual"},
	})

	collection.Fields.Add(&core.JSONField{
		Id:       "org_source_ids",
		Name:     "source_ids",
		Required: false,
		MaxSize:  5000,
	})

	// HubSpot sync fields
	collection.Fields.Add(&core.TextField{
		Id:       "org_hubspot_id",
		Name:     "hubspot_company_id",
		Required: false,
		Max:      100,
	})

	collection.Fields.Add(&core.DateField{
		Id:       "org_hubspot_synced",
		Name:     "hubspot_synced_at",
		Required: false,
	})

	// Indexes
	collection.Indexes = []string{
		"CREATE INDEX idx_organisations_name ON organisations (name)",
		"CREATE INDEX idx_organisations_status ON organisations (status)",
	}

	// Access rules - authenticated users can read, admins can write
	collection.ListRule = types.Pointer("@request.auth.id != ''")
	collection.ViewRule = types.Pointer("@request.auth.id != ''")
	collection.CreateRule = types.Pointer("@request.auth.role = 'admin'")
	collection.UpdateRule = types.Pointer("@request.auth.role = 'admin'")
	collection.DeleteRule = types.Pointer("@request.auth.role = 'admin'")

	return app.Save(collection)
}

func createActivitiesCollection(app core.App) error {
	existing, _ := app.FindCollectionByNameOrId("activities")
	if existing != nil {
		return nil // Already exists
	}

	collection := core.NewBaseCollection("activities")

	// Look up collection IDs for relations
	contactsCollection, _ := app.FindCollectionByNameOrId("contacts")
	contactsCollectionId := ""
	if contactsCollection != nil {
		contactsCollectionId = contactsCollection.Id
	}
	orgsCollection, _ := app.FindCollectionByNameOrId("organisations")
	orgsCollectionId := ""
	if orgsCollection != nil {
		orgsCollectionId = orgsCollection.Id
	}

	// Relations (optional - activity may be for contact, org, or both)
	collection.Fields.Add(&core.RelationField{
		Id:           "act_contact",
		Name:         "contact",
		Required:     false,
		CollectionId: contactsCollectionId,
		MaxSelect:    1,
	})

	collection.Fields.Add(&core.RelationField{
		Id:           "act_organisation",
		Name:         "organisation",
		Required:     false,
		CollectionId: orgsCollectionId,
		MaxSelect:    1,
	})

	// Activity type
	collection.Fields.Add(&core.TextField{
		Id:       "act_type",
		Name:     "type",
		Required: true,
		Max:      100,
	})

	collection.Fields.Add(&core.TextField{
		Id:       "act_title",
		Name:     "title",
		Required: false,
		Max:      500,
	})

	// Source tracking
	collection.Fields.Add(&core.SelectField{
		Id:        "act_source_app",
		Name:      "source_app",
		Required:  true,
		MaxSelect: 1,
		Values:    []string{"presentations", "awards", "events", "dam", "hubspot", "crm"},
	})

	collection.Fields.Add(&core.TextField{
		Id:       "act_source_id",
		Name:     "source_id",
		Required: false,
		Max:      100,
	})

	collection.Fields.Add(&core.URLField{
		Id:       "act_source_url",
		Name:     "source_url",
		Required: false,
	})

	// Metadata
	collection.Fields.Add(&core.JSONField{
		Id:       "act_metadata",
		Name:     "metadata",
		Required: false,
		MaxSize:  50000,
	})

	// When the activity occurred (may differ from created timestamp)
	collection.Fields.Add(&core.DateField{
		Id:       "act_occurred_at",
		Name:     "occurred_at",
		Required: false,
	})

	// Indexes
	collection.Indexes = []string{
		"CREATE INDEX idx_activities_contact ON activities (contact)",
		"CREATE INDEX idx_activities_organisation ON activities (organisation)",
		"CREATE INDEX idx_activities_type ON activities (type)",
		"CREATE INDEX idx_activities_source_app ON activities (source_app)",
		"CREATE INDEX idx_activities_occurred ON activities (occurred_at)",
	}

	// Access rules - authenticated users can read, system can write (via API)
	collection.ListRule = types.Pointer("@request.auth.id != ''")
	collection.ViewRule = types.Pointer("@request.auth.id != ''")
	// Create/Update/Delete are handled via custom API endpoints with webhook auth
	collection.CreateRule = nil
	collection.UpdateRule = nil
	collection.DeleteRule = nil

	return app.Save(collection)
}

func createAppSettingsCollection(app core.App) error {
	existing, _ := app.FindCollectionByNameOrId("app_settings")
	if existing != nil {
		return nil // Already exists
	}

	collection := core.NewBaseCollection("app_settings")

	// App identification fields (ui-kit compatible)
	collection.Fields.Add(&core.TextField{
		Name:     "app_id",
		Required: true,
	})
	collection.Fields.Add(&core.TextField{
		Name:     "app_name",
		Required: true,
	})
	collection.Fields.Add(&core.TextField{
		Name: "app_title",
	})
	collection.Fields.Add(&core.URLField{
		Name: "app_url",
	})
	collection.Fields.Add(&core.TextField{
		Name: "app_icon",
	})
	collection.Fields.Add(&core.TextField{
		Name: "required_role",
	})
	collection.Fields.Add(&core.NumberField{
		Name: "sort_order",
	})
	collection.Fields.Add(&core.BoolField{
		Name: "is_active",
	})

	// JSON configuration fields
	collection.Fields.Add(&core.JSONField{
		Name: "menu_items",
	})
	collection.Fields.Add(&core.JSONField{
		Name: "domain_actions",
	})
	collection.Fields.Add(&core.JSONField{
		Name: "search_config",
	})
	collection.Fields.Add(&core.JSONField{
		Name: "routing",
	})
	collection.Fields.Add(&core.JSONField{
		Name: "pagination",
	})
	collection.Fields.Add(&core.JSONField{
		Name: "cache_ttl",
	})
	collection.Fields.Add(&core.JSONField{
		Name: "external_urls",
	})
	collection.Fields.Add(&core.JSONField{
		Name: "features",
	})
	collection.Fields.Add(&core.JSONField{
		Name: "meta",
	})

	// Indexes
	collection.Indexes = []string{
		"CREATE UNIQUE INDEX idx_app_settings_app_id ON app_settings (app_id)",
	}

	// CRITICAL: Access rules - public read (for initAppShell), admin write
	// Use empty string "" for public access, NOT nil
	publicRule := ""
	adminRule := "@request.auth.role = 'admin'"
	collection.ListRule = &publicRule
	collection.ViewRule = &publicRule
	collection.CreateRule = &adminRule
	collection.UpdateRule = &adminRule
	collection.DeleteRule = &adminRule

	if err := app.Save(collection); err != nil {
		return err
	}

	// Seed CRM app settings
	return seedAppSettings(app, collection)
}

// fieldExists checks if a field with the given name exists in the collection
func fieldExists(collection *core.Collection, fieldName string) bool {
	for _, f := range collection.Fields {
		if f.GetName() == fieldName {
			return true
		}
	}
	return false
}

// seedAppSettings creates the initial app_settings records
func seedAppSettings(app core.App, collection *core.Collection) error {
	// CRM menu items
	menuItems := []map[string]interface{}{
		{
			"id":           "contacts",
			"icon":         "people",
			"title":        "Contacts",
			"type":         "navigate",
			"path":         "/contacts",
			"roles":        []string{"admin", "viewer"},
			"sectionLabel": "Directory",
		},
		{
			"id":    "organisations",
			"icon":  "building",
			"title": "Organisations",
			"type":  "navigate",
			"path":  "/organisations",
			"roles": []string{"admin", "viewer"},
		},
		{
			"id":      "logout",
			"icon":    "box-arrow-right",
			"title":   "Log out",
			"type":    "action",
			"action":  "logout",
			"roles":   []string{"admin", "viewer"},
			"section": "secondary",
		},
	}

	searchConfig := map[string]interface{}{
		"collections": []map[string]interface{}{
			{
				"name":         "contacts",
				"displayField": "name",
				"searchFields": []string{"name", "email"},
				"icon":         "person",
				"pathTemplate": "/contacts/{id}",
			},
			{
				"name":         "organisations",
				"displayField": "name",
				"searchFields": []string{"name"},
				"icon":         "building",
				"pathTemplate": "/organisations/{id}",
			},
		},
		"placeholder": "Search contacts, organisations...",
	}

	routing := map[string]interface{}{
		"default_route":            "/contacts",
		"login_redirect":           "/contacts",
		"logout_redirect":          "/login",
		"unauthenticated_redirect": "/login",
	}

	pagination := map[string]interface{}{
		"default_per_page":  24,
		"max_visible_pages": 7,
		"options":           []int{12, 24, 48, 96},
	}

	cacheTTL := map[string]interface{}{
		"list":    60000,
		"detail":  30000,
		"summary": 30000,
		"static":  300000,
	}

	features := map[string]interface{}{
		"activities": true,
	}

	menuItemsJSON, _ := json.Marshal(menuItems)
	searchConfigJSON, _ := json.Marshal(searchConfig)
	routingJSON, _ := json.Marshal(routing)
	paginationJSON, _ := json.Marshal(pagination)
	cacheTTLJSON, _ := json.Marshal(cacheTTL)
	featuresJSON, _ := json.Marshal(features)

	// Create CRM app record
	record := core.NewRecord(collection)
	record.Set("app_id", "crm")
	record.Set("app_name", "CRM")
	record.Set("app_title", "Contact Relationship Manager")
	record.Set("app_url", "https://crm.theoutlook.io")
	record.Set("app_icon", "person-vcard")
	record.Set("required_role", "crm_access")
	record.Set("sort_order", 3)
	record.Set("is_active", true)
	record.Set("menu_items", string(menuItemsJSON))
	record.Set("search_config", string(searchConfigJSON))
	record.Set("routing", string(routingJSON))
	record.Set("pagination", string(paginationJSON))
	record.Set("cache_ttl", string(cacheTTLJSON))
	record.Set("features", string(featuresJSON))

	if err := app.Save(record); err != nil {
		return err
	}

	// Add other apps for app switcher
	otherApps := []map[string]interface{}{
		{
			"app_id":        "events",
			"app_name":      "Events",
			"app_title":     "Event Financials",
			"app_url":       "https://events.theoutlook.io",
			"app_icon":      "calendar4-event",
			"required_role": "events_access",
			"sort_order":    1,
			"is_active":     true,
		},
		{
			"app_id":        "presentations",
			"app_name":      "Presentations",
			"app_title":     "Presentation Manager",
			"app_url":       "https://presentations.theoutlook.io",
			"app_icon":      "easel",
			"required_role": "presentations_access",
			"sort_order":    2,
			"is_active":     true,
		},
		{
			"app_id":        "awards",
			"app_name":      "Awards",
			"app_title":     "The Outlook Awards",
			"app_url":       "https://awards.theoutlook.io/dashboard",
			"app_icon":      "trophy",
			"required_role": "awards_access",
			"sort_order":    4,
			"is_active":     true,
		},
		{
			"app_id":        "dam",
			"app_name":      "DAM",
			"app_title":     "Digital Asset Manager",
			"app_url":       "https://dam.theoutlook.io",
			"app_icon":      "images",
			"required_role": "dam_access",
			"sort_order":    5,
			"is_active":     true,
		},
	}

	for _, appData := range otherApps {
		otherRecord := core.NewRecord(collection)
		for key, value := range appData {
			otherRecord.Set(key, value)
		}
		if err := app.Save(otherRecord); err != nil {
			return err
		}
	}

	return nil
}
