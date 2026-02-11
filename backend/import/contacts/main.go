package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type Config struct {
	CSVFile     string
	CRMURL      string
	CRMEmail    string
	CRMPassword string
	DryRun      bool
}

type Importer struct {
	config   Config
	client   *http.Client
	crmToken string
	orgCache map[string]string // name -> id
}

type Result struct {
	Created int
	Updated int
	Skipped int
	Errors  []string
}

func main() {
	csvFile := flag.String("csv", "", "Path to CSV file (required)")
	crmURL := flag.String("crm-url", "http://localhost:8090", "CRM API URL")
	crmEmail := flag.String("crm-email", "", "CRM admin email (required)")
	crmPassword := flag.String("crm-password", "", "CRM admin password (required)")
	dryRun := flag.Bool("dry-run", false, "Parse and print without importing")
	flag.Parse()

	if *csvFile == "" || *crmEmail == "" || *crmPassword == "" {
		fmt.Println("Usage: go run ./backend/import/contacts -csv FILE -crm-email EMAIL -crm-password PASS")
		fmt.Println("\nFlags:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	config := Config{
		CSVFile:     *csvFile,
		CRMURL:      *crmURL,
		CRMEmail:    *crmEmail,
		CRMPassword: *crmPassword,
		DryRun:      *dryRun,
	}

	importer := &Importer{
		config:   config,
		client:   &http.Client{Timeout: 30 * time.Second},
		orgCache: make(map[string]string),
	}

	result, err := importer.Run()
	if err != nil {
		log.Fatalf("Import failed: %v", err)
	}

	fmt.Println("\n=== Import Complete ===")
	fmt.Printf("Created: %d\n", result.Created)
	fmt.Printf("Updated: %d\n", result.Updated)
	fmt.Printf("Skipped: %d\n", result.Skipped)
	if len(result.Errors) > 0 {
		fmt.Printf("Errors: %d\n", len(result.Errors))
		for _, e := range result.Errors {
			fmt.Printf("  - %s\n", e)
		}
	}
}

func (i *Importer) Run() (*Result, error) {
	result := &Result{Errors: []string{}}

	// Parse CSV
	rows, err := i.parseCSV()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}
	log.Printf("Parsed %d rows from CSV", len(rows))

	if i.config.DryRun {
		for _, r := range rows {
			fmt.Printf("  %s | %s | %s | degrees=%s | relationship=%d | notes=%s\n",
				r["name"], r["job_title"], r["company"], r["degrees"], parseStars(r["relationship"]), r["notes"])
		}
		return result, nil
	}

	// Authenticate
	log.Println("Authenticating to CRM...")
	if err := i.authenticateCRM(); err != nil {
		return nil, fmt.Errorf("CRM auth failed: %w", err)
	}

	// Pre-fetch all organisations for matching
	log.Println("Fetching organisations...")
	if err := i.loadOrganisations(); err != nil {
		return nil, fmt.Errorf("failed to load organisations: %w", err)
	}
	log.Printf("Loaded %d organisations", len(i.orgCache))

	// Import each row
	for _, row := range rows {
		if err := i.importRow(row, result); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", row["name"], err))
		}
	}

	return result, nil
}

func (i *Importer) parseCSV() ([]map[string]string, error) {
	f, err := os.Open(i.config.CSVFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.LazyQuotes = true

	// Read header
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	// Map header names to our field names
	colMap := map[string]int{}
	for idx, h := range header {
		switch strings.TrimSpace(strings.ToLower(h)) {
		case "name":
			colMap["name"] = idx
		case "role":
			colMap["job_title"] = idx
		case "company":
			colMap["company"] = idx
		case "linkedin":
			colMap["linkedin"] = idx
		case "city":
			colMap["location"] = idx
		case "degrees":
			colMap["degrees"] = idx
		case "relationship":
			colMap["relationship"] = idx
		case "notes":
			colMap["notes"] = idx
		}
	}

	var rows []map[string]string
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Warning: skipping malformed row: %v", err)
			continue
		}

		row := map[string]string{}
		for field, idx := range colMap {
			if idx < len(record) {
				row[field] = strings.TrimSpace(record[idx])
			}
		}

		// Skip empty rows
		if row["name"] == "" {
			continue
		}

		rows = append(rows, row)
	}

	return rows, nil
}

// parseStars converts unicode star strings like "★★★☆☆" to a number 0-5
func parseStars(s string) int {
	count := 0
	for _, r := range s {
		if r == '★' {
			count++
		}
	}
	return count
}

func (i *Importer) importRow(row map[string]string, result *Result) error {
	name := row["name"]

	// Search for existing contact by name
	existingID, err := i.findContactByName(name)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	// Build payload with enrichment fields
	payload := map[string]any{}

	if row["job_title"] != "" {
		payload["job_title"] = row["job_title"]
	}
	if row["linkedin"] != "" {
		payload["linkedin"] = row["linkedin"]
	}
	if row["location"] != "" {
		payload["location"] = row["location"]
	}
	if row["degrees"] != "" {
		payload["degrees"] = row["degrees"]
	}
	if rel := row["relationship"]; rel != "" {
		payload["relationship"] = parseStars(rel)
	}
	if row["notes"] != "" {
		payload["notes"] = row["notes"]
	}

	// Match company to organisation
	if company := row["company"]; company != "" {
		if orgID := i.matchOrganisation(company); orgID != "" {
			payload["organisation"] = orgID
		} else {
			log.Printf("  Warning: no org match for %q (contact: %s)", company, name)
		}
	}

	if existingID != "" {
		// Update existing contact
		if err := i.updateContact(existingID, payload); err != nil {
			return err
		}
		log.Printf("  Updated: %s", name)
		result.Updated++
	} else {
		// Create new contact — need name and a placeholder email
		payload["name"] = name
		payload["status"] = "active"
		payload["source"] = "manual"
		if err := i.createContact(payload); err != nil {
			return err
		}
		log.Printf("  Created: %s", name)
		result.Created++
	}

	return nil
}

func (i *Importer) findContactByName(name string) (string, error) {
	apiURL := fmt.Sprintf("%s/api/contacts?search=%s&perPage=5",
		i.config.CRMURL, url.QueryEscape(name))

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", i.crmToken)

	resp, err := i.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("search failed: %d", resp.StatusCode)
	}

	var listResp struct {
		Data struct {
			Items []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return "", err
	}

	// Exact name match
	for _, item := range listResp.Data.Items {
		if strings.EqualFold(item.Name, name) {
			return item.ID, nil
		}
	}
	return "", nil
}

func (i *Importer) createContact(payload map[string]any) error {
	jsonBody, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", i.config.CRMURL+"/api/contacts", bytes.NewReader(jsonBody))
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

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create failed: %d - %s", resp.StatusCode, string(body))
	}
	return nil
}

func (i *Importer) updateContact(id string, payload map[string]any) error {
	jsonBody, _ := json.Marshal(payload)

	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/api/contacts/%s", i.config.CRMURL, id), bytes.NewReader(jsonBody))
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
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update failed: %d - %s", resp.StatusCode, string(body))
	}
	return nil
}

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

	var authResp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return err
	}

	i.crmToken = authResp.Token
	return nil
}

func (i *Importer) loadOrganisations() error {
	page := 1
	for {
		apiURL := fmt.Sprintf("%s/api/organisations?page=%d&perPage=200", i.config.CRMURL, page)

		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", i.crmToken)

		resp, err := i.client.Do(req)
		if err != nil {
			return err
		}

		var listResp struct {
			Data struct {
				Items []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"items"`
				TotalPages int `json:"totalPages"`
			} `json:"data"`
		}
		json.NewDecoder(resp.Body).Decode(&listResp)
		resp.Body.Close()

		for _, org := range listResp.Data.Items {
			i.orgCache[strings.ToLower(org.Name)] = org.ID
		}

		if page >= listResp.Data.TotalPages {
			break
		}
		page++
	}

	return nil
}

// matchOrganisation finds an org by name with fuzzy matching
func (i *Importer) matchOrganisation(company string) string {
	lower := strings.ToLower(strings.TrimSpace(company))

	// Exact match
	if id, ok := i.orgCache[lower]; ok {
		return id
	}

	// Try common variations: strip suffixes like "Group", "Australia", etc.
	for orgName, id := range i.orgCache {
		// Check if either contains the other
		if strings.Contains(orgName, lower) || strings.Contains(lower, orgName) {
			return id
		}
	}

	return ""
}
