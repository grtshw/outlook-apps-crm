package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ProfileOrgsImportConfig holds the import configuration
type ProfileOrgsImportConfig struct {
	DrupalURL   string
	DrupalToken string
	CRMURL      string
	CRMEmail    string
	CRMPassword string
	CRMToken    string
}

// ProfileOrgsImporter handles linking CRM contacts to organisations based on Drupal profile data
type ProfileOrgsImporter struct {
	config      ProfileOrgsImportConfig
	client      *http.Client
	crmToken    string
	includedMap map[string]DrupalEntity // UUID -> entity for resolving relationships
}

// ProfileOrgsResult tracks the import statistics
type ProfileOrgsResult struct {
	Processed  int
	Linked     int
	AlreadySet int
	NoOrg      int
	NotFound   []string
	Errors     []string
}

func runProfileOrgsImport(args []string) {
	fs := flag.NewFlagSet("profile-orgs", flag.ExitOnError)
	drupalURL := fs.String("drupal-url", "https://the-outlook.ddev.site", "Drupal website URL")
	drupalToken := fs.String("drupal-token", "", "Drupal export API token (required)")
	crmURL := fs.String("crm-url", "http://localhost:8090", "CRM API URL")
	crmEmail := fs.String("crm-email", "", "CRM admin email")
	crmPassword := fs.String("crm-password", "", "CRM admin password")
	crmToken := fs.String("crm-token", "", "CRM auth token from dev tools (alternative to email/password)")
	fs.Parse(args)

	if *drupalToken == "" {
		log.Fatal("Required flag: -drupal-token")
	}
	if *crmToken == "" && (*crmEmail == "" || *crmPassword == "") {
		log.Fatal("Required: -crm-token OR both -crm-email and -crm-password")
	}

	config := ProfileOrgsImportConfig{
		DrupalURL:   *drupalURL,
		DrupalToken: *drupalToken,
		CRMURL:      *crmURL,
		CRMEmail:    *crmEmail,
		CRMPassword: *crmPassword,
		CRMToken:    *crmToken,
	}

	importer := &ProfileOrgsImporter{
		config:      config,
		client:      &http.Client{Timeout: 60 * time.Second},
		includedMap: make(map[string]DrupalEntity),
	}

	result, err := importer.Run()
	if err != nil {
		log.Fatalf("Profile-orgs import failed: %v", err)
	}

	fmt.Println("\n=== Profile → Organisation Link Import Complete ===")
	fmt.Printf("Profiles processed: %d\n", result.Processed)
	fmt.Printf("Contacts linked to org: %d\n", result.Linked)
	fmt.Printf("Already had org set: %d\n", result.AlreadySet)
	fmt.Printf("No org in Drupal: %d\n", result.NoOrg)
	if len(result.NotFound) > 0 {
		fmt.Printf("Not found in CRM (%d):\n", len(result.NotFound))
		for _, name := range result.NotFound {
			fmt.Printf("  - %s\n", name)
		}
	}
	if len(result.Errors) > 0 {
		fmt.Printf("Errors (%d):\n", len(result.Errors))
		for _, e := range result.Errors {
			fmt.Printf("  - %s\n", e)
		}
	}
}

// Run executes the profile-orgs import process
func (i *ProfileOrgsImporter) Run() (*ProfileOrgsResult, error) {
	result := &ProfileOrgsResult{
		NotFound: []string{},
		Errors:   []string{},
	}

	// Use provided token or authenticate
	if i.config.CRMToken != "" {
		i.crmToken = i.config.CRMToken
		log.Println("Using provided CRM token")
	} else {
		log.Println("Authenticating to CRM...")
		if err := i.authenticateCRM(); err != nil {
			return nil, fmt.Errorf("CRM authentication failed: %w", err)
		}
		log.Println("Authenticated successfully")
	}

	// Fetch profiles from Drupal
	log.Println("Fetching profiles from Drupal...")
	drupalProfiles, err := i.fetchDrupalProfiles()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch profiles from Drupal: %w", err)
	}
	log.Printf("Found %d profiles in Drupal", len(drupalProfiles.Data))

	// Build included entities map for resolving organisation references
	for _, entity := range drupalProfiles.Included {
		i.includedMap[entity.ID] = entity
	}

	// Process each profile
	for _, entity := range drupalProfiles.Data {
		if entity.Type != "node--profile" {
			continue
		}

		profileName := getStringAttrPO(entity.Attributes, "title")
		if profileName == "" {
			continue
		}
		result.Processed++

		// Extract organisation name from the field_organisation relationship
		orgName := i.extractOrganisationName(entity)
		if orgName == "" {
			result.NoOrg++
			continue
		}

		// Find matching CRM contact by name
		crmContact, err := i.findCRMContactByName(profileName)
		if err != nil {
			result.NotFound = append(result.NotFound, fmt.Sprintf("contact: %s", profileName))
			continue
		}

		// Check if contact already has an organisation set
		if existingOrg, ok := crmContact["organisation_id"].(string); ok && existingOrg != "" {
			result.AlreadySet++
			continue
		}

		// Find matching CRM organisation by name
		crmOrg, err := i.findCRMOrgByName(orgName)
		if err != nil {
			result.NotFound = append(result.NotFound, fmt.Sprintf("org: %s (for contact %s)", orgName, profileName))
			continue
		}

		// Update the contact's organisation relation
		orgID := crmOrg["id"].(string)
		log.Printf("  Linking %s → %s", profileName, orgName)
		if err := i.updateContactOrg(crmContact["id"].(string), orgID); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", profileName, err))
			continue
		}

		result.Linked++
	}

	return result, nil
}

// authenticateCRM logs into CRM and stores the auth token
func (i *ProfileOrgsImporter) authenticateCRM() error {
	body := map[string]string{
		"identity": i.config.CRMEmail,
		"password": i.config.CRMPassword,
	}
	jsonBody, _ := json.Marshal(body)

	resp, err := i.client.Post(
		i.config.CRMURL+"/api/collections/users/auth-with-password",
		"application/json",
		bytes.NewReader(jsonBody),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("auth failed: %d - %s", resp.StatusCode, string(respBody))
	}

	var authResp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return err
	}

	i.crmToken = authResp.Token
	return nil
}

// fetchDrupalProfiles fetches profiles from Drupal export API
func (i *ProfileOrgsImporter) fetchDrupalProfiles() (*DrupalExportResponse, error) {
	apiURL := fmt.Sprintf("%s/api/export/profiles?token=%s", i.config.DrupalURL, i.config.DrupalToken)

	resp, err := i.client.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Drupal API error: %d - %s", resp.StatusCode, string(body))
	}

	var result DrupalExportResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// extractOrganisationName resolves the field_organisation relationship to an org name
func (i *ProfileOrgsImporter) extractOrganisationName(entity DrupalEntity) string {
	orgRel, ok := entity.Relationships["field_organisation"]
	if !ok {
		return ""
	}

	refs := extractRelationRefsPO(orgRel.Data)
	if len(refs) == 0 {
		return ""
	}

	// Use the first organisation reference (profiles can reference multiple, but CRM only supports one)
	ref := refs[0]
	if included, exists := i.includedMap[ref.ID]; exists {
		return getStringAttrPO(included.Attributes, "title")
	}

	return ""
}

// findCRMContactByName searches for a CRM contact by name
func (i *ProfileOrgsImporter) findCRMContactByName(name string) (map[string]interface{}, error) {
	apiURL := fmt.Sprintf("%s/api/contacts?search=%s&perPage=10",
		i.config.CRMURL, url.QueryEscape(name))

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", i.crmToken)

	resp, err := i.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search failed: %d", resp.StatusCode)
	}

	var listResp struct {
		Items []map[string]interface{} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, err
	}

	// Exact name match (case insensitive)
	for _, item := range listResp.Items {
		if itemName, ok := item["name"].(string); ok {
			if strings.EqualFold(itemName, name) {
				return item, nil
			}
		}
	}

	return nil, fmt.Errorf("not found")
}

// findCRMOrgByName finds a CRM organisation by name
func (i *ProfileOrgsImporter) findCRMOrgByName(name string) (map[string]interface{}, error) {
	filter := fmt.Sprintf(`name="%s"`, strings.ReplaceAll(name, `"`, `\"`))
	apiURL := fmt.Sprintf("%s/api/collections/organisations/records?filter=%s&perPage=1",
		i.config.CRMURL, url.QueryEscape(filter))

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", i.crmToken)

	resp, err := i.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lookup failed: %d", resp.StatusCode)
	}

	var listResp struct {
		Items      []map[string]interface{} `json:"items"`
		TotalItems int                      `json:"totalItems"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, err
	}

	if listResp.TotalItems == 0 || len(listResp.Items) == 0 {
		return nil, fmt.Errorf("not found")
	}

	return listResp.Items[0], nil
}

// updateContactOrg sets the organisation relation on a CRM contact
func (i *ProfileOrgsImporter) updateContactOrg(contactID, orgID string) error {
	payload := map[string]string{
		"organisation": orgID,
	}
	jsonBody, _ := json.Marshal(payload)

	apiURL := fmt.Sprintf("%s/api/contacts/%s", i.config.CRMURL, contactID)
	req, err := http.NewRequest("PATCH", apiURL, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", i.crmToken)

	resp, err := i.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update failed: %d - %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// Helper functions (suffixed with PO to avoid redeclaration)

func getStringAttrPO(attrs map[string]interface{}, key string) string {
	if v, ok := attrs[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func extractRelationRefsPO(data interface{}) []RelationRefD {
	if data == nil {
		return nil
	}

	// Single reference
	if m, ok := data.(map[string]interface{}); ok {
		id, _ := m["id"].(string)
		typ, _ := m["type"].(string)
		if id != "" {
			return []RelationRefD{{Type: typ, ID: id}}
		}
		return nil
	}

	// Array of references
	if arr, ok := data.([]interface{}); ok {
		refs := make([]RelationRefD, 0, len(arr))
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				id, _ := m["id"].(string)
				typ, _ := m["type"].(string)
				if id != "" {
					refs = append(refs, RelationRefD{Type: typ, ID: id})
				}
			}
		}
		return refs
	}

	return nil
}
