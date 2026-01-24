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
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/grtshw/outlook-apps-crm/utils"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

// --- Public API Handlers (for COPE projections) ---

// handlePublicContacts returns contacts for COPE consumers
func handlePublicContacts(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	// Only return active contacts (not archived)
	records, err := app.FindRecordsByFilter(utils.CollectionContacts, "status != 'archived'", "name", 1000, 0)
	if err != nil {
		return utils.DataResponse(re, map[string]any{"items": []any{}})
	}

	// Pre-fetch all organisations to avoid N+1 queries
	orgsMap := buildOrganisationsMap(records, app)

	baseURL := getBaseURL()
	items := make([]map[string]any, len(records))
	for i, r := range records {
		items[i] = buildContactProjection(r, app, baseURL, orgsMap)
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
	name, _ := input["name"].(string)
	if email == "" {
		return utils.BadRequestResponse(re, "Email is required")
	}
	if name == "" {
		return utils.BadRequestResponse(re, "Name is required")
	}

	// Normalize email
	email = strings.ToLower(strings.TrimSpace(email))

	// Check for existing contact by email
	existing, _ := app.FindRecordsByFilter(utils.CollectionContacts, "email = {:email}", "", 1, 0, map[string]any{"email": email})
	if len(existing) > 0 {
		// Return existing contact instead of error
		return utils.DataResponse(re, map[string]any{
			"id":       existing[0].Id,
			"email":    existing[0].GetString("email"),
			"name":     existing[0].GetString("name"),
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
	record.Set("name", name)
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
		return utils.InternalErrorResponse(re, "Failed to create contact")
	}

	log.Printf("[ExternalContactCreate] Created contact: id=%s email=%s", record.Id, email)

	// Webhook fires automatically via PocketBase hooks
	return re.JSON(http.StatusCreated, map[string]any{
		"id":       record.Id,
		"email":    record.GetString("email"),
		"name":     record.GetString("name"),
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
		"name", "phone", "pronouns", "bio", "job_title",
		"linkedin", "instagram", "website", "location",
	}

	for _, field := range allowedFields {
		if val, ok := input[field]; ok {
			record.Set(field, val)
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
		"name":    record.GetString("name"),
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
	// Fetch all contacts once and count by status in-memory
	// This is more efficient than 3 separate queries
	allContacts, _ := app.FindRecordsByFilter(utils.CollectionContacts, "", "", 0, 0)
	activeContactsCount := 0
	inactiveContactsCount := 0
	archivedContactsCount := 0
	for _, c := range allContacts {
		switch c.GetString("status") {
		case "active":
			activeContactsCount++
		case "inactive":
			inactiveContactsCount++
		case "archived":
			archivedContactsCount++
		}
	}

	// Fetch all organisations once and count by status in-memory
	allOrgs, _ := app.FindRecordsByFilter(utils.CollectionOrganisations, "", "", 0, 0)
	activeOrgsCount := 0
	archivedOrgsCount := 0
	for _, o := range allOrgs {
		switch o.GetString("status") {
		case "active":
			activeOrgsCount++
		case "archived":
			archivedOrgsCount++
		}
	}

	// Count recent activities (last 30 days)
	recentActivities, _ := app.FindRecordsByFilter(utils.CollectionActivities, "created >= @todayStart - 2592000", "-created", 100, 0)

	return utils.DataResponse(re, map[string]any{
		"contacts": map[string]int{
			"active":   activeContactsCount,
			"inactive": inactiveContactsCount,
			"archived": archivedContactsCount,
			"total":    activeContactsCount + inactiveContactsCount + archivedContactsCount,
		},
		"organisations": map[string]int{
			"active":   activeOrgsCount,
			"archived": archivedOrgsCount,
			"total":    activeOrgsCount + archivedOrgsCount,
		},
		"recent_activities": len(recentActivities),
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
	if perPage < 1 || perPage > 100 {
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
		searchFilter := "(name ~ {:search} || email ~ {:search})"
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

	// Fetch one extra record to detect if there are more pages
	// This avoids the expensive full table scan for counting
	offset := (page - 1) * perPage
	records, err := app.FindRecordsByFilter(utils.CollectionContacts, filter, sort, perPage+1, offset, params)
	if err != nil {
		return utils.DataResponse(re, map[string]any{
			"items":      []any{},
			"page":       page,
			"perPage":    perPage,
			"totalItems": 0,
			"totalPages": 0,
		})
	}

	// Check if there are more pages
	hasMore := len(records) > perPage
	if hasMore {
		records = records[:perPage] // Trim to requested size
	}

	// Calculate totalItems and totalPages based on what we know
	totalItems := offset + len(records)
	if hasMore {
		totalItems++ // At least one more item exists
	}
	totalPages := page
	if hasMore {
		totalPages++ // At least one more page exists
	}

	// Pre-fetch all organisations to avoid N+1 queries
	orgsMap := buildOrganisationsMap(records, app)

	baseURL := getBaseURL()
	items := make([]map[string]any, len(records))
	for i, r := range records {
		items[i] = buildContactResponse(r, app, baseURL, orgsMap)
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

	// For single record, fetch organisation if needed
	orgsMap := make(map[string]*core.Record)
	if orgID := record.GetString("organisation"); orgID != "" {
		org, err := app.FindRecordById(utils.CollectionOrganisations, orgID)
		if err == nil {
			orgsMap[org.Id] = org
		}
	}

	baseURL := getBaseURL()
	return utils.DataResponse(re, buildContactResponse(record, app, baseURL, orgsMap))
}

// handleContactCreate creates a new contact
func handleContactCreate(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	var input map[string]any
	if err := json.NewDecoder(re.Request.Body).Decode(&input); err != nil {
		return utils.BadRequestResponse(re, "Invalid request body")
	}

	// Validate required fields
	email, _ := input["email"].(string)
	name, _ := input["name"].(string)
	if email == "" {
		return utils.BadRequestResponse(re, "Email is required")
	}
	if name == "" {
		return utils.BadRequestResponse(re, "Name is required")
	}

	// Check for duplicate email
	existing, _ := app.FindRecordsByFilter(utils.CollectionContacts, "email = {:email}", "", 1, 0, map[string]any{"email": strings.ToLower(strings.TrimSpace(email))})
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
	record.Set("name", name)
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
		log.Printf("[ContactCreate] Failed to save: %v", err)
		return utils.InternalErrorResponse(re, "Failed to create contact")
	}

	// For single record, fetch organisation if needed
	orgsMap := make(map[string]*core.Record)
	if orgID := record.GetString("organisation"); orgID != "" {
		org, err := app.FindRecordById(utils.CollectionOrganisations, orgID)
		if err == nil {
			orgsMap[org.Id] = org
		}
	}

	baseURL := getBaseURL()
	return re.JSON(http.StatusCreated, buildContactResponse(record, app, baseURL, orgsMap))
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
	if v, ok := input["name"].(string); ok && v != "" {
		record.Set("name", v)
	}
	if v, ok := input["email"].(string); ok && v != "" {
		// Check for duplicate email (excluding current record)
		email := strings.ToLower(strings.TrimSpace(v))
		existing, _ := app.FindRecordsByFilter(utils.CollectionContacts, "email = {:email} && id != {:id}", "", 1, 0, map[string]any{"email": email, "id": id})
		if len(existing) > 0 {
			return utils.BadRequestResponse(re, "A contact with this email already exists")
		}
		record.Set("email", email)
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
	if v, ok := input["status"].(string); ok {
		record.Set("status", v)
	}

	if err := app.Save(record); err != nil {
		log.Printf("[ContactUpdate] Failed to save: %v", err)
		return utils.InternalErrorResponse(re, "Failed to update contact")
	}

	// For single record, fetch organisation if needed
	orgsMap := make(map[string]*core.Record)
	if orgID := record.GetString("organisation"); orgID != "" {
		org, err := app.FindRecordById(utils.CollectionOrganisations, orgID)
		if err == nil {
			orgsMap[org.Id] = org
		}
	}

	baseURL := getBaseURL()
	return utils.DataResponse(re, buildContactResponse(record, app, baseURL, orgsMap))
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

// handleContactAvatarUpload handles avatar file upload for a contact
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

	// Validate file type
	contentType := header.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		return utils.BadRequestResponse(re, "File must be an image")
	}

	// Read file content
	fileBytes := make([]byte, header.Size)
	if _, err := file.Read(fileBytes); err != nil {
		return utils.InternalErrorResponse(re, "Failed to read file")
	}

	// Create file from bytes
	fsFile, err := filesystem.NewFileFromBytes(fileBytes, header.Filename)
	if err != nil {
		return utils.InternalErrorResponse(re, "Failed to process file")
	}

	record.Set("avatar", fsFile)

	if err := app.Save(record); err != nil {
		log.Printf("[ContactAvatarUpload] Failed to save: %v", err)
		return utils.InternalErrorResponse(re, "Failed to save avatar")
	}

	// For single record, fetch organisation if needed
	orgsMap := make(map[string]*core.Record)
	if orgID := record.GetString("organisation"); orgID != "" {
		org, err := app.FindRecordById(utils.CollectionOrganisations, orgID)
		if err == nil {
			orgsMap[org.Id] = org
		}
	}

	baseURL := getBaseURL()
	return utils.DataResponse(re, buildContactResponse(record, app, baseURL, orgsMap))
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
	if perPage < 1 || perPage > 100 {
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

	// Fetch one extra record to detect if there are more pages
	// This avoids the expensive full table scan for counting
	offset := (page - 1) * perPage
	records, err := app.FindRecordsByFilter(utils.CollectionOrganisations, filter, sort, perPage+1, offset, params)
	if err != nil {
		return utils.DataResponse(re, map[string]any{
			"items":      []any{},
			"page":       page,
			"perPage":    perPage,
			"totalItems": 0,
			"totalPages": 0,
		})
	}

	// Check if there are more pages
	hasMore := len(records) > perPage
	if hasMore {
		records = records[:perPage] // Trim to requested size
	}

	// Calculate totalItems and totalPages based on what we know
	totalItems := offset + len(records)
	if hasMore {
		totalItems++ // At least one more item exists
	}
	totalPages := page
	if hasMore {
		totalPages++ // At least one more page exists
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

	// Fetch one extra record to detect if there are more pages
	// This avoids the expensive full table scan for counting
	offset := (page - 1) * perPage
	records, err := app.FindRecordsByFilter(utils.CollectionActivities, filter, "-occurred_at", perPage+1, offset, params)
	if err != nil {
		return utils.DataResponse(re, map[string]any{
			"items":      []any{},
			"page":       page,
			"perPage":    perPage,
			"totalItems": 0,
			"totalPages": 0,
		})
	}

	// Check if there are more pages
	hasMore := len(records) > perPage
	if hasMore {
		records = records[:perPage] // Trim to requested size
	}

	// Calculate totalItems and totalPages based on what we know
	totalItems := offset + len(records)
	if hasMore {
		totalItems++ // At least one more item exists
	}
	totalPages := page
	if hasMore {
		totalPages++ // At least one more page exists
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
	secret := os.Getenv("ACTIVITY_WEBHOOK_SECRET")

	// Read body for HMAC validation
	bodyBytes, err := io.ReadAll(re.Request.Body)
	if err != nil {
		log.Printf("[ActivityWebhook] Failed to read body from %s: %v", re.RealIP(), err)
		return utils.BadRequestResponse(re, "Failed to read request body")
	}

	// Restore body for JSON decoding later
	re.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// Validate HMAC signature if secret is configured
	if secret != "" {
		providedSig := re.Request.Header.Get("X-Webhook-Signature")
		if providedSig == "" {
			log.Printf("[ActivityWebhook] Missing signature from %s", re.RealIP())
			return re.JSON(http.StatusUnauthorized, map[string]string{
				"error": "Missing X-Webhook-Signature header",
			})
		}

		// Compute expected signature
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(bodyBytes)
		expectedSig := hex.EncodeToString(mac.Sum(nil))

		// Constant-time comparison to prevent timing attacks
		if !hmac.Equal([]byte(providedSig), []byte(expectedSig)) {
			log.Printf("[ActivityWebhook] Invalid signature from %s", re.RealIP())
			return re.JSON(http.StatusUnauthorized, map[string]string{
				"error": "Invalid webhook signature",
			})
		}

		log.Printf("[ActivityWebhook] Valid signature from %s", re.RealIP())
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

	if err := json.NewDecoder(re.Request.Body).Decode(&payload); err != nil {
		log.Printf("[ActivityWebhook] Invalid JSON from %s: %v", re.RealIP(), err)
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
	counts, err := ProjectAll(app)
	if err != nil {
		return utils.InternalErrorResponse(re, err.Error())
	}
	return utils.DataResponse(re, map[string]any{
		"status": "projected",
		"counts": counts,
		"total":  counts["contacts"] + counts["organisations"],
	})
}

// --- Response Builders ---

// buildOrganisationsMap pre-fetches organisations for a list of contacts
// This eliminates N+1 queries when building contact responses
func buildOrganisationsMap(records []*core.Record, app *pocketbase.PocketBase) map[string]*core.Record {
	// Collect unique organisation IDs
	orgIDsSet := make(map[string]bool)
	for _, r := range records {
		if orgID := r.GetString("organisation"); orgID != "" {
			orgIDsSet[orgID] = true
		}
	}

	// Convert set to slice
	orgIDs := make([]string, 0, len(orgIDsSet))
	for orgID := range orgIDsSet {
		orgIDs = append(orgIDs, orgID)
	}

	// Build map
	orgsMap := make(map[string]*core.Record)
	if len(orgIDs) == 0 {
		return orgsMap
	}

	// Fetch all organisations in one query using IN clause
	filter := "id IN {:ids}"
	params := map[string]any{"ids": orgIDs}

	orgs, err := app.FindRecordsByFilter(
		utils.CollectionOrganisations,
		filter,
		"",
		0, 0,
		params,
	)

	if err != nil {
		log.Printf("[BuildOrgsMap] Failed to fetch organisations: %v", err)
		return orgsMap
	}

	// Populate map
	for _, org := range orgs {
		orgsMap[org.Id] = org
	}

	log.Printf("[BuildOrgsMap] Pre-fetched %d organisations for %d contacts", len(orgsMap), len(records))

	return orgsMap
}

// buildContactResponse builds a contact response object
func buildContactResponse(r *core.Record, app *pocketbase.PocketBase, baseURL string, orgsMap map[string]*core.Record) map[string]any {
	data := map[string]any{
		"id":         r.Id,
		"email":      r.GetString("email"),
		"name":       r.GetString("name"),
		"phone":      r.GetString("phone"),
		"pronouns":   r.GetString("pronouns"),
		"bio":        r.GetString("bio"),
		"job_title":  r.GetString("job_title"),
		"linkedin":   r.GetString("linkedin"),
		"instagram":  r.GetString("instagram"),
		"website":    r.GetString("website"),
		"location":   r.GetString("location"),
		"tags":       r.Get("tags"),
		"status":     r.GetString("status"),
		"source":     r.GetString("source"),
		"source_ids": r.Get("source_ids"),
		"created":    r.GetString("created"),
		"updated":    r.GetString("updated"),
	}

	// Avatar URL
	if avatar := r.GetString("avatar"); avatar != "" {
		data["avatar_url"] = getFileURL(baseURL, r.Collection().Id, r.Id, avatar)
	}

	// Organisation relation - use map lookup instead of query
	if orgID := r.GetString("organisation"); orgID != "" {
		if org, exists := orgsMap[orgID]; exists {
			data["organisation_id"] = org.Id
			data["organisation_name"] = org.GetString("name")
		}
	}

	return data
}

// buildContactProjection builds a contact projection for COPE consumers
func buildContactProjection(r *core.Record, app *pocketbase.PocketBase, baseURL string, orgsMap map[string]*core.Record) map[string]any {
	data := map[string]any{
		"id":          r.Id,
		"email":       r.GetString("email"),
		"name":        r.GetString("name"),
		"phone":       r.GetString("phone"),
		"pronouns":    r.GetString("pronouns"),
		"bio":         r.GetString("bio"),
		"job_title":   r.GetString("job_title"),
		"linkedin":    r.GetString("linkedin"),
		"instagram":   r.GetString("instagram"),
		"website":     r.GetString("website"),
		"location":    r.GetString("location"),
		"do_position": r.GetString("do_position"),
		"tags":        r.Get("tags"),
		"created":     r.GetString("created"),
		"updated":     r.GetString("updated"),
	}

	// Avatar URL
	if avatar := r.GetString("avatar"); avatar != "" {
		data["avatar_url"] = getFileURL(baseURL, r.Collection().Id, r.Id, avatar)
	}

	// Organisation relation - use map lookup instead of query
	if orgID := r.GetString("organisation"); orgID != "" {
		if org, exists := orgsMap[orgID]; exists {
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
		"status":             r.GetString("status"),
		"source":             r.GetString("source"),
		"created":            r.GetString("created"),
		"updated":            r.GetString("updated"),
	}

	// Logo URLs
	collectionId := r.Collection().Id
	if logo := r.GetString("logo_square"); logo != "" {
		data["logo_square_url"] = getFileURL(baseURL, collectionId, r.Id, logo)
	}
	if logo := r.GetString("logo_standard"); logo != "" {
		data["logo_standard_url"] = getFileURL(baseURL, collectionId, r.Id, logo)
	}
	if logo := r.GetString("logo_inverted"); logo != "" {
		data["logo_inverted_url"] = getFileURL(baseURL, collectionId, r.Id, logo)
	}

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

	// Logo URLs
	collectionId := r.Collection().Id
	if logo := r.GetString("logo_square"); logo != "" {
		data["logo_square_url"] = getFileURL(baseURL, collectionId, r.Id, logo)
	}
	if logo := r.GetString("logo_standard"); logo != "" {
		data["logo_standard_url"] = getFileURL(baseURL, collectionId, r.Id, logo)
	}
	if logo := r.GetString("logo_inverted"); logo != "" {
		data["logo_inverted_url"] = getFileURL(baseURL, collectionId, r.Id, logo)
	}

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

// getFileURL constructs a URL to a file stored in PocketBase
func getFileURL(baseURL, collectionID, recordID, filename string) string {
	return baseURL + "/api/files/" + collectionID + "/" + recordID + "/" + filename
}

// getBaseURL returns the public base URL for the app
func getBaseURL() string {
	baseURL := os.Getenv("PUBLIC_BASE_URL")
	if baseURL == "" {
		baseURL = "https://crm.theoutlook.io"
	}
	return baseURL
}

// Placeholder to ensure filesystem import is used
var _ = filesystem.NewFileFromBytes
