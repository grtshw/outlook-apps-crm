package main

import (
	"fmt"
	"strings"
	"time"
)

// ICSEvent holds the data needed to generate an .ics calendar file.
type ICSEvent struct {
	UID         string
	Summary     string
	Description string
	Location    string
	Start       time.Time
	End         time.Time
	Timezone    string
}

// generateICS produces an RFC 5545 iCalendar file as bytes.
func generateICS(event ICSEvent) []byte {
	tz := event.Timezone
	if tz == "" {
		tz = "Australia/Sydney"
	}

	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
		tz = "UTC"
	}

	start := event.Start.In(loc)
	end := event.End.In(loc)

	// If end is the same as start, default to 2 hours
	if !end.After(start) {
		end = start.Add(2 * time.Hour)
	}

	dtStart := start.Format("20060102T150405")
	dtEnd := end.Format("20060102T150405")
	dtStamp := time.Now().UTC().Format("20060102T150405Z")

	uid := event.UID
	if uid == "" {
		uid = fmt.Sprintf("%d@crm.theoutlook.io", time.Now().UnixNano())
	}

	description := escapeICSText(event.Description)
	summary := escapeICSText(event.Summary)
	location := escapeICSText(event.Location)

	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//The Outlook//CRM//EN\r\n")
	b.WriteString("CALSCALE:GREGORIAN\r\n")
	b.WriteString("METHOD:PUBLISH\r\n")
	b.WriteString("BEGIN:VEVENT\r\n")
	fmt.Fprintf(&b, "UID:%s\r\n", uid)
	fmt.Fprintf(&b, "DTSTAMP:%s\r\n", dtStamp)
	fmt.Fprintf(&b, "DTSTART;TZID=%s:%s\r\n", tz, dtStart)
	fmt.Fprintf(&b, "DTEND;TZID=%s:%s\r\n", tz, dtEnd)
	fmt.Fprintf(&b, "SUMMARY:%s\r\n", summary)
	if description != "" {
		fmt.Fprintf(&b, "DESCRIPTION:%s\r\n", description)
	}
	if location != "" {
		fmt.Fprintf(&b, "LOCATION:%s\r\n", location)
	}
	b.WriteString("STATUS:CONFIRMED\r\n")
	b.WriteString("END:VEVENT\r\n")
	b.WriteString("END:VCALENDAR\r\n")

	return []byte(b.String())
}

// escapeICSText escapes special characters for iCalendar text values.
func escapeICSText(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, ";", "\\;")
	s = strings.ReplaceAll(s, ",", "\\,")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

// buildICSEventFromGuestList creates an ICSEvent from a guest list and its event projection.
// Returns nil if insufficient date information is available.
func buildICSEventFromGuestList(guestListID, eventName, eventDescription, startDate, endDate, startTime, endTime, timezone, eventLocation string) *ICSEvent {
	if startDate == "" {
		return nil
	}

	start, err := parseEventDateTime(startDate, startTime)
	if err != nil {
		return nil
	}

	end, err := parseEventDateTime(endDate, endTime)
	if err != nil {
		// Default: same day as start, 2 hours later
		end = start.Add(2 * time.Hour)
	}

	return &ICSEvent{
		UID:         fmt.Sprintf("%s@crm.theoutlook.io", guestListID),
		Summary:     eventName,
		Description: eventDescription,
		Location:    eventLocation,
		Start:       start,
		End:         end,
		Timezone:    timezone,
	}
}

// parseEventDateTime parses a date string and optional time string into a time.Time.
// Supports ISO 8601 date (2026-03-15) and time formats (14:00, 14:00:00).
func parseEventDateTime(dateStr, timeStr string) (time.Time, error) {
	if dateStr == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}

	// Try full ISO 8601 datetime first
	if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02T15:04:05", dateStr); err == nil {
		return t, nil
	}

	// Parse date-only + optional time
	if timeStr != "" {
		combined := dateStr + " " + timeStr
		for _, layout := range []string{
			"2006-01-02 15:04:05",
			"2006-01-02 15:04",
			"2006-01-02 3:04 PM",
			"2006-01-02 3:04PM",
		} {
			if t, err := time.Parse(layout, combined); err == nil {
				return t, nil
			}
		}
	}

	// Date only — assume 09:00
	if t, err := time.Parse("2006-01-02", dateStr); err == nil {
		return t.Add(9 * time.Hour), nil
	}

	return time.Time{}, fmt.Errorf("could not parse date: %s", dateStr)
}
