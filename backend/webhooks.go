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
	"sync"
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

// ProjectionConsumer represents a consumer of CRM projections
type ProjectionConsumer struct {
	ID            string
	Name          string
	AppID         string
	EndpointURL   string
	WebhookSecret string
	Enabled       bool
}

// getProjectionConsumers returns all enabled consumers from the database
// Falls back to environment variables if collection doesn't exist (migration safety)
func getProjectionConsumers(app *pocketbase.PocketBase) []ProjectionConsumer {
	consumers := []ProjectionConsumer{}

	// Try to get consumers from database
	records, err := app.FindAllRecords("projection_consumers")
	if err != nil {
		log.Printf("[Webhook] projection_consumers collection not found, falling back to env vars: %v", err)
		return getConsumersFromEnv()
	}

	// Default secret from env var - used when database field is empty
	defaultSecret := os.Getenv("PROJECTION_WEBHOOK_SECRET")

	for _, r := range records {
		if !r.GetBool("enabled") {
			continue
		}
		// Use database secret if set, otherwise fall back to env var
		secret := r.GetString("webhook_secret")
		if secret == "" {
			secret = defaultSecret
		}
		consumers = append(consumers, ProjectionConsumer{
			ID:            r.Id,
			Name:          r.GetString("name"),
			AppID:         r.GetString("app_id"),
			EndpointURL:   r.GetString("endpoint_url"),
			WebhookSecret: secret,
			Enabled:       true,
		})
	}

	if len(consumers) == 0 {
		log.Printf("[Webhook] No enabled consumers in database, falling back to env vars")
		return getConsumersFromEnv()
	}

	return consumers
}

// getConsumersFromEnv returns consumers from environment variables (fallback)
func getConsumersFromEnv() []ProjectionConsumer {
	consumers := []ProjectionConsumer{}
	secret := os.Getenv("PROJECTION_WEBHOOK_SECRET")

	if url := os.Getenv("PRESENTATIONS_WEBHOOK_URL"); url != "" {
		consumers = append(consumers, ProjectionConsumer{
			Name:          "Presentations",
			AppID:         "presentations",
			EndpointURL:   url,
			WebhookSecret: secret,
			Enabled:       true,
		})
	}

	if url := os.Getenv("DAM_CONTACT_WEBHOOK_URL"); url != "" {
		consumers = append(consumers, ProjectionConsumer{
			Name:          "DAM",
			AppID:         "dam",
			EndpointURL:   url,
			WebhookSecret: secret,
			Enabled:       true,
		})
	}

	if url := os.Getenv("WEBSITE_WEBHOOK_URL"); url != "" {
		consumers = append(consumers, ProjectionConsumer{
			Name:          "Website",
			AppID:         "website",
			EndpointURL:   url,
			WebhookSecret: secret,
			Enabled:       true,
		})
	}

	return consumers
}

// updateConsumerStatus updates the last_consumption, last_status, and last_message for a consumer
func updateConsumerStatus(app *pocketbase.PocketBase, consumerID, status, message string) {
	if consumerID == "" {
		return
	}
	record, err := app.FindRecordById("projection_consumers", consumerID)
	if err != nil {
		return
	}
	record.Set("last_consumption", time.Now().UTC().Format(time.RFC3339))
	record.Set("last_status", status)
	record.Set("last_message", message)
	app.Save(record)
}

const maxWebhookRetries = 3

// sendWebhookToConsumer sends a webhook to a consumer and updates its status
func sendWebhookToConsumer(app *pocketbase.PocketBase, payload WebhookPayload, consumer ProjectionConsumer) error {
	if consumer.EndpointURL == "" {
		return nil
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		updateConsumerStatus(app, consumer.ID, "error", "Failed to marshal payload: "+err.Error())
		return err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	var lastErr error

	for attempt := 1; attempt <= maxWebhookRetries; attempt++ {
		req, err := http.NewRequest("POST", consumer.EndpointURL, bytes.NewBuffer(jsonData))
		if err != nil {
			updateConsumerStatus(app, consumer.ID, "error", "Failed to create request: "+err.Error())
			return err
		}

		req.Header.Set("Content-Type", "application/json")

		// Sign the payload with HMAC-SHA256 if secret is configured
		if consumer.WebhookSecret != "" {
			mac := hmac.New(sha256.New, []byte(consumer.WebhookSecret))
			mac.Write(jsonData)
			signature := hex.EncodeToString(mac.Sum(nil))
			req.Header.Set("X-Webhook-Signature", signature)
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			log.Printf("[Webhook] Attempt %d/%d failed for %s: %v", attempt, maxWebhookRetries, consumer.Name, err)
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
			log.Printf("[Webhook] Attempt %d/%d: %s returned %d (server error)", attempt, maxWebhookRetries, consumer.Name, resp.StatusCode)
			if attempt < maxWebhookRetries {
				backoff := time.Duration(1<<(attempt-1)) * time.Second
				time.Sleep(backoff)
			}
			lastErr = err
			continue
		}

		if resp.StatusCode >= 400 {
			// Client error - don't retry
			msg := "HTTP " + http.StatusText(resp.StatusCode)
			log.Printf("[Webhook] %s returned %d (not retrying)", consumer.Name, resp.StatusCode)
			updateConsumerStatus(app, consumer.ID, "error", msg)
			return nil
		}

		// Success
		log.Printf("[Webhook] Successfully sent %s webhook for %s to %s", payload.Action, payload.Collection, consumer.Name)
		updateConsumerStatus(app, consumer.ID, "ok", "")
		return nil
	}

	// All retries exhausted
	log.Printf("[Webhook] All %d attempts failed for %s", maxWebhookRetries, consumer.Name)
	if lastErr != nil {
		updateConsumerStatus(app, consumer.ID, "error", "All retries failed: "+lastErr.Error())
	} else {
		updateConsumerStatus(app, consumer.ID, "error", "All retries failed")
	}
	return lastErr
}

// sendWebhookToURL sends a webhook to a specific URL with given secret and retry logic (legacy)
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

// sendGenericWebhookToConsumer sends any JSON payload to a consumer URL with HMAC signing and status tracking
func sendGenericWebhookToConsumer(app *pocketbase.PocketBase, payload any, consumerID, url, secret, destination string) error {
	if url == "" {
		return nil
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		updateConsumerStatus(app, consumerID, "error", "Failed to marshal payload: "+err.Error())
		return err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	var lastErr error

	for attempt := 1; attempt <= maxWebhookRetries; attempt++ {
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			updateConsumerStatus(app, consumerID, "error", "Failed to create request: "+err.Error())
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
			msg := "HTTP " + http.StatusText(resp.StatusCode)
			log.Printf("[Webhook] %s returned %d (not retrying)", destination, resp.StatusCode)
			updateConsumerStatus(app, consumerID, "error", msg)
			return nil
		}

		log.Printf("[Webhook] Successfully sent webhook to %s", destination)
		updateConsumerStatus(app, consumerID, "ok", "")
		return nil
	}

	log.Printf("[Webhook] All %d attempts failed for %s", maxWebhookRetries, destination)
	if lastErr != nil {
		updateConsumerStatus(app, consumerID, "error", "All retries failed: "+lastErr.Error())
	} else {
		updateConsumerStatus(app, consumerID, "error", "All retries failed")
	}
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
func buildDAMContactPayload(r *core.Record, app *pocketbase.PocketBase, baseURL, action string) WebhookPayload {
	data := map[string]any{
		"id":             r.Id, // DAM maps this to presenter_id
		"email":          utils.DecryptField(r.GetString("email")),
		"first_name":     r.GetString("first_name"),
		"last_name":      r.GetString("last_name"),
		"name":           strings.TrimSpace(r.GetString("first_name") + " " + r.GetString("last_name")),
		"personal_email": utils.DecryptField(r.GetString("personal_email")),
		"phone":          utils.DecryptField(r.GetString("phone")),
		"pronouns":       r.GetString("pronouns"),
		"bio":            utils.DecryptField(r.GetString("bio")),
		"job_title":      r.GetString("job_title"),
		"linkedin":       r.GetString("linkedin"),
		"instagram":      r.GetString("instagram"),
		"website":        r.GetString("website"),
		"location":       utils.DecryptField(r.GetString("location")),
		"preferred_name": r.GetString("preferred_name"),
		"do_position":    r.GetString("do_position"),
		"dietary_requirements":              r.Get("dietary_requirements"),
		"dietary_requirements_other":        r.GetString("dietary_requirements_other"),
		"accessibility_requirements":        r.Get("accessibility_requirements"),
		"accessibility_requirements_other":  r.GetString("accessibility_requirements_other"),
		"created":        r.GetString("created"),
		"updated":        r.GetString("updated"),
	}

	// Avatar URL (stored by DAM, not local file)
	if avatarURL := r.GetString("avatar_url"); avatarURL != "" {
		data["avatar_url"] = avatarURL
	}

	// Organisation relation
	if orgID := r.GetString("organisation"); orgID != "" {
		org, err := app.FindRecordById(utils.CollectionOrganisations, orgID)
		if err == nil {
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

	// Logo URLs are managed by DAM — CRM does not project logo files

	return DAMOrganisationPayload{
		Action:     action,
		Projection: projection,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}
}

// getConsumerByAppID returns a specific consumer by app_id
func getConsumerByAppID(app *pocketbase.PocketBase, appID string) *ProjectionConsumer {
	consumers := getProjectionConsumers(app)
	for _, c := range consumers {
		if c.AppID == appID {
			return &c
		}
	}
	return nil
}

// sendContactToDAM sends a contact to DAM's presenter-projection endpoint
func sendContactToDAM(r *core.Record, app *pocketbase.PocketBase, baseURL, action string) {
	consumer := getConsumerByAppID(app, "dam")
	if consumer == nil || consumer.EndpointURL == "" {
		return
	}

	// DAM uses a different endpoint for contacts (presenters)
	// Replace the contact-projection endpoint with presenter-projection
	presenterURL := strings.Replace(consumer.EndpointURL, "/contact-projection", "/presenter-projection", 1)

	payload := buildDAMContactPayload(r, app, baseURL, action)
	go sendGenericWebhookToConsumer(app, payload, consumer.ID, presenterURL, consumer.WebhookSecret, "DAM-Contact")
}

// sendOrganisationToDAM sends an organisation to DAM's organization-projection endpoint
func sendOrganisationToDAM(r *core.Record, app *pocketbase.PocketBase, baseURL, action string) {
	consumer := getConsumerByAppID(app, "dam")
	if consumer == nil || consumer.EndpointURL == "" {
		return
	}

	// DAM uses a different endpoint for organisations
	// Replace the contact-projection endpoint with organization-projection
	orgURL := strings.Replace(consumer.EndpointURL, "/contact-projection", "/organization-projection", 1)

	payload := buildDAMOrganisationPayload(r, baseURL, action)
	go sendGenericWebhookToConsumer(app, payload, consumer.ID, orgURL, consumer.WebhookSecret, "DAM-Org")
}

// sendWebhookToAllConsumers sends a webhook to all enabled consumers from the database
func sendWebhookToAllConsumers(app *pocketbase.PocketBase, payload WebhookPayload) {
	consumers := getProjectionConsumers(app)
	for _, consumer := range consumers {
		// Skip DAM for standard contact/org webhooks - it uses a different format
		if consumer.AppID == "dam" {
			continue
		}
		go sendWebhookToConsumer(app, payload, consumer)
	}
}

// sendWebhookToAllConsumersSync is like sendWebhookToAllConsumers but uses a WaitGroup for synchronization
func sendWebhookToAllConsumersSync(app *pocketbase.PocketBase, payload WebhookPayload, wg *sync.WaitGroup) {
	consumers := getProjectionConsumers(app)
	for _, consumer := range consumers {
		// Skip DAM for standard contact/org webhooks - it uses a different format
		if consumer.AppID == "dam" {
			continue
		}
		wg.Add(1)
		go func(c ProjectionConsumer) {
			defer wg.Done()
			sendWebhookToConsumer(app, payload, c)
		}(consumer)
	}
}

// sendContactToDAMSync is like sendContactToDAM but uses a WaitGroup for synchronization
func sendContactToDAMSync(r *core.Record, app *pocketbase.PocketBase, baseURL, action string, wg *sync.WaitGroup) {
	consumer := getConsumerByAppID(app, "dam")
	if consumer == nil || consumer.EndpointURL == "" {
		return
	}

	// DAM uses a different endpoint for contacts (presenters)
	presenterURL := strings.Replace(consumer.EndpointURL, "/contact-projection", "/presenter-projection", 1)

	payload := buildDAMContactPayload(r, app, baseURL, action)
	wg.Add(1)
	go func() {
		defer wg.Done()
		sendGenericWebhookToConsumer(app, payload, consumer.ID, presenterURL, consumer.WebhookSecret, "DAM-Contact")
	}()
}

// sendOrganisationToDAMSync is like sendOrganisationToDAM but uses a WaitGroup for synchronization
func sendOrganisationToDAMSync(r *core.Record, app *pocketbase.PocketBase, baseURL, action string, wg *sync.WaitGroup) {
	consumer := getConsumerByAppID(app, "dam")
	if consumer == nil || consumer.EndpointURL == "" {
		return
	}

	// DAM uses a different endpoint for organisations
	orgURL := strings.Replace(consumer.EndpointURL, "/contact-projection", "/organization-projection", 1)

	payload := buildDAMOrganisationPayload(r, baseURL, action)
	wg.Add(1)
	go func() {
		defer wg.Done()
		sendGenericWebhookToConsumer(app, payload, consumer.ID, orgURL, consumer.WebhookSecret, "DAM-Org")
	}()
}

// buildContactWebhookPayload builds the webhook payload for a contact
func buildContactWebhookPayload(r *core.Record, app *pocketbase.PocketBase, baseURL string) map[string]any {
	data := map[string]any{
		"id":             r.Id,
		"email":          utils.DecryptField(r.GetString("email")),
		"first_name":     r.GetString("first_name"),
		"last_name":      r.GetString("last_name"),
		"name":           strings.TrimSpace(r.GetString("first_name") + " " + r.GetString("last_name")),
		"personal_email": utils.DecryptField(r.GetString("personal_email")),
		"phone":          utils.DecryptField(r.GetString("phone")),
		"pronouns":       r.GetString("pronouns"),
		"bio":            utils.DecryptField(r.GetString("bio")),
		"job_title":      r.GetString("job_title"),
		"linkedin":       r.GetString("linkedin"),
		"instagram":      r.GetString("instagram"),
		"website":        r.GetString("website"),
		"location":       utils.DecryptField(r.GetString("location")),
		"preferred_name": r.GetString("preferred_name"),
		"do_position":    r.GetString("do_position"),
		"dietary_requirements":              r.Get("dietary_requirements"),
		"dietary_requirements_other":        r.GetString("dietary_requirements_other"),
		"accessibility_requirements":        r.Get("accessibility_requirements"),
		"accessibility_requirements_other":  r.GetString("accessibility_requirements_other"),
		"created":        r.GetString("created"),
		"updated":        r.GetString("updated"),
	}

	// Avatar URL (stored by DAM, not local file)
	if avatarURL := r.GetString("avatar_url"); avatarURL != "" {
		data["avatar_url"] = avatarURL
	}

	// DAM avatar variant URLs (from DAM sync)
	avatarUrls := map[string]string{}
	if thumb := r.GetString("avatar_thumb_url"); thumb != "" {
		avatarUrls["thumb"] = thumb
	}
	if small := r.GetString("avatar_small_url"); small != "" {
		avatarUrls["small"] = small
	}
	if original := r.GetString("avatar_original_url"); original != "" {
		avatarUrls["original"] = original
	}
	if len(avatarUrls) > 0 {
		data["avatar_urls"] = avatarUrls
	}

	// Organisation relation
	if orgID := r.GetString("organisation"); orgID != "" {
		org, err := app.FindRecordById(utils.CollectionOrganisations, orgID)
		if err == nil {
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

	// Logo URLs are managed by DAM — CRM does not project logo files

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
			// Send to Presentations/Website (standard format)
			payload := WebhookPayload{
				Action:     "upsert",
				Collection: "contacts",
				Record:     buildContactWebhookPayload(e.Record, app, baseURL),
				Timestamp:  time.Now().UTC().Format(time.RFC3339),
			}
			sendWebhookToAllConsumers(app, payload)

			// Send to DAM (presenter format)
			sendContactToDAM(e.Record, app, baseURL, "upsert")
		}()
		return e.Next()
	})

	app.OnRecordAfterUpdateSuccess(utils.CollectionContacts).BindFunc(func(e *core.RecordEvent) error {
		go func() {
			if shouldProjectContact(e.Record) {
				// Contact is projectable - send upsert
				payload := WebhookPayload{
					Action:     "upsert",
					Collection: "contacts",
					Record:     buildContactWebhookPayload(e.Record, app, baseURL),
					Timestamp:  time.Now().UTC().Format(time.RFC3339),
				}
				sendWebhookToAllConsumers(app, payload)

				// Send to DAM (presenter format)
				sendContactToDAM(e.Record, app, baseURL, "upsert")
			} else {
				// Contact was archived - send delete
				payload := WebhookPayload{
					Action:     "delete",
					Collection: "contacts",
					Record:     map[string]any{"id": e.Record.Id},
					Timestamp:  time.Now().UTC().Format(time.RFC3339),
				}
				sendWebhookToAllConsumers(app, payload)

				// Send delete to DAM
				sendContactToDAM(e.Record, app, baseURL, "delete")
			}
		}()
		return e.Next()
	})

	app.OnRecordAfterDeleteSuccess(utils.CollectionContacts).BindFunc(func(e *core.RecordEvent) error {
		go func() {
			payload := WebhookPayload{
				Action:     "delete",
				Collection: "contacts",
				Record:     map[string]any{"id": e.Record.Id},
				Timestamp:  time.Now().UTC().Format(time.RFC3339),
			}
			sendWebhookToAllConsumers(app, payload)

			// Send delete to DAM
			sendContactToDAM(e.Record, app, baseURL, "delete")
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
			sendWebhookToAllConsumers(app, payload)

			// Send to DAM (projection format)
			sendOrganisationToDAM(e.Record, app, baseURL, "upsert")
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
				sendWebhookToAllConsumers(app, payload)

				// Send to DAM (projection format)
				sendOrganisationToDAM(e.Record, app, baseURL, "upsert")
			} else {
				// Send delete to Presentations/Website
				payload := WebhookPayload{
					Action:     "delete",
					Collection: "organisations",
					Record:     map[string]any{"id": e.Record.Id},
					Timestamp:  time.Now().UTC().Format(time.RFC3339),
				}
				sendWebhookToAllConsumers(app, payload)

				// Send delete to DAM
				sendOrganisationToDAM(e.Record, app, baseURL, "delete")
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
			sendWebhookToAllConsumers(app, payload)

			// Send to DAM
			sendOrganisationToDAM(e.Record, app, baseURL, "delete")
		}()
		return e.Next()
	})

	// Log that hooks are registered (consumers will be queried at runtime when DB is ready)
	log.Printf("[Webhook] Registered hooks for collections: contacts, organisations")
}

// ProjectAllResult contains the result of projecting all records.
type ProjectAllResult struct {
	ProjectionID   string         `json:"projection_id"`
	Counts         map[string]int `json:"counts"`
	Total          int            `json:"total"`
	ConsumerNames  []string       `json:"consumer_names"`
}

// ProjectAll sends all contacts and organisations to consumers (for initial sync or resync)
func ProjectAll(app *pocketbase.PocketBase) (ProjectAllResult, error) {
	baseURL := os.Getenv("PUBLIC_BASE_URL")
	if baseURL == "" {
		baseURL = "https://crm.theoutlook.io"
	}

	result := ProjectAllResult{
		Counts: map[string]int{
			"contacts":      0,
			"organisations": 0,
		},
	}

	// Get consumer names for the projection log
	consumers := getProjectionConsumers(app)
	for _, c := range consumers {
		result.ConsumerNames = append(result.ConsumerNames, c.AppID)
	}

	// Count records first
	contacts, err := app.FindAllRecords(utils.CollectionContacts)
	if err != nil {
		log.Printf("[ProjectAll] Failed to fetch contacts: %v", err)
	}
	organisations, orgErr := app.FindAllRecords(utils.CollectionOrganisations)
	if orgErr != nil {
		log.Printf("[ProjectAll] Failed to fetch organisations: %v", orgErr)
	}

	// Count projectable records
	for _, r := range contacts {
		if shouldProjectContact(r) {
			result.Counts["contacts"]++
		}
	}
	for _, r := range organisations {
		if shouldProjectOrganisation(r) {
			result.Counts["organisations"]++
		}
	}
	result.Total = result.Counts["contacts"] + result.Counts["organisations"]

	// Create projection log record
	logsCollection, logErr := app.FindCollectionByNameOrId("projection_logs")
	if logErr != nil {
		log.Printf("[ProjectAll] Warning: projection_logs collection not found: %v", logErr)
	} else {
		logRecord := core.NewRecord(logsCollection)
		logRecord.Set("record_count", result.Total)
		logRecord.Set("consumers", result.ConsumerNames)
		if err := app.Save(logRecord); err != nil {
			log.Printf("[ProjectAll] Failed to create projection log: %v", err)
		} else {
			result.ProjectionID = logRecord.Id
			log.Printf("[ProjectAll] Created projection log %s", result.ProjectionID)
		}
	}

	// WaitGroup to track all webhook goroutines
	var wg sync.WaitGroup

	// Project all active/inactive contacts
	for _, r := range contacts {
		if shouldProjectContact(r) {
			payload := WebhookPayload{
				Action:     "upsert",
				Collection: "contacts",
				Record:     buildContactWebhookPayload(r, app, baseURL),
				Timestamp:  time.Now().UTC().Format(time.RFC3339),
			}
			sendWebhookToAllConsumersSync(app, payload, &wg)
			sendContactToDAMSync(r, app, baseURL, "upsert", &wg)
		}
	}

	// Project all active organisations
	for _, r := range organisations {
		if shouldProjectOrganisation(r) {
			payload := WebhookPayload{
				Action:     "upsert",
				Collection: "organisations",
				Record:     buildOrganisationWebhookPayload(r, baseURL),
				Timestamp:  time.Now().UTC().Format(time.RFC3339),
			}
			sendWebhookToAllConsumersSync(app, payload, &wg)
			sendOrganisationToDAMSync(r, app, baseURL, "upsert", &wg)
		}
	}

	// Wait for all webhooks to complete
	log.Printf("[ProjectAll] Waiting for %d contacts and %d organisations webhooks to complete...", result.Counts["contacts"], result.Counts["organisations"])
	wg.Wait()

	log.Printf("[ProjectAll] Projected %d contacts, %d organisations (projection_id: %s)", result.Counts["contacts"], result.Counts["organisations"], result.ProjectionID)
	return result, nil
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
