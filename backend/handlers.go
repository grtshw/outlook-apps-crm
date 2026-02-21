package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/grtshw/outlook-apps-crm/utils"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// --- Public API Handlers (for COPE projections) ---

// handlePublicContacts returns contacts for COPE consumers
func handlePublicContacts(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	// Only return active contacts (not archived)
	records, err := app.FindRecordsByFilter(utils.CollectionContacts, "status != 'archived'", "name", 1000, 0)
	if err != nil {
		return utils.DataResponse(re, map[string]any{"items": []any{}})
	}

	baseURL := getBaseURL()
	items := make([]map[string]any, len(records))
	for i, r := range records {
		items[i] = buildContactProjection(r, app, baseURL)
	}

	return utils.DataResponse(re, map[string]any{"items": items})
}

// handlePublicOrganisations returns organisations for COPE consumers
func handlePublicOrganisations(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	// Only return active organisations (not archived)
	records, err := app.FindRecordsByFilter(utils.CollectionOrganisations, "status != 'archived'", "name", 1000, 0)
	if err != nil {
		return utils.DataResponse(re, map[string]any{"items": []any{}})
	}

	baseURL := getBaseURL()
	items := make([]map[string]any, len(records))
	for i, r := range records {
		items[i] = buildOrganisationProjection(r, baseURL)
	}

	return utils.DataResponse(re, map[string]any{"items": items})
}

// --- External API Handlers (for Presentations self-registration) ---

// handleExternalContactCreate creates a contact from an external service (Presentations)
// Auth: Service token via X-Service-Token header
func handleExternalContactCreate(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	// Validate service token
	serviceToken := os.Getenv("PRESENTATIONS_SERVICE_TOKEN")
	if serviceToken == "" {
		return utils.InternalErrorResponse(re, "External contact creation not configured")
	}

	providedToken := re.Request.Header.Get("X-Service-Token")
	if providedToken == "" || providedToken != serviceToken {
		return re.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid service token"})
	}

	var input map[string]any
	if err := json.NewDecoder(re.Request.Body).Decode(&input); err != nil {
		return utils.BadRequestResponse(re, "Invalid request body")
	}

	// Validate required fields
	email, _ := input["email"].(string)
	if email == "" {
		return utils.BadRequestResponse(re, "Email is required")
	}

	// Accept first_name/last_name or fall back to splitting name
	firstName, _ := input["first_name"].(string)
	lastName, _ := input["last_name"].(string)
	if firstName == "" {
		name, _ := input["name"].(string)
		if name == "" {
			return utils.BadRequestResponse(re, "Name is required")
		}
		parts := strings.SplitN(strings.TrimSpace(name), " ", 2)
		firstName = parts[0]
		if len(parts) > 1 {
			lastName = parts[1]
		}
	}

	// Normalize email
	email = strings.ToLower(strings.TrimSpace(email))

	// Check for existing contact by email (search by blind index if encryption is enabled)
	var existing []*core.Record
	if utils.IsEncryptionEnabled() {
		blindIndex := utils.BlindIndex(email)
		existing, _ = app.FindRecordsByFilter(utils.CollectionContacts, "email_index = {:idx}", "", 1, 0, map[string]any{"idx": blindIndex})
	} else {
		existing, _ = app.FindRecordsByFilter(utils.CollectionContacts, "email = {:email}", "", 1, 0, map[string]any{"email": email})
	}
	if len(existing) > 0 {
		// Return existing contact instead of error (decrypt email)
		return utils.DataResponse(re, map[string]any{
			"id":       existing[0].Id,
			"email":    utils.DecryptField(existing[0].GetString("email")),
			"name":     strings.TrimSpace(existing[0].GetString("first_name") + " " + existing[0].GetString("last_name")),
			"existing": true,
		})
	}

	collection, err := app.FindCollectionByNameOrId(utils.CollectionContacts)
	if err != nil {
		return utils.InternalErrorResponse(re, "Failed to find contacts collection")
	}

	record := core.NewRecord(collection)

	// Set fields
	record.Set("email", email)
	record.Set("first_name", firstName)
	record.Set("last_name", lastName)
	record.Set("name", strings.TrimSpace(firstName+" "+lastName))
	record.Set("source", "presentations") // Always mark source
	record.Set("status", "active")

	// Optional fields
	if v, ok := input["phone"].(string); ok {
		record.Set("phone", v)
	}
	if v, ok := input["pronouns"].(string); ok {
		record.Set("pronouns", v)
	}
	if v, ok := input["bio"].(string); ok {
		record.Set("bio", v)
	}
	if v, ok := input["job_title"].(string); ok {
		record.Set("job_title", v)
	}
	if v, ok := input["linkedin"].(string); ok {
		record.Set("linkedin", v)
	}
	if v, ok := input["instagram"].(string); ok {
		record.Set("instagram", v)
	}
	if v, ok := input["website"].(string); ok {
		record.Set("website", v)
	}
	if v, ok := input["location"].(string); ok {
		record.Set("location", v)
	}
	if v, ok := input["organisation"].(string); ok {
		record.Set("organisation", v)
	}

	if err := app.Save(record); err != nil {
		log.Printf("[ExternalContactCreate] Failed to save: %v", err)
		return utils.BadRequestResponse(re, err.Error())
	}

	fullName := strings.TrimSpace(firstName + " " + lastName)
	log.Printf("[ExternalContactCreate] Created contact: id=%s email=%s", record.Id, email)

	// Webhook fires automatically via PocketBase hooks
	// Return original (unencrypted) values to the caller
	return re.JSON(http.StatusCreated, map[string]any{
		"id":       record.Id,
		"email":    email, // Use original email, not encrypted record value
		"name":     fullName,
		"existing": false,
	})
}

// handleExternalContactUpdate updates a contact from an external service (Presentations)
// Auth: Service token via X-Service-Token header
func handleExternalContactUpdate(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	// Validate service token
	serviceToken := os.Getenv("PRESENTATIONS_SERVICE_TOKEN")
	if serviceToken == "" {
		return utils.InternalErrorResponse(re, "External contact update not configured")
	}

	providedToken := re.Request.Header.Get("X-Service-Token")
	if providedToken == "" || providedToken != serviceToken {
		return re.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid service token"})
	}

	id := re.Request.PathValue("id")
	if id == "" {
		return utils.BadRequestResponse(re, "Contact ID required")
	}

	record, err := app.FindRecordById(utils.CollectionContacts, id)
	if err != nil {
		return utils.NotFoundResponse(re, "Contact not found")
	}

	var input map[string]any
	if err := json.NewDecoder(re.Request.Body).Decode(&input); err != nil {
		return utils.BadRequestResponse(re, "Invalid request body")
	}

	// Allowed fields for external update
	allowedFields := []string{
		"first_name", "last_name", "preferred_name", "phone", "pronouns", "bio", "job_title",
		"linkedin", "instagram", "website", "location",
	}

	for _, field := range allowedFields {
		if val, ok := input[field]; ok {
			record.Set(field, val)
		}
	}

	// Backwards compatibility: accept "name" and split into first/last
	if name, ok := input["name"].(string); ok && name != "" {
		parts := strings.SplitN(strings.TrimSpace(name), " ", 2)
		record.Set("first_name", parts[0])
		if len(parts) > 1 {
			record.Set("last_name", parts[1])
		}
	}

	// Recompute denormalized name
	fn := record.GetString("first_name")
	ln := record.GetString("last_name")
	record.Set("name", strings.TrimSpace(fn+" "+ln))

	// Decrypt PII fields before save so PocketBase validation passes
	// (the encryption hook will re-encrypt them before DB write)
	for _, field := range []string{"email", "personal_email", "phone", "bio", "location"} {
		val := record.GetString(field)
		if val != "" {
			record.Set(field, utils.DecryptField(val))
		}
	}

	if err := app.Save(record); err != nil {
		log.Printf("[ExternalContactUpdate] Failed to save: %v", err)
		return utils.InternalErrorResponse(re, "Failed to update contact")
	}

	log.Printf("[ExternalContactUpdate] Updated contact: id=%s", record.Id)

	// Webhook fires automatically via PocketBase hooks
	return utils.DataResponse(re, map[string]any{
		"id":      record.Id,
		"email":   record.GetString("email"),
		"name":    strings.TrimSpace(record.GetString("first_name") + " " + record.GetString("last_name")),
		"updated": true,
	})
}

// handleExternalOrganisationCreate creates an organisation from an external service (Presentations)
// Auth: Service token via X-Service-Token header
func handleExternalOrganisationCreate(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	// Validate service token
	serviceToken := os.Getenv("PRESENTATIONS_SERVICE_TOKEN")
	if serviceToken == "" {
		return utils.InternalErrorResponse(re, "External organisation creation not configured")
	}

	providedToken := re.Request.Header.Get("X-Service-Token")
	if providedToken == "" || providedToken != serviceToken {
		return re.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid service token"})
	}

	var input map[string]any
	if err := json.NewDecoder(re.Request.Body).Decode(&input); err != nil {
		return utils.BadRequestResponse(re, "Invalid request body")
	}

	// Validate required fields
	name, _ := input["name"].(string)
	if name == "" {
		return utils.BadRequestResponse(re, "Name is required")
	}

	// Check for existing organisation by name
	existing, _ := app.FindRecordsByFilter(utils.CollectionOrganisations, "name = {:name}", "", 1, 0, map[string]any{"name": name})
	if len(existing) > 0 {
		// Return existing organisation instead of error
		return utils.DataResponse(re, map[string]any{
			"id":       existing[0].Id,
			"name":     existing[0].GetString("name"),
			"existing": true,
		})
	}

	collection, err := app.FindCollectionByNameOrId(utils.CollectionOrganisations)
	if err != nil {
		return utils.InternalErrorResponse(re, "Failed to find organisations collection")
	}

	record := core.NewRecord(collection)

	// Set fields
	record.Set("name", name)
	record.Set("status", "active")

	// Optional fields
	if v, ok := input["website"].(string); ok {
		record.Set("website", v)
	}
	if v, ok := input["linkedin"].(string); ok {
		record.Set("linkedin", v)
	}
	if v, ok := input["description_short"].(string); ok {
		record.Set("description_short", v)
	}
	if v, ok := input["description_medium"].(string); ok {
		record.Set("description_medium", v)
	}
	if v, ok := input["description_long"].(string); ok {
		record.Set("description_long", v)
	}
	if v, ok := input["contacts"]; ok {
		record.Set("contacts", v)
	}

	if err := app.Save(record); err != nil {
		log.Printf("[ExternalOrgCreate] Failed to save: %v", err)
		return utils.InternalErrorResponse(re, "Failed to create organisation")
	}

	log.Printf("[ExternalOrgCreate] Created organisation: id=%s, name=%s", record.Id, name)

	// Webhook fires automatically via PocketBase hooks
	return re.JSON(http.StatusCreated, map[string]any{
		"id":   record.Id,
		"name": record.GetString("name"),
	})
}

// handleExternalOrganisationUpdate updates an organisation from an external service (Presentations)
// Auth: Service token via X-Service-Token header
func handleExternalOrganisationUpdate(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	// Validate service token
	serviceToken := os.Getenv("PRESENTATIONS_SERVICE_TOKEN")
	if serviceToken == "" {
		return utils.InternalErrorResponse(re, "External organisation update not configured")
	}

	providedToken := re.Request.Header.Get("X-Service-Token")
	if providedToken == "" || providedToken != serviceToken {
		return re.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid service token"})
	}

	id := re.Request.PathValue("id")
	if id == "" {
		return utils.BadRequestResponse(re, "Organisation ID required")
	}

	record, err := app.FindRecordById(utils.CollectionOrganisations, id)
	if err != nil {
		return utils.NotFoundResponse(re, "Organisation not found")
	}

	var input map[string]any
	if err := json.NewDecoder(re.Request.Body).Decode(&input); err != nil {
		return utils.BadRequestResponse(re, "Invalid request body")
	}

	// Allowed fields for external update
	allowedFields := []string{
		"name", "website", "linkedin",
		"description_short", "description_medium", "description_long",
		"contacts",
	}

	for _, field := range allowedFields {
		if val, ok := input[field]; ok {
			record.Set(field, val)
		}
	}

	if err := app.Save(record); err != nil {
		log.Printf("[ExternalOrgUpdate] Failed to save: %v", err)
		return utils.InternalErrorResponse(re, "Failed to update organisation")
	}

	log.Printf("[ExternalOrgUpdate] Updated organisation: id=%s", record.Id)

	// Webhook fires automatically via PocketBase hooks
	return utils.DataResponse(re, map[string]any{
		"id":      record.Id,
		"name":    record.GetString("name"),
		"updated": true,
	})
}

// --- Dashboard Handlers ---

// handleDashboardStats returns dashboard statistics
func handleDashboardStats(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	// Count contacts by status using CountRecords (avoids loading all records into memory)
	activeContacts, _ := app.CountRecords(utils.CollectionContacts, dbx.NewExp("status = 'active'"))
	inactiveContacts, _ := app.CountRecords(utils.CollectionContacts, dbx.NewExp("status = 'inactive'"))
	archivedContacts, _ := app.CountRecords(utils.CollectionContacts, dbx.NewExp("status = 'archived'"))

	// Count organisations by status
	activeOrgs, _ := app.CountRecords(utils.CollectionOrganisations, dbx.NewExp("status = 'active'"))
	archivedOrgs, _ := app.CountRecords(utils.CollectionOrganisations, dbx.NewExp("status = 'archived'"))

	// Count recent activities (last 30 days)
	thirtyDaysAgo := time.Now().UTC().AddDate(0, 0, -30).Format("2006-01-02 15:04:05.000Z")
	recentActivities, _ := app.CountRecords(utils.CollectionActivities, dbx.NewExp("created >= {:since}", dbx.Params{"since": thirtyDaysAgo}))

	return utils.DataResponse(re, map[string]any{
		"contacts": map[string]int64{
			"active":   activeContacts,
			"inactive": inactiveContacts,
			"archived": archivedContacts,
			"total":    activeContacts + inactiveContacts + archivedContacts,
		},
		"organisations": map[string]int64{
			"active":   activeOrgs,
			"archived": archivedOrgs,
			"total":    activeOrgs + archivedOrgs,
		},
		"recent_activities": recentActivities,
	})
}

// --- Contacts Handlers ---

// handleContactsList returns paginated contacts list
func handleContactsList(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	// Parse query params
	page, _ := strconv.Atoi(re.Request.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(re.Request.URL.Query().Get("perPage"))
	if perPage < 1 || perPage > 500 {
		perPage = 50
	}
	search := re.Request.URL.Query().Get("search")
	status := re.Request.URL.Query().Get("status")
	humanitixEvent := re.Request.URL.Query().Get("humanitix_event")
	sort := re.Request.URL.Query().Get("sort")
	if sort == "" {
		sort = "name"
	}

	// If filtering by Humanitix event, find contact IDs from activities first
	var humanitixContactIDs []string
	if humanitixEvent != "" {
		activities, _ := app.FindRecordsByFilter(
			utils.CollectionActivities,
			"source_app = 'humanitix' && type = 'ticket_purchased'",
			"", 0, 0, nil,
		)
		seen := map[string]bool{}
		for _, a := range activities {
			meta := a.Get("metadata")
			if metaMap, ok := meta.(map[string]any); ok {
				if eid, ok := metaMap["event_id"].(string); ok && eid == humanitixEvent {
					cid := a.GetString("contact")
					if cid != "" && !seen[cid] {
						humanitixContactIDs = append(humanitixContactIDs, cid)
						seen[cid] = true
					}
				}
			}
		}
		if len(humanitixContactIDs) == 0 {
			return utils.DataResponse(re, map[string]any{
				"items":      []any{},
				"page":       page,
				"perPage":    perPage,
				"totalItems": 0,
				"totalPages": 0,
			})
		}
	}

	// Build filter
	filter := ""
	if status != "" {
		filter = "status = {:status}"
	}
	if search != "" {
		// Search by name fields with LIKE, and by email using blind index (exact match)
		searchFilter := "(name ~ {:search} || first_name ~ {:search} || last_name ~ {:search}"
		if utils.IsEncryptionEnabled() {
			searchFilter += " || email_index = {:emailIdx}"
		} else {
			searchFilter += " || email ~ {:search}"
		}
		searchFilter += ")"
		if filter != "" {
			filter = filter + " && " + searchFilter
		} else {
			filter = searchFilter
		}
	}
	if len(humanitixContactIDs) > 0 {
		// Build an IN clause with parameterized IDs
		idPlaceholders := make([]string, len(humanitixContactIDs))
		for i := range humanitixContactIDs {
			idPlaceholders[i] = fmt.Sprintf("{:hid%d}", i)
		}
		idFilter := fmt.Sprintf("id IN (%s)", strings.Join(idPlaceholders, ", "))
		if filter != "" {
			filter = filter + " && " + idFilter
		} else {
			filter = idFilter
		}
	}

	params := map[string]any{
		"status":   status,
		"search":   search,
		"emailIdx": utils.BlindIndex(strings.ToLower(strings.TrimSpace(search))),
	}
	for i, cid := range humanitixContactIDs {
		params[fmt.Sprintf("hid%d", i)] = cid
	}

	// Get total count
	allRecords, _ := app.FindRecordsByFilter(utils.CollectionContacts, filter, "", 0, 0, params)
	totalItems := len(allRecords)

	// Get paginated records
	offset := (page - 1) * perPage
	records, err := app.FindRecordsByFilter(utils.CollectionContacts, filter, sort, perPage, offset, params)
	if err != nil {
		return utils.DataResponse(re, map[string]any{
			"items":      []any{},
			"page":       page,
			"perPage":    perPage,
			"totalItems": 0,
			"totalPages": 0,
		})
	}

	baseURL := getBaseURL()
	items := make([]map[string]any, len(records))
	for i, r := range records {
		items[i] = buildContactResponse(r, app, baseURL)
	}

	totalPages := (totalItems + perPage - 1) / perPage

	return utils.DataResponse(re, map[string]any{
		"items":      items,
		"page":       page,
		"perPage":    perPage,
		"totalItems": totalItems,
		"totalPages": totalPages,
	})
}

// handleContactGet returns a single contact by ID
func handleContactGet(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	id := re.Request.PathValue("id")
	if id == "" {
		return utils.BadRequestResponse(re, "Contact ID required")
	}

	record, err := app.FindRecordById(utils.CollectionContacts, id)
	if err != nil {
		return utils.NotFoundResponse(re, "Contact not found")
	}

	baseURL := getBaseURL()
	data := buildContactResponse(record, app, baseURL)

	// Include linked contacts for single-contact detail view
	if links, err := getContactLinks(app, id); err == nil {
		data["linked_contacts"] = links
	}

	return utils.DataResponse(re, data)
}

// handleContactCreate creates a new contact
func handleContactCreate(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	var input map[string]any
	if err := json.NewDecoder(re.Request.Body).Decode(&input); err != nil {
		return utils.BadRequestResponse(re, "Invalid request body")
	}

	// Validate required fields
	email, _ := input["email"].(string)
	firstName, _ := input["first_name"].(string)
	if email == "" {
		return utils.BadRequestResponse(re, "Email is required")
	}
	if firstName == "" {
		return utils.BadRequestResponse(re, "First name is required")
	}
	lastName, _ := input["last_name"].(string)

	// Check for duplicate email using blind index
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	var existing []*core.Record
	if utils.IsEncryptionEnabled() {
		blindIndex := utils.BlindIndex(normalizedEmail)
		existing, _ = app.FindRecordsByFilter(utils.CollectionContacts, "email_index = {:idx}", "", 1, 0, map[string]any{"idx": blindIndex})
	} else {
		existing, _ = app.FindRecordsByFilter(utils.CollectionContacts, "email = {:email}", "", 1, 0, map[string]any{"email": normalizedEmail})
	}
	if len(existing) > 0 {
		return utils.BadRequestResponse(re, "A contact with this email already exists")
	}

	collection, err := app.FindCollectionByNameOrId(utils.CollectionContacts)
	if err != nil {
		return utils.InternalErrorResponse(re, "Failed to find contacts collection")
	}

	record := core.NewRecord(collection)

	// Set fields
	record.Set("email", strings.ToLower(strings.TrimSpace(email)))
	record.Set("first_name", firstName)
	record.Set("last_name", lastName)
	record.Set("name", strings.TrimSpace(firstName+" "+lastName))
	if v, ok := input["personal_email"].(string); ok && v != "" {
		record.Set("personal_email", strings.ToLower(strings.TrimSpace(v)))
	}
	if v, ok := input["phone"].(string); ok {
		record.Set("phone", v)
	}
	if v, ok := input["pronouns"].(string); ok {
		record.Set("pronouns", v)
	}
	if v, ok := input["bio"].(string); ok {
		record.Set("bio", v)
	}
	if v, ok := input["job_title"].(string); ok {
		record.Set("job_title", v)
	}
	if v, ok := input["linkedin"].(string); ok {
		record.Set("linkedin", v)
	}
	if v, ok := input["instagram"].(string); ok {
		record.Set("instagram", v)
	}
	if v, ok := input["website"].(string); ok {
		record.Set("website", v)
	}
	if v, ok := input["location"].(string); ok {
		record.Set("location", v)
	}
	if v, ok := input["organisation"].(string); ok {
		record.Set("organisation", v)
	}
	if v, ok := input["tags"].([]any); ok {
		record.Set("tags", v)
	}
	if v, ok := input["roles"].([]any); ok {
		record.Set("roles", v)
	}
	if v, ok := input["do_position"].(string); ok {
		record.Set("do_position", v)
	}
	if v, ok := input["status"].(string); ok {
		record.Set("status", v)
	} else {
		record.Set("status", "active")
	}
	if v, ok := input["source"].(string); ok {
		record.Set("source", v)
	} else {
		record.Set("source", "manual")
	}
	if v, ok := input["domain"]; ok {
		record.Set("domain", v)
	}
	if v, ok := input["degrees"].(string); ok {
		record.Set("degrees", v)
	}
	if v, ok := input["relationship"].(float64); ok {
		record.Set("relationship", int(v))
	}
	if v, ok := input["notes"].(string); ok {
		record.Set("notes", v)
	}
	if v, ok := input["dietary_requirements"].([]any); ok {
		record.Set("dietary_requirements", v)
	}
	if v, ok := input["dietary_requirements_other"].(string); ok {
		record.Set("dietary_requirements_other", v)
	}
	if v, ok := input["accessibility_requirements"].([]any); ok {
		record.Set("accessibility_requirements", v)
	}
	if v, ok := input["accessibility_requirements_other"].(string); ok {
		record.Set("accessibility_requirements_other", v)
	}

	if err := app.Save(record); err != nil {
		log.Printf("[ContactCreate] Failed to save: %v", err)
		return utils.InternalErrorResponse(re, "Failed to create contact")
	}

	baseURL := getBaseURL()
	return re.JSON(http.StatusCreated, buildContactResponse(record, app, baseURL))
}

// handleContactUpdate updates an existing contact
func handleContactUpdate(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	id := re.Request.PathValue("id")
	if id == "" {
		return utils.BadRequestResponse(re, "Contact ID required")
	}

	record, err := app.FindRecordById(utils.CollectionContacts, id)
	if err != nil {
		return utils.NotFoundResponse(re, "Contact not found")
	}

	var input map[string]any
	if err := json.NewDecoder(re.Request.Body).Decode(&input); err != nil {
		return utils.BadRequestResponse(re, "Invalid request body")
	}

	// Update fields if provided
	if v, ok := input["first_name"].(string); ok {
		record.Set("first_name", v)
	}
	if v, ok := input["last_name"].(string); ok {
		record.Set("last_name", v)
	}
	// Recompute denormalized name
	fn := record.GetString("first_name")
	ln := record.GetString("last_name")
	record.Set("name", strings.TrimSpace(fn+" "+ln))

	if v, ok := input["email"].(string); ok && v != "" {
		// Check for duplicate email using blind index (excluding current record)
		email := strings.ToLower(strings.TrimSpace(v))
		var existing []*core.Record
		if utils.IsEncryptionEnabled() {
			blindIndex := utils.BlindIndex(email)
			existing, _ = app.FindRecordsByFilter(utils.CollectionContacts, "email_index = {:idx} && id != {:id}", "", 1, 0, map[string]any{"idx": blindIndex, "id": id})
		} else {
			existing, _ = app.FindRecordsByFilter(utils.CollectionContacts, "email = {:email} && id != {:id}", "", 1, 0, map[string]any{"email": email, "id": id})
		}
		if len(existing) > 0 {
			return utils.BadRequestResponse(re, "A contact with this email already exists")
		}
		record.Set("email", email)
	}
	if v, ok := input["personal_email"].(string); ok {
		if v != "" {
			record.Set("personal_email", strings.ToLower(strings.TrimSpace(v)))
		} else {
			record.Set("personal_email", "")
			record.Set("personal_email_index", "")
		}
	}
	if v, ok := input["phone"].(string); ok {
		record.Set("phone", v)
	}
	if v, ok := input["pronouns"].(string); ok {
		record.Set("pronouns", v)
	}
	if v, ok := input["bio"].(string); ok {
		record.Set("bio", v)
	}
	if v, ok := input["job_title"].(string); ok {
		record.Set("job_title", v)
	}
	if v, ok := input["linkedin"].(string); ok {
		record.Set("linkedin", v)
	}
	if v, ok := input["instagram"].(string); ok {
		record.Set("instagram", v)
	}
	if v, ok := input["website"].(string); ok {
		record.Set("website", v)
	}
	if v, ok := input["location"].(string); ok {
		record.Set("location", v)
	}
	if v, ok := input["organisation"].(string); ok {
		record.Set("organisation", v)
	}
	if v, ok := input["tags"].([]any); ok {
		record.Set("tags", v)
	}
	if v, ok := input["roles"].([]any); ok {
		record.Set("roles", v)
	}
	if v, ok := input["do_position"].(string); ok {
		record.Set("do_position", v)
	}
	if v, ok := input["status"].(string); ok {
		record.Set("status", v)
	}
	if v, ok := input["domain"]; ok {
		record.Set("domain", v)
	}
	if v, ok := input["degrees"].(string); ok {
		record.Set("degrees", v)
	}
	if v, ok := input["relationship"].(float64); ok {
		record.Set("relationship", int(v))
	}
	if v, ok := input["notes"].(string); ok {
		record.Set("notes", v)
	}
	if v, ok := input["dietary_requirements"].([]any); ok {
		record.Set("dietary_requirements", v)
	}
	if v, ok := input["dietary_requirements_other"].(string); ok {
		record.Set("dietary_requirements_other", v)
	}
	if v, ok := input["accessibility_requirements"].([]any); ok {
		record.Set("accessibility_requirements", v)
	}
	if v, ok := input["accessibility_requirements_other"].(string); ok {
		record.Set("accessibility_requirements_other", v)
	}

	// Decrypt PII fields before save so PocketBase validation passes
	// (encrypted values like "enc:..." fail EmailField validation).
	// The OnRecordUpdateExecute hook will re-encrypt them.
	for _, field := range []string{"email", "personal_email", "phone", "bio", "location"} {
		val := record.GetString(field)
		if val != "" {
			record.Set(field, utils.DecryptField(val))
		}
	}

	if err := app.Save(record); err != nil {
		log.Printf("[ContactUpdate] Failed to save: %v", err)
		return utils.InternalErrorResponse(re, "Failed to update contact")
	}

	baseURL := getBaseURL()
	return utils.DataResponse(re, buildContactResponse(record, app, baseURL))
}

// handleContactDelete deletes a contact
func handleContactDelete(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	id := re.Request.PathValue("id")
	if id == "" {
		return utils.BadRequestResponse(re, "Contact ID required")
	}

	record, err := app.FindRecordById(utils.CollectionContacts, id)
	if err != nil {
		return utils.NotFoundResponse(re, "Contact not found")
	}

	if err := app.Delete(record); err != nil {
		log.Printf("[ContactDelete] Failed to delete: %v", err)
		return utils.InternalErrorResponse(re, "Failed to delete contact")
	}

	return utils.SuccessResponse(re, "Contact deleted successfully")
}

// handleContactAvatarUpload handles avatar file upload for a contact.
// The file is proxied to DAM via HMAC-authenticated endpoint. CRM does not store files locally.
func handleContactAvatarUpload(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	id := re.Request.PathValue("id")
	if id == "" {
		return utils.BadRequestResponse(re, "Contact ID required")
	}

	record, err := app.FindRecordById(utils.CollectionContacts, id)
	if err != nil {
		return utils.NotFoundResponse(re, "Contact not found")
	}

	// Parse multipart form
	if err := re.Request.ParseMultipartForm(5 << 20); err != nil { // 5MB max
		return utils.BadRequestResponse(re, "Failed to parse form data")
	}

	file, header, err := re.Request.FormFile("avatar")
	if err != nil {
		return utils.BadRequestResponse(re, "No avatar file provided")
	}
	defer file.Close()

	// Validate file type (reject SVG which can contain JavaScript)
	contentType := header.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") || contentType == "image/svg+xml" {
		return utils.BadRequestResponse(re, "File must be a raster image (JPEG, PNG, WebP)")
	}

	// Read file content
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return utils.InternalErrorResponse(re, "Failed to read file")
	}

	// Proxy upload to DAM
	damURL := os.Getenv("DAM_PUBLIC_URL")
	if damURL == "" {
		damURL = "https://outlook-apps-dam.fly.dev"
	}

	secret := os.Getenv("PROJECTION_WEBHOOK_SECRET")
	if secret == "" {
		return utils.InternalErrorResponse(re, "Projection webhook secret not configured")
	}

	// Generate HMAC signature: crm_id:avatar:timestamp:upload
	timestamp := time.Now().UTC().Format(time.RFC3339)
	tokenData := fmt.Sprintf("%s:avatar:%s:upload", id, timestamp)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(tokenData))
	signature := hex.EncodeToString(mac.Sum(nil))

	// Build multipart request to DAM
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("avatar", header.Filename)
	if err != nil {
		return utils.InternalErrorResponse(re, "Failed to build upload request")
	}
	if _, err := part.Write(fileBytes); err != nil {
		return utils.InternalErrorResponse(re, "Failed to build upload request")
	}
	writer.Close()

	damReq, err := http.NewRequest("POST", damURL+"/api/avatar/"+id, &buf)
	if err != nil {
		return utils.InternalErrorResponse(re, "Failed to create DAM request")
	}
	damReq.Header.Set("Content-Type", writer.FormDataContentType())
	damReq.Header.Set("X-Upload-Signature", signature)
	damReq.Header.Set("X-Upload-Timestamp", timestamp)

	resp, err := http.DefaultClient.Do(damReq)
	if err != nil {
		log.Printf("[ContactAvatarUpload] Failed to proxy to DAM: %v", err)
		return utils.InternalErrorResponse(re, "Failed to upload avatar to DAM")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[ContactAvatarUpload] DAM returned %d: %s", resp.StatusCode, string(respBody))
		return utils.InternalErrorResponse(re, "DAM rejected avatar upload")
	}

	// Parse DAM response to get avatar URLs
	var damResp map[string]any
	json.NewDecoder(resp.Body).Decode(&damResp)

	// Update avatar_url on the contact record with the DAM URL
	if avatarURL, ok := damResp["avatar_url"].(string); ok && avatarURL != "" {
		record.Set("avatar_url", avatarURL)

		// Decrypt PII fields before saving so PocketBase validation passes
		piiFields := []string{"email", "personal_email", "phone", "bio", "location"}
		for _, field := range piiFields {
			if v := record.GetString(field); v != "" {
				record.Set(field, utils.DecryptField(v))
			}
		}

		if err := app.Save(record); err != nil {
			log.Printf("[ContactAvatarUpload] Failed to save avatar_url: %v", err)
		}
	}

	log.Printf("[ContactAvatarUpload] Proxied avatar to DAM for contact %s", id)

	baseURL := getBaseURL()
	return utils.DataResponse(re, buildContactResponse(record, app, baseURL))
}

// handleContactActivities returns activities for a contact
func handleContactActivities(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	id := re.Request.PathValue("id")
	if id == "" {
		return utils.BadRequestResponse(re, "Contact ID required")
	}

	// Verify contact exists
	_, err := app.FindRecordById(utils.CollectionContacts, id)
	if err != nil {
		return utils.NotFoundResponse(re, "Contact not found")
	}

	// Get activities for this contact
	records, err := app.FindRecordsByFilter(utils.CollectionActivities, "contact = {:contactId}", "-occurred_at", 100, 0, map[string]any{"contactId": id})
	if err != nil {
		return utils.DataResponse(re, []any{})
	}

	items := make([]map[string]any, len(records))
	for i, r := range records {
		items[i] = buildActivityResponse(r)
	}

	return utils.DataResponse(re, items)
}

// --- Organisations Handlers ---

// handleOrganisationsList returns paginated organisations list
func handleOrganisationsList(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	// Parse query params
	page, _ := strconv.Atoi(re.Request.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(re.Request.URL.Query().Get("perPage"))
	if perPage < 1 || perPage > 500 {
		perPage = 50
	}
	search := re.Request.URL.Query().Get("search")
	status := re.Request.URL.Query().Get("status")
	sort := re.Request.URL.Query().Get("sort")
	if sort == "" {
		sort = "name"
	}

	// Build filter
	filter := ""
	if status != "" {
		filter = "status = {:status}"
	}
	if search != "" {
		searchFilter := "name ~ {:search}"
		if filter != "" {
			filter = filter + " && " + searchFilter
		} else {
			filter = searchFilter
		}
	}

	params := map[string]any{
		"status": status,
		"search": search,
	}

	// Get total count
	allRecords, _ := app.FindRecordsByFilter(utils.CollectionOrganisations, filter, "", 0, 0, params)
	totalItems := len(allRecords)

	// Get paginated records
	offset := (page - 1) * perPage
	records, err := app.FindRecordsByFilter(utils.CollectionOrganisations, filter, sort, perPage, offset, params)
	if err != nil {
		return utils.DataResponse(re, map[string]any{
			"items":      []any{},
			"page":       page,
			"perPage":    perPage,
			"totalItems": 0,
			"totalPages": 0,
		})
	}

	baseURL := getBaseURL()
	items := make([]map[string]any, len(records))
	for i, r := range records {
		items[i] = buildOrganisationResponse(r, baseURL)
	}

	totalPages := (totalItems + perPage - 1) / perPage

	return utils.DataResponse(re, map[string]any{
		"items":      items,
		"page":       page,
		"perPage":    perPage,
		"totalItems": totalItems,
		"totalPages": totalPages,
	})
}

// handleOrganisationGet returns a single organisation by ID
func handleOrganisationGet(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	id := re.Request.PathValue("id")
	if id == "" {
		return utils.BadRequestResponse(re, "Organisation ID required")
	}

	record, err := app.FindRecordById(utils.CollectionOrganisations, id)
	if err != nil {
		return utils.NotFoundResponse(re, "Organisation not found")
	}

	baseURL := getBaseURL()
	return utils.DataResponse(re, buildOrganisationResponse(record, baseURL))
}

// handleOrganisationCreate creates a new organisation
func handleOrganisationCreate(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	var input map[string]any
	if err := json.NewDecoder(re.Request.Body).Decode(&input); err != nil {
		return utils.BadRequestResponse(re, "Invalid request body")
	}

	// Validate required fields
	name, _ := input["name"].(string)
	if name == "" {
		return utils.BadRequestResponse(re, "Name is required")
	}

	collection, err := app.FindCollectionByNameOrId(utils.CollectionOrganisations)
	if err != nil {
		return utils.InternalErrorResponse(re, "Failed to find organisations collection")
	}

	record := core.NewRecord(collection)

	// Set fields
	record.Set("name", name)
	if v, ok := input["website"].(string); ok {
		record.Set("website", v)
	}
	if v, ok := input["linkedin"].(string); ok {
		record.Set("linkedin", v)
	}
	if v, ok := input["description_short"].(string); ok {
		record.Set("description_short", v)
	}
	if v, ok := input["description_medium"].(string); ok {
		record.Set("description_medium", v)
	}
	if v, ok := input["description_long"].(string); ok {
		record.Set("description_long", v)
	}
	if v, ok := input["contacts"].([]any); ok {
		record.Set("contacts", v)
	}
	if v, ok := input["tags"].([]any); ok {
		record.Set("tags", v)
	}
	if v, ok := input["industry"].(string); ok {
		record.Set("industry", v)
	}
	if v, ok := input["status"].(string); ok {
		record.Set("status", v)
	} else {
		record.Set("status", "active")
	}
	if v, ok := input["source"].(string); ok {
		record.Set("source", v)
	} else {
		record.Set("source", "manual")
	}

	if err := app.Save(record); err != nil {
		log.Printf("[OrganisationCreate] Failed to save: %v", err)
		return utils.InternalErrorResponse(re, "Failed to create organisation")
	}

	baseURL := getBaseURL()
	return re.JSON(http.StatusCreated, buildOrganisationResponse(record, baseURL))
}

// handleOrganisationUpdate updates an existing organisation
func handleOrganisationUpdate(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	id := re.Request.PathValue("id")
	if id == "" {
		return utils.BadRequestResponse(re, "Organisation ID required")
	}

	record, err := app.FindRecordById(utils.CollectionOrganisations, id)
	if err != nil {
		return utils.NotFoundResponse(re, "Organisation not found")
	}

	var input map[string]any
	if err := json.NewDecoder(re.Request.Body).Decode(&input); err != nil {
		return utils.BadRequestResponse(re, "Invalid request body")
	}

	// Update fields if provided
	if v, ok := input["name"].(string); ok && v != "" {
		record.Set("name", v)
	}
	if v, ok := input["website"].(string); ok {
		record.Set("website", v)
	}
	if v, ok := input["linkedin"].(string); ok {
		record.Set("linkedin", v)
	}
	if v, ok := input["description_short"].(string); ok {
		record.Set("description_short", v)
	}
	if v, ok := input["description_medium"].(string); ok {
		record.Set("description_medium", v)
	}
	if v, ok := input["description_long"].(string); ok {
		record.Set("description_long", v)
	}
	if v, ok := input["contacts"].([]any); ok {
		record.Set("contacts", v)
	}
	if v, ok := input["tags"].([]any); ok {
		record.Set("tags", v)
	}
	if v, ok := input["industry"].(string); ok {
		record.Set("industry", v)
	}
	if v, ok := input["status"].(string); ok {
		record.Set("status", v)
	}

	if err := app.Save(record); err != nil {
		log.Printf("[OrganisationUpdate] Failed to save: %v", err)
		return utils.InternalErrorResponse(re, "Failed to update organisation")
	}

	baseURL := getBaseURL()
	return utils.DataResponse(re, buildOrganisationResponse(record, baseURL))
}

// handleOrganisationDelete deletes an organisation
func handleOrganisationDelete(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	id := re.Request.PathValue("id")
	if id == "" {
		return utils.BadRequestResponse(re, "Organisation ID required")
	}

	record, err := app.FindRecordById(utils.CollectionOrganisations, id)
	if err != nil {
		return utils.NotFoundResponse(re, "Organisation not found")
	}

	if err := app.Delete(record); err != nil {
		log.Printf("[OrganisationDelete] Failed to delete: %v", err)
		return utils.InternalErrorResponse(re, "Failed to delete organisation")
	}

	return utils.SuccessResponse(re, "Organisation deleted successfully")
}

// handleOrganisationLogoUploadToken generates an HMAC-signed token for uploading logos to DAM
// This allows the frontend to upload directly to DAM with authentication
func handleOrganisationLogoUploadToken(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	id := re.Request.PathValue("id")
	logoType := re.Request.PathValue("type")

	if id == "" {
		return utils.BadRequestResponse(re, "Organisation ID required")
	}

	// Validate logo type
	validTypes := []string{"square", "standard", "inverted"}
	isValid := false
	for _, t := range validTypes {
		if logoType == t {
			isValid = true
			break
		}
	}
	if !isValid {
		return utils.BadRequestResponse(re, "Invalid logo type. Must be: square, standard, or inverted")
	}

	// Verify organisation exists
	_, err := app.FindRecordById(utils.CollectionOrganisations, id)
	if err != nil {
		return utils.NotFoundResponse(re, "Organisation not found")
	}

	// Generate signed token for DAM upload
	secret := os.Getenv("PROJECTION_WEBHOOK_SECRET")
	if secret == "" {
		return utils.InternalErrorResponse(re, "Upload token generation not configured")
	}

	// Create token payload: org_id:logo_type:timestamp:action
	timestamp := time.Now().UTC().Format(time.RFC3339)
	action := "upload"
	if re.Request.URL.Query().Get("action") == "delete" {
		action = "delete"
	}
	tokenData := fmt.Sprintf("%s:%s:%s:%s", id, logoType, timestamp, action)

	// Sign with HMAC-SHA256
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(tokenData))
	signature := hex.EncodeToString(mac.Sum(nil))

	damURL := os.Getenv("DAM_PUBLIC_URL")
	if damURL == "" {
		damURL = "https://outlook-apps-dam.fly.dev"
	}

	return utils.DataResponse(re, map[string]any{
		"org_id":     id,
		"logo_type":  logoType,
		"timestamp":  timestamp,
		"action":     action,
		"signature":  signature,
		"dam_url":    damURL,
		"expires_in": 300, // 5 minutes
	})
}

// --- Activities Handlers ---

// handleActivitiesList returns paginated activities list
func handleActivitiesList(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	// Parse query params
	page, _ := strconv.Atoi(re.Request.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(re.Request.URL.Query().Get("perPage"))
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}
	sourceApp := re.Request.URL.Query().Get("source_app")
	activityType := re.Request.URL.Query().Get("type")

	// Build filter
	filter := ""
	if sourceApp != "" {
		filter = "source_app = {:sourceApp}"
	}
	if activityType != "" {
		typeFilter := "type = {:activityType}"
		if filter != "" {
			filter = filter + " && " + typeFilter
		} else {
			filter = typeFilter
		}
	}

	params := map[string]any{
		"sourceApp":    sourceApp,
		"activityType": activityType,
	}

	// Get total count
	allRecords, _ := app.FindRecordsByFilter(utils.CollectionActivities, filter, "", 0, 0, params)
	totalItems := len(allRecords)

	// Get paginated records
	offset := (page - 1) * perPage
	records, err := app.FindRecordsByFilter(utils.CollectionActivities, filter, "-occurred_at", perPage, offset, params)
	if err != nil {
		return utils.DataResponse(re, map[string]any{
			"items":      []any{},
			"page":       page,
			"perPage":    perPage,
			"totalItems": 0,
			"totalPages": 0,
		})
	}

	items := make([]map[string]any, len(records))
	for i, r := range records {
		items[i] = buildActivityResponse(r)
	}

	totalPages := (totalItems + perPage - 1) / perPage

	return utils.DataResponse(re, map[string]any{
		"items":      items,
		"page":       page,
		"perPage":    perPage,
		"totalItems": totalItems,
		"totalPages": totalPages,
	})
}

// handleActivityWebhook receives activity data from other apps
func handleActivityWebhook(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	// Read raw body for HMAC validation
	bodyBytes, err := io.ReadAll(re.Request.Body)
	if err != nil {
		return utils.BadRequestResponse(re, "Failed to read request body")
	}

	// Validate HMAC signature if secret is configured
	secret := os.Getenv("ACTIVITY_WEBHOOK_SECRET")
	if secret != "" {
		signature := re.Request.Header.Get("X-Webhook-Signature")
		if signature == "" {
			log.Printf("[ActivityWebhook] Missing signature from %s", re.RealIP())
			return re.JSON(http.StatusUnauthorized, map[string]string{"error": "Missing signature"})
		}

		// Compute expected signature using HMAC-SHA256
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(bodyBytes)
		expectedSig := hex.EncodeToString(mac.Sum(nil))

		// Constant-time comparison to prevent timing attacks
		if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
			log.Printf("[ActivityWebhook] Invalid signature from %s", re.RealIP())
			return re.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid signature"})
		}
	}

	var payload struct {
		Type       string         `json:"type"`
		Title      string         `json:"title"`
		ContactID  string         `json:"contact_id"`
		OrgID      string         `json:"organisation_id"`
		SourceApp  string         `json:"source_app"`
		SourceID   string         `json:"source_id"`
		SourceURL  string         `json:"source_url"`
		Metadata   map[string]any `json:"metadata"`
		OccurredAt string         `json:"occurred_at"`
	}

	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		return utils.BadRequestResponse(re, "Invalid request body")
	}

	if payload.Type == "" || payload.SourceApp == "" {
		return utils.BadRequestResponse(re, "type and source_app are required")
	}

	collection, err := app.FindCollectionByNameOrId(utils.CollectionActivities)
	if err != nil {
		return utils.InternalErrorResponse(re, "Failed to find activities collection")
	}

	record := core.NewRecord(collection)
	record.Set("type", payload.Type)
	record.Set("title", payload.Title)
	record.Set("source_app", payload.SourceApp)
	record.Set("source_id", payload.SourceID)
	record.Set("source_url", payload.SourceURL)
	record.Set("metadata", payload.Metadata)

	if payload.ContactID != "" {
		record.Set("contact", payload.ContactID)
	}
	if payload.OrgID != "" {
		record.Set("organisation", payload.OrgID)
	}
	if payload.OccurredAt != "" {
		record.Set("occurred_at", payload.OccurredAt)
	}

	if err := app.Save(record); err != nil {
		log.Printf("[ActivityWebhook] Failed to save: %v", err)
		return utils.InternalErrorResponse(re, "Failed to create activity")
	}

	log.Printf("[ActivityWebhook] Created activity: type=%s source=%s", payload.Type, payload.SourceApp)
	return utils.SuccessResponse(re, "Activity recorded")
}

// handleProjectAll triggers projection of all contacts and organisations to consumers
func handleProjectAll(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	result, err := ProjectAll(app)
	if err != nil {
		return utils.InternalErrorResponse(re, err.Error())
	}
	go RefreshDAMAvatarCache()
	return utils.DataResponse(re, map[string]any{
		"status":        "projected",
		"projection_id": result.ProjectionID,
		"counts":        result.Counts,
		"total":         result.Total,
		"consumers":     result.ConsumerNames,
	})
}

// handleAvatarURLWebhook receives avatar variant URLs from DAM after processing
func handleAvatarURLWebhook(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	// Read raw body for HMAC validation
	bodyBytes, err := io.ReadAll(re.Request.Body)
	if err != nil {
		return utils.BadRequestResponse(re, "Failed to read request body")
	}

	// Validate HMAC signature
	secret := os.Getenv("PROJECTION_WEBHOOK_SECRET")
	if secret != "" {
		signature := re.Request.Header.Get("X-Webhook-Signature")
		if signature == "" {
			log.Printf("[AvatarURLWebhook] Missing signature from %s", re.RealIP())
			return re.JSON(http.StatusUnauthorized, map[string]string{"error": "Missing signature"})
		}

		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(bodyBytes)
		expectedSig := hex.EncodeToString(mac.Sum(nil))

		if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
			log.Printf("[AvatarURLWebhook] Invalid signature from %s", re.RealIP())
			return re.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid signature"})
		}
	}

	var payload struct {
		CrmID            string `json:"crm_id"`
		AvatarThumbURL   string `json:"avatar_thumb_url"`
		AvatarSmallURL   string `json:"avatar_small_url"`
		AvatarOriginalURL string `json:"avatar_original_url"`
	}

	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		return utils.BadRequestResponse(re, "Invalid request body")
	}

	if payload.CrmID == "" {
		return utils.BadRequestResponse(re, "crm_id is required")
	}

	record, err := app.FindRecordById(utils.CollectionContacts, payload.CrmID)
	if err != nil {
		return utils.NotFoundResponse(re, "Contact not found")
	}

	if payload.AvatarThumbURL != "" {
		record.Set("avatar_thumb_url", payload.AvatarThumbURL)
	}
	if payload.AvatarSmallURL != "" {
		record.Set("avatar_small_url", payload.AvatarSmallURL)
	}
	if payload.AvatarOriginalURL != "" {
		record.Set("avatar_original_url", payload.AvatarOriginalURL)
	}

	// Decrypt PII fields before save so encryption hooks re-encrypt correctly
	piiFields := []string{"email", "personal_email", "phone", "bio", "location"}
	for _, field := range piiFields {
		if v := record.GetString(field); v != "" {
			record.Set(field, utils.DecryptField(v))
		}
	}

	if err := app.Save(record); err != nil {
		log.Printf("[AvatarURLWebhook] Failed to save contact %s: %v", payload.CrmID, err)
		return utils.InternalErrorResponse(re, "Failed to update contact")
	}

	log.Printf("[AvatarURLWebhook] Updated avatar URLs for contact %s", payload.CrmID)
	go RefreshDAMAvatarCache()
	return utils.SuccessResponse(re, "Avatar URLs updated")
}

// syncAvatarURLsResult holds the result of a DAM avatar URL sync
type syncAvatarURLsResult struct {
	Updated int
	Skipped int
	Total   int
}

// syncAvatarURLsFromDAM pulls avatar URLs from DAM for all contacts.
// Usable from both CLI and HTTP handler.
func syncAvatarURLsFromDAM(app *pocketbase.PocketBase) (*syncAvatarURLsResult, error) {
	damURL := os.Getenv("DAM_PUBLIC_URL")
	if damURL == "" {
		damURL = "https://outlook-apps-dam.fly.dev"
	}

	resp, err := http.Get(damURL + "/api/public/people")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch people from DAM: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DAM returned status %d", resp.StatusCode)
	}

	var damResp struct {
		Items []struct {
			ID                string `json:"id"`
			CrmID             string `json:"crm_id"`
			AvatarThumbURL    string `json:"avatar_thumb_url"`
			AvatarSmallURL    string `json:"avatar_small_url"`
			AvatarOriginalURL string `json:"avatar_original_url"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&damResp); err != nil {
		return nil, fmt.Errorf("failed to parse DAM response: %w", err)
	}

	updated := 0
	skipped := 0
	for _, person := range damResp.Items {
		if person.CrmID == "" || person.ID == "" || (person.AvatarSmallURL == "" && person.AvatarThumbURL == "" && person.AvatarOriginalURL == "") {
			skipped++
			continue
		}

		record, err := app.FindRecordById(utils.CollectionContacts, person.CrmID)
		if err != nil {
			skipped++
			continue
		}

		// Build DAM proxy URLs (Tigris URLs 403 — must go through DAM's proxy handler)
		thumbURL := damURL + "/api/people/" + person.ID + "/avatar/thumb"
		smallURL := damURL + "/api/people/" + person.ID + "/avatar/small"
		originalURL := damURL + "/api/people/" + person.ID + "/avatar/original"

		// Check if URLs have changed
		changed := false
		if record.GetString("avatar_thumb_url") != thumbURL {
			changed = true
		}
		if record.GetString("avatar_small_url") != smallURL {
			changed = true
		}
		if record.GetString("avatar_original_url") != originalURL {
			changed = true
		}
		if !changed {
			skipped++
			continue
		}

		record.Set("avatar_thumb_url", thumbURL)
		record.Set("avatar_small_url", smallURL)
		record.Set("avatar_original_url", originalURL)

		// Decrypt PII fields before save
		piiFields := []string{"email", "personal_email", "phone", "bio", "location"}
		for _, field := range piiFields {
			if v := record.GetString(field); v != "" {
				record.Set(field, utils.DecryptField(v))
			}
		}

		if err := app.Save(record); err != nil {
			log.Printf("[SyncAvatarURLs] Failed to save contact %s: %v", person.CrmID, err)
			continue
		}
		updated++
	}

	log.Printf("[SyncAvatarURLs] Updated %d contacts, skipped %d", updated, skipped)
	return &syncAvatarURLsResult{Updated: updated, Skipped: skipped, Total: len(damResp.Items)}, nil
}

// handleSyncAvatarURLs is the HTTP handler wrapper for syncAvatarURLsFromDAM
func handleSyncAvatarURLs(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	result, err := syncAvatarURLsFromDAM(app)
	if err != nil {
		return utils.InternalErrorResponse(re, err.Error())
	}
	return utils.DataResponse(re, map[string]any{
		"updated": result.Updated,
		"skipped": result.Skipped,
		"total":   result.Total,
	})
}

// --- Merge Handler ---

// MergeContactsInput is the request payload for merging contacts
type MergeContactsInput struct {
	PrimaryID       string            `json:"primary_id"`
	MergedIDs       []string          `json:"merged_ids"`
	FieldSelections map[string]string `json:"field_selections"` // field -> source contact ID
	MergedRoles                    []string          `json:"merged_roles"`
	MergedTags                     []string          `json:"merged_tags"`
	MergedDietaryRequirements      []string          `json:"merged_dietary_requirements"`
	MergedAccessibilityRequirements []string         `json:"merged_accessibility_requirements"`
}

// handleContactsMerge merges multiple contacts into a single primary contact
func handleContactsMerge(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	var input MergeContactsInput
	if err := json.NewDecoder(re.Request.Body).Decode(&input); err != nil {
		return utils.BadRequestResponse(re, "Invalid request body")
	}

	// Validate input
	if input.PrimaryID == "" {
		return utils.BadRequestResponse(re, "primary_id is required")
	}
	if len(input.MergedIDs) == 0 {
		return utils.BadRequestResponse(re, "At least one merged_id is required")
	}
	for _, mid := range input.MergedIDs {
		if mid == input.PrimaryID {
			return utils.BadRequestResponse(re, "primary_id cannot be in merged_ids")
		}
	}

	// Load all contacts
	primaryRecord, err := app.FindRecordById(utils.CollectionContacts, input.PrimaryID)
	if err != nil {
		return utils.NotFoundResponse(re, "Primary contact not found")
	}

	allContacts := map[string]*core.Record{input.PrimaryID: primaryRecord}
	for _, mid := range input.MergedIDs {
		record, err := app.FindRecordById(utils.CollectionContacts, mid)
		if err != nil {
			return utils.NotFoundResponse(re, fmt.Sprintf("Contact %s not found", mid))
		}
		allContacts[mid] = record
	}

	// Build merged values from field_selections
	scalarFields := []string{
		"first_name", "last_name", "email", "personal_email", "phone", "pronouns", "bio", "job_title",
		"linkedin", "instagram", "website", "location", "do_position",
		"organisation", "status", "source",
		"avatar_url", "avatar_thumb_url", "avatar_small_url", "avatar_original_url",
		"hubspot_contact_id", "hubspot_synced_at",
		"degrees", "relationship", "notes",
		"dietary_requirements_other", "accessibility_requirements_other",
	}
	piiFields := map[string]bool{"email": true, "personal_email": true, "phone": true, "bio": true, "location": true}

	for _, field := range scalarFields {
		sourceID, ok := input.FieldSelections[field]
		if !ok {
			continue
		}
		sourceRecord, ok := allContacts[sourceID]
		if !ok {
			return utils.BadRequestResponse(re, fmt.Sprintf("Invalid source contact for field %s", field))
		}
		val := sourceRecord.GetString(field)
		if piiFields[field] {
			val = utils.DecryptField(val)
		}
		primaryRecord.Set(field, val)
	}

	// Recompute denormalized name after merge
	fn := primaryRecord.GetString("first_name")
	ln := primaryRecord.GetString("last_name")
	primaryRecord.Set("name", strings.TrimSpace(fn+" "+ln))

	// Set array fields
	primaryRecord.Set("roles", input.MergedRoles)
	primaryRecord.Set("tags", input.MergedTags)
	primaryRecord.Set("dietary_requirements", input.MergedDietaryRequirements)
	primaryRecord.Set("accessibility_requirements", input.MergedAccessibilityRequirements)

	// Deep merge source_ids from all contacts (PocketBase returns types.JSONRaw for JSON fields)
	mergedSourceIDs := map[string]any{}
	for _, record := range allContacts {
		if sourceIDs := record.Get("source_ids"); sourceIDs != nil {
			var m map[string]any
			b, _ := json.Marshal(sourceIDs)
			if err := json.Unmarshal(b, &m); err == nil {
				for k, v := range m {
					mergedSourceIDs[k] = v
				}
			}
		}
	}
	primaryRecord.Set("source_ids", mergedSourceIDs)

	// Execute in transaction
	activitiesReassigned := 0

	err = app.RunInTransaction(func(txApp core.App) error {
		for _, mid := range input.MergedIDs {
			record := allContacts[mid]

			// Reassign activities to primary
			activities, _ := txApp.FindRecordsByFilter(
				utils.CollectionActivities,
				"contact = {:contactId}", "", 0, 0,
				map[string]any{"contactId": mid},
			)
			for _, activity := range activities {
				activity.Set("contact", input.PrimaryID)
				if err := txApp.Save(activity); err != nil {
					return fmt.Errorf("failed to reassign activity %s: %w", activity.Id, err)
				}
				activitiesReassigned++
			}

			// Reassign or remove guest list items referencing the merged contact
			guestListItems, _ := txApp.FindRecordsByFilter(
				"guest_list_items",
				"contact = {:contactId}", "", 0, 0,
				map[string]any{"contactId": mid},
			)
			for _, item := range guestListItems {
				guestListID := item.GetString("guest_list")
				// Check if primary already has an item in this guest list (unique constraint)
				existing, _ := txApp.FindRecordsByFilter(
					"guest_list_items",
					"guest_list = {:glId} && contact = {:primaryId}", "", 1, 0,
					map[string]any{"glId": guestListID, "primaryId": input.PrimaryID},
				)
				if len(existing) > 0 {
					// Primary already in this guest list — delete the duplicate
					if err := txApp.Delete(item); err != nil {
						return fmt.Errorf("failed to delete duplicate guest list item %s: %w", item.Id, err)
					}
				} else {
					// Reassign to primary
					item.Set("contact", input.PrimaryID)
					if err := txApp.Save(item); err != nil {
						return fmt.Errorf("failed to reassign guest list item %s: %w", item.Id, err)
					}
				}
			}

			// Reassign or remove contact links referencing the merged contact
			contactLinks, _ := txApp.FindRecordsByFilter(
				"contact_links",
				"contact_a = {:contactId} || contact_b = {:contactId}", "", 0, 0,
				map[string]any{"contactId": mid},
			)
			for _, link := range contactLinks {
				contactA := link.GetString("contact_a")
				contactB := link.GetString("contact_b")

				// If the link connects to the primary (or another merged contact), just delete it
				otherID := contactA
				if contactA == mid {
					otherID = contactB
				}
				if otherID == input.PrimaryID {
					if err := txApp.Delete(link); err != nil {
						return fmt.Errorf("failed to delete self-referencing link %s: %w", link.Id, err)
					}
					continue
				}

				// Reassign: point the merged contact's side to primary
				if contactA == mid {
					link.Set("contact_a", input.PrimaryID)
				} else {
					link.Set("contact_b", input.PrimaryID)
				}
				if err := txApp.Save(link); err != nil {
					// May fail if a link between primary and otherID already exists — delete instead
					if err2 := txApp.Delete(link); err2 != nil {
						return fmt.Errorf("failed to handle contact link %s: %w", link.Id, err)
					}
				}
			}

			// Delete the merged contact (now safe — no more references)
			if err := txApp.Delete(record); err != nil {
				return fmt.Errorf("failed to delete contact %s: %w", mid, err)
			}
		}

		// Decrypt PII fields on primary before save (encryption hooks re-encrypt)
		for _, field := range []string{"email", "personal_email", "phone", "bio", "location"} {
			if v := primaryRecord.GetString(field); v != "" {
				primaryRecord.Set(field, utils.DecryptField(v))
			}
		}

		// Save the updated primary contact
		if err := txApp.Save(primaryRecord); err != nil {
			return fmt.Errorf("failed to save primary contact: %w", err)
		}

		return nil
	})

	if err != nil {
		log.Printf("[ContactMerge] Transaction failed: %v", err)
		return utils.InternalErrorResponse(re, "Failed to merge contacts")
	}

	// Audit log
	utils.LogFromRequest(app, re, "merge", "contacts", input.PrimaryID, "success",
		map[string]any{
			"primary_id":            input.PrimaryID,
			"merged_ids":            input.MergedIDs,
			"activities_reassigned": activitiesReassigned,
		}, "")

	return utils.DataResponse(re, map[string]any{
		"id":                    input.PrimaryID,
		"activities_reassigned": activitiesReassigned,
		"contacts_deleted":      len(input.MergedIDs),
	})
}

// --- Response Builders ---

// buildContactResponse builds a contact response object
func buildContactResponse(r *core.Record, app *pocketbase.PocketBase, baseURL string) map[string]any {
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
		"do_position":    r.GetString("do_position"),
		"tags":           r.Get("tags"),
		"roles":          r.Get("roles"),
		"status":         r.GetString("status"),
		"source":         r.GetString("source"),
		"source_ids":     r.Get("source_ids"),
		"domain":         r.Get("domain"),
		"degrees":        r.GetString("degrees"),
		"relationship":   r.GetInt("relationship"),
		"notes":          r.GetString("notes"),
		"dietary_requirements":              r.Get("dietary_requirements"),
		"dietary_requirements_other":        r.GetString("dietary_requirements_other"),
		"accessibility_requirements":        r.Get("accessibility_requirements"),
		"accessibility_requirements_other":  r.GetString("accessibility_requirements_other"),
		"created":        r.GetString("created"),
		"updated":        r.GetString("updated"),
	}

	// Avatar URL (stored by DAM)
	if avatarURL := r.GetString("avatar_url"); avatarURL != "" {
		data["avatar_url"] = avatarURL
	}

	// DAM avatar variant URLs — prefer stored values, fall back to cached DAM lookup
	thumb := r.GetString("avatar_thumb_url")
	small := r.GetString("avatar_small_url")
	original := r.GetString("avatar_original_url")

	if small == "" {
		if cached, ok := GetDAMAvatarURLs(r.Id); ok {
			if thumb == "" {
				thumb = cached.ThumbURL
			}
			small = cached.SmallURL
			if original == "" {
				original = cached.OriginalURL
			}
		}
	}

	if thumb != "" {
		data["avatar_thumb_url"] = thumb
	}
	if small != "" {
		data["avatar_small_url"] = small
	}
	if original != "" {
		data["avatar_original_url"] = original
	}

	// Organisation relation
	orgID := r.GetString("organisation")
	data["organisation"] = orgID
	if orgID != "" {
		org, err := app.FindRecordById(utils.CollectionOrganisations, orgID)
		if err == nil {
			data["organisation_id"] = org.Id
			data["organisation_name"] = org.GetString("name")
		}
	}

	return data
}

// buildContactProjection builds a contact projection for COPE consumers
func buildContactProjection(r *core.Record, app *pocketbase.PocketBase, baseURL string) map[string]any {
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
		"do_position":    r.GetString("do_position"),
		"tags":           r.Get("tags"),
		"roles":          r.Get("roles"),
		"domain":         r.Get("domain"),
		"dietary_requirements":              r.Get("dietary_requirements"),
		"dietary_requirements_other":        r.GetString("dietary_requirements_other"),
		"accessibility_requirements":        r.Get("accessibility_requirements"),
		"accessibility_requirements_other":  r.GetString("accessibility_requirements_other"),
		"created":        r.GetString("created"),
		"updated":        r.GetString("updated"),
	}

	// Avatar URL (stored by DAM)
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

// buildOrganisationResponse builds an organisation response object
func buildOrganisationResponse(r *core.Record, baseURL string) map[string]any {
	data := map[string]any{
		"id":                 r.Id,
		"name":               r.GetString("name"),
		"website":            r.GetString("website"),
		"linkedin":           r.GetString("linkedin"),
		"description_short":  r.GetString("description_short"),
		"description_medium": r.GetString("description_medium"),
		"description_long":   r.GetString("description_long"),
		"contacts":           r.Get("contacts"),
		"tags":               r.Get("tags"),
		"industry":           r.GetString("industry"),
		"status":             r.GetString("status"),
		"source":             r.GetString("source"),
		"created":            r.GetString("created"),
		"updated":            r.GetString("updated"),
	}

	// Logo URLs are managed by DAM — CRM does not store logo files

	return data
}

// buildOrganisationProjection builds an organisation projection for COPE consumers
func buildOrganisationProjection(r *core.Record, baseURL string) map[string]any {
	data := map[string]any{
		"id":                 r.Id,
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

	// Logo URLs are managed by DAM — CRM does not store logo files

	return data
}

// buildActivityResponse builds an activity response object
func buildActivityResponse(r *core.Record) map[string]any {
	return map[string]any{
		"id":           r.Id,
		"type":         r.GetString("type"),
		"title":        r.GetString("title"),
		"contact":      r.GetString("contact"),
		"organisation": r.GetString("organisation"),
		"source_app":   r.GetString("source_app"),
		"source_id":    r.GetString("source_id"),
		"source_url":   r.GetString("source_url"),
		"metadata":     r.Get("metadata"),
		"occurred_at":  r.GetString("occurred_at"),
		"created":      r.GetString("created"),
	}
}

// --- Utility Functions ---

// getBaseURL returns the base URL for the admin app
func getBaseURL() string {
	baseURL := os.Getenv("PUBLIC_BASE_URL")
	if baseURL == "" {
		baseURL = "https://crm.theoutlook.io"
	}
	return baseURL
}

// getPublicBaseURL returns the public-facing base URL (for RSVP pages, share links, emails)
func getPublicBaseURL() string {
	baseURL := os.Getenv("PUBLIC_RSVP_URL")
	if baseURL == "" {
		return getBaseURL()
	}
	return baseURL
}
