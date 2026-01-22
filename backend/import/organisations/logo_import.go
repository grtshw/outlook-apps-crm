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

// DrupalExportResponse represents the JSON:API export response from Drupal
type DrupalExportResponse struct {
	Data     []DrupalEntity `json:"data"`
	Included []DrupalEntity `json:"included"`
}

// DrupalEntity represents a generic entity from the Drupal export
type DrupalEntity struct {
	ID            string                    `json:"id"`
	Type          string                    `json:"type"`
	Attributes    map[string]interface{}    `json:"attributes"`
	Relationships map[string]DrupalRelation `json:"relationships"`
}

// DrupalRelation represents an entity reference field
type DrupalRelation struct {
	Data interface{} `json:"data"`
}

// ImageField represents a Drupal image field value
type ImageField struct {
	URL string `json:"url"`
}

// LogoImportConfig holds the import configuration
type LogoImportConfig struct {
	DrupalURL   string
	DrupalToken string
	CRMURL      string
	CRMEmail    string
	CRMPassword string
}

// LogoImporter handles importing logos from Drupal to CRM
type LogoImporter struct {
	config      LogoImportConfig
	client      *http.Client
	crmToken    string
	includedMap map[string]DrupalEntity
}

// LogoImportResult tracks the import statistics
type LogoImportResult struct {
	Matched      int
	Updated      int
	LogosCopied  int
	LogosFailed  int
	NotFound     []string
	Errors       []string
}

func runLogoImport(args []string) {
	fs := flag.NewFlagSet("logos", flag.ExitOnError)
	drupalURL := fs.String("drupal-url", "https://the-outlook.ddev.site", "Drupal website URL")
	drupalToken := fs.String("drupal-token", "", "Drupal export API token (required)")
	crmURL := fs.String("crm-url", "http://localhost:8090", "CRM API URL")
	crmEmail := fs.String("crm-email", "", "CRM admin email (required)")
	crmPassword := fs.String("crm-password", "", "CRM admin password (required)")
	fs.Parse(args)

	if *drupalToken == "" || *crmEmail == "" || *crmPassword == "" {
		log.Fatal("Required flags: -drupal-token, -crm-email, -crm-password")
	}

	config := LogoImportConfig{
		DrupalURL:   *drupalURL,
		DrupalToken: *drupalToken,
		CRMURL:      *crmURL,
		CRMEmail:    *crmEmail,
		CRMPassword: *crmPassword,
	}

	importer := &LogoImporter{
		config:      config,
		client:      &http.Client{Timeout: 60 * time.Second},
		includedMap: make(map[string]DrupalEntity),
	}

	result, err := importer.Run()
	if err != nil {
		log.Fatalf("Logo import failed: %v", err)
	}

	fmt.Println("\n=== Logo Import Complete ===")
	fmt.Printf("Matched orgs: %d\n", result.Matched)
	fmt.Printf("Updated with logos: %d\n", result.Updated)
	fmt.Printf("Logos copied: %d\n", result.LogosCopied)
	fmt.Printf("Logos failed: %d\n", result.LogosFailed)
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

// Run executes the logo import process
func (i *LogoImporter) Run() (*LogoImportResult, error) {
	result := &LogoImportResult{
		NotFound: []string{},
		Errors:   []string{},
	}

	// Authenticate to CRM
	log.Println("Authenticating to CRM...")
	if err := i.authenticateCRM(); err != nil {
		return nil, fmt.Errorf("CRM authentication failed: %w", err)
	}
	log.Println("Authenticated successfully")

	// Fetch organisations from Drupal
	log.Println("Fetching organisations from Drupal...")
	drupalOrgs, err := i.fetchDrupalOrganisations()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from Drupal: %w", err)
	}
	log.Printf("Found %d organisations in Drupal", len(drupalOrgs.Data))

	// Build included entities map for media lookups
	for _, entity := range drupalOrgs.Included {
		i.includedMap[entity.ID] = entity
	}

	// Process each Drupal organisation
	for _, entity := range drupalOrgs.Data {
		if entity.Type != "node--organisation" {
			continue
		}

		name := getStringAttrD(entity.Attributes, "title")
		if name == "" {
			continue
		}

		// Find matching CRM organisation by name
		crmOrg, err := i.findCRMOrgByName(name)
		if err != nil {
			result.NotFound = append(result.NotFound, name)
			continue
		}
		result.Matched++

		// Extract logo URLs from Drupal
		logos := i.extractLogos(entity)
		if len(logos) == 0 {
			continue
		}

		log.Printf("  Updating logos for %s (%d logos)...", name, len(logos))

		// Upload logos to CRM
		updated, copied, failed := i.uploadLogos(crmOrg["id"].(string), logos)
		if updated {
			result.Updated++
		}
		result.LogosCopied += copied
		result.LogosFailed += failed
	}

	return result, nil
}

// authenticateCRM logs into CRM and stores the auth token
func (i *LogoImporter) authenticateCRM() error {
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
func (i *LogoImporter) fetchDrupalOrganisations() (*DrupalExportResponse, error) {
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
func (i *LogoImporter) findCRMOrgByName(name string) (map[string]interface{}, error) {
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

// extractLogos extracts logo URLs from Drupal entity
func (i *LogoImporter) extractLogos(entity DrupalEntity) map[string]string {
	logos := make(map[string]string)

	// field_organisation_logo (image field) -> logo_square
	if imageData := getImageAttrD(entity.Attributes, "field_organisation_logo"); imageData != nil && imageData.URL != "" {
		logos["logo_square"] = imageData.URL
	}

	// field_organisation_logo_svg (media reference) -> logo_standard
	if logoSvgRel, ok := entity.Relationships["field_organisation_logo_svg"]; ok {
		if refs := extractRelationRefsD(logoSvgRel.Data); len(refs) > 0 {
			if fileURL := i.resolveMediaURL(refs[0].ID); fileURL != "" {
				logos["logo_standard"] = fileURL
			}
		}
	}

	// field_partner_logo (image field) -> logo_inverted
	if imageData := getImageAttrD(entity.Attributes, "field_partner_logo"); imageData != nil && imageData.URL != "" {
		logos["logo_inverted"] = imageData.URL
	}

	return logos
}

// resolveMediaURL resolves a media entity reference to a file URL
func (i *LogoImporter) resolveMediaURL(mediaID string) string {
	media, exists := i.includedMap[mediaID]
	if !exists {
		return ""
	}

	// Find file reference in media relationships
	var fileID string
	for _, relName := range []string{"field_media_image", "field_media_svg"} {
		if fileRel, ok := media.Relationships[relName]; ok {
			if refs := extractRelationRefsD(fileRel.Data); len(refs) > 0 {
				fileID = refs[0].ID
				break
			}
		}
	}

	if fileID == "" {
		return ""
	}

	file, exists := i.includedMap[fileID]
	if !exists {
		return ""
	}

	// Get file URL
	if uriAttr, ok := file.Attributes["uri"].(map[string]interface{}); ok {
		if fileURL, _ := uriAttr["url"].(string); fileURL != "" {
			return fileURL
		}
	}

	return ""
}

// uploadLogos uploads logos to CRM organisation
func (i *LogoImporter) uploadLogos(orgID string, logos map[string]string) (updated bool, copied, failed int) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	for field, logoURL := range logos {
		fileData, filename, err := i.downloadFile(logoURL)
		if err != nil {
			log.Printf("    Failed to download %s: %v", field, err)
			failed++
			continue
		}

		part, err := writer.CreateFormFile(field, filename)
		if err != nil {
			failed++
			continue
		}
		if _, err := part.Write(fileData); err != nil {
			failed++
			continue
		}
		copied++
	}

	writer.Close()

	if copied == 0 {
		return false, 0, failed
	}

	// PATCH the organisation
	apiURL := fmt.Sprintf("%s/api/collections/organisations/records/%s", i.config.CRMURL, orgID)
	req, err := http.NewRequest("PATCH", apiURL, &buf)
	if err != nil {
		return false, copied, failed
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", i.crmToken)

	resp, err := i.client.Do(req)
	if err != nil {
		return false, copied, failed
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("    Failed to update: %s", string(body))
		return false, copied, failed
	}

	return true, copied, failed
}

// downloadFile downloads a file from URL
func (i *LogoImporter) downloadFile(fileURL string) ([]byte, string, error) {
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

// Helper functions

type RelationRefD struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

func getStringAttrD(attrs map[string]interface{}, key string) string {
	if v, ok := attrs[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getImageAttrD(attrs map[string]interface{}, key string) *ImageField {
	v, ok := attrs[key]
	if !ok || v == nil {
		return nil
	}

	m, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}

	urlStr, _ := m["url"].(string)
	if urlStr == "" {
		return nil
	}

	return &ImageField{URL: urlStr}
}

func extractRelationRefsD(data interface{}) []RelationRefD {
	if data == nil {
		return nil
	}

	// Single reference
	if m, ok := data.(map[string]interface{}); ok {
		id, _ := m["id"].(string)
		typ, _ := m["type"].(string)
		return []RelationRefD{{Type: typ, ID: id}}
	}

	// Array of references
	if arr, ok := data.([]interface{}); ok {
		refs := make([]RelationRefD, 0, len(arr))
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				id, _ := m["id"].(string)
				typ, _ := m["type"].(string)
				refs = append(refs, RelationRefD{Type: typ, ID: id})
			}
		}
		return refs
	}

	return nil
}
