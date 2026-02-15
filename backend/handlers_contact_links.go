package main

import (
	"encoding/json"
	"net/http"

	"github.com/grtshw/outlook-apps-crm/utils"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// handleContactLinksList returns all contacts linked to the given contact
func handleContactLinksList(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	contactID := re.Request.PathValue("id")
	if contactID == "" {
		return utils.BadRequestResponse(re, "Contact ID required")
	}

	// Verify the contact exists
	if _, err := app.FindRecordById(utils.CollectionContacts, contactID); err != nil {
		return utils.NotFoundResponse(re, "Contact not found")
	}

	links, err := getContactLinks(app, contactID)
	if err != nil {
		return utils.InternalErrorResponse(re, "Failed to fetch links")
	}

	return utils.DataResponse(re, links)
}

// handleContactLinkCreate creates a bidirectional link between two contacts
func handleContactLinkCreate(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	contactID := re.Request.PathValue("id")
	if contactID == "" {
		return utils.BadRequestResponse(re, "Contact ID required")
	}

	var input struct {
		TargetContactID string `json:"target_contact_id"`
		Notes           string `json:"notes"`
	}
	if err := json.NewDecoder(re.Request.Body).Decode(&input); err != nil {
		return utils.BadRequestResponse(re, "Invalid request body")
	}

	if input.TargetContactID == "" {
		return utils.BadRequestResponse(re, "Target contact ID required")
	}

	if contactID == input.TargetContactID {
		return utils.BadRequestResponse(re, "Cannot link a contact to itself")
	}

	// Verify both contacts exist
	if _, err := app.FindRecordById(utils.CollectionContacts, contactID); err != nil {
		return utils.NotFoundResponse(re, "Contact not found")
	}
	if _, err := app.FindRecordById(utils.CollectionContacts, input.TargetContactID); err != nil {
		return utils.NotFoundResponse(re, "Target contact not found")
	}

	// Normalise pair order (alphabetical) for dedup
	a, b := contactID, input.TargetContactID
	if a > b {
		a, b = b, a
	}

	// Check for existing link
	existing, _ := app.FindRecordsByFilter(
		utils.CollectionContactLinks,
		"contact_a = {:a} && contact_b = {:b}",
		"", 1, 0,
		map[string]any{"a": a, "b": b},
	)
	if len(existing) > 0 {
		return re.JSON(http.StatusConflict, map[string]string{"error": "These contacts are already linked"})
	}

	// Create the link
	collection, err := app.FindCollectionByNameOrId(utils.CollectionContactLinks)
	if err != nil {
		return utils.InternalErrorResponse(re, "Failed to find contact_links collection")
	}

	record := core.NewRecord(collection)
	record.Set("contact_a", a)
	record.Set("contact_b", b)
	record.Set("verified", true) // Admin-created links are trusted
	record.Set("source", "manual")

	// Set created_by to the admin user ID
	if re.Auth != nil {
		record.Set("created_by", re.Auth.Id)
	}

	if input.Notes != "" {
		record.Set("notes", input.Notes)
	}

	if err := app.Save(record); err != nil {
		return utils.InternalErrorResponse(re, "Failed to create link")
	}

	utils.LogFromRequest(app, re, "create", utils.CollectionContactLinks, record.Id, "success", nil, "")

	return re.JSON(http.StatusCreated, map[string]any{
		"id":         record.Id,
		"contact_a":  a,
		"contact_b":  b,
		"verified":   true,
		"source":     "manual",
		"created_by": record.GetString("created_by"),
		"notes":      record.GetString("notes"),
		"created":    record.GetString("created"),
	})
}

// handleContactLinkDelete removes a link between two contacts
func handleContactLinkDelete(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	linkID := re.Request.PathValue("linkId")
	if linkID == "" {
		return utils.BadRequestResponse(re, "Link ID required")
	}

	record, err := app.FindRecordById(utils.CollectionContactLinks, linkID)
	if err != nil {
		return utils.NotFoundResponse(re, "Link not found")
	}

	if err := app.Delete(record); err != nil {
		return utils.InternalErrorResponse(re, "Failed to delete link")
	}

	utils.LogFromRequest(app, re, "delete", utils.CollectionContactLinks, linkID, "success", nil, "")

	return utils.SuccessResponse(re, "Link removed")
}

// getContactLinks returns all linked contacts for a given contact ID
// with basic profile info for each linked contact
func getContactLinks(app *pocketbase.PocketBase, contactID string) ([]map[string]any, error) {
	// Find links where this contact is on either side
	linksA, err := app.FindRecordsByFilter(
		utils.CollectionContactLinks,
		"contact_a = {:id}",
		"-created", 0, 0,
		map[string]any{"id": contactID},
	)
	if err != nil {
		return nil, err
	}

	linksB, err := app.FindRecordsByFilter(
		utils.CollectionContactLinks,
		"contact_b = {:id}",
		"-created", 0, 0,
		map[string]any{"id": contactID},
	)
	if err != nil {
		return nil, err
	}

	allLinks := append(linksA, linksB...)

	result := make([]map[string]any, 0, len(allLinks))
	for _, link := range allLinks {
		// Determine which side is the "other" contact
		otherID := link.GetString("contact_a")
		if otherID == contactID {
			otherID = link.GetString("contact_b")
		}

		// Fetch basic profile info for the other contact
		other, err := app.FindRecordById(utils.CollectionContacts, otherID)
		if err != nil {
			continue // Skip if contact was deleted
		}

		result = append(result, map[string]any{
			"link_id":          link.Id,
			"contact_id":       otherID,
			"name":             other.GetString("first_name") + " " + other.GetString("last_name"),
			"email":            utils.DecryptField(other.GetString("email")),
			"avatar_thumb_url": other.GetString("avatar_thumb_url"),
			"organisation":     resolveOrgName(app, other.GetString("organisation")),
			"verified":         link.GetBool("verified"),
			"source":           link.GetString("source"),
			"notes":            link.GetString("notes"),
			"created":          link.GetString("created"),
		})
	}

	return result, nil
}

// resolveOrgName fetches the organisation name for a given org ID
func resolveOrgName(app *pocketbase.PocketBase, orgID string) string {
	if orgID == "" {
		return ""
	}
	org, err := app.FindRecordById(utils.CollectionOrganisations, orgID)
	if err != nil {
		return ""
	}
	return org.GetString("name")
}
