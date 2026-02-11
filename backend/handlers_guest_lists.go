package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/grtshw/outlook-apps-crm/utils"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// ============================================================================
// Admin CRUD — Guest Lists
// ============================================================================

func handleGuestListsList(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	status := re.Request.URL.Query().Get("status")
	search := re.Request.URL.Query().Get("search")

	filter := ""
	params := map[string]any{}

	if status != "" {
		filter = "status = {:status}"
		params["status"] = status
	}
	if search != "" {
		if filter != "" {
			filter += " && "
		}
		filter += "name ~ {:search}"
		params["search"] = search
	}

	sort := "-created"
	records, err := app.FindRecordsByFilter(utils.CollectionGuestLists, filter, sort, 100, 0, params)
	if err != nil {
		return re.JSON(http.StatusOK, map[string]any{"items": []any{}})
	}

	items := make([]map[string]any, len(records))
	for i, r := range records {
		// Count items and shares
		itemCount := countRecords(app, utils.CollectionGuestListItems, "guest_list = {:id}", r.Id)
		shareCount := countRecords(app, utils.CollectionGuestListShares, "guest_list = {:id} && revoked = false", r.Id)

		// Get event name from projection
		eventName := ""
		if epID := r.GetString("event_projection"); epID != "" {
			if ep, err := app.FindRecordById(utils.CollectionEventProjections, epID); err == nil {
				eventName = ep.GetString("name")
			}
		}

		items[i] = map[string]any{
			"id":          r.Id,
			"name":        r.GetString("name"),
			"description": r.GetString("description"),
			"event_name":  eventName,
			"event_projection": r.GetString("event_projection"),
			"status":      r.GetString("status"),
			"item_count":  itemCount,
			"share_count": shareCount,
			"created":     r.GetString("created"),
			"updated":     r.GetString("updated"),
		}
	}

	return re.JSON(http.StatusOK, map[string]any{"items": items})
}

func handleGuestListGet(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	id := re.Request.PathValue("id")
	record, err := app.FindRecordById(utils.CollectionGuestLists, id)
	if err != nil {
		return utils.NotFoundResponse(re, "Guest list not found")
	}

	itemCount := countRecords(app, utils.CollectionGuestListItems, "guest_list = {:id}", record.Id)
	shareCount := countRecords(app, utils.CollectionGuestListShares, "guest_list = {:id} && revoked = false", record.Id)

	eventName := ""
	if epID := record.GetString("event_projection"); epID != "" {
		if ep, err := app.FindRecordById(utils.CollectionEventProjections, epID); err == nil {
			eventName = ep.GetString("name")
		}
	}

	return re.JSON(http.StatusOK, map[string]any{
		"id":               record.Id,
		"name":             record.GetString("name"),
		"description":      record.GetString("description"),
		"event_projection": record.GetString("event_projection"),
		"event_name":       eventName,
		"status":           record.GetString("status"),
		"created_by":       record.GetString("created_by"),
		"item_count":       itemCount,
		"share_count":      shareCount,
		"created":          record.GetString("created"),
		"updated":          record.GetString("updated"),
	})
}

func handleGuestListCreate(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	var input map[string]any
	if err := json.NewDecoder(re.Request.Body).Decode(&input); err != nil {
		return utils.BadRequestResponse(re, "Invalid JSON")
	}

	collection, err := app.FindCollectionByNameOrId(utils.CollectionGuestLists)
	if err != nil {
		return utils.InternalErrorResponse(re, "Collection not found")
	}

	record := core.NewRecord(collection)
	record.Set("name", input["name"])
	record.Set("description", input["description"])
	record.Set("event_projection", input["event_projection"])
	record.Set("created_by", re.Auth.Id)
	record.Set("status", stringOrDefault(input["status"], "draft"))

	if err := app.Save(record); err != nil {
		return utils.InternalErrorResponse(re, "Failed to create guest list")
	}

	utils.LogFromRequest(app, re, "create", utils.CollectionGuestLists, record.Id, "success", nil, "")

	return re.JSON(http.StatusCreated, map[string]any{
		"id":     record.Id,
		"name":   record.GetString("name"),
		"status": record.GetString("status"),
	})
}

func handleGuestListUpdate(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	id := re.Request.PathValue("id")
	record, err := app.FindRecordById(utils.CollectionGuestLists, id)
	if err != nil {
		return utils.NotFoundResponse(re, "Guest list not found")
	}

	var input map[string]any
	if err := json.NewDecoder(re.Request.Body).Decode(&input); err != nil {
		return utils.BadRequestResponse(re, "Invalid JSON")
	}

	if v, ok := input["name"].(string); ok {
		record.Set("name", v)
	}
	if v, ok := input["description"].(string); ok {
		record.Set("description", v)
	}
	if v, ok := input["event_projection"].(string); ok {
		record.Set("event_projection", v)
	}
	if v, ok := input["status"].(string); ok {
		record.Set("status", v)
	}

	if err := app.Save(record); err != nil {
		return utils.InternalErrorResponse(re, "Failed to update guest list")
	}

	utils.LogFromRequest(app, re, "update", utils.CollectionGuestLists, record.Id, "success", nil, "")
	return utils.SuccessResponse(re, "Guest list updated")
}

func handleGuestListDelete(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	id := re.Request.PathValue("id")
	record, err := app.FindRecordById(utils.CollectionGuestLists, id)
	if err != nil {
		return utils.NotFoundResponse(re, "Guest list not found")
	}

	// Cascade delete: OTP codes → shares → items → list
	shares, _ := app.FindRecordsByFilter(utils.CollectionGuestListShares, "guest_list = {:id}", "", 0, 0, map[string]any{"id": id})
	for _, share := range shares {
		otps, _ := app.FindRecordsByFilter(utils.CollectionGuestListOTPCodes, "share = {:sid}", "", 0, 0, map[string]any{"sid": share.Id})
		for _, otp := range otps {
			app.Delete(otp)
		}
		app.Delete(share)
	}

	items, _ := app.FindRecordsByFilter(utils.CollectionGuestListItems, "guest_list = {:id}", "", 0, 0, map[string]any{"id": id})
	for _, item := range items {
		app.Delete(item)
	}

	if err := app.Delete(record); err != nil {
		return utils.InternalErrorResponse(re, "Failed to delete guest list")
	}

	utils.LogFromRequest(app, re, "delete", utils.CollectionGuestLists, id, "success", nil, "")
	return utils.SuccessResponse(re, "Guest list deleted")
}

// ============================================================================
// Admin CRUD — Guest List Items
// ============================================================================

func handleGuestListItemsList(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	listID := re.Request.PathValue("id")

	// Verify the guest list exists
	if _, err := app.FindRecordById(utils.CollectionGuestLists, listID); err != nil {
		return utils.NotFoundResponse(re, "Guest list not found")
	}

	records, err := app.FindRecordsByFilter(
		utils.CollectionGuestListItems,
		"guest_list = {:id}",
		"sort_order,created",
		0, 0,
		map[string]any{"id": listID},
	)
	if err != nil {
		return re.JSON(http.StatusOK, map[string]any{"items": []any{}})
	}

	baseURL := getBaseURL()
	items := make([]map[string]any, len(records))
	for i, r := range records {
		item := map[string]any{
			"id":                        r.Id,
			"contact_id":                r.GetString("contact"),
			"contact_name":              r.GetString("contact_name"),
			"contact_job_title":         r.GetString("contact_job_title"),
			"contact_organisation_name": r.GetString("contact_organisation_name"),
			"contact_linkedin":          r.GetString("contact_linkedin"),
			"contact_location":          r.GetString("contact_location"),
			"contact_degrees":           r.GetString("contact_degrees"),
			"contact_relationship":      r.GetInt("contact_relationship"),
			"invite_round":              r.GetString("invite_round"),
			"invite_status":             r.GetString("invite_status"),
			"notes":                     r.GetString("notes"),
			"client_notes":              r.GetString("client_notes"),
			"sort_order":                r.GetInt("sort_order"),
			"created":                   r.GetString("created"),
		}

		// Enrich with live contact status for admin view
		if contactID := r.GetString("contact"); contactID != "" {
			if contact, err := app.FindRecordById(utils.CollectionContacts, contactID); err == nil {
				item["contact_status"] = contact.GetString("status")
				// Update denormalized fields with latest live data
				item["contact_name"] = contact.GetString("name")
				item["contact_job_title"] = contact.GetString("job_title")
				item["contact_linkedin"] = contact.GetString("linkedin")
				item["contact_location"] = utils.DecryptField(contact.GetString("location"))
				item["contact_degrees"] = contact.GetString("degrees")
				item["contact_relationship"] = contact.GetInt("relationship")
				if orgID := contact.GetString("organisation"); orgID != "" {
					if org, err := app.FindRecordById(utils.CollectionOrganisations, orgID); err == nil {
						item["contact_organisation_name"] = org.GetString("name")
					}
				}
				// Avatar URLs
				if avatarURL := contact.GetString("avatar_url"); avatarURL != "" {
					item["contact_avatar_url"] = avatarURL
				} else if avatar := contact.GetString("avatar"); avatar != "" {
					item["contact_avatar_url"] = getFileURL(baseURL, contact.Collection().Id, contact.Id, avatar)
				}
				if thumb := contact.GetString("avatar_thumb_url"); thumb != "" {
					item["contact_avatar_thumb_url"] = thumb
				}
				if small := contact.GetString("avatar_small_url"); small != "" {
					item["contact_avatar_small_url"] = small
				}
			} else {
				item["contact_status"] = "deleted"
			}
		}

		items[i] = item
	}

	return re.JSON(http.StatusOK, map[string]any{"items": items})
}

func handleGuestListItemCreate(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	listID := re.Request.PathValue("id")

	if _, err := app.FindRecordById(utils.CollectionGuestLists, listID); err != nil {
		return utils.NotFoundResponse(re, "Guest list not found")
	}

	var input map[string]any
	if err := json.NewDecoder(re.Request.Body).Decode(&input); err != nil {
		return utils.BadRequestResponse(re, "Invalid JSON")
	}

	contactID, ok := input["contact_id"].(string)
	if !ok || contactID == "" {
		return utils.BadRequestResponse(re, "contact_id is required")
	}

	contact, err := app.FindRecordById(utils.CollectionContacts, contactID)
	if err != nil {
		return utils.NotFoundResponse(re, "Contact not found")
	}

	collection, err := app.FindCollectionByNameOrId(utils.CollectionGuestListItems)
	if err != nil {
		return utils.InternalErrorResponse(re, "Collection not found")
	}

	// Get next sort order
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
	record.Set("contact", contactID)
	record.Set("invite_round", input["invite_round"])
	record.Set("invite_status", stringOrDefault(input["invite_status"], "pending"))
	record.Set("notes", input["notes"])
	record.Set("sort_order", nextSort)

	// Denormalize contact fields
	record.Set("contact_name", contact.GetString("name"))
	record.Set("contact_job_title", contact.GetString("job_title"))
	record.Set("contact_organisation_name", orgName)
	record.Set("contact_linkedin", contact.GetString("linkedin"))
	record.Set("contact_location", utils.DecryptField(contact.GetString("location")))
	record.Set("contact_degrees", contact.GetString("degrees"))
	record.Set("contact_relationship", contact.GetInt("relationship"))

	if err := app.Save(record); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return utils.BadRequestResponse(re, "Contact already in this guest list")
		}
		return utils.InternalErrorResponse(re, "Failed to add contact")
	}

	return re.JSON(http.StatusCreated, map[string]any{
		"id":           record.Id,
		"contact_id":   contactID,
		"contact_name": contact.GetString("name"),
	})
}

func handleGuestListItemBulkAdd(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	listID := re.Request.PathValue("id")

	if _, err := app.FindRecordById(utils.CollectionGuestLists, listID); err != nil {
		return utils.NotFoundResponse(re, "Guest list not found")
	}

	var input struct {
		ContactIDs  []string `json:"contact_ids"`
		InviteRound string   `json:"invite_round"`
	}
	if err := json.NewDecoder(re.Request.Body).Decode(&input); err != nil {
		return utils.BadRequestResponse(re, "Invalid JSON")
	}

	if len(input.ContactIDs) == 0 {
		return utils.BadRequestResponse(re, "contact_ids is required")
	}

	collection, err := app.FindCollectionByNameOrId(utils.CollectionGuestListItems)
	if err != nil {
		return utils.InternalErrorResponse(re, "Collection not found")
	}

	nextSort := getNextSortOrder(app, listID)
	added := 0

	for _, contactID := range input.ContactIDs {
		contact, err := app.FindRecordById(utils.CollectionContacts, contactID)
		if err != nil {
			continue
		}

		orgName := ""
		if orgID := contact.GetString("organisation"); orgID != "" {
			if org, err := app.FindRecordById(utils.CollectionOrganisations, orgID); err == nil {
				orgName = org.GetString("name")
			}
		}

		record := core.NewRecord(collection)
		record.Set("guest_list", listID)
		record.Set("contact", contactID)
		record.Set("invite_round", input.InviteRound)
		record.Set("invite_status", "pending")
		record.Set("sort_order", nextSort)
		record.Set("contact_name", contact.GetString("name"))
		record.Set("contact_job_title", contact.GetString("job_title"))
		record.Set("contact_organisation_name", orgName)
		record.Set("contact_linkedin", contact.GetString("linkedin"))
		record.Set("contact_location", utils.DecryptField(contact.GetString("location")))
		record.Set("contact_degrees", contact.GetString("degrees"))
		record.Set("contact_relationship", contact.GetInt("relationship"))

		if err := app.Save(record); err == nil {
			added++
			nextSort++
		}
	}

	return re.JSON(http.StatusOK, map[string]any{"added": added})
}

func handleGuestListItemUpdate(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	itemID := re.Request.PathValue("itemId")
	record, err := app.FindRecordById(utils.CollectionGuestListItems, itemID)
	if err != nil {
		return utils.NotFoundResponse(re, "Guest list item not found")
	}

	var input map[string]any
	if err := json.NewDecoder(re.Request.Body).Decode(&input); err != nil {
		return utils.BadRequestResponse(re, "Invalid JSON")
	}

	if v, ok := input["invite_round"].(string); ok {
		record.Set("invite_round", v)
	}
	if v, ok := input["invite_status"].(string); ok {
		record.Set("invite_status", v)
	}
	if v, ok := input["notes"].(string); ok {
		record.Set("notes", v)
	}
	if v, ok := input["sort_order"].(float64); ok {
		record.Set("sort_order", int(v))
	}

	if err := app.Save(record); err != nil {
		return utils.InternalErrorResponse(re, "Failed to update item")
	}

	return utils.SuccessResponse(re, "Item updated")
}

func handleGuestListItemDelete(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	itemID := re.Request.PathValue("itemId")
	record, err := app.FindRecordById(utils.CollectionGuestListItems, itemID)
	if err != nil {
		return utils.NotFoundResponse(re, "Guest list item not found")
	}

	if err := app.Delete(record); err != nil {
		return utils.InternalErrorResponse(re, "Failed to remove item")
	}

	return utils.SuccessResponse(re, "Item removed")
}

// ============================================================================
// Admin — Share Management
// ============================================================================

func handleGuestListSharesList(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	listID := re.Request.PathValue("id")

	records, err := app.FindRecordsByFilter(
		utils.CollectionGuestListShares,
		"guest_list = {:id}",
		"-created",
		0, 0,
		map[string]any{"id": listID},
	)
	if err != nil {
		return re.JSON(http.StatusOK, map[string]any{"items": []any{}})
	}

	items := make([]map[string]any, len(records))
	for i, r := range records {
		items[i] = map[string]any{
			"id":               r.Id,
			"token":            r.GetString("token"),
			"recipient_email":  r.GetString("recipient_email"),
			"recipient_name":   r.GetString("recipient_name"),
			"expires_at":       r.GetString("expires_at"),
			"revoked":          r.GetBool("revoked"),
			"verified_at":      r.GetString("verified_at"),
			"last_accessed_at": r.GetString("last_accessed_at"),
			"access_count":     r.GetInt("access_count"),
			"created":          r.GetString("created"),
		}
	}

	return re.JSON(http.StatusOK, map[string]any{"items": items})
}

func handleGuestListShareCreate(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	listID := re.Request.PathValue("id")

	guestList, err := app.FindRecordById(utils.CollectionGuestLists, listID)
	if err != nil {
		return utils.NotFoundResponse(re, "Guest list not found")
	}

	var input struct {
		RecipientEmail string `json:"recipient_email"`
		RecipientName  string `json:"recipient_name"`
	}
	if err := json.NewDecoder(re.Request.Body).Decode(&input); err != nil {
		return utils.BadRequestResponse(re, "Invalid JSON")
	}

	if input.RecipientEmail == "" {
		return utils.BadRequestResponse(re, "recipient_email is required")
	}

	token, err := generateToken()
	if err != nil {
		return utils.InternalErrorResponse(re, "Failed to generate token")
	}

	collection, err := app.FindCollectionByNameOrId(utils.CollectionGuestListShares)
	if err != nil {
		return utils.InternalErrorResponse(re, "Collection not found")
	}

	expiresAt := time.Now().AddDate(0, 0, 30).UTC().Format(time.RFC3339)

	record := core.NewRecord(collection)
	record.Set("guest_list", listID)
	record.Set("token", token)
	record.Set("recipient_email", input.RecipientEmail)
	record.Set("recipient_name", input.RecipientName)
	record.Set("expires_at", expiresAt)
	record.Set("revoked", false)
	record.Set("access_count", 0)

	if err := app.Save(record); err != nil {
		return utils.InternalErrorResponse(re, "Failed to create share")
	}

	// Build share URL
	shareURL := fmt.Sprintf("%s/shared/%s", getBaseURL(), token)

	// Get event name for email
	eventName := ""
	if epID := guestList.GetString("event_projection"); epID != "" {
		if ep, err := app.FindRecordById(utils.CollectionEventProjections, epID); err == nil {
			eventName = ep.GetString("name")
		}
	}

	// Send notification email
	go sendShareNotificationEmail(app, input.RecipientEmail, input.RecipientName, shareURL, guestList.GetString("name"), eventName)

	utils.LogFromRequest(app, re, "create", utils.CollectionGuestListShares, record.Id, "success", map[string]any{
		"recipient_email": input.RecipientEmail,
		"guest_list":      listID,
	}, "")

	return re.JSON(http.StatusCreated, map[string]any{
		"id":        record.Id,
		"token":     token,
		"share_url": shareURL,
		"expires_at": expiresAt,
	})
}

func handleGuestListShareRevoke(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	shareID := re.Request.PathValue("shareId")
	record, err := app.FindRecordById(utils.CollectionGuestListShares, shareID)
	if err != nil {
		return utils.NotFoundResponse(re, "Share not found")
	}

	record.Set("revoked", true)
	if err := app.Save(record); err != nil {
		return utils.InternalErrorResponse(re, "Failed to revoke share")
	}

	utils.LogFromRequest(app, re, "delete", utils.CollectionGuestListShares, shareID, "success", nil, "")
	return utils.SuccessResponse(re, "Share revoked")
}

// ============================================================================
// Public Endpoints — Share Access
// ============================================================================

func handlePublicGuestListInfo(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	token := re.Request.PathValue("token")

	share, err := findShareByToken(app, token)
	if err != nil {
		return utils.NotFoundResponse(re, "Share link not found")
	}

	if share.GetBool("revoked") {
		return re.JSON(http.StatusGone, map[string]string{"error": "This share link has been revoked"})
	}

	if isExpired(share.GetString("expires_at")) {
		return re.JSON(http.StatusGone, map[string]string{"error": "This share link has expired"})
	}

	guestList, err := app.FindRecordById(utils.CollectionGuestLists, share.GetString("guest_list"))
	if err != nil {
		return utils.NotFoundResponse(re, "Guest list not found")
	}

	eventName := ""
	if epID := guestList.GetString("event_projection"); epID != "" {
		if ep, err := app.FindRecordById(utils.CollectionEventProjections, epID); err == nil {
			eventName = ep.GetString("name")
		}
	}

	// Mask email
	maskedEmail := maskEmail(share.GetString("recipient_email"))

	return re.JSON(http.StatusOK, map[string]any{
		"list_name":             guestList.GetString("name"),
		"event_name":           eventName,
		"recipient_name":       share.GetString("recipient_name"),
		"masked_email":         maskedEmail,
		"requires_verification": true,
	})
}

func handlePublicGuestListSendOTP(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	token := re.Request.PathValue("token")

	share, err := findShareByToken(app, token)
	if err != nil {
		return utils.NotFoundResponse(re, "Share link not found")
	}

	if share.GetBool("revoked") {
		return re.JSON(http.StatusGone, map[string]string{"error": "This share link has been revoked"})
	}
	if isExpired(share.GetString("expires_at")) {
		return re.JSON(http.StatusGone, map[string]string{"error": "This share link has expired"})
	}

	email := share.GetString("recipient_email")

	// Rate limit: max 3 OTP sends per 10 minutes per share
	tenMinAgo := time.Now().Add(-10 * time.Minute).UTC().Format(time.RFC3339)
	recentCodes, _ := app.FindRecordsByFilter(
		utils.CollectionGuestListOTPCodes,
		"share = {:sid} && created >= {:since}",
		"",
		0, 0,
		map[string]any{"sid": share.Id, "since": tenMinAgo},
	)
	if len(recentCodes) >= 3 {
		return re.JSON(http.StatusTooManyRequests, map[string]string{
			"error": "Too many verification codes requested. Please wait a few minutes.",
		})
	}

	code, err := generateOTPCode()
	if err != nil {
		return utils.InternalErrorResponse(re, "Failed to generate code")
	}

	// Store hashed code
	collection, err := app.FindCollectionByNameOrId(utils.CollectionGuestListOTPCodes)
	if err != nil {
		return utils.InternalErrorResponse(re, "Collection not found")
	}

	otpRecord := core.NewRecord(collection)
	otpRecord.Set("share", share.Id)
	otpRecord.Set("code_hash", hashOTPCode(code))
	otpRecord.Set("email", email)
	otpRecord.Set("expires_at", time.Now().Add(10*time.Minute).UTC().Format(time.RFC3339))
	otpRecord.Set("used", false)
	otpRecord.Set("attempts", 0)
	otpRecord.Set("ip_address", re.RealIP())

	if err := app.Save(otpRecord); err != nil {
		return utils.InternalErrorResponse(re, "Failed to save code")
	}

	// Send email
	go sendOTPEmail(app, email, share.GetString("recipient_name"), code)

	return re.JSON(http.StatusOK, map[string]any{
		"sent":    true,
		"email":   maskEmail(email),
		"expires": 10,
	})
}

func handlePublicGuestListVerify(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	token := re.Request.PathValue("token")

	share, err := findShareByToken(app, token)
	if err != nil {
		return utils.NotFoundResponse(re, "Share link not found")
	}

	if share.GetBool("revoked") {
		return re.JSON(http.StatusGone, map[string]string{"error": "This share link has been revoked"})
	}
	if isExpired(share.GetString("expires_at")) {
		return re.JSON(http.StatusGone, map[string]string{"error": "This share link has expired"})
	}

	var input struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(re.Request.Body).Decode(&input); err != nil {
		return utils.BadRequestResponse(re, "Invalid JSON")
	}

	if input.Code == "" {
		return utils.BadRequestResponse(re, "code is required")
	}

	// Find most recent unused OTP for this share
	otpRecords, err := app.FindRecordsByFilter(
		utils.CollectionGuestListOTPCodes,
		"share = {:sid} && used = false && expires_at >= {:now}",
		"-created",
		1, 0,
		map[string]any{
			"sid": share.Id,
			"now": time.Now().UTC().Format(time.RFC3339),
		},
	)
	if err != nil || len(otpRecords) == 0 {
		return re.JSON(http.StatusUnauthorized, map[string]string{"error": "No valid code found. Please request a new code."})
	}

	otpRecord := otpRecords[0]
	attempts := otpRecord.GetInt("attempts")

	if attempts >= 5 {
		otpRecord.Set("used", true)
		app.Save(otpRecord)
		return re.JSON(http.StatusUnauthorized, map[string]string{"error": "Too many attempts. Please request a new code."})
	}

	// Increment attempts
	otpRecord.Set("attempts", attempts+1)

	if !verifyOTPCode(input.Code, otpRecord.GetString("code_hash")) {
		app.Save(otpRecord)
		remaining := 4 - attempts
		return re.JSON(http.StatusUnauthorized, map[string]any{
			"error":     "Invalid code",
			"remaining": remaining,
		})
	}

	// Mark OTP as used
	otpRecord.Set("used", true)
	app.Save(otpRecord)

	// Mark share as verified
	if share.GetString("verified_at") == "" {
		share.Set("verified_at", time.Now().UTC().Format(time.RFC3339))
		app.Save(share)
	}

	// Create session token (2 hour TTL)
	sessionToken, err := utils.CreateShareSession(share.Id, token, 7200)
	if err != nil {
		return utils.InternalErrorResponse(re, "Failed to create session")
	}

	utils.LogAudit(app, utils.AuditEntry{
		Action:       "read",
		ResourceType: utils.CollectionGuestListShares,
		ResourceID:   share.Id,
		IPAddress:    re.RealIP(),
		UserAgent:    re.Request.UserAgent(),
		Status:       "success",
		Metadata:     map[string]any{"event": "otp_verified"},
	})

	return re.JSON(http.StatusOK, map[string]any{
		"verified":      true,
		"session_token": sessionToken,
		"expires_in":    7200,
	})
}

func handlePublicGuestListView(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	token := re.Request.PathValue("token")

	// Validate session token
	claims, err := validatePublicSession(re, token)
	if err != nil {
		return re.JSON(http.StatusUnauthorized, map[string]string{"error": err.Error()})
	}

	share, err := findShareByToken(app, token)
	if err != nil {
		return utils.NotFoundResponse(re, "Share link not found")
	}

	if share.GetBool("revoked") {
		return re.JSON(http.StatusGone, map[string]string{"error": "This share link has been revoked"})
	}
	if isExpired(share.GetString("expires_at")) {
		return re.JSON(http.StatusGone, map[string]string{"error": "This share link has expired"})
	}

	guestList, err := app.FindRecordById(utils.CollectionGuestLists, share.GetString("guest_list"))
	if err != nil {
		return utils.NotFoundResponse(re, "Guest list not found")
	}

	eventName := ""
	if epID := guestList.GetString("event_projection"); epID != "" {
		if ep, err := app.FindRecordById(utils.CollectionEventProjections, epID); err == nil {
			eventName = ep.GetString("name")
		}
	}

	// Fetch items — read from denormalized fields only (no contact table access)
	records, err := app.FindRecordsByFilter(
		utils.CollectionGuestListItems,
		"guest_list = {:id}",
		"sort_order,created",
		0, 0,
		map[string]any{"id": guestList.Id},
	)
	if err != nil {
		records = []*core.Record{}
	}

	items := make([]map[string]any, len(records))
	for i, r := range records {
		items[i] = map[string]any{
			"id":           r.Id,
			"name":         r.GetString("contact_name"),
			"role":         r.GetString("contact_job_title"),
			"company":      r.GetString("contact_organisation_name"),
			"invite_round": r.GetString("invite_round"),
			"linkedin":     r.GetString("contact_linkedin"),
			"city":         r.GetString("contact_location"),
			"degrees":      r.GetString("contact_degrees"),
			"relationship": r.GetInt("contact_relationship"),
			"notes":        r.GetString("notes"),
			"client_notes": r.GetString("client_notes"),
		}
	}

	// Update access tracking
	share.Set("last_accessed_at", time.Now().UTC().Format(time.RFC3339))
	share.Set("access_count", share.GetInt("access_count")+1)
	app.Save(share)

	utils.LogAudit(app, utils.AuditEntry{
		Action:       "read",
		ResourceType: utils.CollectionGuestListShares,
		ResourceID:   claims.ShareID,
		IPAddress:    re.RealIP(),
		UserAgent:    re.Request.UserAgent(),
		Status:       "success",
		Metadata:     map[string]any{"event": "list_viewed"},
	})

	return re.JSON(http.StatusOK, map[string]any{
		"list_name":    guestList.GetString("name"),
		"event_name":   eventName,
		"items":        items,
		"total_guests": len(items),
		"shared_by":    "The Outlook",
		"shared_at":    share.GetString("created"),
	})
}

func handlePublicGuestListItemUpdate(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	token := re.Request.PathValue("token")
	itemID := re.Request.PathValue("itemId")

	// Validate session
	if _, err := validatePublicSession(re, token); err != nil {
		return re.JSON(http.StatusUnauthorized, map[string]string{"error": err.Error()})
	}

	share, err := findShareByToken(app, token)
	if err != nil {
		return utils.NotFoundResponse(re, "Share link not found")
	}
	if share.GetBool("revoked") || isExpired(share.GetString("expires_at")) {
		return re.JSON(http.StatusGone, map[string]string{"error": "Share link is no longer active"})
	}

	record, err := app.FindRecordById(utils.CollectionGuestListItems, itemID)
	if err != nil {
		return utils.NotFoundResponse(re, "Item not found")
	}

	// Verify item belongs to this shared list
	if record.GetString("guest_list") != share.GetString("guest_list") {
		return utils.ForbiddenResponse(re, "Access denied")
	}

	var input struct {
		InviteRound *string `json:"invite_round"`
		ClientNotes *string `json:"client_notes"`
	}
	if err := json.NewDecoder(re.Request.Body).Decode(&input); err != nil {
		return utils.BadRequestResponse(re, "Invalid JSON")
	}

	if input.InviteRound != nil {
		allowed := map[string]bool{"1st": true, "2nd": true, "maybe": true}
		if !allowed[*input.InviteRound] {
			return utils.BadRequestResponse(re, "Invalid invite_round value")
		}
		record.Set("invite_round", *input.InviteRound)
	}

	if input.ClientNotes != nil {
		notes := *input.ClientNotes
		if len(notes) > 2000 {
			return utils.BadRequestResponse(re, "Notes must be 2000 characters or less")
		}
		record.Set("client_notes", notes)
	}

	if input.InviteRound == nil && input.ClientNotes == nil {
		return utils.BadRequestResponse(re, "No valid fields to update")
	}

	if err := app.Save(record); err != nil {
		return utils.InternalErrorResponse(re, "Failed to update")
	}

	return utils.SuccessResponse(re, "Updated")
}

// ============================================================================
// Helpers
// ============================================================================

func findShareByToken(app *pocketbase.PocketBase, token string) (*core.Record, error) {
	records, err := app.FindRecordsByFilter(
		utils.CollectionGuestListShares,
		"token = {:token}",
		"",
		1, 0,
		map[string]any{"token": token},
	)
	if err != nil || len(records) == 0 {
		return nil, fmt.Errorf("share not found")
	}
	return records[0], nil
}

func isExpired(expiresAt string) bool {
	if expiresAt == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return false
	}
	return time.Now().After(t)
}

func maskEmail(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return "***"
	}
	local := parts[0]
	if len(local) <= 2 {
		return local[:1] + "***@" + parts[1]
	}
	return local[:2] + "***@" + parts[1]
}

func validatePublicSession(re *core.RequestEvent, expectedToken string) (*utils.ShareSessionClaims, error) {
	authHeader := re.Request.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, fmt.Errorf("authorization required")
	}

	sessionToken := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := utils.ValidateShareSession(sessionToken)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired session")
	}

	if claims.Token != expectedToken {
		return nil, fmt.Errorf("session does not match share")
	}

	return claims, nil
}

func countRecords(app *pocketbase.PocketBase, collection, filter string, id string) int {
	records, err := app.FindRecordsByFilter(collection, filter, "", 0, 0, map[string]any{"id": id})
	if err != nil {
		return 0
	}
	return len(records)
}

func getNextSortOrder(app *pocketbase.PocketBase, listID string) int {
	records, err := app.FindRecordsByFilter(
		utils.CollectionGuestListItems,
		"guest_list = {:id}",
		"-sort_order",
		1, 0,
		map[string]any{"id": listID},
	)
	if err != nil || len(records) == 0 {
		return 0
	}
	return records[0].GetInt("sort_order") + 1
}

func stringOrDefault(v any, defaultVal string) string {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return defaultVal
}
