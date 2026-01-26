// migrate-encryption encrypts existing contact PII fields
// Run: go run ./cmd/migrate-encryption
package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/grtshw/outlook-apps-crm/utils"
	"github.com/pocketbase/pocketbase"
)

func main() {
	// Load .env if exists
	loadEnv()

	if !utils.IsEncryptionEnabled() {
		log.Fatal("ENCRYPTION_KEY not set. Please set it in .env or environment")
	}

	app := pocketbase.New()

	// Bootstrap the app (loads database) without starting the server
	if err := app.Bootstrap(); err != nil {
		log.Fatalf("Failed to bootstrap app: %v", err)
	}

	// Run migration
	if err := migrateContacts(app); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	log.Println("Migration complete!")
}

func loadEnv() {
	data, err := os.ReadFile("../.env")
	if err != nil {
		data, _ = os.ReadFile(".env")
	}
	if data == nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			os.Setenv(parts[0], parts[1])
		}
	}
}

func migrateContacts(app *pocketbase.PocketBase) error {
	log.Println("Starting PII encryption migration for contacts...")

	records, err := app.FindAllRecords("contacts")
	if err != nil {
		return fmt.Errorf("failed to fetch contacts: %w", err)
	}

	log.Printf("Found %d contacts to process", len(records))

	piiFields := []string{"email", "phone", "bio", "location"}
	migrated := 0
	skipped := 0

	for _, record := range records {
		needsUpdate := false

		for _, field := range piiFields {
			val := record.GetString(field)
			if val == "" {
				continue
			}

			// Check if already encrypted (has "enc:" prefix)
			if strings.HasPrefix(val, "enc:") {
				continue
			}

			// Encrypt the field
			encrypted, err := utils.Encrypt(val)
			if err != nil {
				log.Printf("  Warning: failed to encrypt %s for contact %s: %v", field, record.Id, err)
				continue
			}

			record.Set(field, encrypted)
			needsUpdate = true
		}

		// Update email_index for blind search
		email := record.GetString("email")
		if email != "" {
			// Get original email (decrypt if needed)
			originalEmail := utils.DecryptField(email)
			blindIndex := utils.BlindIndex(originalEmail)
			if record.GetString("email_index") != blindIndex {
				record.Set("email_index", blindIndex)
				needsUpdate = true
			}
		}

		if needsUpdate {
			if err := app.Save(record); err != nil {
				log.Printf("  Error: failed to save contact %s: %v", record.Id, err)
				continue
			}
			migrated++
			log.Printf("  Encrypted contact: %s", record.Id)
		} else {
			skipped++
		}
	}

	log.Printf("Migration complete: %d migrated, %d already encrypted/empty", migrated, skipped)
	return nil
}
