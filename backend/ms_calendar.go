package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/grtshw/outlook-apps-crm/utils"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// graphEvent represents a Microsoft Graph calendar event.
type graphEvent struct {
	Subject string         `json:"subject"`
	Body    graphBody      `json:"body"`
	Start   graphDateTime  `json:"start"`
	End     graphDateTime  `json:"end"`
	Location *graphLocation `json:"location,omitempty"`
	ResponseRequested bool `json:"responseRequested"`
}

type graphBody struct {
	ContentType string `json:"contentType"`
	Content     string `json:"content"`
}

type graphDateTime struct {
	DateTime string `json:"dateTime"`
	TimeZone string `json:"timeZone"`
}

type graphLocation struct {
	DisplayName string `json:"displayName"`
}

type graphAttendee struct {
	EmailAddress graphEmail `json:"emailAddress"`
	Type         string     `json:"type"`
}

type graphEmail struct {
	Address string `json:"address"`
	Name    string `json:"name"`
}

type attendeeInfo struct {
	Email string
	Name  string
}

// createCalendarEvent creates an Outlook calendar event on the host's calendar.
// Returns the Graph API event ID.
func createCalendarEvent(app *pocketbase.PocketBase, hostUserID string, guestList *core.Record) (string, error) {
	token, err := getValidMicrosoftToken(app, hostUserID)
	if err != nil {
		return "", fmt.Errorf("failed to get token: %w", err)
	}

	// Resolve event details
	eventName := guestList.GetString("name")
	var startDate, endDate, startTime, endTime, timezone, description, location string

	if epID := guestList.GetString("event_projection"); epID != "" {
		if ep, err := app.FindRecordById(utils.CollectionEventProjections, epID); err == nil {
			if n := ep.GetString("name"); n != "" {
				eventName = n
			}
			startDate = ep.GetString("start_date")
			endDate = ep.GetString("end_date")
			startTime = ep.GetString("start_time")
			endTime = ep.GetString("end_time")
			timezone = ep.GetString("timezone")
			description = ep.GetString("description")
		}
	}

	// Guest list overrides
	if glDate := guestList.GetString("event_date"); glDate != "" && startDate == "" {
		startDate = glDate
	}
	if glTime := guestList.GetString("event_time"); glTime != "" && startTime == "" {
		startTime = glTime
	}
	if glLoc := guestList.GetString("event_location"); glLoc != "" {
		location = glLoc
	}

	if startDate == "" {
		return "", fmt.Errorf("no event date available — set event dates before creating a calendar event")
	}

	if timezone == "" {
		timezone = "Australia/Sydney"
	}

	start, err := parseEventDateTime(startDate, startTime)
	if err != nil {
		return "", fmt.Errorf("failed to parse start date: %w", err)
	}

	var end time.Time
	if endDate != "" {
		end, err = parseEventDateTime(endDate, endTime)
		if err != nil {
			end = start.Add(2 * time.Hour)
		}
	} else {
		end = start.Add(2 * time.Hour)
	}

	event := graphEvent{
		Subject: eventName,
		Body: graphBody{
			ContentType: "text",
			Content:     description,
		},
		Start: graphDateTime{
			DateTime: start.Format("2006-01-02T15:04:05"),
			TimeZone: timezone,
		},
		End: graphDateTime{
			DateTime: end.Format("2006-01-02T15:04:05"),
			TimeZone: timezone,
		},
		ResponseRequested: false,
	}

	if location != "" {
		event.Location = &graphLocation{DisplayName: location}
	}

	body, err := json.Marshal(event)
	if err != nil {
		return "", fmt.Errorf("failed to marshal event: %w", err)
	}

	req, err := http.NewRequest("POST", "https://graph.microsoft.com/v1.0/me/events", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("graph API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("graph API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse event response: %w", err)
	}

	// Store the event ID on the guest list
	guestList.Set("ms_calendar_event_id", result.ID)
	if err := app.Save(guestList); err != nil {
		log.Printf("[Calendar] Created event %s but failed to save ID to guest list: %v", result.ID, err)
	}

	log.Printf("[Calendar] Created calendar event %s for guest list %s", result.ID, guestList.Id)
	return result.ID, nil
}

// addAttendeeToCalendarEvent adds a single attendee to an existing calendar event.
func addAttendeeToCalendarEvent(app *pocketbase.PocketBase, hostUserID, calendarEventID, email, name string) error {
	token, err := getValidMicrosoftToken(app, hostUserID)
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}

	// Get existing attendees
	existing, err := getEventAttendees(token, calendarEventID)
	if err != nil {
		return fmt.Errorf("failed to get existing attendees: %w", err)
	}

	// Check if already present
	emailLower := strings.ToLower(email)
	for _, a := range existing {
		if strings.ToLower(a.EmailAddress.Address) == emailLower {
			log.Printf("[Calendar] Attendee %s already on event %s, skipping", email, calendarEventID)
			return nil
		}
	}

	// Append new attendee
	existing = append(existing, graphAttendee{
		EmailAddress: graphEmail{Address: email, Name: name},
		Type:         "required",
	})

	return patchEventAttendees(token, calendarEventID, existing)
}

// addMultipleAttendeesToCalendarEvent bulk-adds attendees with a single PATCH.
func addMultipleAttendeesToCalendarEvent(app *pocketbase.PocketBase, hostUserID, calendarEventID string, attendees []attendeeInfo) (added, skipped int, err error) {
	token, err := getValidMicrosoftToken(app, hostUserID)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get token: %w", err)
	}

	existing, err := getEventAttendees(token, calendarEventID)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get existing attendees: %w", err)
	}

	// Build lookup of existing emails
	existingEmails := make(map[string]bool)
	for _, a := range existing {
		existingEmails[strings.ToLower(a.EmailAddress.Address)] = true
	}

	for _, att := range attendees {
		if existingEmails[strings.ToLower(att.Email)] {
			skipped++
			continue
		}
		existing = append(existing, graphAttendee{
			EmailAddress: graphEmail{Address: att.Email, Name: att.Name},
			Type:         "required",
		})
		added++
	}

	if added == 0 {
		return 0, skipped, nil
	}

	if err := patchEventAttendees(token, calendarEventID, existing); err != nil {
		return 0, 0, err
	}

	log.Printf("[Calendar] Added %d attendees to event %s (skipped %d duplicates)", added, calendarEventID, skipped)
	return added, skipped, nil
}

// getEventAttendees fetches the current attendee list for a calendar event.
func getEventAttendees(token, eventID string) ([]graphAttendee, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://graph.microsoft.com/v1.0/me/events/%s?$select=attendees", eventID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("graph API returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Attendees []graphAttendee `json:"attendees"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Attendees, nil
}

// patchEventAttendees updates the attendee list on a calendar event.
func patchEventAttendees(token, eventID string, attendees []graphAttendee) error {
	payload := map[string]any{
		"attendees": attendees,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("https://graph.microsoft.com/v1.0/me/events/%s", eventID), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("graph API PATCH returned %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// handleListAdminUsers returns a list of admin users (for event host selection).
func handleListAdminUsers(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	records, err := app.FindRecordsByFilter(
		"users",
		"role = 'admin'",
		"name",
		0, 0,
		nil,
	)
	if err != nil {
		return utils.InternalErrorResponse(re, "Failed to fetch users")
	}

	var users []map[string]any
	for _, r := range records {
		hasTokens := r.GetString("ms_access_token") != ""
		users = append(users, map[string]any{
			"id":         r.Id,
			"name":       r.GetString("name"),
			"email":      r.GetString("email"),
			"has_tokens": hasTokens,
		})
	}

	return re.JSON(http.StatusOK, map[string]any{
		"items": users,
	})
}

// handleCalendarCreate creates an Outlook calendar event for a guest list.
func handleCalendarCreate(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	id := re.Request.PathValue("id")
	guestList, err := app.FindRecordById(utils.CollectionGuestLists, id)
	if err != nil {
		return utils.NotFoundResponse(re, "Guest list not found")
	}

	hostUserID := guestList.GetString("event_host")
	if hostUserID == "" {
		return utils.BadRequestResponse(re, "No event host set — assign an event host first")
	}

	if guestList.GetString("ms_calendar_event_id") != "" {
		return utils.BadRequestResponse(re, "Calendar event already exists")
	}

	eventID, err := createCalendarEvent(app, hostUserID, guestList)
	if err != nil {
		log.Printf("[Calendar] Failed to create event for guest list %s: %v", id, err)
		// Check if token-related error
		if strings.Contains(err.Error(), "token") || strings.Contains(err.Error(), "log in") {
			return re.JSON(http.StatusUnauthorized, map[string]any{
				"error": err.Error(),
				"code":  "token_expired",
			})
		}
		return utils.InternalErrorResponse(re, "Failed to create calendar event")
	}

	utils.LogFromRequest(app, re, "calendar_event_created", utils.CollectionGuestLists, id, "success", map[string]any{
		"calendar_event_id": eventID,
		"host_user_id":      hostUserID,
	}, "")

	return re.JSON(http.StatusOK, map[string]any{
		"calendar_event_id": eventID,
	})
}

// handleCalendarSendAll adds all accepted guests to the calendar event.
func handleCalendarSendAll(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	id := re.Request.PathValue("id")
	guestList, err := app.FindRecordById(utils.CollectionGuestLists, id)
	if err != nil {
		return utils.NotFoundResponse(re, "Guest list not found")
	}

	hostUserID := guestList.GetString("event_host")
	calendarEventID := guestList.GetString("ms_calendar_event_id")

	if hostUserID == "" || calendarEventID == "" {
		return utils.BadRequestResponse(re, "Calendar event not configured — create a calendar event first")
	}

	// Fetch all accepted items
	items, err := app.FindRecordsByFilter(
		utils.CollectionGuestListItems,
		"guest_list = {:id} && rsvp_status = 'accepted'",
		"",
		0, 0,
		map[string]any{"id": id},
	)
	if err != nil {
		return utils.InternalErrorResponse(re, "Failed to fetch guest list items")
	}

	// Resolve attendee details
	var attendees []attendeeInfo
	for _, item := range items {
		contactID := item.GetString("contact")
		if contactID == "" {
			continue
		}
		contact, err := app.FindRecordById(utils.CollectionContacts, contactID)
		if err != nil {
			continue
		}
		email := utils.DecryptField(contact.GetString("email"))
		if email == "" || !strings.Contains(email, "@") {
			continue
		}
		attendees = append(attendees, attendeeInfo{
			Email: email,
			Name:  contact.GetString("name"),
		})
	}

	if len(attendees) == 0 {
		return re.JSON(http.StatusOK, map[string]any{
			"added":   0,
			"skipped": 0,
			"message": "No accepted guests with email addresses",
		})
	}

	added, skipped, err := addMultipleAttendeesToCalendarEvent(app, hostUserID, calendarEventID, attendees)
	if err != nil {
		log.Printf("[Calendar] Failed to send calendar invites for guest list %s: %v", id, err)
		if strings.Contains(err.Error(), "token") || strings.Contains(err.Error(), "log in") {
			return re.JSON(http.StatusUnauthorized, map[string]any{
				"error": err.Error(),
				"code":  "token_expired",
			})
		}
		return utils.InternalErrorResponse(re, "Failed to add attendees to calendar event")
	}

	utils.LogFromRequest(app, re, "calendar_send_all", utils.CollectionGuestLists, id, "success", map[string]any{
		"added":   added,
		"skipped": skipped,
	}, "")

	return re.JSON(http.StatusOK, map[string]any{
		"added":   added,
		"skipped": skipped,
	})
}

// handleCalendarStatus returns the calendar configuration status for a guest list.
func handleCalendarStatus(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	id := re.Request.PathValue("id")
	guestList, err := app.FindRecordById(utils.CollectionGuestLists, id)
	if err != nil {
		return utils.NotFoundResponse(re, "Guest list not found")
	}

	hostUserID := guestList.GetString("event_host")
	calendarEventID := guestList.GetString("ms_calendar_event_id")

	status := "not_configured"
	hostStatus := ""

	if hostUserID != "" {
		hostStatus = userCalendarStatus(app, hostUserID)
		if calendarEventID != "" {
			status = "event_created"
		} else {
			status = "host_set"
		}
	}

	return re.JSON(http.StatusOK, map[string]any{
		"status":            status,
		"calendar_event_id": calendarEventID,
		"host_user_id":      hostUserID,
		"host_status":       hostStatus,
	})
}
