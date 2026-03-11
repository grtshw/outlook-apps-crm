package main

import (
	"sort"
	"strings"

	"github.com/grtshw/outlook-apps-crm/utils"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// handleAttendeeCompanies returns a company list across one or more events.
// Query params:
//   - events: comma-separated event IDs (optional, defaults to all)
//   - search: filter companies by name
//
// Returns company names, logo URLs, attendee counts, event counts, and titles.
// No PII (names, emails) — safe to share with sponsors.
func handleAttendeeCompanies(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	eventsParam := re.Request.URL.Query().Get("events")
	searchParam := strings.ToLower(re.Request.URL.Query().Get("search"))

	// Build filter for ticket_purchased activities
	filter := "type = 'ticket_purchased' && source_app = 'humanitix'"
	filterParams := map[string]any{}

	var eventFilter []string
	if eventsParam != "" {
		for _, eid := range strings.Split(eventsParam, ",") {
			eid = strings.TrimSpace(eid)
			if eid != "" {
				eventFilter = append(eventFilter, eid)
			}
		}
	}

	// If specific events requested, add metadata filter for each
	if len(eventFilter) > 0 {
		clauses := make([]string, 0, len(eventFilter))
		for i, eid := range eventFilter {
			key := "ef" + strings.Repeat("x", i) // unique param name
			clauses = append(clauses, "metadata ~ {:"+key+"}")
			filterParams[key] = `"event_id":"` + eid + `"`
		}
		filter += " && (" + strings.Join(clauses, " || ") + ")"
	}

	activities, err := app.FindRecordsByFilter(
		utils.CollectionActivities,
		filter,
		"", 50000, 0,
		filterParams,
	)
	if err != nil || len(activities) == 0 {
		return utils.DataResponse(re, map[string]any{
			"companies":       []any{},
			"events":          []any{},
			"total_companies": 0,
			"total_attendees": 0,
		})
	}

	// Map contact → set of events
	contactEvents := map[string]map[string]string{} // contactID → {eventID → eventName}
	allEvents := map[string]string{}                 // eventID → eventName
	for _, a := range activities {
		cid := a.GetString("contact")
		if cid == "" {
			continue
		}
		md := a.Get("metadata")
		m, ok := md.(map[string]any)
		if !ok {
			continue
		}
		eid, _ := m["event_id"].(string)
		ename, _ := m["event_name"].(string)
		if eid == "" {
			continue
		}
		if contactEvents[cid] == nil {
			contactEvents[cid] = map[string]string{}
		}
		contactEvents[cid][eid] = ename
		allEvents[eid] = ename
	}

	// Aggregate by company
	type companyEntry struct {
		Name    string
		LogoURL string
		Titles  []string
		Events  map[string]string // eventID → eventName
		Count   int
	}

	companies := map[string]*companyEntry{} // keyed by lowercase org name
	orgCache := map[string]*core.Record{}   // orgID → record
	noCompanyCount := 0

	for cid, events := range contactEvents {
		contact, err := app.FindRecordById(utils.CollectionContacts, cid)
		if err != nil {
			continue
		}

		orgID := contact.GetString("organisation")
		if orgID == "" {
			noCompanyCount++
			continue
		}

		// Cache org lookups
		org, ok := orgCache[orgID]
		if !ok {
			org, err = app.FindRecordById(utils.CollectionOrganisations, orgID)
			if err != nil {
				noCompanyCount++
				continue
			}
			orgCache[orgID] = org
		}

		orgName := org.GetString("name")
		if orgName == "" {
			noCompanyCount++
			continue
		}

		key := strings.ToLower(orgName)

		// Apply search filter
		if searchParam != "" && !strings.Contains(key, searchParam) {
			continue
		}

		entry, exists := companies[key]
		if !exists {
			logoURL := org.GetString("logo_square_url")
			// Fall back to Clearbit if no stored logo
			if logoURL == "" {
				if website := org.GetString("website"); website != "" {
					logoURL = guessLogoURL(website)
				}
			}
			entry = &companyEntry{
				Name:    orgName,
				LogoURL: logoURL,
				Events:  map[string]string{},
			}
			companies[key] = entry
		}
		entry.Count++
		jobTitle := contact.GetString("job_title")
		if jobTitle != "" {
			entry.Titles = append(entry.Titles, jobTitle)
		}
		for eid, ename := range events {
			entry.Events[eid] = ename
		}
	}

	// Build result
	result := make([]map[string]any, 0, len(companies))
	for _, entry := range companies {
		eventNames := make([]string, 0, len(entry.Events))
		for _, ename := range entry.Events {
			if ename != "" {
				eventNames = append(eventNames, ename)
			}
		}
		sort.Strings(eventNames)

		result = append(result, map[string]any{
			"company":        entry.Name,
			"logo_url":       entry.LogoURL,
			"titles":         deduplicateStrings(entry.Titles),
			"attendee_count": entry.Count,
			"event_count":    len(entry.Events),
			"events":         eventNames,
		})
	}

	// Sort by attendee count descending, then name ascending
	sort.Slice(result, func(i, j int) bool {
		ci := result[i]["attendee_count"].(int)
		cj := result[j]["attendee_count"].(int)
		if ci != cj {
			return ci > cj
		}
		return result[i]["company"].(string) < result[j]["company"].(string)
	})

	// Build events list
	eventsList := make([]map[string]string, 0, len(allEvents))
	for eid, ename := range allEvents {
		eventsList = append(eventsList, map[string]string{"event_id": eid, "event_name": ename})
	}
	sort.Slice(eventsList, func(i, j int) bool {
		return eventsList[i]["event_name"] < eventsList[j]["event_name"]
	})

	totalAttendees := 0
	for _, entry := range companies {
		totalAttendees += entry.Count
	}

	return utils.DataResponse(re, map[string]any{
		"companies":        result,
		"events":           eventsList,
		"total_companies":  len(result),
		"total_attendees":  totalAttendees,
		"no_company_count": noCompanyCount,
	})
}

// handleAttendeeCompanyList returns a blind company list for a given event (legacy).
func handleAttendeeCompanyList(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	eventID := re.Request.PathValue("event_id")
	if eventID == "" {
		return utils.BadRequestResponse(re, "Event ID required")
	}
	re.Request.URL.RawQuery = "events=" + eventID
	return handleAttendeeCompanies(re, app)
}

// handleAttendeeCompanyListBySlug returns a blind company list using an event projection slug.
func handleAttendeeCompanyListBySlug(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	slug := re.Request.PathValue("slug")
	if slug == "" {
		return utils.BadRequestResponse(re, "Event slug required")
	}

	projections, err := app.FindRecordsByFilter(
		utils.CollectionEventProjections,
		"slug = {:slug}",
		"", 1, 0,
		map[string]any{"slug": slug},
	)
	if err != nil || len(projections) == 0 {
		return utils.NotFoundResponse(re, "Event not found")
	}

	eventID := projections[0].GetString("event_id")
	re.Request.URL.RawQuery = "events=" + eventID
	return handleAttendeeCompanies(re, app)
}

// handleRepeatCompanies returns companies that appear across multiple events.
func handleRepeatCompanies(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	// Use the unified handler — it returns all companies across all events
	// The frontend can filter to event_count >= 2
	return handleAttendeeCompanies(re, app)
}

func deduplicateStrings(input []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(input))
	for _, s := range input {
		key := strings.ToLower(s)
		if !seen[key] {
			seen[key] = true
			result = append(result, s)
		}
	}
	return result
}
