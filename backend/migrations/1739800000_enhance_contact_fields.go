package migrations

import (
	"log"
	"strings"

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

		// first_name - split from existing name field
		if !fieldExists(collection, "first_name") {
			collection.Fields.Add(&core.TextField{
				Id:       "cont_first_name",
				Name:     "first_name",
				Required: true,
				Max:      200,
			})
			changed = true
		}

		// last_name - split from existing name field (not required for single-word names)
		if !fieldExists(collection, "last_name") {
			collection.Fields.Add(&core.TextField{
				Id:       "cont_last_name",
				Name:     "last_name",
				Required: false,
				Max:      200,
			})
			changed = true
		}

		// personal_email - encrypted, optional secondary email
		if !fieldExists(collection, "personal_email") {
			collection.Fields.Add(&core.TextField{
				Id:       "cont_personal_email",
				Name:     "personal_email",
				Required: false,
				Max:      500,
			})
			changed = true
		}

		// personal_email_index - SHA-256 blind index for encrypted email lookups
		if !fieldExists(collection, "personal_email_index") {
			collection.Fields.Add(&core.TextField{
				Id:       "cont_personal_email_index",
				Name:     "personal_email_index",
				Required: false,
				Max:      64,
			})
			changed = true
		}

		// dietary_requirements - multi-select
		if !fieldExists(collection, "dietary_requirements") {
			collection.Fields.Add(&core.SelectField{
				Id:        "cont_dietary_requirements",
				Name:      "dietary_requirements",
				Required:  false,
				MaxSelect: 7,
				Values:    []string{"vegetarian", "vegan", "gluten_free", "dairy_free", "nut_allergy", "halal", "kosher"},
			})
			changed = true
		}

		// dietary_requirements_other - freetext for custom dietary needs
		if !fieldExists(collection, "dietary_requirements_other") {
			collection.Fields.Add(&core.TextField{
				Id:       "cont_dietary_requirements_other",
				Name:     "dietary_requirements_other",
				Required: false,
				Max:      500,
			})
			changed = true
		}

		// accessibility_requirements - multi-select
		if !fieldExists(collection, "accessibility_requirements") {
			collection.Fields.Add(&core.SelectField{
				Id:        "cont_accessibility_requirements",
				Name:      "accessibility_requirements",
				Required:  false,
				MaxSelect: 5,
				Values:    []string{"wheelchair_access", "hearing_assistance", "vision_assistance", "sign_language_interpreter", "mobility_assistance"},
			})
			changed = true
		}

		// accessibility_requirements_other - freetext for custom accessibility needs
		if !fieldExists(collection, "accessibility_requirements_other") {
			collection.Fields.Add(&core.TextField{
				Id:       "cont_accessibility_requirements_other",
				Name:     "accessibility_requirements_other",
				Required: false,
				Max:      500,
			})
			changed = true
		}

		if !changed {
			log.Println("[Migration] Enhanced contact fields already exist")
			return nil
		}

		// Add indexes
		hasPersonalEmailIdx := false
		hasFirstNameIdx := false
		hasLastNameIdx := false
		for _, idx := range collection.Indexes {
			if strings.Contains(idx, "personal_email_index") {
				hasPersonalEmailIdx = true
			}
			if strings.Contains(idx, "first_name") {
				hasFirstNameIdx = true
			}
			if strings.Contains(idx, "last_name") {
				hasLastNameIdx = true
			}
		}
		if !hasPersonalEmailIdx {
			collection.Indexes = append(collection.Indexes,
				"CREATE UNIQUE INDEX idx_contacts_personal_email_index ON contacts (personal_email_index) WHERE personal_email_index != ''",
			)
		}
		if !hasFirstNameIdx {
			collection.Indexes = append(collection.Indexes,
				"CREATE INDEX idx_contacts_first_name ON contacts (first_name)",
			)
		}
		if !hasLastNameIdx {
			collection.Indexes = append(collection.Indexes,
				"CREATE INDEX idx_contacts_last_name ON contacts (last_name)",
			)
		}

		if err := app.Save(collection); err != nil {
			return err
		}

		log.Println("[Migration] Added first_name, last_name, personal_email, dietary_requirements, accessibility_requirements fields")

		// Migrate existing name data into first_name + last_name
		return migrateNameSplit(app)
	}, func(app core.App) error {
		// Rollback: remove the new fields
		collection, err := app.FindCollectionByNameOrId("contacts")
		if err != nil {
			return nil
		}

		collection.Fields.RemoveByName("first_name")
		collection.Fields.RemoveByName("last_name")
		collection.Fields.RemoveByName("personal_email")
		collection.Fields.RemoveByName("personal_email_index")
		collection.Fields.RemoveByName("dietary_requirements")
		collection.Fields.RemoveByName("dietary_requirements_other")
		collection.Fields.RemoveByName("accessibility_requirements")
		collection.Fields.RemoveByName("accessibility_requirements_other")

		// Remove indexes
		var newIndexes []string
		for _, idx := range collection.Indexes {
			if !strings.Contains(idx, "personal_email_index") &&
				!strings.Contains(idx, "first_name") &&
				!strings.Contains(idx, "last_name") {
				newIndexes = append(newIndexes, idx)
			}
		}
		collection.Indexes = newIndexes

		return app.Save(collection)
	})
}

// migrateNameSplit splits existing name values into first_name and last_name
func migrateNameSplit(app core.App) error {
	records, err := app.FindAllRecords("contacts")
	if err != nil {
		return err
	}

	migrated := 0
	for _, record := range records {
		name := record.GetString("name")
		if name == "" {
			continue
		}

		// Skip if already migrated
		if record.GetString("first_name") != "" {
			continue
		}

		parts := strings.SplitN(strings.TrimSpace(name), " ", 2)
		record.Set("first_name", parts[0])
		if len(parts) > 1 {
			record.Set("last_name", parts[1])
		} else {
			record.Set("last_name", "")
		}

		// Use SaveNoValidate — encrypted email values fail EmailField validation
		if err := app.SaveNoValidate(record); err != nil {
			log.Printf("[Migration] Failed to split name for contact %s: %v", record.Id, err)
			continue
		}
		migrated++
	}

	log.Printf("[Migration] Split name for %d contacts", migrated)
	return nil
}
