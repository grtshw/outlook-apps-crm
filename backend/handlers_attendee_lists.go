package main

import (
	"sort"
	"strings"

	"github.com/grtshw/outlook-apps-crm/utils"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// handleAttendeeCompanyList returns a blind company list for a given event.
// Shows company names and job titles only — no names, emails, or other PII.
// Grouped by company with attendee count.
func handleAttendeeCompanyList(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	eventID := re.Request.PathValue("event_id")
	if eventID == "" {
		return utils.BadRequestResponse(re, "Event ID required")
	}

	// Find all ticket_purchased activities for this event
	activities, err := app.FindRecordsByFilter(
		utils.CollectionActivities,
		"type = 'ticket_purchased' && source_app = 'humanitix' && metadata ~ {:eventFilter}",
		"", 10000, 0,
		map[string]any{"eventFilter": `"event_id":"` + eventID + `"`},
	)
	if err != nil || len(activities) == 0 {
		return utils.DataResponse(re, map[string]any{
			"event_id":  eventID,
			"companies": []any{},
			"total":     0,
		})
	}

	// Collect unique contact IDs
	contactIDs := make([]string, 0, len(activities))
	for _, a := range activities {
		cid := a.GetString("contact")
		if cid != "" {
			contactIDs = append(contactIDs, cid)
		}
	}

	if len(contactIDs) == 0 {
		return utils.DataResponse(re, map[string]any{
			"event_id":  eventID,
			"companies": []any{},
			"total":     0,
		})
	}

	// Fetch contacts and their organisations
	type attendeeInfo struct {
		jobTitle    string
		orgID       string
		orgName     string
	}

	// Build company aggregation
	type companyEntry struct {
		Name       string   `json:"name"`
		Titles     []string `json:"titles"`
		Count      int      `json:"count"`
	}

	companies := map[string]*companyEntry{} // keyed by lowercase company name
	noCompanyCount := 0

	for _, cid := range contactIDs {
		contact, err := app.FindRecordById(utils.CollectionContacts, cid)
		if err != nil {
			continue
		}

		jobTitle := contact.GetString("job_title")
		orgID := contact.GetString("organisation")

		var orgName string
		if orgID != "" {
			org, err := app.FindRecordById(utils.CollectionOrganisations, orgID)
			if err == nil {
				orgName = org.GetString("name")
			}
		}

		if orgName == "" {
			noCompanyCount++
			continue
		}

		key := strings.ToLower(orgName)
		entry, ok := companies[key]
		if !ok {
			entry = &companyEntry{Name: orgName}
			companies[key] = entry
		}
		entry.Count++
		if jobTitle != "" {
			entry.Titles = append(entry.Titles, jobTitle)
		}
	}

	// Sort by count descending, then name ascending
	result := make([]map[string]any, 0, len(companies))
	for _, entry := range companies {
		// Deduplicate titles
		uniqueTitles := deduplicateStrings(entry.Titles)
		result = append(result, map[string]any{
			"company":  entry.Name,
			"titles":   uniqueTitles,
			"count":    entry.Count,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		ci := result[i]["count"].(int)
		cj := result[j]["count"].(int)
		if ci != cj {
			return ci > cj
		}
		return result[i]["company"].(string) < result[j]["company"].(string)
	})

	// Find event name from any activity's metadata
	eventName := ""
	for _, a := range activities {
		md := a.Get("metadata")
		if m, ok := md.(map[string]any); ok {
			if name, ok := m["event_name"].(string); ok && name != "" {
				eventName = name
				break
			}
		}
	}

	return utils.DataResponse(re, map[string]any{
		"event_id":         eventID,
		"event_name":       eventName,
		"companies":        result,
		"total_companies":  len(result),
		"total_attendees":  len(contactIDs),
		"no_company_count": noCompanyCount,
	})
}

// handleAttendeeCompanyListBySlug returns a blind company list using an event projection slug.
func handleAttendeeCompanyListBySlug(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	slug := re.Request.PathValue("slug")
	if slug == "" {
		return utils.BadRequestResponse(re, "Event slug required")
	}

	// Look up event projection by slug to get the event_id
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
	// Override the path value so the main handler can use it
	re.Request.SetPathValue("event_id", eventID)
	return handleAttendeeCompanyList(re, app)
}

// handleRepeatCompanies returns companies that appear across multiple events.
func handleRepeatCompanies(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	// Find all ticket_purchased activities
	activities, err := app.FindRecordsByFilter(
		utils.CollectionActivities,
		"type = 'ticket_purchased' && source_app = 'humanitix'",
		"", 50000, 0, nil,
	)
	if err != nil || len(activities) == 0 {
		return utils.DataResponse(re, map[string]any{
			"companies": []any{},
		})
	}

	// Map contact → set of events
	contactEvents := map[string]map[string]string{} // contactID → {eventID → eventName}
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
	}

	// Map company → set of events
	type companyEvents struct {
		Name   string
		Events map[string]string // eventID → eventName
		Count  int               // total attendees from this company
	}

	companies := map[string]*companyEvents{} // keyed by lowercase org name

	for cid, events := range contactEvents {
		contact, err := app.FindRecordById(utils.CollectionContacts, cid)
		if err != nil {
			continue
		}
		orgID := contact.GetString("organisation")
		if orgID == "" {
			continue
		}
		org, err := app.FindRecordById(utils.CollectionOrganisations, orgID)
		if err != nil {
			continue
		}
		orgName := org.GetString("name")
		if orgName == "" {
			continue
		}

		key := strings.ToLower(orgName)
		entry, ok := companies[key]
		if !ok {
			entry = &companyEvents{
				Name:   orgName,
				Events: map[string]string{},
			}
			companies[key] = entry
		}
		entry.Count++
		for eid, ename := range events {
			entry.Events[eid] = ename
		}
	}

	// Filter to companies appearing in 2+ events
	result := make([]map[string]any, 0)
	for _, entry := range companies {
		if len(entry.Events) < 2 {
			continue
		}
		events := make([]map[string]string, 0, len(entry.Events))
		for eid, ename := range entry.Events {
			events = append(events, map[string]string{"event_id": eid, "event_name": ename})
		}
		result = append(result, map[string]any{
			"company":      entry.Name,
			"event_count":  len(entry.Events),
			"events":       events,
			"total_tickets": entry.Count,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		ei := result[i]["event_count"].(int)
		ej := result[j]["event_count"].(int)
		if ei != ej {
			return ei > ej
		}
		return result[i]["company"].(string) < result[j]["company"].(string)
	})

	return utils.DataResponse(re, map[string]any{
		"companies":       result,
		"total_companies": len(result),
	})
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
