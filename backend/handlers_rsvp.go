package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
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

	// Landing page fields — resolve speaker avatars from contacts
	landingProgram := resolveProgramAvatars(app, result.GuestList.Get("landing_program"))

	// Fetch theme (single query, no N+1)
	theme := fetchThemeForGuestList(app, result.GuestList)

	response := map[string]any{
		"type":        result.Type,
		"list_name":   result.GuestList.GetString("name"),
		"event_name":  eventName,
		"description": result.GuestList.GetString("description"),
		"theme":       theme,
		// Landing page
		"landing_enabled":     result.GuestList.GetBool("landing_enabled"),
		"landing_headline":    result.GuestList.GetString("landing_headline"),
		"landing_description": result.GuestList.GetString("landing_description"),
		"landing_image_url":   resolveGuestListImageURL(app, result.GuestList),
		"landing_program":     landingProgram,
		"landing_content":     result.GuestList.GetString("landing_content"),
		"program_description":    result.GuestList.GetString("program_description"),
		"program_title":          result.GuestList.GetString("program_title"),
		"plus_ones_enabled":      result.GuestList.GetBool("rsvp_plus_ones_enabled"),
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
	} else if orgID := result.GuestList.GetString("organisation"); orgID != "" {
		if damURL := os.Getenv("DAM_PUBLIC_URL"); damURL != "" {
			if logoURL := fetchDAMOrgLogo(damURL, orgID); logoURL != "" {
				response["organisation_logo_url"] = logoURL
			}
		}
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
	if input.PlusOne {
		if strings.TrimSpace(input.PlusOneName) == "" {
			return utils.BadRequestResponse(re, "Plus-one first name is required")
		}
		if strings.TrimSpace(input.PlusOneEmail) == "" || !strings.Contains(input.PlusOneEmail, "@") {
			return utils.BadRequestResponse(re, "Plus-one email is required")
		}
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
// Returns the contact record (or nil if no plus-one or no email).
func upsertPlusOneContact(app *pocketbase.PocketBase, input *rsvpInput) *core.Record {
	if !input.PlusOne || strings.TrimSpace(input.PlusOneEmail) == "" {
		return nil
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
			return contact
		}
	}

	// No existing contact — create a new pending one
	contactCollection, err := app.FindCollectionByNameOrId(utils.CollectionContacts)
	if err != nil {
		log.Printf("[RSVP] Failed to find contacts collection for plus-one: %v", err)
		return nil
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
		return nil
	}
	return newContact
}

// addPlusOneToGuestList adds the plus-one contact to the guest list with invite_round "maybe".
// Skips if the contact is already on the list.
func addPlusOneToGuestList(app *pocketbase.PocketBase, contact *core.Record, input *rsvpInput, listID string) {
	if contact == nil {
		return
	}

	// Check if already on this guest list
	existing, err := app.FindRecordsByFilter(
		utils.CollectionGuestListItems,
		"guest_list = {:listId} && contact = {:contactId}",
		"", 1, 0,
		map[string]any{"listId": listID, "contactId": contact.Id},
	)
	if err == nil && len(existing) > 0 {
		log.Printf("[RSVP] Plus-one contact %s already on guest list %s, skipping", contact.Id, listID)
		return
	}

	collection, err := app.FindCollectionByNameOrId(utils.CollectionGuestListItems)
	if err != nil {
		log.Printf("[RSVP] Failed to find guest list items collection for plus-one: %v", err)
		return
	}

	// Build plus-one name
	firstName := strings.TrimSpace(input.PlusOneName)
	lastName := strings.TrimSpace(input.PlusOneLastName)
	plusOneName := firstName
	if lastName != "" {
		plusOneName = firstName + " " + lastName
	}

	record := core.NewRecord(collection)
	record.Set("guest_list", listID)
	record.Set("contact", contact.Id)
	record.Set("invite_round", "maybe")
	record.Set("invite_status", "")
	record.Set("sort_order", getNextSortOrder(app, listID))
	if token, err := generateToken(); err == nil {
		record.Set("rsvp_token", token)
	}
	record.Set("contact_name", plusOneName)
	record.Set("contact_job_title", input.PlusOneJobTitle)
	record.Set("contact_organisation_name", input.PlusOneCompany)

	if err := app.Save(record); err != nil {
		log.Printf("[RSVP] Failed to create plus-one guest list item: %v", err)
	} else {
		log.Printf("[RSVP] Added plus-one %s to guest list %s as maybe", contact.Id, listID)
	}
}

// sendPlusOneNotificationAsync sends a notification email when someone requests a plus-one.
func sendPlusOneNotificationAsync(app *pocketbase.PocketBase, result *rsvpLookupResult, input *rsvpInput, requesterName string) {
	if !input.PlusOne {
		return
	}
	gl := result.GuestList

	// Resolve event name from projection or fall back to list name
	eventName := gl.GetString("name")
	if epID := gl.GetString("event_projection"); epID != "" {
		if ep, err := app.FindRecordById(utils.CollectionEventProjections, epID); err == nil {
			if n := ep.GetString("name"); n != "" {
				eventName = n
			}
		}
	}

	toEmails := extractBCCEmails(gl)

	// Build plus-one name
	firstName := strings.TrimSpace(input.PlusOneName)
	lastName := strings.TrimSpace(input.PlusOneLastName)
	plusOneName := firstName
	if lastName != "" {
		plusOneName = firstName + " " + lastName
	}

	go func() {
		if err := sendPlusOneNotificationEmail(app, requesterName, plusOneName, input.PlusOneJobTitle, input.PlusOneCompany, input.PlusOneEmail, eventName, gl.Id, toEmails); err != nil {
			log.Printf("[RSVP] Failed to send plus-one notification for %s: %v", requesterName, err)
		}
	}()
}

// extractBCCEmails reads the denormalized rsvp_bcc_contacts JSON field and returns a slice of email addresses.
func extractBCCEmails(guestList *core.Record) []string {
	raw := guestList.Get("rsvp_bcc_contacts")
	entries, ok := raw.([]any)
	if !ok {
		return nil
	}
	var emails []string
	for _, entry := range entries {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if email, ok := m["email"].(string); ok && email != "" {
			emails = append(emails, email)
		}
	}
	return emails
}

// sendRSVPConfirmationAsync sends a confirmation email in the background if the response is "accepted".
func sendRSVPConfirmationAsync(app *pocketbase.PocketBase, result *rsvpLookupResult, input *rsvpInput, fullName string) {
	if input.Response != "accepted" {
		return
	}
	gl := result.GuestList

	// Resolve event name from projection or fall back to list name
	eventName := gl.GetString("name")
	if epID := gl.GetString("event_projection"); epID != "" {
		if ep, err := app.FindRecordById(utils.CollectionEventProjections, epID); err == nil {
			if n := ep.GetString("name"); n != "" {
				eventName = n
			}
		}
	}

	eventDate := gl.GetString("event_date")
	eventTime := gl.GetString("event_time")
	eventLocation := gl.GetString("event_location")
	bccEmails := extractBCCEmails(gl)

	go func() {
		if err := sendRSVPConfirmationEmail(app, input.Email, fullName, eventName, eventDate, eventTime, eventLocation, bccEmails); err != nil {
			log.Printf("[RSVP] Failed to send confirmation email to %s: %v", input.Email, err)
		}
	}()
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

	// Upsert plus-one as a contact and add to guest list as "maybe"
	plusOneContact := upsertPlusOneContact(app, input)
	addPlusOneToGuestList(app, plusOneContact, input, result.GuestList.Id)

	utils.LogAudit(app, utils.AuditEntry{
		Action:       "rsvp_submit",
		ResourceType: utils.CollectionGuestListItems,
		ResourceID:   item.Id,
		IPAddress:    re.RealIP(),
		UserAgent:    re.Request.UserAgent(),
		Status:       "success",
		Metadata:     map[string]any{"type": "personal", "response": input.Response},
	})

	sendRSVPConfirmationAsync(app, result, input, fullName)
	sendPlusOneNotificationAsync(app, result, input, fullName)

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

			// Upsert plus-one as a contact and add to guest list as "maybe"
			plusOneContact := upsertPlusOneContact(app, input)
			addPlusOneToGuestList(app, plusOneContact, input, listID)

			utils.LogAudit(app, utils.AuditEntry{
				Action:       "rsvp_submit",
				ResourceType: utils.CollectionGuestListItems,
				ResourceID:   item.Id,
				IPAddress:    re.RealIP(),
				UserAgent:    re.Request.UserAgent(),
				Status:       "success",
				Metadata:     map[string]any{"type": "generic", "response": input.Response, "matched_existing_item": true},
			})

			sendRSVPConfirmationAsync(app, result, input, fullName)
			sendPlusOneNotificationAsync(app, result, input, fullName)

			return re.JSON(http.StatusOK, map[string]string{"message": "RSVP submitted successfully"})
		}

		// Contact exists but not on this list — create new item linking to existing contact
		return createGuestListItemFromRSVP(re, app, result, existingContact, input, fullName, now)
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

	return createGuestListItemFromRSVP(re, app, result, newContact, input, fullName, now)
}

func createGuestListItemFromRSVP(re *core.RequestEvent, app *pocketbase.PocketBase, result *rsvpLookupResult, contact *core.Record, input *rsvpInput, fullName, now string) error {
	listID := result.GuestList.Id
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
	if token, err := generateToken(); err == nil {
		record.Set("rsvp_token", token)
	}

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

	// Upsert plus-one as a contact and add to guest list as "maybe"
	plusOneContact := upsertPlusOneContact(app, input)
	addPlusOneToGuestList(app, plusOneContact, input, listID)

	utils.LogAudit(app, utils.AuditEntry{
		Action:       "rsvp_submit",
		ResourceType: utils.CollectionGuestListItems,
		ResourceID:   record.Id,
		IPAddress:    re.RealIP(),
		UserAgent:    re.Request.UserAgent(),
		Status:       "success",
		Metadata:     map[string]any{"type": "generic", "response": input.Response, "contact_id": contact.Id, "new_contact": contact.GetString("status") == "pending"},
	})

	sendRSVPConfirmationAsync(app, result, input, fullName)
	sendPlusOneNotificationAsync(app, result, input, fullName)

	return re.JSON(http.StatusOK, map[string]string{"message": "RSVP submitted successfully"})
}

// resolveProgramAvatars enriches landing_program items with current avatar URLs from contacts.
func resolveProgramAvatars(app *pocketbase.PocketBase, raw any) any {
	if raw == nil {
		return nil
	}

	// Handle types.JSONRaw (raw JSON bytes) by unmarshalling first
	var items []any
	switch v := raw.(type) {
	case []any:
		items = v
	case json.RawMessage:
		if err := json.Unmarshal(v, &items); err != nil {
			return raw
		}
	default:
		// Try marshalling then unmarshalling as a fallback for types like types.JSONRaw
		b, err := json.Marshal(v)
		if err != nil {
			return raw
		}
		if err := json.Unmarshal(b, &items); err != nil {
			return raw
		}
	}
	for _, entry := range items {
		item, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		contactID, _ := item["speaker_contact_id"].(string)
		if contactID == "" {
			continue
		}
		if contact, err := app.FindRecordById(utils.CollectionContacts, contactID); err == nil {
			avatarURL := contact.GetString("avatar_small_url")
			if avatarURL == "" {
				avatarURL = contact.GetString("avatar_thumb_url")
			}
			if avatarURL == "" {
				avatarURL = contact.GetString("avatar_url")
			}
			if avatarURL == "" {
				if cached, ok := GetDAMAvatarURLs(contactID); ok {
					avatarURL = cached.SmallURL
					if avatarURL == "" {
						avatarURL = cached.ThumbURL
					}
					if avatarURL == "" {
						avatarURL = cached.OriginalURL
					}
				}
			}
			if avatarURL != "" {
				item["speaker_image_url"] = avatarURL
			}
		}
	}
	return items
}

// ============================================================================
// Public RSVP Forward
// ============================================================================

type rsvpForwardInput struct {
	ForwarderName    string `json:"forwarder_name"`
	ForwarderEmail   string `json:"forwarder_email"`
	ForwarderCompany string `json:"forwarder_company"`
	RecipientName    string `json:"recipient_name"`
	RecipientEmail   string `json:"recipient_email"`
	RecipientCompany string `json:"recipient_company"`
}

func handlePublicRSVPForward(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	token := re.Request.PathValue("token")

	result, err := lookupRSVPToken(app, token)
	if err != nil {
		return utils.NotFoundResponse(re, "RSVP link not found")
	}

	if !result.GuestList.GetBool("rsvp_enabled") {
		return re.JSON(http.StatusGone, map[string]string{"error": "RSVP is no longer available for this event"})
	}

	var input rsvpForwardInput
	if err := json.NewDecoder(re.Request.Body).Decode(&input); err != nil {
		return utils.BadRequestResponse(re, "Invalid JSON")
	}

	// Validate required fields
	input.ForwarderName = strings.TrimSpace(input.ForwarderName)
	input.ForwarderEmail = strings.TrimSpace(input.ForwarderEmail)
	input.ForwarderCompany = strings.TrimSpace(input.ForwarderCompany)
	input.RecipientName = strings.TrimSpace(input.RecipientName)
	input.RecipientEmail = strings.TrimSpace(input.RecipientEmail)
	input.RecipientCompany = strings.TrimSpace(input.RecipientCompany)

	if input.ForwarderName == "" {
		return utils.BadRequestResponse(re, "Your name is required")
	}
	if input.ForwarderEmail == "" || !strings.Contains(input.ForwarderEmail, "@") {
		return utils.BadRequestResponse(re, "Valid email is required")
	}
	if input.RecipientName == "" {
		return utils.BadRequestResponse(re, "Their name is required")
	}
	if input.RecipientEmail == "" || !strings.Contains(input.RecipientEmail, "@") {
		return utils.BadRequestResponse(re, "Valid email is required for the recipient")
	}
	if strings.EqualFold(input.ForwarderEmail, input.RecipientEmail) {
		return utils.BadRequestResponse(re, "You can't forward an invitation to yourself")
	}

	listID := result.GuestList.Id

	// Check if recipient is already on this guest list
	recipientBlindIdx := utils.BlindIndex(input.RecipientEmail)
	var existingContact *core.Record

	if recipientBlindIdx != "" {
		contacts, err := app.FindRecordsByFilter(
			utils.CollectionContacts,
			"email_index = {:idx}",
			"", 1, 0,
			map[string]any{"idx": recipientBlindIdx},
		)
		if err == nil && len(contacts) > 0 {
			existingContact = contacts[0]

			// Check if already on this guest list
			existingItems, err := app.FindRecordsByFilter(
				utils.CollectionGuestListItems,
				"guest_list = {:listId} && contact = {:contactId}",
				"", 1, 0,
				map[string]any{"listId": listID, "contactId": existingContact.Id},
			)
			if err == nil && len(existingItems) > 0 {
				return re.JSON(http.StatusConflict, map[string]string{
					"error": "This person is already on the guest list for this event",
				})
			}
		}
	}

	// Parse recipient name into first/last
	nameParts := strings.Fields(input.RecipientName)
	recipientFirstName := nameParts[0]
	recipientLastName := ""
	if len(nameParts) > 1 {
		recipientLastName = strings.Join(nameParts[1:], " ")
	}

	// Find or create recipient contact
	var contact *core.Record
	if existingContact != nil {
		contact = existingContact
		// Update name/company if provided
		needsSave := false
		if recipientFirstName != "" && recipientFirstName != contact.GetString("first_name") {
			contact.Set("first_name", recipientFirstName)
			needsSave = true
		}
		if recipientLastName != "" && recipientLastName != contact.GetString("last_name") {
			contact.Set("last_name", recipientLastName)
			needsSave = true
		}
		if input.RecipientName != contact.GetString("name") {
			contact.Set("name", input.RecipientName)
			needsSave = true
		}
		if needsSave {
			if err := app.Save(contact); err != nil {
				log.Printf("[Forward] Failed to update contact %s: %v", contact.Id, err)
			}
		}
	} else {
		// Create new pending contact
		contactCollection, err := app.FindCollectionByNameOrId(utils.CollectionContacts)
		if err != nil {
			return utils.InternalErrorResponse(re, "Failed to find contacts collection")
		}

		contact = core.NewRecord(contactCollection)
		contact.Set("name", input.RecipientName)
		contact.Set("first_name", recipientFirstName)
		contact.Set("last_name", recipientLastName)
		contact.Set("email", input.RecipientEmail)
		contact.Set("email_index", utils.BlindIndex(input.RecipientEmail))
		contact.Set("status", "pending")
		contact.Set("source", "rsvp_forward")

		// Try to match organisation by name
		if input.RecipientCompany != "" {
			orgs, err := app.FindRecordsByFilter(
				utils.CollectionOrganisations,
				"name ~ {:name}",
				"", 1, 0,
				map[string]any{"name": input.RecipientCompany},
			)
			if err == nil && len(orgs) > 0 {
				contact.Set("organisation", orgs[0].Id)
			}
		}

		if err := app.Save(contact); err != nil {
			log.Printf("[Forward] Failed to create contact: %v", err)
			return utils.InternalErrorResponse(re, "Failed to create contact")
		}
	}

	// Create guest list item with personal RSVP token
	itemCollection, err := app.FindCollectionByNameOrId(utils.CollectionGuestListItems)
	if err != nil {
		return utils.InternalErrorResponse(re, "Collection not found")
	}

	rsvpToken, err := generateToken()
	if err != nil {
		return utils.InternalErrorResponse(re, "Failed to generate token")
	}

	nextSort := getNextSortOrder(app, listID)

	// Get organisation name
	orgName := input.RecipientCompany
	if orgID := contact.GetString("organisation"); orgID != "" {
		if org, err := app.FindRecordById(utils.CollectionOrganisations, orgID); err == nil {
			orgName = org.GetString("name")
		}
	}

	item := core.NewRecord(itemCollection)
	item.Set("guest_list", listID)
	item.Set("contact", contact.Id)
	item.Set("rsvp_token", rsvpToken)
	item.Set("invite_status", "invited")
	item.Set("rsvp_invited_by", input.ForwarderName)
	item.Set("sort_order", nextSort)

	// Denormalize contact fields
	item.Set("contact_name", input.RecipientName)
	item.Set("contact_job_title", contact.GetString("job_title"))
	item.Set("contact_organisation_name", orgName)
	item.Set("contact_linkedin", contact.GetString("linkedin"))
	item.Set("contact_location", utils.DecryptField(contact.GetString("location")))

	if err := app.Save(item); err != nil {
		log.Printf("[Forward] Failed to create guest list item: %v", err)
		return utils.InternalErrorResponse(re, "Failed to forward invitation")
	}

	// Send email in background
	eventName := result.GuestList.GetString("name")
	if epID := result.GuestList.GetString("event_projection"); epID != "" {
		if ep, err := app.FindRecordById(utils.CollectionEventProjections, epID); err == nil {
			if n := ep.GetString("name"); n != "" {
				eventName = n
			}
		}
	}

	rsvpURL := fmt.Sprintf("%s/rsvp/%s", getPublicBaseURL(), rsvpToken)
	listDescription := result.GuestList.GetString("description")
	eventDate := result.GuestList.GetString("event_date")
	eventTime := result.GuestList.GetString("event_time")
	eventLocation := result.GuestList.GetString("event_location")
	go sendRSVPForwardEmail(app, input.RecipientEmail, input.RecipientName, input.ForwarderName, input.ForwarderEmail, rsvpURL, listDescription, eventName, eventDate, eventTime, eventLocation)

	utils.LogAudit(app, utils.AuditEntry{
		Action:       "rsvp_forward",
		ResourceType: utils.CollectionGuestListItems,
		ResourceID:   item.Id,
		IPAddress:    re.RealIP(),
		UserAgent:    re.Request.UserAgent(),
		Status:       "success",
		Metadata: map[string]any{
			"forwarder_name":  input.ForwarderName,
			"recipient_name":  input.RecipientName,
			"guest_list_id":   listID,
			"contact_id":      contact.Id,
			"new_contact":     existingContact == nil,
		},
	})

	return re.JSON(http.StatusOK, map[string]string{"message": "Invitation sent"})
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
		genericURL = fmt.Sprintf("%s/rsvp/%s", getPublicBaseURL(), record.GetString("rsvp_generic_token"))
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
	listDescription := guestList.GetString("description")
	eventDate := guestList.GetString("event_date")
	eventTime := guestList.GetString("event_time")
	eventLocation := guestList.GetString("event_location")

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
		rsvpURL := fmt.Sprintf("%s/rsvp/%s", getPublicBaseURL(), item.GetString("rsvp_token"))
		recipientName := contact.GetString("name")

		go sendRSVPInviteEmail(app, email, recipientName, rsvpURL, item.GetString("rsvp_token"), listName, listDescription, eventName, eventDate, eventTime, eventLocation)
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

// handlePublicRSVPEmailPreview renders a browser-viewable preview of the RSVP invite email
func handlePublicRSVPEmailPreview(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	token := re.Request.PathValue("token")

	result, err := lookupRSVPToken(app, token)
	if err != nil {
		return utils.NotFoundResponse(re, "RSVP link not found")
	}

	gl := result.GuestList

	// Resolve event name
	eventName := ""
	if epID := gl.GetString("event_projection"); epID != "" {
		if ep, err := app.FindRecordById(utils.CollectionEventProjections, epID); err == nil {
			eventName = ep.GetString("name")
		}
	}

	listName := gl.GetString("name")
	listDescription := gl.GetString("description")
	eventDate := gl.GetString("event_date")
	eventTime := gl.GetString("event_time")
	eventLocation := gl.GetString("event_location")

	eventContext := listName
	if eventName != "" {
		eventContext = eventName
	}

	// Resolve recipient name for personal tokens
	firstName := "there"
	if result.Type == "personal" && result.Item != nil {
		if contactID := result.Item.GetString("contact"); contactID != "" {
			if contact, err := app.FindRecordById(utils.CollectionContacts, contactID); err == nil {
				name := utils.DecryptField(contact.GetString("name"))
				if name != "" {
					firstName = strings.Fields(name)[0]
				}
			}
		}
	}

	rsvpURL := fmt.Sprintf("%s/rsvp/%s", getBaseURL(), token)

	// Build the same email content as sendRSVPInviteEmail
	detailsHTML := ""
	if eventDate != "" || eventTime != "" || eventLocation != "" {
		detailsHTML = `<div style="padding: 24px 0; margin: 24px 0; border-top: 1px solid #333; border-bottom: 1px solid #333;">`
		if eventDate != "" {
			detailsHTML += fmt.Sprintf(`<p style="color: rgba(255,255,255,0.7); font-size: 15px; margin: 0 0 4px 0;">%s</p>`, eventDate)
		}
		if eventTime != "" {
			detailsHTML += fmt.Sprintf(`<p style="color: rgba(255,255,255,0.7); font-size: 15px; margin: 0 0 4px 0;">%s</p>`, eventTime)
		}
		if eventLocation != "" {
			detailsHTML += fmt.Sprintf(`<p style="color: rgba(255,255,255,0.7); font-size: 15px; margin: 0;">%s</p>`, eventLocation)
		}
		detailsHTML += `</div>`
	}

	descriptionHTML := ""
	if listDescription != "" {
		descriptionHTML = fmt.Sprintf(`<p style="color: rgba(255,255,255,0.8); font-size: 16px; line-height: 1.6; margin: 0 0 16px 0;">%s</p>
            <p style="color: rgba(255,255,255,0.8); font-size: 16px; line-height: 1.6; margin: 0 0 8px 0;">We'd love to see you there.</p>`, listDescription)
	} else {
		descriptionHTML = `<p style="color: rgba(255,255,255,0.8); font-size: 16px; line-height: 1.6; margin: 0 0 8px 0;">We'd love for you to join us for an evening of conversation, connection and great food.</p>`
	}

	content := fmt.Sprintf(`
            <p style="color: rgba(255,255,255,0.5); font-size: 12px; text-transform: uppercase; letter-spacing: 2px; margin: 0 0 16px 0;">You're invited</p>
            <h1 style="color: #ffffff; font-size: 32px; line-height: 1.1; margin: 0 0 20px 0;">%s</h1>
            <p style="color: rgba(255,255,255,0.8); font-size: 16px; line-height: 1.6; margin: 0 0 16px 0;">Hi %s,</p>
            %s
            %s
            <p style="color: rgba(255,255,255,0.6); font-size: 15px; line-height: 1.6; margin: 0 0 32px 0;">
                Spaces are limited, so please let us know if you can make it.
            </p>
            <div style="display: inline-block; width: 48%%; vertical-align: top;">
                <a href="%s" style="display: block; background: #E95139; color: #ffffff; padding: 16px 12px; text-decoration: none; font-size: 14px; text-align: center; border: 1px solid #E95139;">
                    I can make it
                </a>
            </div>
            <div style="display: inline-block; width: 48%%; vertical-align: top;">
                <a href="%s" style="display: block; background: transparent; color: #ffffff; padding: 16px 12px; text-decoration: none; font-size: 14px; text-align: center; border: 1px solid #555;">
                    I can't make it
                </a>
            </div>
            <p style="color: rgba(255,255,255,0.8); font-size: 13px; margin: 16px 0 24px 0;">
                This link is personal to you. Please don't share it.
            </p>
            <p style="color: rgba(255,255,255,0.3); font-size: 13px; margin: 0 0 8px 0;">
                Copy and paste if the link doesn't work:
            </p>
            <div style="background: rgba(255,255,255,0.05); padding: 12px 16px; margin: 0;">
                <p style="color: rgba(255,255,255,0.4); font-size: 12px; font-family: 'Courier New', Courier, monospace; word-break: break-all; margin: 0;">
                    %s
                </p>
            </div>
`, eventContext, firstName, descriptionHTML, detailsHTML, rsvpURL, rsvpURL, rsvpURL)

	html := wrapRSVPEmailHTML(content)

	re.Response.Header().Set("Content-Type", "text/html; charset=utf-8")
	re.Response.WriteHeader(http.StatusOK)
	re.Response.Write([]byte(html))
	return nil
}

// ============================================================================
// Invite Tracking (open pixel + click redirect)
// ============================================================================

// 1x1 transparent GIF
var trackingPixelGIF, _ = base64.StdEncoding.DecodeString("R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7")

func handleTrackOpen(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	token := re.Request.PathValue("token")

	items, err := app.FindRecordsByFilter(
		utils.CollectionGuestListItems,
		"rsvp_token = {:token}",
		"", 1, 0,
		map[string]any{"token": token},
	)
	if err == nil && len(items) > 0 {
		item := items[0]
		if !item.GetBool("invite_opened") {
			item.Set("invite_opened", true)
			if err := app.Save(item); err != nil {
				log.Printf("[Tracking] Failed to save invite_opened for item %s: %v", item.Id, err)
			}
		}
	}

	re.Response.Header().Set("Content-Type", "image/gif")
	re.Response.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	re.Response.WriteHeader(http.StatusOK)
	re.Response.Write(trackingPixelGIF)
	return nil
}

func handleTrackClick(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	token := re.Request.PathValue("token")
	dest := re.Request.URL.Query().Get("url")

	// Validate destination URL is same-origin to prevent open redirect
	baseURL := getBaseURL()
	publicURL := getPublicBaseURL()
	if dest == "" || (!strings.HasPrefix(dest, baseURL) && !strings.HasPrefix(dest, publicURL)) {
		return re.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid URL"})
	}

	if _, err := url.ParseRequestURI(dest); err != nil {
		return re.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid URL"})
	}

	items, err := app.FindRecordsByFilter(
		utils.CollectionGuestListItems,
		"rsvp_token = {:token}",
		"", 1, 0,
		map[string]any{"token": token},
	)
	if err == nil && len(items) > 0 {
		item := items[0]
		if !item.GetBool("invite_clicked") {
			item.Set("invite_clicked", true)
			if err := app.Save(item); err != nil {
				log.Printf("[Tracking] Failed to save invite_clicked for item %s: %v", item.Id, err)
			}
		}
	}

	http.Redirect(re.Response, re.Request, dest, http.StatusFound)
	return nil
}
