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

	// Get event details from projection
	eventName := ""
	var eventDetails map[string]any
	if epID := result.GuestList.GetString("event_projection"); epID != "" {
		if ep, err := app.FindRecordById(utils.CollectionEventProjections, epID); err == nil {
			eventName = ep.GetString("name")
			eventDetails = map[string]any{
				"event_date":          ep.GetString("date"),
				"event_start_time":    ep.GetString("start_time"),
				"event_end_time":      ep.GetString("end_time"),
				"event_start_date":    ep.GetString("start_date"),
				"event_end_date":      ep.GetString("end_date"),
				"event_venue":         ep.GetString("venue"),
				"event_venue_city":    ep.GetString("venue_city"),
				"event_venue_country": ep.GetString("venue_country"),
				"event_timezone":      ep.GetString("timezone"),
				"event_description":   ep.GetString("description"),
			}
		}
	}

	// Landing page fields
	var landingProgram any
	if raw := result.GuestList.Get("landing_program"); raw != nil {
		landingProgram = raw
	}

	response := map[string]any{
		"type":        result.Type,
		"list_name":   result.GuestList.GetString("name"),
		"event_name":  eventName,
		"description": result.GuestList.GetString("description"),
		// Landing page
		"landing_enabled":     result.GuestList.GetBool("landing_enabled"),
		"landing_headline":    result.GuestList.GetString("landing_headline"),
		"landing_description": result.GuestList.GetString("landing_description"),
		"landing_image_url":   result.GuestList.GetString("landing_image_url"),
		"landing_program":     landingProgram,
		"landing_content":     result.GuestList.GetString("landing_content"),
	}

	// Merge event projection details
	for k, v := range eventDetails {
		response[k] = v
	}

	// Guest list event details override event projection values
	if v := result.GuestList.GetString("event_date"); v != "" {
		response["event_date"] = v
	}
	if v := result.GuestList.GetString("event_time"); v != "" {
		response["event_time"] = v
	}
	if v := result.GuestList.GetString("event_location"); v != "" {
		response["event_location"] = v
	}
	if v := result.GuestList.GetString("event_location_address"); v != "" {
		response["event_location_address"] = v
	}

	if v := result.GuestList.GetString("organisation_name"); v != "" {
		response["organisation_name"] = v
	}
	if v := result.GuestList.GetString("organisation_logo_url"); v != "" {
		response["organisation_logo_url"] = v
	}

	if result.Type == "personal" && result.Item != nil {
		// Pre-fill from linked contact
		prefilledFirstName := ""
		prefilledLastName := ""
		prefilledEmail := ""
		prefilledPhone := ""
		var dietaryReqs []string
		dietaryOther := ""
		var accessibilityReqs []string
		accessibilityOther := ""

		if contactID := result.Item.GetString("contact"); contactID != "" {
			if contact, err := app.FindRecordById(utils.CollectionContacts, contactID); err == nil {
				prefilledFirstName = contact.GetString("first_name")
				prefilledLastName = contact.GetString("last_name")
				prefilledEmail = utils.DecryptField(contact.GetString("email"))
				prefilledPhone = utils.DecryptField(contact.GetString("phone"))

				// Dietary/accessibility from contact
				if raw := contact.Get("dietary_requirements"); raw != nil {
					if arr, ok := raw.([]string); ok {
						dietaryReqs = arr
					}
				}
				dietaryOther = contact.GetString("dietary_requirements_other")

				if raw := contact.Get("accessibility_requirements"); raw != nil {
					if arr, ok := raw.([]string); ok {
						accessibilityReqs = arr
					}
				}
				accessibilityOther = contact.GetString("accessibility_requirements_other")
			}
		}

		response["prefilled_first_name"] = prefilledFirstName
		response["prefilled_last_name"] = prefilledLastName
		response["prefilled_email"] = prefilledEmail
		response["prefilled_phone"] = prefilledPhone
		response["prefilled_dietary_requirements"] = dietaryReqs
		response["prefilled_dietary_requirements_other"] = dietaryOther
		response["prefilled_accessibility_requirements"] = accessibilityReqs
		response["prefilled_accessibility_requirements_other"] = accessibilityOther

		// Include existing RSVP data if already responded
		rsvpStatus := result.Item.GetString("rsvp_status")
		response["already_responded"] = rsvpStatus != ""
		response["rsvp_status"] = rsvpStatus

		if rsvpStatus != "" {
			response["rsvp_plus_one"] = result.Item.GetBool("rsvp_plus_one")
			response["rsvp_plus_one_name"] = result.Item.GetString("rsvp_plus_one_name")
			response["rsvp_plus_one_last_name"] = result.Item.GetString("rsvp_plus_one_last_name")
			response["rsvp_plus_one_job_title"] = result.Item.GetString("rsvp_plus_one_job_title")
			response["rsvp_plus_one_company"] = result.Item.GetString("rsvp_plus_one_company")
			response["rsvp_plus_one_email"] = result.Item.GetString("rsvp_plus_one_email")
			response["rsvp_plus_one_dietary"] = result.Item.GetString("rsvp_plus_one_dietary")
			response["rsvp_comments"] = result.Item.GetString("rsvp_comments")
		}
	}

	return re.JSON(http.StatusOK, response)
}

type rsvpInput struct {
	FirstName                    string   `json:"first_name"`
	LastName                     string   `json:"last_name"`
	Email                        string   `json:"email"`
	Phone                        string   `json:"phone"`
	DietaryRequirements          []string `json:"dietary_requirements"`
	DietaryRequirementsOther     string   `json:"dietary_requirements_other"`
	AccessibilityRequirements    []string `json:"accessibility_requirements"`
	AccessibilityRequirementsOther string `json:"accessibility_requirements_other"`
	PlusOne                      bool     `json:"plus_one"`
	PlusOneName                  string   `json:"plus_one_name"`
	PlusOneLastName              string   `json:"plus_one_last_name"`
	PlusOneJobTitle              string   `json:"plus_one_job_title"`
	PlusOneCompany               string   `json:"plus_one_company"`
	PlusOneEmail                 string   `json:"plus_one_email"`
	PlusOneDietary               string   `json:"plus_one_dietary"`
	Response                     string   `json:"response"`
	InvitedBy                    string   `json:"invited_by"`
	Comments                     string   `json:"comments"`
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

	var input rsvpInput
	if err := json.NewDecoder(re.Request.Body).Decode(&input); err != nil {
		return utils.BadRequestResponse(re, "Invalid JSON")
	}

	// Validate required fields
	input.FirstName = strings.TrimSpace(input.FirstName)
	input.LastName = strings.TrimSpace(input.LastName)
	input.Email = strings.TrimSpace(input.Email)
	if input.FirstName == "" {
		return utils.BadRequestResponse(re, "First name is required")
	}
	if input.Email == "" || !strings.Contains(input.Email, "@") {
		return utils.BadRequestResponse(re, "Valid email is required")
	}
	if input.Response != "accepted" && input.Response != "declined" {
		return utils.BadRequestResponse(re, "Response must be 'accepted' or 'declined'")
	}
	if len(input.PlusOneDietary) > 1000 {
		return utils.BadRequestResponse(re, "Plus-one dietary requirements must be 1000 characters or less")
	}
	if len(input.Comments) > 2000 {
		return utils.BadRequestResponse(re, "Comments must be 2000 characters or less")
	}

	// Compose full name for backward compat
	fullName := input.FirstName
	if input.LastName != "" {
		fullName = input.FirstName + " " + input.LastName
	}

	now := time.Now().UTC().Format(time.RFC3339)

	if result.Type == "personal" {
		return handlePersonalRSVP(re, app, result, &input, fullName, now)
	}
	return handleGenericRSVP(re, app, result, &input, fullName, now)
}

func setItemRSVPFields(item *core.Record, input *rsvpInput, fullName, now string) {
	item.Set("rsvp_status", input.Response)
	item.Set("rsvp_plus_one", input.PlusOne)
	item.Set("rsvp_plus_one_name", input.PlusOneName)
	item.Set("rsvp_plus_one_last_name", input.PlusOneLastName)
	item.Set("rsvp_plus_one_job_title", input.PlusOneJobTitle)
	item.Set("rsvp_plus_one_company", input.PlusOneCompany)
	item.Set("rsvp_plus_one_email", input.PlusOneEmail)
	item.Set("rsvp_plus_one_dietary", input.PlusOneDietary)
	item.Set("rsvp_responded_at", now)
	item.Set("rsvp_comments", input.Comments)
	item.Set("invite_status", input.Response) // sync invite_status
}

func upsertContactFromRSVP(app *pocketbase.PocketBase, contact *core.Record, input *rsvpInput, fullName string) {
	needsSave := false

	// Name
	if input.FirstName != "" && input.FirstName != contact.GetString("first_name") {
		contact.Set("first_name", input.FirstName)
		needsSave = true
	}
	if input.LastName != contact.GetString("last_name") {
		contact.Set("last_name", input.LastName)
		needsSave = true
	}
	if fullName != "" && fullName != contact.GetString("name") {
		contact.Set("name", fullName)
		needsSave = true
	}

	// Email
	currentEmail := utils.DecryptField(contact.GetString("email"))
	if input.Email != "" && !strings.EqualFold(input.Email, currentEmail) {
		contact.Set("email", input.Email)
		contact.Set("email_index", utils.BlindIndex(input.Email))
		needsSave = true
	}

	// Phone
	currentPhone := utils.DecryptField(contact.GetString("phone"))
	if input.Phone != "" && input.Phone != currentPhone {
		contact.Set("phone", input.Phone)
		needsSave = true
	}

	// Dietary requirements
	if len(input.DietaryRequirements) > 0 {
		contact.Set("dietary_requirements", input.DietaryRequirements)
		needsSave = true
	}
	if input.DietaryRequirementsOther != "" {
		contact.Set("dietary_requirements_other", input.DietaryRequirementsOther)
		needsSave = true
	}

	// Accessibility requirements
	if len(input.AccessibilityRequirements) > 0 {
		contact.Set("accessibility_requirements", input.AccessibilityRequirements)
		needsSave = true
	}
	if input.AccessibilityRequirementsOther != "" {
		contact.Set("accessibility_requirements_other", input.AccessibilityRequirementsOther)
		needsSave = true
	}

	if needsSave {
		if err := app.Save(contact); err != nil {
			log.Printf("[RSVP] Failed to update contact %s: %v", contact.Id, err)
		}
	}
}

// upsertPlusOneContact silently creates or updates a contact record for the plus-one guest.
func upsertPlusOneContact(app *pocketbase.PocketBase, input *rsvpInput) {
	if !input.PlusOne || strings.TrimSpace(input.PlusOneEmail) == "" {
		return
	}

	email := strings.TrimSpace(input.PlusOneEmail)
	firstName := strings.TrimSpace(input.PlusOneName)
	lastName := strings.TrimSpace(input.PlusOneLastName)
	fullName := firstName
	if lastName != "" {
		fullName = firstName + " " + lastName
	}

	// Look up existing contact by email blind index
	blindIdx := utils.BlindIndex(email)
	if blindIdx != "" {
		contacts, err := app.FindRecordsByFilter(
			utils.CollectionContacts,
			"email_index = {:idx}",
			"", 1, 0,
			map[string]any{"idx": blindIdx},
		)
		if err == nil && len(contacts) > 0 {
			// Silently update existing contact
			contact := contacts[0]
			needsSave := false

			if firstName != "" && firstName != contact.GetString("first_name") {
				contact.Set("first_name", firstName)
				needsSave = true
			}
			if lastName != "" && lastName != contact.GetString("last_name") {
				contact.Set("last_name", lastName)
				needsSave = true
			}
			if fullName != "" && fullName != contact.GetString("name") {
				contact.Set("name", fullName)
				needsSave = true
			}
			if input.PlusOneJobTitle != "" && input.PlusOneJobTitle != contact.GetString("job_title") {
				contact.Set("job_title", input.PlusOneJobTitle)
				needsSave = true
			}

			if needsSave {
				if err := app.Save(contact); err != nil {
					log.Printf("[RSVP] Failed to update plus-one contact %s: %v", contact.Id, err)
				}
			}
			return
		}
	}

	// No existing contact — create a new pending one
	contactCollection, err := app.FindCollectionByNameOrId(utils.CollectionContacts)
	if err != nil {
		log.Printf("[RSVP] Failed to find contacts collection for plus-one: %v", err)
		return
	}

	newContact := core.NewRecord(contactCollection)
	newContact.Set("name", fullName)
	newContact.Set("first_name", firstName)
	newContact.Set("last_name", lastName)
	newContact.Set("email", email) // encryption hooks handle this
	newContact.Set("email_index", utils.BlindIndex(email))
	if input.PlusOneJobTitle != "" {
		newContact.Set("job_title", input.PlusOneJobTitle)
	}
	newContact.Set("status", "pending")
	newContact.Set("source", "manual")

	if err := app.Save(newContact); err != nil {
		log.Printf("[RSVP] Failed to create plus-one contact: %v", err)
	}
}

func handlePersonalRSVP(re *core.RequestEvent, app *pocketbase.PocketBase, result *rsvpLookupResult, input *rsvpInput, fullName, now string) error {
	item := result.Item

	// Update RSVP fields on the item
	setItemRSVPFields(item, input, fullName, now)

	// Update denormalized name if changed
	if fullName != item.GetString("contact_name") {
		item.Set("contact_name", fullName)
	}

	if err := app.Save(item); err != nil {
		return utils.InternalErrorResponse(re, "Failed to save RSVP")
	}

	// Upsert the linked contact directly
	if contactID := item.GetString("contact"); contactID != "" {
		if contact, err := app.FindRecordById(utils.CollectionContacts, contactID); err == nil {
			upsertContactFromRSVP(app, contact, input, fullName)
		}
	}

	// Silently upsert plus-one as a contact
	upsertPlusOneContact(app, input)

	utils.LogAudit(app, utils.AuditEntry{
		Action:       "rsvp_submit",
		ResourceType: utils.CollectionGuestListItems,
		ResourceID:   item.Id,
		IPAddress:    re.RealIP(),
		UserAgent:    re.Request.UserAgent(),
		Status:       "success",
		Metadata:     map[string]any{"type": "personal", "response": input.Response},
	})

	return re.JSON(http.StatusOK, map[string]string{"message": "RSVP submitted successfully"})
}

func handleGenericRSVP(re *core.RequestEvent, app *pocketbase.PocketBase, result *rsvpLookupResult, input *rsvpInput, fullName, now string) error {
	listID := result.GuestList.Id

	// Check for existing contact by email using blind index
	blindIdx := utils.BlindIndex(input.Email)
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
		// Update contact with RSVP data
		upsertContactFromRSVP(app, existingContact, input, fullName)

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
			setItemRSVPFields(item, input, fullName, now)
			item.Set("rsvp_invited_by", input.InvitedBy)

			if err := app.Save(item); err != nil {
				return utils.InternalErrorResponse(re, "Failed to save RSVP")
			}

			// Silently upsert plus-one as a contact
			upsertPlusOneContact(app, input)

			utils.LogAudit(app, utils.AuditEntry{
				Action:       "rsvp_submit",
				ResourceType: utils.CollectionGuestListItems,
				ResourceID:   item.Id,
				IPAddress:    re.RealIP(),
				UserAgent:    re.Request.UserAgent(),
				Status:       "success",
				Metadata:     map[string]any{"type": "generic", "response": input.Response, "matched_existing_item": true},
			})

			return re.JSON(http.StatusOK, map[string]string{"message": "RSVP submitted successfully"})
		}

		// Contact exists but not on this list — create new item linking to existing contact
		return createGuestListItemFromRSVP(re, app, listID, existingContact, input, fullName, now)
	}

	// No matching contact — create new pending contact
	contactCollection, err := app.FindCollectionByNameOrId(utils.CollectionContacts)
	if err != nil {
		return utils.InternalErrorResponse(re, "Failed to find contacts collection")
	}

	newContact := core.NewRecord(contactCollection)
	newContact.Set("name", fullName)
	newContact.Set("first_name", input.FirstName)
	newContact.Set("last_name", input.LastName)
	newContact.Set("email", input.Email) // encryption hooks handle this
	newContact.Set("email_index", utils.BlindIndex(input.Email))
	if input.Phone != "" {
		newContact.Set("phone", input.Phone) // encryption hooks handle this
	}
	newContact.Set("status", "pending")
	newContact.Set("source", "manual")

	if len(input.DietaryRequirements) > 0 {
		newContact.Set("dietary_requirements", input.DietaryRequirements)
	}
	if input.DietaryRequirementsOther != "" {
		newContact.Set("dietary_requirements_other", input.DietaryRequirementsOther)
	}
	if len(input.AccessibilityRequirements) > 0 {
		newContact.Set("accessibility_requirements", input.AccessibilityRequirements)
	}
	if input.AccessibilityRequirementsOther != "" {
		newContact.Set("accessibility_requirements_other", input.AccessibilityRequirementsOther)
	}

	if err := app.Save(newContact); err != nil {
		log.Printf("[RSVP] Failed to create contact: %v", err)
		return utils.InternalErrorResponse(re, "Failed to create contact")
	}

	return createGuestListItemFromRSVP(re, app, listID, newContact, input, fullName, now)
}

func createGuestListItemFromRSVP(re *core.RequestEvent, app *pocketbase.PocketBase, listID string, contact *core.Record, input *rsvpInput, fullName, now string) error {
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
	record.Set("invite_status", input.Response)
	record.Set("sort_order", nextSort)

	// Denormalize contact fields
	record.Set("contact_name", fullName)
	record.Set("contact_job_title", contact.GetString("job_title"))
	record.Set("contact_organisation_name", orgName)
	record.Set("contact_linkedin", contact.GetString("linkedin"))
	record.Set("contact_location", utils.DecryptField(contact.GetString("location")))
	record.Set("contact_degrees", contact.GetString("degrees"))
	record.Set("contact_relationship", contact.GetInt("relationship"))

	// RSVP fields
	setItemRSVPFields(record, input, fullName, now)
	record.Set("rsvp_invited_by", input.InvitedBy)

	if err := app.Save(record); err != nil {
		log.Printf("[RSVP] Failed to create guest list item: %v", err)
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return utils.BadRequestResponse(re, "You have already RSVP'd for this event")
		}
		return utils.InternalErrorResponse(re, "Failed to save RSVP")
	}

	// Silently upsert plus-one as a contact
	upsertPlusOneContact(app, input)

	utils.LogAudit(app, utils.AuditEntry{
		Action:       "rsvp_submit",
		ResourceType: utils.CollectionGuestListItems,
		ResourceID:   record.Id,
		IPAddress:    re.RealIP(),
		UserAgent:    re.Request.UserAgent(),
		Status:       "success",
		Metadata:     map[string]any{"type": "generic", "response": input.Response, "contact_id": contact.Id, "new_contact": contact.GetString("status") == "pending"},
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
