package main

import (
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
	ID        string `json:"_id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
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

	utils.LogFromRequest(app, re, "humanitix_sync", "humanitix", input.EventID, "success", nil, "")

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

// runHumanitixSync performs the actual sync operation
func runHumanitixSync(app *pocketbase.PocketBase, syncLogID, eventID string, fieldMapping map[string]string) {
	var syncErrors []string
	created, updated, processed := 0, 0, 0

	defer func() {
		// Update sync log with results
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
	eventBody, err := humanitixGet(fmt.Sprintf("/events/%s?page=1", eventID))
	if err == nil {
		var eventResp struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(eventBody, &eventResp) == nil && eventResp.Name != "" {
			if syncLog, err := app.FindRecordById("humanitix_sync_log", syncLogID); err == nil {
				syncLog.Set("event_name", eventResp.Name)
				app.Save(syncLog)
			}
		}
	}

	// Fetch all orders for this event (paginated)
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

		if page*resp.PageSize >= resp.Total {
			break
		}
		page++
	}

	// Fetch all tickets for this event (paginated)
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

		if page*resp.PageSize >= resp.Total {
			break
		}
		page++
	}

	log.Printf("[Humanitix] Syncing %d tickets from %d orders for event %s", len(allTickets), len(orderMap), eventID)

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

	for _, ticket := range allTickets {
		processed++

		if ticket.Status != "complete" {
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
			syncErrors = append(syncErrors, fmt.Sprintf("Ticket %s has no email", ticket.ID))
			continue
		}

		// Get other fields from additional fields or ticket-level data
		firstName := ticket.FirstName
		lastName := ticket.LastName
		phone := ""
		jobTitle := ""
		organisation := ticket.Organisation

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
					organisation = v
				}
			}
		}

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

			// Decrypt email before save so encryption hooks can re-encrypt
			record.Set("email", utils.DecryptField(record.GetString("email")))

			if err := app.Save(record); err != nil {
				syncErrors = append(syncErrors, fmt.Sprintf("Failed to update contact %s: %v", contactID, err))
				continue
			}
			updated++
		} else {
			// Create new contact
			record := core.NewRecord(contactsCollection)
			record.Set("email", email)
			record.Set("first_name", firstName)
			record.Set("last_name", lastName)
			if phone != "" {
				record.Set("phone", phone)
			}
			if jobTitle != "" {
				record.Set("job_title", jobTitle)
			}
			record.Set("status", "pending")
			record.Set("source", "humanitix")
			record.Set("source_ids", map[string]any{"humanitix": ticket.ID})
			record.Set("humanitix_attendee_id", ticket.ID)
			record.Set("humanitix_order_id", ticket.OrderID)
			record.Set("roles", []string{"attendee"})

			if err := app.Save(record); err != nil {
				syncErrors = append(syncErrors, fmt.Sprintf("Failed to create contact for %s: %v", email, err))
				continue
			}
			contactID = record.Id
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
				"organisation":     organisation,
				"event_id":         eventID,
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

	log.Printf("[Humanitix] Sync complete: %d processed, %d created, %d updated, %d errors",
		processed, created, updated, len(syncErrors))
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
