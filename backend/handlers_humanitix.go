package main

import (
	"encoding/csv"
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

const humanitixBaseURL = "https://api.humanitix.com/v1"

// --- Humanitix API response types ---

type humanitixEventsResponse struct {
	Total    int              `json:"total"`
	PageSize int              `json:"pageSize"`
	Page     int              `json:"page"`
	Events   []humanitixEvent `json:"events"`
}

type humanitixEvent struct {
	ID        string          `json:"_id"`
	Name      string          `json:"name"`
	Slug      string          `json:"slug"`
	StartDate string          `json:"startDate"`
	EndDate   string          `json:"endDate"`
	Location  *humanitixLocation `json:"location,omitempty"`
	Venue     string          `json:"venue,omitempty"`
	Address   string          `json:"address,omitempty"`
}

type humanitixLocation struct {
	Address    string  `json:"address,omitempty"`
	City       string  `json:"city,omitempty"`
	State      string  `json:"state,omitempty"`
	Country    string  `json:"country,omitempty"`
	PostalCode string  `json:"postalCode,omitempty"`
	Lat        float64 `json:"lat,omitempty"`
	Lng        float64 `json:"lng,omitempty"`
}

// getEventCity extracts a city string from a Humanitix event
func (e *humanitixEvent) getEventCity() string {
	if e.Location != nil && e.Location.City != "" {
		return e.Location.City
	}
	return ""
}

type humanitixOrdersResponse struct {
	Total    int               `json:"total"`
	PageSize int               `json:"pageSize"`
	Page     int               `json:"page"`
	Orders   []humanitixOrder  `json:"orders"`
}

type humanitixOrder struct {
	ID        string `json:"_id"`
	OrderName string `json:"orderName"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Mobile    string `json:"mobile"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

type humanitixTicketsResponse struct {
	Total    int                `json:"total"`
	PageSize int                `json:"pageSize"`
	Page     int                `json:"page"`
	Tickets  []humanitixTicket  `json:"tickets"`
}

type humanitixTicket struct {
	ID              string                     `json:"_id"`
	FirstName       string                     `json:"firstName"`
	LastName        string                     `json:"lastName"`
	Organisation    string                     `json:"organisation"`
	TicketTypeName  string                     `json:"ticketTypeName"`
	TicketTypeID    string                     `json:"ticketTypeId"`
	OrderID         string                     `json:"orderId"`
	OrderName       string                     `json:"orderName"`
	Status          string                     `json:"status"`
	Price           float64                    `json:"price"`
	AdditionalFields []humanitixAdditionalField `json:"additionalFields"`
	CreatedAt       string                     `json:"createdAt"`
}

type humanitixAdditionalField struct {
	QuestionID string `json:"questionId"`
	Value      string `json:"value"`
}

// getAdditionalField returns the value for a given question ID from additional fields
func (t *humanitixTicket) getAdditionalField(questionID string) string {
	for _, f := range t.AdditionalFields {
		if f.QuestionID == questionID {
			return strings.TrimSpace(f.Value)
		}
	}
	return ""
}

// --- Humanitix API client ---

func humanitixGet(path string) ([]byte, error) {
	apiKey := os.Getenv("HUMANITIX_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("HUMANITIX_API_KEY not set")
	}

	req, err := http.NewRequest("GET", humanitixBaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("humanitix API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("humanitix API returned %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// --- Handlers ---

// handleHumanitixEventsList returns events from Humanitix for the sync UI
func handleHumanitixEventsList(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	body, err := humanitixGet("/events?page=1")
	if err != nil {
		log.Printf("[Humanitix] Failed to fetch events: %v", err)
		return utils.InternalErrorResponse(re, "Failed to fetch events from Humanitix")
	}

	var resp humanitixEventsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return utils.InternalErrorResponse(re, "Failed to parse Humanitix response")
	}

	// Return a simplified list
	events := make([]map[string]any, 0, len(resp.Events))
	for _, e := range resp.Events {
		events = append(events, map[string]any{
			"id":         e.ID,
			"name":       e.Name,
			"slug":       e.Slug,
			"start_date": e.StartDate,
			"end_date":   e.EndDate,
		})
	}

	return utils.DataResponse(re, events)
}

// handleHumanitixSync triggers a sync for a specific event
func handleHumanitixSync(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	var input struct {
		EventID string `json:"event_id"`
		// Optional: map Humanitix additional field question IDs to CRM fields
		// If not provided, uses email/phone from the order-level data
		FieldMapping map[string]string `json:"field_mapping"`
	}
	if err := json.NewDecoder(re.Request.Body).Decode(&input); err != nil {
		return utils.BadRequestResponse(re, "Invalid request body")
	}

	if input.EventID == "" {
		return utils.BadRequestResponse(re, "Event ID required")
	}

	utils.LogFromRequest(app, re, "api_call", "humanitix", input.EventID, "success", nil, "")

	// Create sync log entry
	syncLogCollection, err := app.FindCollectionByNameOrId("humanitix_sync_log")
	if err != nil {
		return utils.InternalErrorResponse(re, "Sync log collection not found")
	}
	syncLog := core.NewRecord(syncLogCollection)
	syncLog.Set("sync_type", "manual")
	syncLog.Set("event_id", input.EventID)
	syncLog.Set("status", "running")
	syncLog.Set("started_at", time.Now().UTC().Format(time.RFC3339))
	if err := app.Save(syncLog); err != nil {
		return utils.InternalErrorResponse(re, "Failed to create sync log")
	}

	// Run sync in background
	go runHumanitixSync(app, syncLog.Id, input.EventID, input.FieldMapping)

	return re.JSON(http.StatusAccepted, map[string]any{
		"sync_log_id": syncLog.Id,
		"message":     "Sync started",
	})
}

// handleHumanitixSyncAll triggers a sync for all Humanitix events
func handleHumanitixSyncAll(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	var input struct {
		FieldMapping map[string]string `json:"field_mapping"`
	}
	json.NewDecoder(re.Request.Body).Decode(&input)

	utils.LogFromRequest(app, re, "api_call", "humanitix", "sync_all", "success", nil, "")

	// Fetch all events (paginated)
	var allEvents []humanitixEvent
	page := 1
	for {
		body, err := humanitixGet(fmt.Sprintf("/events?page=%d", page))
		if err != nil {
			log.Printf("[Humanitix] Failed to fetch events page %d: %v", page, err)
			return utils.InternalErrorResponse(re, "Failed to fetch events from Humanitix")
		}

		var resp humanitixEventsResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return utils.InternalErrorResponse(re, "Failed to parse Humanitix response")
		}

		allEvents = append(allEvents, resp.Events...)

		if page*resp.PageSize >= resp.Total {
			break
		}
		page++
	}

	if len(allEvents) == 0 {
		return utils.BadRequestResponse(re, "No events found on Humanitix")
	}

	syncLogCollection, err := app.FindCollectionByNameOrId("humanitix_sync_log")
	if err != nil {
		return utils.InternalErrorResponse(re, "Sync log collection not found")
	}

	// Create one sync log per event and run in background
	var syncLogIDs []string
	for _, event := range allEvents {
		syncLog := core.NewRecord(syncLogCollection)
		syncLog.Set("sync_type", "manual")
		syncLog.Set("event_id", event.ID)
		syncLog.Set("event_name", event.Name)
		syncLog.Set("status", "running")
		syncLog.Set("started_at", time.Now().UTC().Format(time.RFC3339))
		if err := app.Save(syncLog); err != nil {
			log.Printf("[Humanitix] Failed to create sync log for event %s: %v", event.ID, err)
			continue
		}
		syncLogIDs = append(syncLogIDs, syncLog.Id)
		go runHumanitixSync(app, syncLog.Id, event.ID, input.FieldMapping)
	}

	return re.JSON(http.StatusAccepted, map[string]any{
		"sync_log_ids": syncLogIDs,
		"events_count": len(allEvents),
		"message":      fmt.Sprintf("Syncing %d events", len(allEvents)),
	})
}

// runHumanitixSync performs the actual sync operation
func runHumanitixSync(app *pocketbase.PocketBase, syncLogID, eventID string, fieldMapping map[string]string) {
	var syncErrors []string
	created, updated, skipped, orgsCreated, processed := 0, 0, 0, 0, 0

	log.Printf("[Humanitix] === Starting sync for event %s (syncLog: %s) ===", eventID, syncLogID)

	if fieldMapping != nil {
		log.Printf("[Humanitix] Field mapping: %v", fieldMapping)
	} else {
		log.Printf("[Humanitix] No field mapping provided — will use ticket-level + order-level data only")
	}

	defer func() {
		log.Printf("[Humanitix] === Sync finished for event %s: %d processed, %d created, %d updated, %d skipped, %d orgs created, %d errors ===",
			eventID, processed, created, updated, skipped, orgsCreated, len(syncErrors))
		if len(syncErrors) > 0 {
			for i, e := range syncErrors {
				log.Printf("[Humanitix]   error[%d]: %s", i, e)
			}
		}

		if syncLogID == "" {
			return
		}
		syncLog, err := app.FindRecordById("humanitix_sync_log", syncLogID)
		if err != nil {
			log.Printf("[Humanitix] Failed to find sync log %s: %v", syncLogID, err)
			return
		}
		syncLog.Set("records_processed", processed)
		syncLog.Set("records_created", created)
		syncLog.Set("records_updated", updated)
		syncLog.Set("completed_at", time.Now().UTC().Format(time.RFC3339))
		if len(syncErrors) > 0 {
			syncLog.Set("errors", syncErrors)
			syncLog.Set("status", "failed")
		} else {
			syncLog.Set("status", "completed")
		}
		if err := app.Save(syncLog); err != nil {
			log.Printf("[Humanitix] Failed to update sync log: %v", err)
		}
	}()

	// Fetch event info for the sync log
	var eventName string
	var eventCity string
	log.Printf("[Humanitix] Fetching event info for %s...", eventID)
	eventBody, err := humanitixGet(fmt.Sprintf("/events/%s", eventID))
	if err != nil {
		log.Printf("[Humanitix] Failed to fetch event info: %v", err)
	} else {
		// Log raw event JSON keys to discover available fields
		var rawEvent map[string]json.RawMessage
		if json.Unmarshal(eventBody, &rawEvent) == nil {
			keys := make([]string, 0, len(rawEvent))
			for k := range rawEvent {
				keys = append(keys, k)
			}
			log.Printf("[Humanitix] Event response keys: %v", keys)
			// Log location-related fields specifically
			for _, key := range []string{"location", "venue", "address", "city", "locationAddress", "physicalAddress", "eventLocation"} {
				if v, ok := rawEvent[key]; ok {
					log.Printf("[Humanitix] Event %s = %s", key, string(v))
				}
			}
		}

		var event humanitixEvent
		if json.Unmarshal(eventBody, &event) == nil && event.Name != "" {
			eventName = event.Name
			eventCity = event.getEventCity()
			log.Printf("[Humanitix] Event name: %s", eventName)
			if eventCity != "" {
				log.Printf("[Humanitix] Event city: %s", eventCity)
			} else {
				log.Printf("[Humanitix] Event city: not found (location: %+v, venue: %q, address: %q)", event.Location, event.Venue, event.Address)
			}
			if syncLogID != "" {
				if syncLog, err := app.FindRecordById("humanitix_sync_log", syncLogID); err == nil {
					syncLog.Set("event_name", event.Name)
					app.Save(syncLog)
				}
			}
		}
	}

	// Fetch all orders for this event (paginated)
	log.Printf("[Humanitix] Fetching orders...")
	orderMap := make(map[string]humanitixOrder)
	page := 1
	for {
		body, err := humanitixGet(fmt.Sprintf("/events/%s/orders?page=%d", eventID, page))
		if err != nil {
			syncErrors = append(syncErrors, fmt.Sprintf("Failed to fetch orders page %d: %v", page, err))
			return
		}

		var resp humanitixOrdersResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			syncErrors = append(syncErrors, fmt.Sprintf("Failed to parse orders page %d", page))
			return
		}

		for _, order := range resp.Orders {
			orderMap[order.ID] = order
		}

		log.Printf("[Humanitix] Orders page %d: %d orders (total: %d)", page, len(resp.Orders), resp.Total)

		if page*resp.PageSize >= resp.Total {
			break
		}
		page++
	}

	// Fetch all tickets for this event (paginated)
	log.Printf("[Humanitix] Fetching tickets...")
	var allTickets []humanitixTicket
	page = 1
	for {
		body, err := humanitixGet(fmt.Sprintf("/events/%s/tickets?page=%d", eventID, page))
		if err != nil {
			syncErrors = append(syncErrors, fmt.Sprintf("Failed to fetch tickets page %d: %v", page, err))
			return
		}

		var resp humanitixTicketsResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			syncErrors = append(syncErrors, fmt.Sprintf("Failed to parse tickets page %d", page))
			return
		}

		allTickets = append(allTickets, resp.Tickets...)

		log.Printf("[Humanitix] Tickets page %d: %d tickets (total: %d)", page, len(resp.Tickets), resp.Total)

		if page*resp.PageSize >= resp.Total {
			break
		}
		page++
	}

	log.Printf("[Humanitix] Ready to process %d tickets from %d orders", len(allTickets), len(orderMap))

	// Log first ticket's additional fields to help debug field mapping
	if len(allTickets) > 0 {
		t := allTickets[0]
		log.Printf("[Humanitix] Sample ticket: id=%s name=%q %q org=%q status=%s additionalFields=%d",
			t.ID, t.FirstName, t.LastName, t.Organisation, t.Status, len(t.AdditionalFields))
		for _, f := range t.AdditionalFields {
			log.Printf("[Humanitix]   field %s = %q", f.QuestionID, f.Value)
		}
		if order, ok := orderMap[t.OrderID]; ok {
			log.Printf("[Humanitix] Sample order: id=%s email=%q name=%q %q", order.ID, order.Email, order.FirstName, order.LastName)
		}
	}

	contactsCollection, err := app.FindCollectionByNameOrId(utils.CollectionContacts)
	if err != nil {
		syncErrors = append(syncErrors, "Failed to find contacts collection")
		return
	}

	activitiesCollection, err := app.FindCollectionByNameOrId(utils.CollectionActivities)
	if err != nil {
		syncErrors = append(syncErrors, "Failed to find activities collection")
		return
	}

	orgsCollection, err := app.FindCollectionByNameOrId(utils.CollectionOrganisations)
	if err != nil {
		syncErrors = append(syncErrors, "Failed to find organisations collection")
		return
	}

	// Pre-load org cache
	orgCache := map[string]string{}
	allOrgs, err := app.FindAllRecords(utils.CollectionOrganisations)
	if err == nil {
		for _, org := range allOrgs {
			orgCache[strings.ToLower(org.GetString("name"))] = org.Id
		}
	}
	log.Printf("[Humanitix] Loaded %d existing organisations into cache", len(orgCache))

	for _, ticket := range allTickets {
		processed++

		if ticket.Status != "complete" {
			log.Printf("[Humanitix] [%d/%d] Ticket %s: status=%q — skipping (not complete)", processed, len(allTickets), ticket.ID, ticket.Status)
			skipped++
			continue
		}

		// Get email — try ticket additional fields first (per-attendee), fall back to order (buyer)
		email := ""
		if fieldMapping != nil {
			if emailQID, ok := fieldMapping["email"]; ok {
				email = ticket.getAdditionalField(emailQID)
			}
		}
		if email == "" {
			// Fall back to order-level email
			if order, ok := orderMap[ticket.OrderID]; ok {
				email = order.Email
			}
		}

		email = strings.ToLower(strings.TrimSpace(email))
		if email == "" {
			log.Printf("[Humanitix] [%d/%d] Ticket %s (%s %s): no email — skipping", processed, len(allTickets), ticket.ID, ticket.FirstName, ticket.LastName)
			skipped++
			continue
		}

		// Get other fields from additional fields or ticket-level data
		firstName := ticket.FirstName
		lastName := ticket.LastName
		phone := ""
		jobTitle := ""
		company := ticket.Organisation
		dietaryRaw := ""
		accessibilityRaw := ""

		if fieldMapping != nil {
			if qid, ok := fieldMapping["first_name"]; ok {
				if v := ticket.getAdditionalField(qid); v != "" {
					firstName = v
				}
			}
			if qid, ok := fieldMapping["last_name"]; ok {
				if v := ticket.getAdditionalField(qid); v != "" {
					lastName = v
				}
			}
			if qid, ok := fieldMapping["phone"]; ok {
				phone = ticket.getAdditionalField(qid)
			}
			if qid, ok := fieldMapping["job_title"]; ok {
				jobTitle = ticket.getAdditionalField(qid)
			}
			if qid, ok := fieldMapping["organisation"]; ok {
				if v := ticket.getAdditionalField(qid); v != "" {
					company = v
				}
			}
			if qid, ok := fieldMapping["dietary_requirements"]; ok {
				dietaryRaw = ticket.getAdditionalField(qid)
			}
			if qid, ok := fieldMapping["accessibility_requirements"]; ok {
				accessibilityRaw = ticket.getAdditionalField(qid)
			}
		}

		if firstName == "" {
			log.Printf("[Humanitix] [%d/%d] Ticket %s: no first name — skipping", processed, len(allTickets), ticket.ID)
			skipped++
			continue
		}

		// Resolve organisation
		var orgID string
		if company != "" {
			companyKey := strings.ToLower(company)
			if id, ok := orgCache[companyKey]; ok {
				orgID = id
			} else {
				orgRecord := core.NewRecord(orgsCollection)
				orgRecord.Set("name", company)
				orgRecord.Set("status", "active")
				orgRecord.Set("source", "humanitix")
				if website := guessWebsiteFromEmail(email); website != "" {
					orgRecord.Set("website", website)
					if logoURL := guessLogoURL(website); logoURL != "" {
						orgRecord.Set("logo_square_url", logoURL)
					}
				}
				if err := app.Save(orgRecord); err != nil {
					log.Printf("[Humanitix] Failed to create org %q: %v", company, err)
				} else {
					orgID = orgRecord.Id
					orgCache[companyKey] = orgID
					orgsCreated++
					log.Printf("[Humanitix] Created org: %s (ID: %s)", company, orgID)
				}
			}
		}

		// Parse dietary requirements
		dietaryKnown, dietaryOther := parseDietaryRequirements(dietaryRaw)

		// Look up existing contact by email blind index
		blindIndex := utils.BlindIndex(email)
		existingRecords, _ := app.FindRecordsByFilter(
			utils.CollectionContacts,
			"email_index = {:idx}",
			"", 1, 0,
			map[string]any{"idx": blindIndex},
		)

		var contactID string

		if len(existingRecords) > 0 {
			// Update existing contact
			record := existingRecords[0]
			contactID = record.Id

			// Update source_ids to include humanitix
			sourceIDs := map[string]any{}
			if existing := record.Get("source_ids"); existing != nil {
				if m, ok := existing.(map[string]any); ok {
					sourceIDs = m
				}
			}
			sourceIDs["humanitix"] = ticket.ID
			record.Set("source_ids", sourceIDs)
			record.Set("humanitix_attendee_id", ticket.ID)
			record.Set("humanitix_order_id", ticket.OrderID)

			// Add "attendee" role if not present
			roles := record.Get("roles")
			if rolesSlice, ok := roles.([]any); ok {
				hasAttendee := false
				for _, r := range rolesSlice {
					if r == "attendee" {
						hasAttendee = true
						break
					}
				}
				if !hasAttendee {
					rolesSlice = append(rolesSlice, "attendee")
					record.Set("roles", rolesSlice)
				}
			}

			// Link org if not already set
			if orgID != "" && record.GetString("organisation") == "" {
				record.Set("organisation", orgID)
			}

			// Decrypt all PII fields before save so encryption hooks can re-encrypt
			// Without this, already-encrypted values get double-encrypted and exceed field max length
			for _, piiField := range []string{"email", "personal_email", "phone", "bio", "location"} {
				if v := record.GetString(piiField); v != "" {
					record.Set(piiField, utils.DecryptField(v))
				}
			}

			if err := app.Save(record); err != nil {
				log.Printf("[Humanitix] [%d/%d] FAILED to update %s %s (%s, ID: %s): %v", processed, len(allTickets), firstName, lastName, email, contactID, err)
				syncErrors = append(syncErrors, fmt.Sprintf("Failed to update contact %s (%s): %v", contactID, email, err))
				continue
			}
			log.Printf("[Humanitix] [%d/%d] Updated: %s %s (%s, ID: %s)", processed, len(allTickets), firstName, lastName, email, contactID)
			updated++
		} else {
			// Create new contact
			record := core.NewRecord(contactsCollection)
			record.Set("email", email)
			record.Set("first_name", firstName)
			record.Set("last_name", lastName)
			record.Set("name", strings.TrimSpace(firstName+" "+lastName))
			if phone != "" {
				record.Set("phone", phone)
			}
			if jobTitle != "" {
				record.Set("job_title", jobTitle)
			}
			if orgID != "" {
				record.Set("organisation", orgID)
			}
			if eventCity != "" {
				record.Set("location", eventCity)
			}
			record.Set("status", "pending")
			record.Set("source", "humanitix")
			record.Set("source_ids", map[string]any{"humanitix": ticket.ID})
			record.Set("humanitix_attendee_id", ticket.ID)
			record.Set("humanitix_order_id", ticket.OrderID)
			record.Set("roles", []string{"attendee"})

			if len(dietaryKnown) > 0 {
				record.Set("dietary_requirements", dietaryKnown)
			}
			if dietaryOther != "" {
				record.Set("dietary_requirements_other", dietaryOther)
			}
			if accessibilityRaw != "" && !strings.EqualFold(accessibilityRaw, "none") && accessibilityRaw != "-" && !strings.EqualFold(accessibilityRaw, "n/a") && !strings.EqualFold(accessibilityRaw, "no") {
				record.Set("accessibility_requirements_other", accessibilityRaw)
			}

			if err := app.Save(record); err != nil {
				log.Printf("[Humanitix] [%d/%d] FAILED to create %s %s (%s): %v", processed, len(allTickets), firstName, lastName, email, err)
				syncErrors = append(syncErrors, fmt.Sprintf("Failed to create contact for %s %s (%s): %v", firstName, lastName, email, err))
				continue
			}
			contactID = record.Id
			log.Printf("[Humanitix] [%d/%d] Created: %s %s (%s, ID: %s, org: %s)", processed, len(allTickets), firstName, lastName, email, contactID, company)
			created++
		}

		// Create ticket_purchased activity (idempotent — check if already exists)
		existingActivities, _ := app.FindRecordsByFilter(
			utils.CollectionActivities,
			"contact = {:cid} && source_app = 'humanitix' && source_id = {:sid}",
			"", 1, 0,
			map[string]any{"cid": contactID, "sid": ticket.ID},
		)

		if len(existingActivities) == 0 {
			activity := core.NewRecord(activitiesCollection)
			activity.Set("contact", contactID)
			activity.Set("type", "ticket_purchased")
			activity.Set("title", fmt.Sprintf("Purchased %s ticket", ticket.TicketTypeName))
			activity.Set("source_app", "humanitix")
			activity.Set("source_id", ticket.ID)
			activity.Set("metadata", map[string]any{
				"order_name":       ticket.OrderName,
				"ticket_type":      ticket.TicketTypeName,
				"ticket_type_id":   ticket.TicketTypeID,
				"organisation":     company,
				"event_id":         eventID,
				"event_name":       eventName,
				"price":            ticket.Price,
			})
			if ticket.CreatedAt != "" {
				activity.Set("occurred_at", ticket.CreatedAt)
			}

			if err := app.Save(activity); err != nil {
				syncErrors = append(syncErrors, fmt.Sprintf("Failed to create activity for ticket %s: %v", ticket.ID, err))
			}
		}
	}
}

// handleHumanitixSyncLogs returns sync log entries
func handleHumanitixSyncLogs(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	records, err := app.FindRecordsByFilter(
		"humanitix_sync_log",
		"", "-created", 50, 0, nil,
	)
	if err != nil {
		return utils.DataResponse(re, []any{})
	}

	logs := make([]map[string]any, 0, len(records))
	for _, r := range records {
		logs = append(logs, map[string]any{
			"id":                r.Id,
			"sync_type":         r.GetString("sync_type"),
			"event_id":          r.GetString("event_id"),
			"event_name":        r.GetString("event_name"),
			"records_processed": r.GetInt("records_processed"),
			"records_created":   r.GetInt("records_created"),
			"records_updated":   r.GetInt("records_updated"),
			"errors":            r.Get("errors"),
			"status":            r.GetString("status"),
			"started_at":        r.GetString("started_at"),
			"completed_at":      r.GetString("completed_at"),
			"created":           r.GetString("created"),
		})
	}

	return utils.DataResponse(re, logs)
}

// --- CSV Import ---

// Free email providers — skip these when guessing company website from email domain
var freeEmailDomains = map[string]bool{
	"gmail.com": true, "googlemail.com": true, "hotmail.com": true, "outlook.com": true,
	"yahoo.com": true, "yahoo.com.au": true, "icloud.com": true, "live.com": true,
	"live.com.au": true, "me.com": true, "aol.com": true, "protonmail.com": true,
	"proton.me": true, "fastmail.com": true, "hey.com": true, "msn.com": true,
	"bigpond.com": true, "bigpond.net.au": true, "optusnet.com.au": true,
}

// parseDietaryRequirements maps Humanitix free-text dietary values to CRM multi-select values
func parseDietaryRequirements(raw string) ([]string, string) {
	if raw == "" || strings.EqualFold(raw, "none") || strings.EqualFold(raw, "n/a") || raw == "-" {
		return nil, ""
	}

	knownMap := map[string]string{
		"vegetarian":  "vegetarian",
		"vegan":       "vegan",
		"gluten free": "gluten_free",
		"dairy free":  "dairy_free",
		"nut allergy": "nut_allergy",
		"halal":       "halal",
		"kosher":      "kosher",
	}

	var known []string
	var other []string

	parts := strings.Split(raw, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || strings.EqualFold(part, "none") || strings.EqualFold(part, "n/a") {
			continue
		}
		if val, ok := knownMap[strings.ToLower(part)]; ok {
			known = append(known, val)
		} else {
			other = append(other, part)
		}
	}

	return known, strings.Join(other, ", ")
}

// guessWebsiteFromEmail extracts domain from email and returns a website URL, skipping free providers
func guessWebsiteFromEmail(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return ""
	}
	domain := strings.ToLower(parts[1])
	if freeEmailDomains[domain] {
		return ""
	}
	return "https://" + domain
}

// guessLogoURL returns a Clearbit logo URL for a domain, or empty string
func guessLogoURL(website string) string {
	if website == "" {
		return ""
	}
	// Strip protocol and path to get bare domain
	domain := strings.TrimPrefix(website, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.SplitN(domain, "/", 2)[0]
	if domain == "" {
		return ""
	}
	return "https://logo.clearbit.com/" + domain
}

// handleHumanitixCSVImport handles CSV file upload and imports attendees as contacts
func handleHumanitixCSVImport(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	// Parse multipart form (max 10MB)
	if err := re.Request.ParseMultipartForm(10 << 20); err != nil {
		return utils.BadRequestResponse(re, "Failed to parse form data")
	}

	file, _, err := re.Request.FormFile("file")
	if err != nil {
		return utils.BadRequestResponse(re, "CSV file is required")
	}
	defer file.Close()

	// Parse CSV
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // Allow variable field counts
	reader.LazyQuotes = true

	records, err := reader.ReadAll()
	if err != nil {
		return utils.BadRequestResponse(re, fmt.Sprintf("Failed to parse CSV: %v", err))
	}

	if len(records) < 2 {
		return utils.BadRequestResponse(re, "CSV must have a header row and at least one data row")
	}

	// Build column index from header
	header := records[0]
	colIdx := map[string]int{}
	for i, col := range header {
		colIdx[strings.TrimSpace(col)] = i
	}

	// Verify required columns exist
	requiredCols := []string{"Email", "First name", "Last name"}
	for _, col := range requiredCols {
		if _, ok := colIdx[col]; !ok {
			return utils.BadRequestResponse(re, fmt.Sprintf("Missing required column: %s", col))
		}
	}

	// Helper to safely get column value
	getCol := func(row []string, name string) string {
		idx, ok := colIdx[name]
		if !ok || idx >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[idx])
	}

	// Extract event name from first data row
	eventName := getCol(records[1], "Event")

	// Create sync log
	syncLogCollection, err := app.FindCollectionByNameOrId("humanitix_sync_log")
	if err != nil {
		return utils.InternalErrorResponse(re, "Sync log collection not found")
	}
	syncLog := core.NewRecord(syncLogCollection)
	syncLog.Set("sync_type", "csv_import")
	syncLog.Set("event_name", eventName)
	syncLog.Set("status", "running")
	syncLog.Set("started_at", time.Now().UTC().Format(time.RFC3339))
	if err := app.Save(syncLog); err != nil {
		return utils.InternalErrorResponse(re, "Failed to create sync log")
	}

	utils.LogFromRequest(app, re, "api_call", "humanitix", syncLog.Id, "success", nil, "")

	// Run import synchronously (CSV is already in memory, no API calls needed)
	result := runHumanitixCSVImport(app, syncLog.Id, records[1:], getCol)

	return re.JSON(http.StatusOK, result)
}

type csvImportResult struct {
	SyncLogID    string `json:"sync_log_id"`
	Processed    int    `json:"processed"`
	Created      int    `json:"created"`
	Updated      int    `json:"updated"`
	Skipped      int    `json:"skipped"`
	OrgsCreated  int    `json:"orgs_created"`
	Errors       int    `json:"errors"`
}

func runHumanitixCSVImport(app *pocketbase.PocketBase, syncLogID string, rows [][]string, getCol func([]string, string) string) csvImportResult {
	var syncErrors []string
	result := csvImportResult{SyncLogID: syncLogID}

	defer func() {
		// Update sync log with results
		syncLog, err := app.FindRecordById("humanitix_sync_log", syncLogID)
		if err != nil {
			log.Printf("[HumanitixCSV] Failed to find sync log %s: %v", syncLogID, err)
			return
		}
		syncLog.Set("records_processed", result.Processed)
		syncLog.Set("records_created", result.Created)
		syncLog.Set("records_updated", result.Updated)
		syncLog.Set("completed_at", time.Now().UTC().Format(time.RFC3339))
		if len(syncErrors) > 0 {
			syncLog.Set("errors", syncErrors)
			syncLog.Set("status", "failed")
		} else {
			syncLog.Set("status", "completed")
		}
		if err := app.Save(syncLog); err != nil {
			log.Printf("[HumanitixCSV] Failed to update sync log: %v", err)
		}
		result.Errors = len(syncErrors)
	}()

	contactsCollection, err := app.FindCollectionByNameOrId(utils.CollectionContacts)
	if err != nil {
		syncErrors = append(syncErrors, "Failed to find contacts collection")
		return result
	}

	activitiesCollection, err := app.FindCollectionByNameOrId(utils.CollectionActivities)
	if err != nil {
		syncErrors = append(syncErrors, "Failed to find activities collection")
		return result
	}

	orgsCollection, err := app.FindCollectionByNameOrId(utils.CollectionOrganisations)
	if err != nil {
		syncErrors = append(syncErrors, "Failed to find organisations collection")
		return result
	}

	// Pre-load all existing organisations into a cache (lowercased name → ID)
	orgCache := map[string]string{}
	allOrgs, err := app.FindAllRecords(utils.CollectionOrganisations)
	if err == nil {
		for _, org := range allOrgs {
			orgCache[strings.ToLower(org.GetString("name"))] = org.Id
		}
	}

	for _, row := range rows {
		result.Processed++

		email := strings.ToLower(strings.TrimSpace(getCol(row, "Email")))
		if email == "" {
			result.Skipped++
			continue
		}

		firstName := getCol(row, "First name")
		lastName := getCol(row, "Last name")
		if firstName == "" {
			result.Skipped++
			continue
		}

		phone := getCol(row, "Mobile")
		jobTitle := getCol(row, "Job title")
		company := getCol(row, "Company")
		orderID := getCol(row, "Order id")
		ticketNo := getCol(row, "Ticket no.")
		dietaryRaw := getCol(row, "Dietary requirements")
		accessibilityRaw := getCol(row, "Accessibility requirements")
		eventName := getCol(row, "Event")

		// Resolve organisation
		var orgID string
		if company != "" {
			companyKey := strings.ToLower(company)
			if id, ok := orgCache[companyKey]; ok {
				orgID = id
			} else {
				// Create new organisation
				orgRecord := core.NewRecord(orgsCollection)
				orgRecord.Set("name", company)
				orgRecord.Set("status", "active")
				orgRecord.Set("source", "humanitix")

				// Guess website from email domain
				if website := guessWebsiteFromEmail(email); website != "" {
					orgRecord.Set("website", website)
					if logoURL := guessLogoURL(website); logoURL != "" {
						orgRecord.Set("logo_square_url", logoURL)
					}
				}

				if err := app.Save(orgRecord); err != nil {
					syncErrors = append(syncErrors, fmt.Sprintf("Failed to create org %q: %v", company, err))
				} else {
					orgID = orgRecord.Id
					orgCache[companyKey] = orgID
					result.OrgsCreated++
					log.Printf("[HumanitixCSV] Created org: %s (ID: %s)", company, orgID)
				}
			}
		}

		// Parse dietary requirements
		dietaryKnown, dietaryOther := parseDietaryRequirements(dietaryRaw)

		// Look up existing contact by email blind index
		blindIndex := utils.BlindIndex(email)
		existingRecords, _ := app.FindRecordsByFilter(
			utils.CollectionContacts,
			"email_index = {:idx}",
			"", 1, 0,
			map[string]any{"idx": blindIndex},
		)

		sourceID := orderID
		if ticketNo != "" {
			sourceID = orderID + "-" + ticketNo
		}

		if len(existingRecords) > 0 {
			// Update existing contact
			record := existingRecords[0]

			// Update source_ids
			sourceIDs := map[string]any{}
			if existing := record.Get("source_ids"); existing != nil {
				if m, ok := existing.(map[string]any); ok {
					sourceIDs = m
				}
			}
			sourceIDs["humanitix_csv"] = sourceID
			record.Set("source_ids", sourceIDs)

			if orderID != "" {
				record.Set("humanitix_order_id", orderID)
			}

			// Add "attendee" role if not present
			roles := record.Get("roles")
			if rolesSlice, ok := roles.([]any); ok {
				hasAttendee := false
				for _, r := range rolesSlice {
					if r == "attendee" {
						hasAttendee = true
						break
					}
				}
				if !hasAttendee {
					rolesSlice = append(rolesSlice, "attendee")
					record.Set("roles", rolesSlice)
				}
			}

			// Link org if not already set
			if orgID != "" && record.GetString("organisation") == "" {
				record.Set("organisation", orgID)
			}

			// Decrypt all PII fields before save so encryption hooks can re-encrypt
			for _, piiField := range []string{"email", "personal_email", "phone", "bio", "location"} {
				if v := record.GetString(piiField); v != "" {
					record.Set(piiField, utils.DecryptField(v))
				}
			}

			if err := app.Save(record); err != nil {
				syncErrors = append(syncErrors, fmt.Sprintf("Failed to update contact %s: %v", record.Id, err))
				continue
			}
			result.Updated++
		} else {
			// Create new contact
			record := core.NewRecord(contactsCollection)
			record.Set("email", email)
			record.Set("first_name", firstName)
			record.Set("last_name", lastName)
			record.Set("name", strings.TrimSpace(firstName+" "+lastName))
			if phone != "" {
				record.Set("phone", phone)
			}
			if jobTitle != "" {
				record.Set("job_title", jobTitle)
			}
			if orgID != "" {
				record.Set("organisation", orgID)
			}
			record.Set("status", "pending")
			record.Set("source", "humanitix")
			record.Set("source_ids", map[string]any{"humanitix_csv": sourceID})
			record.Set("roles", []string{"attendee"})

			if orderID != "" {
				record.Set("humanitix_order_id", orderID)
			}

			if len(dietaryKnown) > 0 {
				record.Set("dietary_requirements", dietaryKnown)
			}
			if dietaryOther != "" {
				record.Set("dietary_requirements_other", dietaryOther)
			}
			if accessibilityRaw != "" && !strings.EqualFold(accessibilityRaw, "none") && accessibilityRaw != "-" && !strings.EqualFold(accessibilityRaw, "n/a") && !strings.EqualFold(accessibilityRaw, "no") {
				record.Set("accessibility_requirements_other", accessibilityRaw)
			}

			if err := app.Save(record); err != nil {
				syncErrors = append(syncErrors, fmt.Sprintf("Failed to create contact for %s: %v", email, err))
				continue
			}
			result.Created++

			// Create ticket_purchased activity (idempotent)
			if sourceID != "" {
				existingActivities, _ := app.FindRecordsByFilter(
					utils.CollectionActivities,
					"contact = {:cid} && source_app = 'humanitix' && source_id = {:sid}",
					"", 1, 0,
					map[string]any{"cid": record.Id, "sid": sourceID},
				)

				if len(existingActivities) == 0 {
					activity := core.NewRecord(activitiesCollection)
					activity.Set("contact", record.Id)
					activity.Set("type", "ticket_purchased")
					activity.Set("title", "Purchased ticket (CSV import)")
					activity.Set("source_app", "humanitix")
					activity.Set("source_id", sourceID)
					activity.Set("metadata", map[string]any{
						"order_id":      orderID,
						"ticket_no":     ticketNo,
						"organisation":  company,
						"event_name":    eventName,
						"import_method": "csv",
					})

					if err := app.Save(activity); err != nil {
						syncErrors = append(syncErrors, fmt.Sprintf("Failed to create activity for %s: %v", email, err))
					}
				}
			}
		}
	}

	log.Printf("[HumanitixCSV] Import complete: %d processed, %d created, %d updated, %d skipped, %d orgs created, %d errors",
		result.Processed, result.Created, result.Updated, result.Skipped, result.OrgsCreated, len(syncErrors))

	return result
}
