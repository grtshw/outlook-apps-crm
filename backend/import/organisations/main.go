package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

// LogoURL represents a logo URL from the projections API
type LogoURL struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Organisation represents the organisation data from Presentations projections API
type Organisation struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	LogoURLs []LogoURL `json:"logo_urls"`
}

// ProjectionResponse is the projections API response format
type ProjectionResponse struct {
	Items      []Organisation `json:"items"`
	TotalItems int            `json:"totalItems"`
}

// ListResponse is the PocketBase list response format (for CRM)
type ListResponse struct {
	Items      []json.RawMessage `json:"items"`
	TotalItems int               `json:"totalItems"`
}

// AuthResponse is the PocketBase auth response
type AuthResponse struct {
	Token  string          `json:"token"`
	Record json.RawMessage `json:"record"`
}

// Config holds the import configuration
type Config struct {
	PresentationsURL string
	CRMURL           string
	CRMEmail         string
	CRMPassword      string
}

// Importer handles the organisation migration
type Importer struct {
	config   Config
	client   *http.Client
	crmToken string
}

// ImportResult tracks the import statistics
type ImportResult struct {
	Created     int
	Skipped     int
	Errors      []string
	LogosCopied int
	LogosFailed int
}

func main() {
	presURL := flag.String("presentations-url", "http://localhost:8091", "Presentations API URL")
	crmURL := flag.String("crm-url", "http://localhost:8090", "CRM API URL")
	crmEmail := flag.String("crm-email", "", "CRM admin email (required)")
	crmPassword := flag.String("crm-password", "", "CRM admin password (required)")
	flag.Parse()

	if *crmEmail == "" || *crmPassword == "" {
		log.Fatal("CRM credentials required: -crm-email and -crm-password")
	}

	config := Config{
		PresentationsURL: *presURL,
		CRMURL:           *crmURL,
		CRMEmail:         *crmEmail,
		CRMPassword:      *crmPassword,
	}

	importer := &Importer{
		config: config,
		client: &http.Client{Timeout: 60 * time.Second},
	}

	result, err := importer.Run()
	if err != nil {
		log.Fatalf("Import failed: %v", err)
	}

	fmt.Println("\n=== Import Complete ===")
	fmt.Printf("Created: %d\n", result.Created)
	fmt.Printf("Skipped: %d (already exist)\n", result.Skipped)
	fmt.Printf("Logos copied: %d\n", result.LogosCopied)
	fmt.Printf("Logos failed: %d\n", result.LogosFailed)
	if len(result.Errors) > 0 {
		fmt.Printf("Errors: %d\n", len(result.Errors))
		for _, e := range result.Errors {
			fmt.Printf("  - %s\n", e)
		}
	}
}

// Run executes the import process
func (i *Importer) Run() (*ImportResult, error) {
	result := &ImportResult{Errors: []string{}}

	// Authenticate to CRM
	log.Println("Authenticating to CRM...")
	if err := i.authenticateCRM(); err != nil {
		return nil, fmt.Errorf("CRM authentication failed: %w", err)
	}
	log.Println("Authenticated successfully")

	// Fetch organisations from Presentations
	log.Println("Fetching organisations from Presentations...")
	orgs, err := i.fetchOrganisations()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch organisations: %w", err)
	}
	log.Printf("Found %d organisations", len(orgs))

	// Import each organisation
	for _, org := range orgs {
		if err := i.importOrganisation(org, result); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", org.Name, err))
		}
	}

	return result, nil
}

// authenticateCRM logs into CRM and stores the auth token
func (i *Importer) authenticateCRM() error {
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

	var authResp AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return err
	}

	i.crmToken = authResp.Token
	return nil
}

// fetchOrganisations gets all organisations from Presentations projections API
func (i *Importer) fetchOrganisations() ([]Organisation, error) {
	apiURL := fmt.Sprintf("%s/api/projections/organisations", i.config.PresentationsURL)

	resp, err := i.client.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fetch failed: %d - %s", resp.StatusCode, string(body))
	}

	var projResp ProjectionResponse
	if err := json.NewDecoder(resp.Body).Decode(&projResp); err != nil {
		return nil, err
	}

	return projResp.Items, nil
}

// importOrganisation creates a single organisation in CRM
func (i *Importer) importOrganisation(org Organisation, result *ImportResult) error {
	// Check if already exists by name
	exists, err := i.organisationExists(org.Name)
	if err != nil {
		return fmt.Errorf("existence check failed: %w", err)
	}
	if exists {
		log.Printf("  Skipping %s (already exists)", org.Name)
		result.Skipped++
		return nil
	}

	log.Printf("  Importing %s...", org.Name)

	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add text fields
	writer.WriteField("name", org.Name)
	writer.WriteField("status", "active")
	writer.WriteField("source", "presentations")

	// Download and add logo files from URLs
	for _, logo := range org.LogoURLs {
		if logo.URL == "" {
			continue
		}

		// Map logo name to field name
		fieldName := ""
		switch logo.Name {
		case "Square":
			fieldName = "logo_square"
		case "Standard":
			fieldName = "logo_standard"
		case "Inverted":
			fieldName = "logo_inverted"
		default:
			continue
		}

		fileData, filename, err := i.downloadFromURL(logo.URL)
		if err != nil {
			log.Printf("    Failed to download %s: %v", fieldName, err)
			result.LogosFailed++
			continue
		}

		// Add file to multipart form
		part, err := writer.CreateFormFile(fieldName, filename)
		if err != nil {
			result.LogosFailed++
			continue
		}
		if _, err := part.Write(fileData); err != nil {
			result.LogosFailed++
			continue
		}
		result.LogosCopied++
	}

	writer.Close()

	// Create record in CRM
	req, err := http.NewRequest("POST", i.config.CRMURL+"/api/collections/organisations/records", &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", i.crmToken)

	resp, err := i.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create failed: %d - %s", resp.StatusCode, string(body))
	}

	result.Created++
	return nil
}

// organisationExists checks if an organisation with the given name exists in CRM
func (i *Importer) organisationExists(name string) (bool, error) {
	// URL encode the filter
	filter := fmt.Sprintf(`name="%s"`, strings.ReplaceAll(name, `"`, `\"`))
	apiURL := fmt.Sprintf("%s/api/collections/organisations/records?filter=%s&perPage=1",
		i.config.CRMURL, url.QueryEscape(filter))

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", i.crmToken)

	resp, err := i.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("check failed: %d", resp.StatusCode)
	}

	var listResp ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return false, err
	}

	return listResp.TotalItems > 0, nil
}

// downloadFromURL downloads a file from a URL and returns the data and filename
func (i *Importer) downloadFromURL(fileURL string) ([]byte, string, error) {
	resp, err := i.client.Get(fileURL)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("download failed: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	// Extract filename from URL
	parsedURL, err := url.Parse(fileURL)
	if err != nil {
		return data, "logo.png", nil
	}
	filename := filepath.Base(parsedURL.Path)

	return data, filename, nil
}
