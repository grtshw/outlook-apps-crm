package main

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/grtshw/outlook-apps-crm/utils"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// --- Mailchimp API helpers ---

func mailchimpAPIURL(path string) string {
	prefix := os.Getenv("MAILCHIMP_SERVER_PREFIX")
	if prefix == "" {
		prefix = "us5"
	}
	return fmt.Sprintf("https://%s.api.mailchimp.com/3.0%s", prefix, path)
}

func mailchimpRequest(method, path string, body any) ([]byte, int, error) {
	apiKey := os.Getenv("MAILCHIMP_API_KEY")
	if apiKey == "" {
		return nil, 0, fmt.Errorf("MAILCHIMP_API_KEY not set")
	}

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, mailchimpAPIURL(path), bodyReader)
	if err != nil {
		return nil, 0, err
	}
	req.SetBasicAuth("anystring", apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("mailchimp request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	return respBody, resp.StatusCode, nil
}

// mailchimpSubscriberHash returns the MD5 hash of a lowercase email (Mailchimp's subscriber ID)
func mailchimpSubscriberHash(email string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(strings.ToLower(strings.TrimSpace(email)))))
}

// --- Handlers ---

// handleMailchimpSync pushes all active contacts to Mailchimp
func handleMailchimpSync(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	listID := os.Getenv("MAILCHIMP_LIST_ID")
	if listID == "" {
		return utils.BadRequestResponse(re, "MAILCHIMP_LIST_ID not configured")
	}

	utils.LogFromRequest(app, re, "mailchimp_sync", "mailchimp", "", "success", nil, "")

	// Run in background
	go runMailchimpSync(app, listID)

	return re.JSON(http.StatusAccepted, map[string]string{"message": "Sync started"})
}

// handleMailchimpSyncContact pushes a single contact to Mailchimp
func handleMailchimpSyncContact(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	contactID := re.Request.PathValue("id")
	if contactID == "" {
		return utils.BadRequestResponse(re, "Contact ID required")
	}

	listID := os.Getenv("MAILCHIMP_LIST_ID")
	if listID == "" {
		return utils.BadRequestResponse(re, "MAILCHIMP_LIST_ID not configured")
	}

	record, err := app.FindRecordById(utils.CollectionContacts, contactID)
	if err != nil {
		return utils.NotFoundResponse(re, "Contact not found")
	}

	email := utils.DecryptField(record.GetString("email"))
	if email == "" {
		return utils.BadRequestResponse(re, "Contact has no email")
	}

	result, err := upsertMailchimpSubscriber(listID, record, email)
	if err != nil {
		return utils.InternalErrorResponse(re, fmt.Sprintf("Mailchimp sync failed: %v", err))
	}

	// Update contact with Mailchimp ID and status
	if mcID, ok := result["id"].(string); ok && mcID != "" {
		record.Set("mailchimp_id", mcID)
	}
	if status, ok := result["status"].(string); ok && status != "" {
		record.Set("mailchimp_status", status)
	}
	// Decrypt email before save so encryption hooks re-encrypt
	record.Set("email", email)
	app.Save(record)

	utils.LogFromRequest(app, re, "mailchimp_sync_contact", "mailchimp", contactID, "success", nil, "")

	return utils.SuccessResponse(re, "Contact synced to Mailchimp")
}

// handleMailchimpWebhook receives engagement and subscription change events from Mailchimp
func handleMailchimpWebhook(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	// Mailchimp sends a GET request to validate the webhook URL — just return 200
	if re.Request.Method == "GET" {
		return re.String(http.StatusOK, "ok")
	}

	// Validate webhook secret
	secret := os.Getenv("MAILCHIMP_WEBHOOK_SECRET")
	if secret != "" {
		querySecret := re.Request.URL.Query().Get("secret")
		if querySecret != secret {
			return re.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid secret"})
		}
	}

	// Parse form data (Mailchimp sends webhooks as form-encoded, not JSON)
	if err := re.Request.ParseForm(); err != nil {
		return utils.BadRequestResponse(re, "Invalid form data")
	}

	webhookType := re.Request.FormValue("type")
	email := re.Request.FormValue("data[email]")

	if email == "" {
		return utils.BadRequestResponse(re, "No email in webhook")
	}

	// Find the contact by email blind index
	blindIndex := utils.BlindIndex(strings.ToLower(strings.TrimSpace(email)))
	contacts, _ := app.FindRecordsByFilter(
		utils.CollectionContacts,
		"email_index = {:idx}",
		"", 1, 0,
		map[string]any{"idx": blindIndex},
	)

	if len(contacts) == 0 {
		// Contact not in CRM — ignore
		return re.JSON(http.StatusOK, map[string]string{"status": "ignored"})
	}

	contact := contacts[0]

	switch webhookType {
	case "subscribe":
		contact.Set("mailchimp_status", "subscribed")
		contact.Set("email", utils.DecryptField(contact.GetString("email")))
		app.Save(contact)

	case "unsubscribe":
		contact.Set("mailchimp_status", "unsubscribed")
		contact.Set("email", utils.DecryptField(contact.GetString("email")))
		app.Save(contact)

	case "cleaned":
		contact.Set("mailchimp_status", "cleaned")
		contact.Set("email", utils.DecryptField(contact.GetString("email")))
		app.Save(contact)

	case "campaign":
		// Campaign send/open/click — create activity
		action := re.Request.FormValue("data[action]")
		campaignID := re.Request.FormValue("data[id]")
		subject := re.Request.FormValue("data[subject]")

		activityType := "email_sent"
		title := "Email sent"
		if action == "open" {
			activityType = "email_opened"
			title = "Opened email"
		} else if action == "click" {
			activityType = "email_clicked"
			title = "Clicked email link"
		}

		if subject != "" {
			title = title + ": " + subject
		}

		activitiesCollection, err := app.FindCollectionByNameOrId(utils.CollectionActivities)
		if err != nil {
			break
		}

		activity := core.NewRecord(activitiesCollection)
		activity.Set("contact", contact.Id)
		activity.Set("type", activityType)
		activity.Set("title", title)
		activity.Set("source_app", "mailchimp")
		activity.Set("source_id", campaignID)
		activity.Set("metadata", map[string]any{
			"campaign_id": campaignID,
			"action":      action,
			"subject":     subject,
		})
		activity.Set("occurred_at", time.Now().UTC().Format(time.RFC3339))
		app.Save(activity)
	}

	utils.LogFromRequest(app, re, "mailchimp_webhook", "mailchimp", contact.Id, "success",
		map[string]any{"type": webhookType}, "")

	return re.JSON(http.StatusOK, map[string]string{"status": "processed"})
}

// --- Internal helpers ---

func runMailchimpSync(app *pocketbase.PocketBase, listID string) {
	records, err := app.FindRecordsByFilter(
		utils.CollectionContacts,
		"status = 'active'",
		"", 0, 0, nil,
	)
	if err != nil {
		log.Printf("[Mailchimp] Failed to fetch contacts: %v", err)
		return
	}

	log.Printf("[Mailchimp] Syncing %d active contacts to list %s", len(records), listID)

	synced, errors := 0, 0
	for _, record := range records {
		email := utils.DecryptField(record.GetString("email"))
		if email == "" {
			continue
		}

		result, err := upsertMailchimpSubscriber(listID, record, email)
		if err != nil {
			log.Printf("[Mailchimp] Failed to sync contact %s: %v", record.Id, err)
			errors++
			continue
		}

		// Update contact with Mailchimp data
		if mcID, ok := result["id"].(string); ok && mcID != "" {
			record.Set("mailchimp_id", mcID)
		}
		if status, ok := result["status"].(string); ok && status != "" {
			record.Set("mailchimp_status", status)
		}
		record.Set("email", email)
		app.Save(record)
		synced++
	}

	log.Printf("[Mailchimp] Sync complete: %d synced, %d errors", synced, errors)
}

func upsertMailchimpSubscriber(listID string, record *core.Record, email string) (map[string]any, error) {
	subscriberHash := mailchimpSubscriberHash(email)

	firstName := record.GetString("first_name")
	lastName := record.GetString("last_name")

	payload := map[string]any{
		"email_address": email,
		"status_if_new": "subscribed",
		"merge_fields": map[string]string{
			"FNAME": firstName,
			"LNAME": lastName,
		},
	}

	body, statusCode, err := mailchimpRequest(
		"PUT",
		fmt.Sprintf("/lists/%s/members/%s", listID, subscriberHash),
		payload,
	)
	if err != nil {
		return nil, err
	}

	if statusCode >= 400 {
		return nil, fmt.Errorf("mailchimp returned %d: %s", statusCode, string(body))
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result, nil
}
