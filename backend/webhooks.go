package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/grtshw/outlook-apps-crm/utils"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// WebhookPayload represents the payload sent to webhook receivers
type WebhookPayload struct {
	Action     string         `json:"action"`     // upsert, delete
	Collection string         `json:"collection"` // contacts, organisations
	Record     map[string]any `json:"record"`     // The record data
	Timestamp  string         `json:"timestamp"`  // ISO timestamp
}

// DAMOrganisationPayload is the format expected by DAM for organisation projections
type DAMOrganisationPayload struct {
	Action     string         `json:"action"`     // upsert, delete
	Projection map[string]any `json:"projection"` // The projection data
	Timestamp  string         `json:"timestamp"`  // ISO timestamp
}

// WebhookConfig holds webhook configuration for consumers
type WebhookConfig struct {
	PresentationsWebhookURL string
	PresentationsSecret     string
	DAMContactWebhookURL    string // DAM endpoint for contacts (as presenters)
	DAMOrgWebhookURL        string // DAM endpoint for organisations
	DAMSecret               string
	WebsiteWebhookURL       string
	WebsiteSecret           string
}

// getWebhookConfig returns webhook configuration from environment
func getWebhookConfig() WebhookConfig {
	return WebhookConfig{
		PresentationsWebhookURL: os.Getenv("PRESENTATIONS_WEBHOOK_URL"),
		PresentationsSecret:     os.Getenv("PROJECTION_WEBHOOK_SECRET"),
		DAMContactWebhookURL:    os.Getenv("DAM_CONTACT_WEBHOOK_URL"),  // /api/webhooks/presenter-projection
		DAMOrgWebhookURL:        os.Getenv("DAM_ORG_WEBHOOK_URL"),      // /api/webhooks/organization-projection
		DAMSecret:               os.Getenv("PROJECTION_WEBHOOK_SECRET"),
		WebsiteWebhookURL:       os.Getenv("WEBSITE_WEBHOOK_URL"),
		WebsiteSecret:           os.Getenv("PROJECTION_WEBHOOK_SECRET"),
	}
}

const maxWebhookRetries = 3

// buildOrganisationsMapFromContacts pre-fetches organisations for a list of contacts
// This eliminates N+1 queries when building webhook payloads
func buildOrganisationsMapFromContacts(records []*core.Record, app *pocketbase.PocketBase) map[string]*core.Record {
	// Collect unique organisation IDs
	orgIDsSet := make(map[string]bool)
	for _, r := range records {
		if orgID := r.GetString("organisation"); orgID != "" {
			orgIDsSet[orgID] = true
		}
	}

	// Convert set to slice
	orgIDs := make([]string, 0, len(orgIDsSet))
	for orgID := range orgIDsSet {
		orgIDs = append(orgIDs, orgID)
	}

	// Build map
	orgsMap := make(map[string]*core.Record)
	if len(orgIDs) == 0 {
		return orgsMap
	}

	// Fetch all organisations in one query using IN clause
	filter := "id IN {:ids}"
	params := map[string]any{"ids": orgIDs}

	orgs, err := app.FindRecordsByFilter(
		utils.CollectionOrganisations,
		filter,
		"",
		0, 0,
		params,
	)

	if err != nil {
		log.Printf("[BuildOrgsMap] Failed to fetch organisations: %v", err)
		return orgsMap
	}

	// Populate map
	for _, org := range orgs {
		orgsMap[org.Id] = org
	}

	log.Printf("[BuildOrgsMap] Pre-fetched %d organisations for %d contacts (webhooks)", len(orgsMap), len(records))

	return orgsMap
}

// sendWebhookToURL sends a webhook to a specific URL with given secret and retry logic
func sendWebhookToURL(payload WebhookPayload, url, secret, destination string) error {
	if url == "" {
		return nil
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	var lastErr error

	for attempt := 1; attempt <= maxWebhookRetries; attempt++ {
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			return err
		}

		req.Header.Set("Content-Type", "application/json")

		// Sign the payload with HMAC-SHA256 if secret is configured
		if secret != "" {
			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write(jsonData)
			signature := hex.EncodeToString(mac.Sum(nil))
			req.Header.Set("X-Webhook-Signature", signature)
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			log.Printf("[Webhook] Attempt %d/%d failed for %s: %v", attempt, maxWebhookRetries, destination, err)
			if attempt < maxWebhookRetries {
				// Exponential backoff: 1s, 2s, 4s
				backoff := time.Duration(1<<(attempt-1)) * time.Second
				time.Sleep(backoff)
			}
			continue
		}

		resp.Body.Close()

		if resp.StatusCode >= 500 {
			// Server error - retry
			log.Printf("[Webhook] Attempt %d/%d: %s returned %d (server error)", attempt, maxWebhookRetries, destination, resp.StatusCode)
			if attempt < maxWebhookRetries {
				backoff := time.Duration(1<<(attempt-1)) * time.Second
				time.Sleep(backoff)
			}
			continue
		}

		if resp.StatusCode >= 400 {
			// Client error - don't retry
			log.Printf("[Webhook] %s returned %d (not retrying)", destination, resp.StatusCode)
			return nil
		}

		// Success
		log.Printf("[Webhook] Successfully sent %s webhook for %s to %s", payload.Action, payload.Collection, destination)
		return nil
	}

	// All retries exhausted
	log.Printf("[Webhook] All %d attempts failed for %s", maxWebhookRetries, destination)
	return lastErr
}

// sendGenericWebhookToURL sends any JSON payload to a URL with HMAC signing
func sendGenericWebhookToURL(payload any, url, secret, destination string) error {
	if url == "" {
		return nil
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	var lastErr error

	for attempt := 1; attempt <= maxWebhookRetries; attempt++ {
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			return err
		}

		req.Header.Set("Content-Type", "application/json")

		// Sign the payload with HMAC-SHA256 if secret is configured
		if secret != "" {
			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write(jsonData)
			signature := hex.EncodeToString(mac.Sum(nil))
			req.Header.Set("X-Webhook-Signature", signature)
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			log.Printf("[Webhook] Attempt %d/%d failed for %s: %v", attempt, maxWebhookRetries, destination, err)
			if attempt < maxWebhookRetries {
				backoff := time.Duration(1<<(attempt-1)) * time.Second
				time.Sleep(backoff)
			}
			continue
		}

		resp.Body.Close()

		if resp.StatusCode >= 500 {
			log.Printf("[Webhook] Attempt %d/%d: %s returned %d (server error)", attempt, maxWebhookRetries, destination, resp.StatusCode)
			if attempt < maxWebhookRetries {
				backoff := time.Duration(1<<(attempt-1)) * time.Second
				time.Sleep(backoff)
			}
			continue
		}

		if resp.StatusCode >= 400 {
			log.Printf("[Webhook] %s returned %d (not retrying)", destination, resp.StatusCode)
			return nil
		}

		log.Printf("[Webhook] Successfully sent webhook to %s", destination)
		return nil
	}

	log.Printf("[Webhook] All %d attempts failed for %s", maxWebhookRetries, destination)
	return lastErr
}

// buildDAMContactPayload builds the payload for DAM's presenter-projection endpoint
// DAM expects contacts as "presenters" with presenter_id field
func buildDAMContactPayload(r *core.Record, baseURL, action string, orgsMap map[string]*core.Record) WebhookPayload {
	data := map[string]any{
		"id":          r.Id, // DAM maps this to presenter_id
		"email":       r.GetString("email"),
		"name":        r.GetString("name"),
		"phone":       r.GetString("phone"),
		"pronouns":    r.GetString("pronouns"),
		"bio":         r.GetString("bio"),
		"job_title":   r.GetString("job_title"),
		"linkedin":    r.GetString("linkedin"),
		"instagram":   r.GetString("instagram"),
		"website":     r.GetString("website"),
		"location":    r.GetString("location"),
		"do_position": r.GetString("do_position"),
		"created":     r.GetString("created"),
		"updated":     r.GetString("updated"),
	}

	// Avatar URL - DAM will download from this URL and generate variants
	if avatar := r.GetString("avatar"); avatar != "" {
		data["avatar_url"] = getFileURL(baseURL, r.Collection().Id, r.Id, avatar)
	}

	// Organisation relation - use map lookup to avoid N+1 query
	if orgID := r.GetString("organisation"); orgID != "" {
		if org, exists := orgsMap[orgID]; exists {
			data["organisation_id"] = org.Id
			data["organisation_name"] = org.GetString("name")
		}
	}

	return WebhookPayload{
		Action:     action,
		Collection: "presenters", // DAM expects this collection name
		Record:     data,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}
}

// buildDAMOrganisationPayload builds the payload for DAM's organization-projection endpoint
// DAM expects a different format with "projection" wrapper
func buildDAMOrganisationPayload(r *core.Record, baseURL, action string) DAMOrganisationPayload {
	projection := map[string]any{
		"org_id":             r.Id,
		"name":               r.GetString("name"),
		"website":            r.GetString("website"),
		"linkedin":           r.GetString("linkedin"),
		"description_short":  r.GetString("description_short"),
		"description_medium": r.GetString("description_medium"),
		"description_long":   r.GetString("description_long"),
		"contacts":           r.Get("contacts"),
	}

	// Logo URL - pick first available typed logo for projection
	collectionId := r.Collection().Id
	if logo := r.GetString("logo_square"); logo != "" {
		projection["logo_url"] = getFileURL(baseURL, collectionId, r.Id, logo)
	} else if logo := r.GetString("logo_standard"); logo != "" {
		projection["logo_url"] = getFileURL(baseURL, collectionId, r.Id, logo)
	} else if logo := r.GetString("logo_inverted"); logo != "" {
		projection["logo_url"] = getFileURL(baseURL, collectionId, r.Id, logo)
	}

	return DAMOrganisationPayload{
		Action:     action,
		Projection: projection,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}
}

// sendContactToDAM sends a contact to DAM's presenter-projection endpoint
func sendContactToDAM(r *core.Record, baseURL, action string, orgsMap map[string]*core.Record, config WebhookConfig) {
	if config.DAMContactWebhookURL == "" {
		return
	}

	payload := buildDAMContactPayload(r, baseURL, action, orgsMap)
	go sendGenericWebhookToURL(payload, config.DAMContactWebhookURL, config.DAMSecret, "DAM-Contact")
}

// sendOrganisationToDAM sends an organisation to DAM's organization-projection endpoint
func sendOrganisationToDAM(r *core.Record, baseURL, action string, config WebhookConfig) {
	if config.DAMOrgWebhookURL == "" {
		return
	}

	payload := buildDAMOrganisationPayload(r, baseURL, action)
	go sendGenericWebhookToURL(payload, config.DAMOrgWebhookURL, config.DAMSecret, "DAM-Org")
}

// sendWebhookToAllConsumers sends a webhook to all configured consumers
// This is for Presentations and Website which use the standard format
func sendWebhookToAllConsumers(payload WebhookPayload, config WebhookConfig) {
	// Send to Presentations (for backward compatibility during migration)
	if config.PresentationsWebhookURL != "" {
		go sendWebhookToURL(payload, config.PresentationsWebhookURL, config.PresentationsSecret, "Presentations")
	}

	// Send to Website
	if config.WebsiteWebhookURL != "" {
		go sendWebhookToURL(payload, config.WebsiteWebhookURL, config.WebsiteSecret, "Website")
	}
}

// buildContactWebhookPayload builds the webhook payload for a contact
func buildContactWebhookPayload(r *core.Record, baseURL string, orgsMap map[string]*core.Record) map[string]any {
	data := map[string]any{
		"id":          r.Id,
		"email":       r.GetString("email"),
		"name":        r.GetString("name"),
		"phone":       r.GetString("phone"),
		"pronouns":    r.GetString("pronouns"),
		"bio":         r.GetString("bio"),
		"job_title":   r.GetString("job_title"),
		"linkedin":    r.GetString("linkedin"),
		"instagram":   r.GetString("instagram"),
		"website":     r.GetString("website"),
		"location":    r.GetString("location"),
		"do_position": r.GetString("do_position"),
		"created":     r.GetString("created"),
		"updated":     r.GetString("updated"),
	}

	// Avatar URL
	if avatar := r.GetString("avatar"); avatar != "" {
		data["avatar_url"] = getFileURL(baseURL, r.Collection().Id, r.Id, avatar)
	}

	// Organisation relation - use map lookup to avoid N+1 query
	if orgID := r.GetString("organisation"); orgID != "" {
		if org, exists := orgsMap[orgID]; exists {
			data["organisation_id"] = org.Id
			data["organisation_name"] = org.GetString("name")
		}
	}

	return data
}

// buildOrganisationWebhookPayload builds the webhook payload for an organisation
func buildOrganisationWebhookPayload(r *core.Record, baseURL string) map[string]any {
	data := map[string]any{
		"org_id":             r.Id,
		"name":               r.GetString("name"),
		"website":            r.GetString("website"),
		"linkedin":           r.GetString("linkedin"),
		"description_short":  r.GetString("description_short"),
		"description_medium": r.GetString("description_medium"),
		"description_long":   r.GetString("description_long"),
		"contacts":           r.Get("contacts"),
		"created":            r.GetString("created"),
		"updated":            r.GetString("updated"),
	}

	// Logo URLs - pick first available typed logo
	collectionId := r.Collection().Id
	if logo := r.GetString("logo_square"); logo != "" {
		data["logo_url"] = getFileURL(baseURL, collectionId, r.Id, logo)
		data["logo_square_url"] = getFileURL(baseURL, collectionId, r.Id, logo)
	}
	if logo := r.GetString("logo_standard"); logo != "" {
		data["logo_standard_url"] = getFileURL(baseURL, collectionId, r.Id, logo)
		if data["logo_url"] == nil {
			data["logo_url"] = getFileURL(baseURL, collectionId, r.Id, logo)
		}
	}
	if logo := r.GetString("logo_inverted"); logo != "" {
		data["logo_inverted_url"] = getFileURL(baseURL, collectionId, r.Id, logo)
		if data["logo_url"] == nil {
			data["logo_url"] = getFileURL(baseURL, collectionId, r.Id, logo)
		}
	}

	return data
}

// shouldProjectContact determines if a contact should be projected to consumers
// Only active and inactive contacts are projected, archived are not
func shouldProjectContact(r *core.Record) bool {
	status := r.GetString("status")
	return status == "active" || status == "inactive"
}

// shouldProjectOrganisation determines if an organisation should be projected
// Only active organisations are projected, archived are not
func shouldProjectOrganisation(r *core.Record) bool {
	status := r.GetString("status")
	return status == "active"
}

// registerWebhookHooks registers record hooks for webhook notifications
func registerWebhookHooks(app *pocketbase.PocketBase) {
	config := getWebhookConfig()
	baseURL := os.Getenv("PUBLIC_BASE_URL")
	if baseURL == "" {
		baseURL = "https://crm.theoutlook.io"
	}

	// Contacts hooks
	app.OnRecordAfterCreateSuccess(utils.CollectionContacts).BindFunc(func(e *core.RecordEvent) error {
		if !shouldProjectContact(e.Record) {
			return e.Next()
		}
		go func() {
			// For single record, fetch organisation if needed
			orgsMap := make(map[string]*core.Record)
			if orgID := e.Record.GetString("organisation"); orgID != "" {
				org, err := app.FindRecordById(utils.CollectionOrganisations, orgID)
				if err == nil {
					orgsMap[org.Id] = org
				}
			}

			// Send to Presentations/Website (standard format)
			payload := WebhookPayload{
				Action:     "upsert",
				Collection: "contacts",
				Record:     buildContactWebhookPayload(e.Record, baseURL, orgsMap),
				Timestamp:  time.Now().UTC().Format(time.RFC3339),
			}
			sendWebhookToAllConsumers(payload, config)

			// Send to DAM (presenter format)
			sendContactToDAM(e.Record, baseURL, "upsert", orgsMap, config)
		}()
		return e.Next()
	})

	app.OnRecordAfterUpdateSuccess(utils.CollectionContacts).BindFunc(func(e *core.RecordEvent) error {
		go func() {
			// For single record, fetch organisation if needed
			orgsMap := make(map[string]*core.Record)
			if orgID := e.Record.GetString("organisation"); orgID != "" {
				org, err := app.FindRecordById(utils.CollectionOrganisations, orgID)
				if err == nil {
					orgsMap[org.Id] = org
				}
			}

			if shouldProjectContact(e.Record) {
				// Contact is projectable - send upsert
				payload := WebhookPayload{
					Action:     "upsert",
					Collection: "contacts",
					Record:     buildContactWebhookPayload(e.Record, baseURL, orgsMap),
					Timestamp:  time.Now().UTC().Format(time.RFC3339),
				}
				sendWebhookToAllConsumers(payload, config)

				// Send to DAM (presenter format)
				sendContactToDAM(e.Record, baseURL, "upsert", orgsMap, config)
			} else {
				// Contact was archived - send delete (no org data needed)
				emptyOrgsMap := make(map[string]*core.Record)
				payload := WebhookPayload{
					Action:     "delete",
					Collection: "contacts",
					Record:     map[string]any{"id": e.Record.Id},
					Timestamp:  time.Now().UTC().Format(time.RFC3339),
				}
				sendWebhookToAllConsumers(payload, config)

				// Send delete to DAM
				sendContactToDAM(e.Record, baseURL, "delete", emptyOrgsMap, config)
			}
		}()
		return e.Next()
	})

	app.OnRecordAfterDeleteSuccess(utils.CollectionContacts).BindFunc(func(e *core.RecordEvent) error {
		go func() {
			// For delete, no org data needed
			emptyOrgsMap := make(map[string]*core.Record)

			payload := WebhookPayload{
				Action:     "delete",
				Collection: "contacts",
				Record:     map[string]any{"id": e.Record.Id},
				Timestamp:  time.Now().UTC().Format(time.RFC3339),
			}
			sendWebhookToAllConsumers(payload, config)

			// Send delete to DAM
			sendContactToDAM(e.Record, baseURL, "delete", emptyOrgsMap, config)
		}()
		return e.Next()
	})

	// Organisations hooks
	app.OnRecordAfterCreateSuccess(utils.CollectionOrganisations).BindFunc(func(e *core.RecordEvent) error {
		if !shouldProjectOrganisation(e.Record) {
			return e.Next()
		}
		go func() {
			// Send to Presentations/Website (standard format)
			payload := WebhookPayload{
				Action:     "upsert",
				Collection: "organisations",
				Record:     buildOrganisationWebhookPayload(e.Record, baseURL),
				Timestamp:  time.Now().UTC().Format(time.RFC3339),
			}
			sendWebhookToAllConsumers(payload, config)

			// Send to DAM (projection format)
			sendOrganisationToDAM(e.Record, baseURL, "upsert", config)
		}()
		return e.Next()
	})

	app.OnRecordAfterUpdateSuccess(utils.CollectionOrganisations).BindFunc(func(e *core.RecordEvent) error {
		go func() {
			if shouldProjectOrganisation(e.Record) {
				// Send to Presentations/Website (standard format)
				payload := WebhookPayload{
					Action:     "upsert",
					Collection: "organisations",
					Record:     buildOrganisationWebhookPayload(e.Record, baseURL),
					Timestamp:  time.Now().UTC().Format(time.RFC3339),
				}
				sendWebhookToAllConsumers(payload, config)

				// Send to DAM (projection format)
				sendOrganisationToDAM(e.Record, baseURL, "upsert", config)
			} else {
				// Send delete to Presentations/Website
				payload := WebhookPayload{
					Action:     "delete",
					Collection: "organisations",
					Record:     map[string]any{"id": e.Record.Id},
					Timestamp:  time.Now().UTC().Format(time.RFC3339),
				}
				sendWebhookToAllConsumers(payload, config)

				// Send delete to DAM
				sendOrganisationToDAM(e.Record, baseURL, "delete", config)
			}
		}()
		return e.Next()
	})

	app.OnRecordAfterDeleteSuccess(utils.CollectionOrganisations).BindFunc(func(e *core.RecordEvent) error {
		go func() {
			// Send to Presentations/Website
			payload := WebhookPayload{
				Action:     "delete",
				Collection: "organisations",
				Record:     map[string]any{"id": e.Record.Id},
				Timestamp:  time.Now().UTC().Format(time.RFC3339),
			}
			sendWebhookToAllConsumers(payload, config)

			// Send to DAM
			sendOrganisationToDAM(e.Record, baseURL, "delete", config)
		}()
		return e.Next()
	})

	// Log configuration
	log.Printf("[Webhook] Registered hooks for collections: contacts, organisations")
	if config.PresentationsWebhookURL != "" {
		log.Printf("[Webhook] Presentations endpoint: %s", maskURL(config.PresentationsWebhookURL))
	}
	if config.DAMContactWebhookURL != "" {
		log.Printf("[Webhook] DAM Contact endpoint: %s", maskURL(config.DAMContactWebhookURL))
	}
	if config.DAMOrgWebhookURL != "" {
		log.Printf("[Webhook] DAM Org endpoint: %s", maskURL(config.DAMOrgWebhookURL))
	}
	if config.WebsiteWebhookURL != "" {
		log.Printf("[Webhook] Website endpoint: %s", maskURL(config.WebsiteWebhookURL))
	}
}

// ProjectAll sends all contacts and organisations to consumers (for initial sync or resync)
func ProjectAll(app *pocketbase.PocketBase) (map[string]int, error) {
	config := getWebhookConfig()
	baseURL := os.Getenv("PUBLIC_BASE_URL")
	if baseURL == "" {
		baseURL = "https://crm.theoutlook.io"
	}

	counts := map[string]int{
		"contacts":      0,
		"organisations": 0,
	}

	// Project all active/inactive contacts
	contacts, err := app.FindAllRecords(utils.CollectionContacts)
	if err != nil {
		log.Printf("[ProjectAll] Failed to fetch contacts: %v", err)
	} else {
		// Pre-fetch all organisations to avoid N+1 queries (CRITICAL for performance)
		orgsMap := buildOrganisationsMapFromContacts(contacts, app)

		for _, r := range contacts {
			if shouldProjectContact(r) {
				// Send to Presentations/Website
				payload := WebhookPayload{
					Action:     "upsert",
					Collection: "contacts",
					Record:     buildContactWebhookPayload(r, baseURL, orgsMap),
					Timestamp:  time.Now().UTC().Format(time.RFC3339),
				}
				sendWebhookToAllConsumers(payload, config)

				// Send to DAM (presenter format)
				sendContactToDAM(r, baseURL, "upsert", orgsMap, config)

				counts["contacts"]++
			}
		}
	}

	// Project all active organisations
	organisations, err := app.FindAllRecords(utils.CollectionOrganisations)
	if err != nil {
		log.Printf("[ProjectAll] Failed to fetch organisations: %v", err)
	} else {
		for _, r := range organisations {
			if shouldProjectOrganisation(r) {
				// Send to Presentations/Website
				payload := WebhookPayload{
					Action:     "upsert",
					Collection: "organisations",
					Record:     buildOrganisationWebhookPayload(r, baseURL),
					Timestamp:  time.Now().UTC().Format(time.RFC3339),
				}
				sendWebhookToAllConsumers(payload, config)

				// Send to DAM (projection format)
				sendOrganisationToDAM(r, baseURL, "upsert", config)

				counts["organisations"]++
			}
		}
	}

	log.Printf("[ProjectAll] Projected %d contacts, %d organisations", counts["contacts"], counts["organisations"])
	return counts, nil
}

// maskURL masks sensitive parts of a URL for logging
func maskURL(url string) string {
	// Just show the domain portion
	parts := strings.Split(url, "/")
	if len(parts) >= 3 {
		return parts[0] + "//" + parts[2] + "/..."
	}
	return url
}
