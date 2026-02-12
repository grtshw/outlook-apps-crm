package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/grtshw/outlook-apps-crm/utils"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// ============================================================================
// Public RSVP Endpoints (no auth)
// ============================================================================

// rsvpLookupResult holds the result of looking up an RSVP token
type rsvpLookupResult struct {
	Type      string       // "personal" or "generic"
	Item      *core.Record // guest_list_item (personal only)
	GuestList *core.Record // the guest list
}

// lookupRSVPToken checks guest_list_items.rsvp_token first, then guest_lists.rsvp_generic_token
func lookupRSVPToken(app *pocketbase.PocketBase, token string) (*rsvpLookupResult, error) {
	// Try personal token first
	items, err := app.FindRecordsByFilter(
		utils.CollectionGuestListItems,
		"rsvp_token = {:token}",
		"", 1, 0,
		map[string]any{"token": token},
	)
	if err == nil && len(items) > 0 {
		item := items[0]
		guestList, err := app.FindRecordById(utils.CollectionGuestLists, item.GetString("guest_list"))
		if err != nil {
			return nil, fmt.Errorf("guest list not found")
		}
		return &rsvpLookupResult{Type: "personal", Item: item, GuestList: guestList}, nil
	}

	// Try generic token
	lists, err := app.FindRecordsByFilter(
		utils.CollectionGuestLists,
		"rsvp_generic_token = {:token}",
		"", 1, 0,
		map[string]any{"token": token},
	)
	if err == nil && len(lists) > 0 {
		return &rsvpLookupResult{Type: "generic", GuestList: lists[0]}, nil
	}

	return nil, fmt.Errorf("RSVP link not found")
}

func handlePublicRSVPInfo(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	token := re.Request.PathValue("token")

	result, err := lookupRSVPToken(app, token)
	if err != nil {
		return utils.NotFoundResponse(re, "RSVP link not found")
	}

	if !result.GuestList.GetBool("rsvp_enabled") {
		return re.JSON(http.StatusGone, map[string]string{"error": "RSVP is no longer available for this event"})
	}

	// Get event name
	eventName := ""
	if epID := result.GuestList.GetString("event_projection"); epID != "" {
		if ep, err := app.FindRecordById(utils.CollectionEventProjections, epID); err == nil {
			eventName = ep.GetString("name")
		}
	}

	response := map[string]any{
		"type":       result.Type,
		"list_name":  result.GuestList.GetString("name"),
		"event_name": eventName,
	}

	if result.Type == "personal" && result.Item != nil {
		// Pre-fill from linked contact
		prefilledName := result.Item.GetString("contact_name")
		prefilledEmail := ""
		prefilledPhone := ""

		if contactID := result.Item.GetString("contact"); contactID != "" {
			if contact, err := app.FindRecordById(utils.CollectionContacts, contactID); err == nil {
				prefilledEmail = utils.DecryptField(contact.GetString("email"))
				prefilledPhone = utils.DecryptField(contact.GetString("phone"))
				prefilledName = contact.GetString("name")
			}
		}

		response["prefilled_name"] = prefilledName
		response["prefilled_email"] = prefilledEmail
		response["prefilled_phone"] = prefilledPhone

		// Include existing RSVP data if already responded
		rsvpStatus := result.Item.GetString("rsvp_status")
		response["already_responded"] = rsvpStatus != ""
		response["rsvp_status"] = rsvpStatus

		if rsvpStatus != "" {
			response["rsvp_dietary"] = result.Item.GetString("rsvp_dietary")
			response["rsvp_plus_one"] = result.Item.GetBool("rsvp_plus_one")
			response["rsvp_plus_one_name"] = result.Item.GetString("rsvp_plus_one_name")
			response["rsvp_plus_one_dietary"] = result.Item.GetString("rsvp_plus_one_dietary")
		}
	}

	return re.JSON(http.StatusOK, response)
}

func handlePublicRSVPSubmit(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	token := re.Request.PathValue("token")

	result, err := lookupRSVPToken(app, token)
	if err != nil {
		return utils.NotFoundResponse(re, "RSVP link not found")
	}

	if !result.GuestList.GetBool("rsvp_enabled") {
		return re.JSON(http.StatusGone, map[string]string{"error": "RSVP is no longer available for this event"})
	}

	var input struct {
		Name            string `json:"name"`
		Email           string `json:"email"`
		Phone           string `json:"phone"`
		Dietary         string `json:"dietary"`
		PlusOne         bool   `json:"plus_one"`
		PlusOneName     string `json:"plus_one_name"`
		PlusOneDietary  string `json:"plus_one_dietary"`
		Response        string `json:"response"`
		InvitedBy       string `json:"invited_by"`
	}
	if err := json.NewDecoder(re.Request.Body).Decode(&input); err != nil {
		return utils.BadRequestResponse(re, "Invalid JSON")
	}

	// Validate required fields
	input.Name = strings.TrimSpace(input.Name)
	input.Email = strings.TrimSpace(input.Email)
	if input.Name == "" {
		return utils.BadRequestResponse(re, "Name is required")
	}
	if input.Email == "" || !strings.Contains(input.Email, "@") {
		return utils.BadRequestResponse(re, "Valid email is required")
	}
	if input.Response != "accepted" && input.Response != "declined" {
		return utils.BadRequestResponse(re, "Response must be 'accepted' or 'declined'")
	}
	if len(input.Dietary) > 1000 {
		return utils.BadRequestResponse(re, "Dietary requirements must be 1000 characters or less")
	}
	if len(input.PlusOneDietary) > 1000 {
		return utils.BadRequestResponse(re, "Plus-one dietary requirements must be 1000 characters or less")
	}

	now := time.Now().UTC().Format(time.RFC3339)

	if result.Type == "personal" {
		return handlePersonalRSVP(re, app, result, input.Name, input.Email, input.Phone, input.Dietary, input.PlusOne, input.PlusOneName, input.PlusOneDietary, input.Response, now)
	}
	return handleGenericRSVP(re, app, result, input.Name, input.Email, input.Phone, input.Dietary, input.PlusOne, input.PlusOneName, input.PlusOneDietary, input.Response, input.InvitedBy, now)
}

func handlePersonalRSVP(re *core.RequestEvent, app *pocketbase.PocketBase, result *rsvpLookupResult, name, email, phone, dietary string, plusOne bool, plusOneName, plusOneDietary, response, now string) error {
	item := result.Item

	// Update RSVP fields on the item
	item.Set("rsvp_status", response)
	item.Set("rsvp_dietary", dietary)
	item.Set("rsvp_plus_one", plusOne)
	item.Set("rsvp_plus_one_name", plusOneName)
	item.Set("rsvp_plus_one_dietary", plusOneDietary)
	item.Set("rsvp_responded_at", now)
	item.Set("invite_status", response) // sync invite_status

	// Update denormalized name if changed
	if name != item.GetString("contact_name") {
		item.Set("contact_name", name)
	}

	if err := app.Save(item); err != nil {
		return utils.InternalErrorResponse(re, "Failed to save RSVP")
	}

	// Upsert the linked contact directly
	if contactID := item.GetString("contact"); contactID != "" {
		if contact, err := app.FindRecordById(utils.CollectionContacts, contactID); err == nil {
			// Decrypt current values for comparison, then set new values
			// The encryption hooks will re-encrypt on save
			currentEmail := utils.DecryptField(contact.GetString("email"))
			currentPhone := utils.DecryptField(contact.GetString("phone"))

			needsSave := false
			if name != "" && name != contact.GetString("name") {
				contact.Set("name", name)
				needsSave = true
			}
			if email != "" && !strings.EqualFold(email, currentEmail) {
				contact.Set("email", email)
				contact.Set("email_index", utils.BlindIndex(email))
				needsSave = true
			}
			if phone != "" && phone != currentPhone {
				contact.Set("phone", phone)
				needsSave = true
			}

			if needsSave {
				if err := app.Save(contact); err != nil {
					log.Printf("[RSVP] Failed to update contact %s: %v", contactID, err)
				}
			}
		}
	}

	utils.LogAudit(app, utils.AuditEntry{
		Action:       "rsvp_submit",
		ResourceType: utils.CollectionGuestListItems,
		ResourceID:   item.Id,
		IPAddress:    re.RealIP(),
		UserAgent:    re.Request.UserAgent(),
		Status:       "success",
		Metadata:     map[string]any{"type": "personal", "response": response},
	})

	return re.JSON(http.StatusOK, map[string]string{"message": "RSVP submitted successfully"})
}

func handleGenericRSVP(re *core.RequestEvent, app *pocketbase.PocketBase, result *rsvpLookupResult, name, email, phone, dietary string, plusOne bool, plusOneName, plusOneDietary, response, invitedBy, now string) error {
	listID := result.GuestList.Id

	// Check for existing contact by email using blind index
	blindIdx := utils.BlindIndex(email)
	var existingContact *core.Record

	if blindIdx != "" {
		contacts, err := app.FindRecordsByFilter(
			utils.CollectionContacts,
			"email_index = {:idx}",
			"", 1, 0,
			map[string]any{"idx": blindIdx},
		)
		if err == nil && len(contacts) > 0 {
			existingContact = contacts[0]
		}
	}

	if existingContact != nil {
		// Check if this contact is already on this guest list
		existingItems, err := app.FindRecordsByFilter(
			utils.CollectionGuestListItems,
			"guest_list = {:listId} && contact = {:contactId}",
			"", 1, 0,
			map[string]any{"listId": listID, "contactId": existingContact.Id},
		)
		if err == nil && len(existingItems) > 0 {
			// Update existing item with RSVP data
			item := existingItems[0]
			item.Set("rsvp_status", response)
			item.Set("rsvp_dietary", dietary)
			item.Set("rsvp_plus_one", plusOne)
			item.Set("rsvp_plus_one_name", plusOneName)
			item.Set("rsvp_plus_one_dietary", plusOneDietary)
			item.Set("rsvp_responded_at", now)
			item.Set("rsvp_invited_by", invitedBy)
			item.Set("invite_status", response)

			if err := app.Save(item); err != nil {
				return utils.InternalErrorResponse(re, "Failed to save RSVP")
			}

			utils.LogAudit(app, utils.AuditEntry{
				Action:       "rsvp_submit",
				ResourceType: utils.CollectionGuestListItems,
				ResourceID:   item.Id,
				IPAddress:    re.RealIP(),
				UserAgent:    re.Request.UserAgent(),
				Status:       "success",
				Metadata:     map[string]any{"type": "generic", "response": response, "matched_existing_item": true},
			})

			return re.JSON(http.StatusOK, map[string]string{"message": "RSVP submitted successfully"})
		}

		// Contact exists but not on this list — create new item linking to existing contact
		return createGuestListItemFromRSVP(re, app, listID, existingContact, dietary, plusOne, plusOneName, plusOneDietary, response, invitedBy, now)
	}

	// No matching contact — create new pending contact
	contactCollection, err := app.FindCollectionByNameOrId(utils.CollectionContacts)
	if err != nil {
		return utils.InternalErrorResponse(re, "Failed to find contacts collection")
	}

	newContact := core.NewRecord(contactCollection)
	newContact.Set("name", name)
	newContact.Set("email", email) // encryption hooks handle this
	newContact.Set("email_index", utils.BlindIndex(email))
	if phone != "" {
		newContact.Set("phone", phone) // encryption hooks handle this
	}
	newContact.Set("status", "pending")
	newContact.Set("source", "manual")

	if err := app.Save(newContact); err != nil {
		log.Printf("[RSVP] Failed to create contact: %v", err)
		return utils.InternalErrorResponse(re, "Failed to create contact")
	}

	return createGuestListItemFromRSVP(re, app, listID, newContact, dietary, plusOne, plusOneName, plusOneDietary, response, invitedBy, now)
}

func createGuestListItemFromRSVP(re *core.RequestEvent, app *pocketbase.PocketBase, listID string, contact *core.Record, dietary string, plusOne bool, plusOneName, plusOneDietary, response, invitedBy, now string) error {
	collection, err := app.FindCollectionByNameOrId(utils.CollectionGuestListItems)
	if err != nil {
		return utils.InternalErrorResponse(re, "Collection not found")
	}

	nextSort := getNextSortOrder(app, listID)

	// Get organisation name
	orgName := ""
	if orgID := contact.GetString("organisation"); orgID != "" {
		if org, err := app.FindRecordById(utils.CollectionOrganisations, orgID); err == nil {
			orgName = org.GetString("name")
		}
	}

	record := core.NewRecord(collection)
	record.Set("guest_list", listID)
	record.Set("contact", contact.Id)
	record.Set("invite_status", response)
	record.Set("sort_order", nextSort)

	// Denormalize contact fields
	record.Set("contact_name", contact.GetString("name"))
	record.Set("contact_job_title", contact.GetString("job_title"))
	record.Set("contact_organisation_name", orgName)
	record.Set("contact_linkedin", contact.GetString("linkedin"))
	record.Set("contact_location", utils.DecryptField(contact.GetString("location")))
	record.Set("contact_degrees", contact.GetString("degrees"))
	record.Set("contact_relationship", contact.GetInt("relationship"))

	// RSVP fields
	record.Set("rsvp_status", response)
	record.Set("rsvp_dietary", dietary)
	record.Set("rsvp_plus_one", plusOne)
	record.Set("rsvp_plus_one_name", plusOneName)
	record.Set("rsvp_plus_one_dietary", plusOneDietary)
	record.Set("rsvp_responded_at", now)
	record.Set("rsvp_invited_by", invitedBy)

	if err := app.Save(record); err != nil {
		log.Printf("[RSVP] Failed to create guest list item: %v", err)
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return utils.BadRequestResponse(re, "You have already RSVP'd for this event")
		}
		return utils.InternalErrorResponse(re, "Failed to save RSVP")
	}

	utils.LogAudit(app, utils.AuditEntry{
		Action:       "rsvp_submit",
		ResourceType: utils.CollectionGuestListItems,
		ResourceID:   record.Id,
		IPAddress:    re.RealIP(),
		UserAgent:    re.Request.UserAgent(),
		Status:       "success",
		Metadata:     map[string]any{"type": "generic", "response": response, "contact_id": contact.Id, "new_contact": contact.GetString("status") == "pending"},
	})

	return re.JSON(http.StatusOK, map[string]string{"message": "RSVP submitted successfully"})
}

// ============================================================================
// Admin RSVP Endpoints
// ============================================================================

func handleGuestListRSVPToggle(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	id := re.Request.PathValue("id")
	record, err := app.FindRecordById(utils.CollectionGuestLists, id)
	if err != nil {
		return utils.NotFoundResponse(re, "Guest list not found")
	}

	var input struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(re.Request.Body).Decode(&input); err != nil {
		return utils.BadRequestResponse(re, "Invalid JSON")
	}

	record.Set("rsvp_enabled", input.Enabled)

	// Generate generic token on first enable
	if input.Enabled && record.GetString("rsvp_generic_token") == "" {
		token, err := generateToken()
		if err != nil {
			return utils.InternalErrorResponse(re, "Failed to generate token")
		}
		record.Set("rsvp_generic_token", token)
	}

	if err := app.Save(record); err != nil {
		return utils.InternalErrorResponse(re, "Failed to update RSVP settings")
	}

	genericURL := ""
	if record.GetString("rsvp_generic_token") != "" {
		genericURL = fmt.Sprintf("%s/rsvp/%s", getBaseURL(), record.GetString("rsvp_generic_token"))
	}

	utils.LogFromRequest(app, re, "update", utils.CollectionGuestLists, id, "success", map[string]any{
		"rsvp_enabled": input.Enabled,
	}, "")

	return re.JSON(http.StatusOK, map[string]any{
		"rsvp_enabled":     input.Enabled,
		"rsvp_generic_url": genericURL,
	})
}

func handleGuestListRSVPSendInvites(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	id := re.Request.PathValue("id")
	guestList, err := app.FindRecordById(utils.CollectionGuestLists, id)
	if err != nil {
		return utils.NotFoundResponse(re, "Guest list not found")
	}

	if !guestList.GetBool("rsvp_enabled") {
		return utils.BadRequestResponse(re, "RSVP is not enabled for this guest list")
	}

	var input struct {
		ItemIDs []string `json:"item_ids"`
	}
	if err := json.NewDecoder(re.Request.Body).Decode(&input); err != nil {
		return utils.BadRequestResponse(re, "Invalid JSON")
	}

	// Get event name for email
	eventName := ""
	if epID := guestList.GetString("event_projection"); epID != "" {
		if ep, err := app.FindRecordById(utils.CollectionEventProjections, epID); err == nil {
			eventName = ep.GetString("name")
		}
	}
	listName := guestList.GetString("name")

	// Find items to send to
	var items []*core.Record
	if len(input.ItemIDs) > 0 {
		// Send to specific items
		for _, itemID := range input.ItemIDs {
			item, err := app.FindRecordById(utils.CollectionGuestListItems, itemID)
			if err != nil {
				continue
			}
			if item.GetString("guest_list") != id {
				continue
			}
			items = append(items, item)
		}
	} else {
		// Send to all items with invite_status = "to_invite" or empty
		allItems, err := app.FindRecordsByFilter(
			utils.CollectionGuestListItems,
			"guest_list = {:id} && (invite_status = 'to_invite' || invite_status = '')",
			"sort_order,created",
			0, 0,
			map[string]any{"id": id},
		)
		if err == nil {
			items = allItems
		}
	}

	sent := 0
	skipped := 0

	for _, item := range items {
		contactID := item.GetString("contact")
		if contactID == "" {
			skipped++
			continue
		}

		contact, err := app.FindRecordById(utils.CollectionContacts, contactID)
		if err != nil {
			skipped++
			continue
		}

		email := utils.DecryptField(contact.GetString("email"))
		if email == "" || !strings.Contains(email, "@") {
			skipped++
			continue
		}

		// Generate RSVP token if not already set
		if item.GetString("rsvp_token") == "" {
			token, err := generateToken()
			if err != nil {
				skipped++
				continue
			}
			item.Set("rsvp_token", token)
		}

		// Update invite_status to "invited"
		item.Set("invite_status", "invited")
		if err := app.Save(item); err != nil {
			log.Printf("[RSVP] Failed to save item %s: %v", item.Id, err)
			skipped++
			continue
		}

		// Build RSVP URL and send email
		rsvpURL := fmt.Sprintf("%s/rsvp/%s", getBaseURL(), item.GetString("rsvp_token"))
		recipientName := contact.GetString("name")

		go sendRSVPInviteEmail(app, email, recipientName, rsvpURL, listName, eventName)
		sent++
	}

	utils.LogFromRequest(app, re, "rsvp_send_invites", utils.CollectionGuestLists, id, "success", map[string]any{
		"sent":    sent,
		"skipped": skipped,
	}, "")

	return re.JSON(http.StatusOK, map[string]any{
		"sent":    sent,
		"skipped": skipped,
	})
}
