package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/grtshw/outlook-apps-crm/utils"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// PresenterFromAPI represents presenter data from Presentations app projection API
type PresenterFromAPI struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Email            string `json:"email"`
	Phone            string `json:"phone,omitempty"`
	Pronouns         string `json:"pronouns,omitempty"`
	Bio              string `json:"bio,omitempty"`
	JobTitle         string `json:"job_title,omitempty"`
	LinkedIn         string `json:"linkedin,omitempty"`
	Instagram        string `json:"instagram,omitempty"`
	Website          string `json:"website,omitempty"`
	Location         string `json:"location,omitempty"`
	DOPosition       string `json:"do_position,omitempty"`
	AvatarURL        string `json:"avatar_url,omitempty"`
	OrganisationID   string `json:"organisation_id,omitempty"`
	OrganisationName string `json:"organisation_name,omitempty"`
	Created          string `json:"created,omitempty"`
	Updated          string `json:"updated,omitempty"`
}

// PresenterProjectionResponse is the API response from Presentations
type PresenterProjectionResponse struct {
	Items []PresenterFromAPI `json:"items"`
}

// DAMAvatarResponse represents the avatar URLs from DAM
type DAMAvatarResponse struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	PresenterID       string `json:"presenter_id"`
	AvatarThumbURL    string `json:"avatar_thumb_url"`
	AvatarSmallURL    string `json:"avatar_small_url"`
	AvatarOriginalURL string `json:"avatar_original_url"`
}

// ImportResult tracks the result of the import operation
type ImportResult struct {
	Total     int      `json:"total"`
	Created   int      `json:"created"`
	Updated   int      `json:"updated"`
	Skipped   int      `json:"skipped"`
	Errors    int      `json:"errors"`
	ErrorMsgs []string `json:"error_messages,omitempty"`
}

// fetchDAMAvatarURLs fetches avatar variant URLs from DAM for a presenter
func fetchDAMAvatarURLs(presenterID string) (*DAMAvatarResponse, error) {
	damURL := os.Getenv("DAM_PUBLIC_URL")
	if damURL == "" {
		damURL = "https://outlook-apps-dam.fly.dev"
	}

	url := damURL + "/api/presenter-lookup/" + presenterID
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // Not found in DAM, not an error
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DAM returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var avatarResp DAMAvatarResponse
	if err := json.Unmarshal(body, &avatarResp); err != nil {
		return nil, err
	}

	return &avatarResp, nil
}

// handleImportPresenters imports presenters from Presentations app as contacts
func handleImportPresenters(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	// Get Presentations API URL from environment
	presentationsAPIURL := os.Getenv("PRESENTATIONS_API_URL")
	if presentationsAPIURL == "" {
		presentationsAPIURL = "https://outlook-apps-presentations.fly.dev"
	}

	// Fetch presenters from Presentations projection API
	url := presentationsAPIURL + "/api/projections/presenters"
	log.Printf("[PresenterImport] Fetching presenters from %s", url)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		log.Printf("[PresenterImport] Failed to fetch presenters: %v", err)
		return utils.InternalErrorResponse(re, "Failed to connect to Presentations app")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[PresenterImport] API returned status %d: %s", resp.StatusCode, string(body))
		return utils.InternalErrorResponse(re, fmt.Sprintf("Presentations API returned status %d", resp.StatusCode))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return utils.InternalErrorResponse(re, "Failed to read response from Presentations app")
	}

	var presentersResp PresenterProjectionResponse
	if err := json.Unmarshal(body, &presentersResp); err != nil {
		log.Printf("[PresenterImport] Failed to parse response: %v", err)
		return utils.InternalErrorResponse(re, "Failed to parse presenters data")
	}

	log.Printf("[PresenterImport] Found %d presenters to import", len(presentersResp.Items))

	// Import each presenter
	result := ImportResult{
		Total: len(presentersResp.Items),
	}

	for _, presenter := range presentersResp.Items {
		if presenter.Email == "" {
			result.Skipped++
			continue
		}

		// Check if contact exists before import to track created vs updated
		existing, _ := app.FindFirstRecordByFilter(
			utils.CollectionContacts,
			"email = {:email}",
			map[string]any{"email": presenter.Email},
		)
		wasNew := existing == nil

		err := importPresenter(app, presenter)
		if err != nil {
			result.Errors++
			result.ErrorMsgs = append(result.ErrorMsgs, fmt.Sprintf("%s: %v", presenter.Email, err))
			log.Printf("[PresenterImport] Error importing %s: %v", presenter.Email, err)
		} else {
			if wasNew {
				result.Created++
			} else {
				result.Updated++
			}
		}
	}

	log.Printf("[PresenterImport] Import complete: %d created, %d updated, %d skipped, %d errors",
		result.Created, result.Updated, result.Skipped, result.Errors)

	return utils.DataResponse(re, result)
}

// importPresenter creates or updates a contact from presenter data
func importPresenter(app *pocketbase.PocketBase, presenter PresenterFromAPI) error {
	collection, err := app.FindCollectionByNameOrId(utils.CollectionContacts)
	if err != nil {
		return fmt.Errorf("contacts collection not found: %w", err)
	}

	// Find existing contact by email
	existing, _ := app.FindFirstRecordByFilter(
		utils.CollectionContacts,
		"email = {:email}",
		map[string]any{"email": presenter.Email},
	)

	var record *core.Record
	if existing != nil {
		record = existing
	} else {
		record = core.NewRecord(collection)
	}

	// Map presenter fields to contact
	record.Set("email", presenter.Email)
	record.Set("name", presenter.Name)
	record.Set("phone", presenter.Phone)
	record.Set("pronouns", presenter.Pronouns)
	record.Set("bio", presenter.Bio)
	record.Set("job_title", presenter.JobTitle)
	record.Set("linkedin", presenter.LinkedIn)
	record.Set("instagram", presenter.Instagram)
	record.Set("website", presenter.Website)
	record.Set("location", presenter.Location)
	record.Set("do_position", presenter.DOPosition)

	// Set avatar_url (external URL from Presentations as fallback)
	if presenter.AvatarURL != "" {
		record.Set("avatar_url", presenter.AvatarURL)
	}

	// Fetch avatar variant URLs from DAM
	damAvatar, err := fetchDAMAvatarURLs(presenter.ID)
	if err != nil {
		log.Printf("[PresenterImport] Warning: could not fetch DAM avatar for %s: %v", presenter.Email, err)
	} else if damAvatar != nil {
		if damAvatar.AvatarThumbURL != "" {
			record.Set("avatar_thumb_url", damAvatar.AvatarThumbURL)
		}
		if damAvatar.AvatarSmallURL != "" {
			record.Set("avatar_small_url", damAvatar.AvatarSmallURL)
		}
		if damAvatar.AvatarOriginalURL != "" {
			record.Set("avatar_original_url", damAvatar.AvatarOriginalURL)
		}
		log.Printf("[PresenterImport] Got DAM avatar URLs for %s", presenter.Email)
	}

	// Set source tracking
	record.Set("source", "presentations")
	sourceIDs := record.Get("source_ids")
	if sourceIDs == nil {
		sourceIDs = map[string]any{}
	}
	sourceIDsMap, ok := sourceIDs.(map[string]any)
	if !ok {
		sourceIDsMap = map[string]any{}
	}
	sourceIDsMap["presentations"] = presenter.ID
	record.Set("source_ids", sourceIDsMap)

	// Add "presenter" role if not already present
	existingRoles := record.GetStringSlice("roles")
	hasPresenterRole := false
	for _, role := range existingRoles {
		if role == "presenter" {
			hasPresenterRole = true
			break
		}
	}
	if !hasPresenterRole {
		existingRoles = append(existingRoles, "presenter")
		record.Set("roles", existingRoles)
	}

	// Set status to active for new records
	if existing == nil {
		record.Set("status", "active")
	}

	// Handle organisation linking
	if presenter.OrganisationName != "" {
		// Try to find or create the organisation
		orgID, err := findOrCreateOrganisation(app, presenter.OrganisationID, presenter.OrganisationName)
		if err != nil {
			log.Printf("[PresenterImport] Warning: could not link organisation for %s: %v", presenter.Email, err)
		} else if orgID != "" {
			record.Set("organisation", orgID)
		}
	}

	return app.Save(record)
}

// findOrCreateOrganisation finds an existing organisation or creates a new one
func findOrCreateOrganisation(app *pocketbase.PocketBase, presentationsOrgID, orgName string) (string, error) {
	// First try to find by source_ids.presentations
	if presentationsOrgID != "" {
		existing, _ := app.FindFirstRecordByFilter(
			utils.CollectionOrganisations,
			"source_ids ~ {:org_id}",
			map[string]any{"org_id": presentationsOrgID},
		)
		if existing != nil {
			return existing.Id, nil
		}
	}

	// Try to find by name
	existing, _ := app.FindFirstRecordByFilter(
		utils.CollectionOrganisations,
		"name = {:name}",
		map[string]any{"name": orgName},
	)
	if existing != nil {
		return existing.Id, nil
	}

	// Create new organisation
	collection, err := app.FindCollectionByNameOrId(utils.CollectionOrganisations)
	if err != nil {
		return "", err
	}

	org := core.NewRecord(collection)
	org.Set("name", orgName)
	org.Set("status", "active")
	org.Set("source", "presentations")
	if presentationsOrgID != "" {
		org.Set("source_ids", map[string]any{"presentations": presentationsOrgID})
	}

	if err := app.Save(org); err != nil {
		return "", err
	}

	return org.Id, nil
}
