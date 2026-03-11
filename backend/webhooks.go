package main

import (
	"encoding/json"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/grtshw/outlook-apps-crm/utils"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	hub "outlook-apps-hub-client"
)

// WebhookPayload represents the payload sent to webhook receivers
type WebhookPayload struct {
	Action     string         `json:"action"`     // upsert, delete
	Collection string         `json:"collection"` // contacts, organisations
	Record     map[string]any `json:"record"`     // The record data
	Timestamp  string         `json:"timestamp"`  // ISO timestamp
}

// hubClient is initialized once at startup if HUB_ENABLED is set.
var hubClient *hub.Client

func initHubClient() {
	if os.Getenv("HUB_ENABLED") != "true" {
		log.Printf("[Webhook] Hub not enabled (set HUB_ENABLED=true to enable)")
		return
	}
	hubURL := os.Getenv("HUB_URL")
	hubSecret := os.Getenv("HUB_SECRET")
	if hubURL == "" || hubSecret == "" {
		log.Printf("[Webhook] HUB_ENABLED=true but HUB_URL or HUB_SECRET missing, hub disabled")
		return
	}
	hubClient = hub.NewClient(hubURL, "crm", hubSecret)
	log.Printf("[Webhook] Hub client initialized: %s", hubURL)
}

// collectionToProjectionType maps CRM collection names to hub projection types.
func collectionToProjectionType(collection string) string {
	switch collection {
	case "contacts":
		return "contact"
	case "organisations":
		return "organisation"
	default:
		return collection
	}
}

// sendWebhookToAllConsumers sends a contact/org projection through the hub
func sendWebhookToAllConsumers(app *pocketbase.PocketBase, payload WebhookPayload) {
	if hubClient == nil {
		log.Printf("[Webhook] Hub not configured, skipping %s/%s", payload.Collection, payload.Action)
		return
	}
	projType := collectionToProjectionType(payload.Collection)
	if err := hubClient.Send(projType, payload.Action, payload); err != nil {
		log.Printf("[Webhook] Hub send failed for %s/%s: %v", projType, payload.Action, err)
	} else {
		log.Printf("[Webhook] Sent %s/%s to hub", projType, payload.Action)
	}
}

// sendWebhookToAllConsumersSync is like sendWebhookToAllConsumers but uses a WaitGroup for synchronization
func sendWebhookToAllConsumersSync(app *pocketbase.PocketBase, payload WebhookPayload, wg *sync.WaitGroup) {
	if hubClient == nil {
		log.Printf("[Webhook] Hub not configured, skipping %s/%s", payload.Collection, payload.Action)
		return
	}
	projType := collectionToProjectionType(payload.Collection)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := hubClient.Send(projType, payload.Action, payload); err != nil {
			log.Printf("[Webhook] Hub send failed for %s/%s: %v", projType, payload.Action, err)
		}
	}()
}

// buildDAMContactPayload builds the payload for DAM's presenter-projection endpoint.
// DAM only needs identity, professional, and avatar fields — no PII like email/phone/bio.
func buildDAMContactPayload(r *core.Record, app *pocketbase.PocketBase, baseURL, action string) WebhookPayload {
	data := map[string]any{
		"id":        r.Id,
		"name":      strings.TrimSpace(r.GetString("first_name") + " " + r.GetString("last_name")),
		"pronouns":  r.GetString("pronouns"),
		"job_title": r.GetString("job_title"),
		"linkedin":  r.GetString("linkedin"),
		"instagram": r.GetString("instagram"),
		"website":   r.GetString("website"),
		"location":  utils.DecryptField(r.GetString("location")),
	}

	if avatarURL := r.GetString("avatar_url"); avatarURL != "" {
		data["avatar_url"] = avatarURL
	}

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

// sendContactToDAM sends a contact to DAM through the hub
func sendContactToDAM(r *core.Record, app *pocketbase.PocketBase, baseURL, action string) {
	if hubClient == nil {
		log.Printf("[Webhook] Hub not configured, skipping DAM contact/%s", action)
		return
	}
	payload := buildDAMContactPayload(r, app, baseURL, action)
	if err := hubClient.Send("contact", action, payload); err != nil {
		log.Printf("[Webhook] Hub send failed for contact/%s: %v", action, err)
	} else {
		log.Printf("[Webhook] Sent contact/%s to hub", action)
	}
}

// sendContactToDAMSync is like sendContactToDAM but uses a WaitGroup for synchronization
func sendContactToDAMSync(r *core.Record, app *pocketbase.PocketBase, baseURL, action string, wg *sync.WaitGroup) {
	if hubClient == nil {
		return
	}
	payload := buildDAMContactPayload(r, app, baseURL, action)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := hubClient.Send("contact", action, payload); err != nil {
			log.Printf("[Webhook] Hub send failed for contact/%s: %v", action, err)
		}
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
		"preferred_name": r.GetString("preferred_name"),
		"phone":          utils.DecryptField(r.GetString("phone")),
		"pronouns":       r.GetString("pronouns"),
		"bio":            utils.DecryptField(r.GetString("bio")),
		"job_title":      r.GetString("job_title"),
		"linkedin":       r.GetString("linkedin"),
		"instagram":      r.GetString("instagram"),
		"website":        r.GetString("website"),
		"location":       utils.DecryptField(r.GetString("location")),
		"do_position":    r.GetString("do_position"),
	}

	// Avatar URL (stored by DAM, not local file)
	if avatarURL := r.GetString("avatar_url"); avatarURL != "" {
		data["avatar_url"] = avatarURL
	}

	// DAM avatar variant URLs — prefer record fields, fall back to in-memory DAM cache
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
	if len(avatarUrls) == 0 {
		// Record doesn't have avatar URLs yet — check DAM cache
		if cached, ok := GetDAMAvatarURLs(r.Id); ok {
			if cached.ThumbURL != "" {
				avatarUrls["thumb"] = cached.ThumbURL
			}
			if cached.SmallURL != "" {
				avatarUrls["small"] = cached.SmallURL
			}
			if cached.OriginalURL != "" {
				avatarUrls["original"] = cached.OriginalURL
			}
		}
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

// buildOrganisationPayload builds the single canonical payload for an organisation.
// Sent once to Hub, which routes to all consumers (Website, DAM, Presentations, Awards).
func buildOrganisationPayload(r *core.Record) map[string]any {
	logoURLs := r.Get("logo_urls")

	// Fall back to DAM logo cache if record doesn't have logos
	if logoURLs == nil {
		if cached, ok := GetDAMLogoURLs(r.Id); ok {
			logoURLs = cached
		}
	} else {
		// Check if logo_urls is an empty array
		if b, err := json.Marshal(logoURLs); err == nil && (string(b) == "null" || string(b) == "[]") {
			if cached, ok := GetDAMLogoURLs(r.Id); ok {
				logoURLs = cached
			}
		}
	}

	data := map[string]any{
		"org_id":             r.Id,
		"name":               r.GetString("name"),
		"website":            r.GetString("website"),
		"linkedin":           r.GetString("linkedin"),
		"description_short":  r.GetString("description_short"),
		"description_medium": r.GetString("description_medium"),
		"description_long":   r.GetString("description_long"),
		"contacts":           r.Get("contacts"),
		"logo_urls":          logoURLs,
		"created":            r.GetString("created"),
		"updated":            r.GetString("updated"),
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
			// Send to all consumers (standard format)
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
			payload := WebhookPayload{
				Action:     "upsert",
				Collection: "organisations",
				Record:     buildOrganisationPayload(e.Record),
				Timestamp:  time.Now().UTC().Format(time.RFC3339),
			}
			sendWebhookToAllConsumers(app, payload)
		}()
		return e.Next()
	})

	app.OnRecordAfterUpdateSuccess(utils.CollectionOrganisations).BindFunc(func(e *core.RecordEvent) error {
		go func() {
			if shouldProjectOrganisation(e.Record) {
				payload := WebhookPayload{
					Action:     "upsert",
					Collection: "organisations",
					Record:     buildOrganisationPayload(e.Record),
					Timestamp:  time.Now().UTC().Format(time.RFC3339),
				}
				sendWebhookToAllConsumers(app, payload)
			} else {
				payload := WebhookPayload{
					Action:     "delete",
					Collection: "organisations",
					Record:     map[string]any{"id": e.Record.Id},
					Timestamp:  time.Now().UTC().Format(time.RFC3339),
				}
				sendWebhookToAllConsumers(app, payload)
			}
		}()
		return e.Next()
	})

	app.OnRecordAfterDeleteSuccess(utils.CollectionOrganisations).BindFunc(func(e *core.RecordEvent) error {
		go func() {
			payload := WebhookPayload{
				Action:     "delete",
				Collection: "organisations",
				Record:     map[string]any{"id": e.Record.Id},
				Timestamp:  time.Now().UTC().Format(time.RFC3339),
			}
			sendWebhookToAllConsumers(app, payload)
		}()
		return e.Next()
	})

	log.Printf("[Webhook] Registered hooks for collections: contacts, organisations")
}

// ProjectAllResult contains the result of projecting all records.
type ProjectAllResult struct {
	ProjectionID string         `json:"projection_id"`
	Counts       map[string]int `json:"counts"`
	Total        int            `json:"total"`
}

// ProjectAll sends all contacts and organisations through the hub (for initial sync or resync)
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

	if hubClient == nil {
		return result, nil
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
		logRecord.Set("consumers", []string{"hub"})
		if err := app.Save(logRecord); err != nil {
			log.Printf("[ProjectAll] Failed to create projection log: %v", err)
		} else {
			result.ProjectionID = logRecord.Id
			log.Printf("[ProjectAll] Created projection log %s", result.ProjectionID)
		}
	}

	// WaitGroup to track all hub sends
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
				Record:     buildOrganisationPayload(r),
				Timestamp:  time.Now().UTC().Format(time.RFC3339),
			}
			sendWebhookToAllConsumersSync(app, payload, &wg)
		}
	}

	// Wait for all hub sends to complete
	log.Printf("[ProjectAll] Waiting for %d contacts and %d organisations to send to hub...", result.Counts["contacts"], result.Counts["organisations"])
	wg.Wait()

	log.Printf("[ProjectAll] Projected %d contacts, %d organisations (projection_id: %s)", result.Counts["contacts"], result.Counts["organisations"], result.ProjectionID)
	return result, nil
}
