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

// ContactsImportConfig holds the import configuration
type ContactsImportConfig struct {
	DrupalURL   string
	DrupalToken string
	CRMURL      string
	CRMEmail    string
	CRMPassword string
	CRMToken    string // Direct token from dev tools
}

// ContactsImporter handles importing contacts from Drupal to CRM organisations
type ContactsImporter struct {
	config   ContactsImportConfig
	client   *http.Client
	crmToken string
}

// ContactsImportResult tracks the import statistics
type ContactsImportResult struct {
	Matched        int
	Updated        int
	ContactsAdded  int
	NotFound       []string
	Errors         []string
}

// OrgContact represents a contact stored on an organisation
type OrgContact struct {
	Name     string `json:"name"`
	LinkedIn string `json:"linkedin,omitempty"`
	Email    string `json:"email,omitempty"`
}

func runContactsImport(args []string) {
	fs := flag.NewFlagSet("contacts", flag.ExitOnError)
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

	config := ContactsImportConfig{
		DrupalURL:   *drupalURL,
		DrupalToken: *drupalToken,
		CRMURL:      *crmURL,
		CRMEmail:    *crmEmail,
		CRMPassword: *crmPassword,
		CRMToken:    *crmToken,
	}

	importer := &ContactsImporter{
		config: config,
		client: &http.Client{Timeout: 60 * time.Second},
	}

	result, err := importer.Run()
	if err != nil {
		log.Fatalf("Contacts import failed: %v", err)
	}

	fmt.Println("\n=== Organisation Contacts Import Complete ===")
	fmt.Printf("Matched orgs: %d\n", result.Matched)
	fmt.Printf("Updated with org contacts: %d\n", result.Updated)
	fmt.Printf("Total org contacts added: %d\n", result.ContactsAdded)
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

// Run executes the contacts import process
func (i *ContactsImporter) Run() (*ContactsImportResult, error) {
	result := &ContactsImportResult{
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

	// Fetch organisations from Drupal
	log.Println("Fetching organisations from Drupal...")
	drupalOrgs, err := i.fetchDrupalOrganisations()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from Drupal: %w", err)
	}
	log.Printf("Found %d organisations in Drupal", len(drupalOrgs.Data))

	// Process each Drupal organisation
	for _, entity := range drupalOrgs.Data {
		if entity.Type != "node--organisation" {
			continue
		}

		name := getStringAttrC(entity.Attributes, "title")
		if name == "" {
			continue
		}

		// Extract contacts from Drupal field_contacts
		contacts := i.extractContacts(entity)
		if len(contacts) == 0 {
			continue
		}

		// Find matching CRM organisation by name
		crmOrg, err := i.findCRMOrgByName(name)
		if err != nil {
			result.NotFound = append(result.NotFound, name)
			continue
		}
		result.Matched++

		log.Printf("  Updating contacts for %s (%d contacts)...", name, len(contacts))

		// Update contacts on CRM organisation
		if err := i.updateOrgContacts(crmOrg["id"].(string), contacts); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", name, err))
			continue
		}

		result.Updated++
		result.ContactsAdded += len(contacts)
	}

	return result, nil
}

// authenticateCRM logs into CRM and stores the auth token
func (i *ContactsImporter) authenticateCRM() error {
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

// fetchDrupalOrganisations fetches organisations from Drupal export API
func (i *ContactsImporter) fetchDrupalOrganisations() (*DrupalExportResponse, error) {
	apiURL := fmt.Sprintf("%s/api/export/organizations?token=%s", i.config.DrupalURL, i.config.DrupalToken)

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

// findCRMOrgByName finds a CRM organisation by name
func (i *ContactsImporter) findCRMOrgByName(name string) (map[string]interface{}, error) {
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

// extractContacts extracts contacts from Drupal entity's field_contacts
func (i *ContactsImporter) extractContacts(entity DrupalEntity) []OrgContact {
	contacts := []OrgContact{}

	// field_contacts is an array of link fields with title=name, uri=linkedin
	// May also have email in newer exports
	linksAttr := entity.Attributes["field_contacts"]
	if linksAttr == nil {
		return contacts
	}

	// Handle array of link values
	if arr, ok := linksAttr.([]interface{}); ok {
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				contact := OrgContact{}

				// title = name
				if title, ok := m["title"].(string); ok && title != "" {
					contact.Name = title
				}

				// uri = linkedin
				if uri, ok := m["uri"].(string); ok && uri != "" {
					contact.LinkedIn = uri
				}

				// email (if present in newer exports)
				if email, ok := m["email"].(string); ok && email != "" {
					contact.Email = email
				}

				// Only add if we have at least a name
				if contact.Name != "" {
					contacts = append(contacts, contact)
				}
			}
		}
	}

	// Handle single link value
	if m, ok := linksAttr.(map[string]interface{}); ok {
		contact := OrgContact{}
		if title, ok := m["title"].(string); ok && title != "" {
			contact.Name = title
		}
		if uri, ok := m["uri"].(string); ok && uri != "" {
			contact.LinkedIn = uri
		}
		if email, ok := m["email"].(string); ok && email != "" {
			contact.Email = email
		}
		if contact.Name != "" {
			contacts = append(contacts, contact)
		}
	}

	return contacts
}

// updateOrgContacts updates the contacts JSON field on a CRM organisation
func (i *ContactsImporter) updateOrgContacts(orgID string, contacts []OrgContact) error {
	contactsJSON, err := json.Marshal(contacts)
	if err != nil {
		return err
	}

	body := map[string]interface{}{
		"contacts": json.RawMessage(contactsJSON),
	}
	jsonBody, _ := json.Marshal(body)

	apiURL := fmt.Sprintf("%s/api/collections/organisations/records/%s", i.config.CRMURL, orgID)
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

// Helper function (suffixed with C to avoid redeclaration)
func getStringAttrC(attrs map[string]interface{}, key string) string {
	if v, ok := attrs[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
