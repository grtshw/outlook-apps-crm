package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/grtshw/outlook-apps-crm/utils"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

const attendeeSessionTTL = 86400 // 24 hours

// handleAttendeeSendOTP sends an OTP code to the given email if a contact exists.
// Always returns 200 to prevent email enumeration.
func handleAttendeeSendOTP(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	var input struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(re.Request.Body).Decode(&input); err != nil {
		return utils.BadRequestResponse(re, "Invalid request body")
	}

	email := strings.ToLower(strings.TrimSpace(input.Email))
	if email == "" {
		return utils.BadRequestResponse(re, "Email required")
	}

	// Always return success to prevent email enumeration
	successResp := map[string]any{
		"message":    "If an account exists, a verification code has been sent",
		"expires_in": 600,
	}

	// Look up contact by email blind index
	blindIndex := utils.BlindIndex(email)
	contacts, _ := app.FindRecordsByFilter(
		utils.CollectionContacts,
		"email_index = {:idx}",
		"", 1, 0,
		map[string]any{"idx": blindIndex},
	)

	if len(contacts) == 0 {
		// No contact found — return success anyway to prevent enumeration
		return re.JSON(http.StatusOK, successResp)
	}

	contact := contacts[0]

	// Rate limit: max 3 OTP requests per contact per 10 minutes
	tenMinAgo := time.Now().UTC().Add(-10 * time.Minute).Format("2006-01-02 15:04:05.000Z")
	recentCodes, _ := app.FindRecordsByFilter(
		utils.CollectionAttendeeOTPCodes,
		"contact = {:cid} && created >= {:since}",
		"", 0, 0,
		map[string]any{"cid": contact.Id, "since": tenMinAgo},
	)
	if len(recentCodes) >= 3 {
		return re.JSON(http.StatusTooManyRequests, map[string]string{
			"error": "Too many verification requests. Please try again later.",
		})
	}

	// Generate OTP
	code, err := generateOTPCode()
	if err != nil {
		log.Printf("[Attendee] Failed to generate OTP: %v", err)
		return re.JSON(http.StatusOK, successResp)
	}

	// Store hashed OTP
	otpCollection, err := app.FindCollectionByNameOrId(utils.CollectionAttendeeOTPCodes)
	if err != nil {
		log.Printf("[Attendee] OTP collection not found: %v", err)
		return re.JSON(http.StatusOK, successResp)
	}

	otpRecord := core.NewRecord(otpCollection)
	otpRecord.Set("contact", contact.Id)
	otpRecord.Set("code_hash", hashOTPCode(code))
	otpRecord.Set("email", email)
	otpRecord.Set("expires_at", time.Now().UTC().Add(10*time.Minute).Format(time.RFC3339))
	otpRecord.Set("used", false)
	otpRecord.Set("attempts", 0)
	otpRecord.Set("ip_address", re.RealIP())

	if err := app.Save(otpRecord); err != nil {
		log.Printf("[Attendee] Failed to save OTP: %v", err)
		return re.JSON(http.StatusOK, successResp)
	}

	// Send OTP email
	contactName := contact.GetString("first_name")
	if contactName == "" {
		contactName = "there"
	}
	sendAttendeeOTPEmail(app, email, contactName, code)

	utils.LogFromRequest(app, re, "attendee_otp_sent", utils.CollectionContacts, contact.Id, "success", nil, "")

	return re.JSON(http.StatusOK, successResp)
}

// handleAttendeeVerifyOTP verifies the OTP code and returns a session token.
func handleAttendeeVerifyOTP(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	var input struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if err := json.NewDecoder(re.Request.Body).Decode(&input); err != nil {
		return utils.BadRequestResponse(re, "Invalid request body")
	}

	email := strings.ToLower(strings.TrimSpace(input.Email))
	code := strings.TrimSpace(input.Code)

	if email == "" || code == "" {
		return utils.BadRequestResponse(re, "Email and code required")
	}

	// Find contact
	blindIndex := utils.BlindIndex(email)
	contacts, _ := app.FindRecordsByFilter(
		utils.CollectionContacts,
		"email_index = {:idx}",
		"", 1, 0,
		map[string]any{"idx": blindIndex},
	)

	if len(contacts) == 0 {
		return re.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid code"})
	}

	contact := contacts[0]

	// Find the most recent unused OTP for this contact
	otpRecords, _ := app.FindRecordsByFilter(
		utils.CollectionAttendeeOTPCodes,
		"contact = {:cid} && used = false && expires_at > {:now}",
		"-created", 1, 0,
		map[string]any{
			"cid": contact.Id,
			"now": time.Now().UTC().Format("2006-01-02 15:04:05.000Z"),
		},
	)

	if len(otpRecords) == 0 {
		return re.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid or expired code"})
	}

	otpRecord := otpRecords[0]

	// Check attempt count
	attempts := otpRecord.GetInt("attempts")
	if attempts >= 5 {
		otpRecord.Set("used", true)
		app.Save(otpRecord)
		return re.JSON(http.StatusUnauthorized, map[string]string{"error": "Too many attempts. Please request a new code."})
	}

	// Verify code
	if !verifyOTPCode(code, otpRecord.GetString("code_hash")) {
		otpRecord.Set("attempts", attempts+1)
		app.Save(otpRecord)
		return re.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid code"})
	}

	// Mark OTP as used
	otpRecord.Set("used", true)
	app.Save(otpRecord)

	// Create session token
	sessionToken, err := utils.CreateAttendeeSession(contact.Id, email, attendeeSessionTTL)
	if err != nil {
		return utils.InternalErrorResponse(re, "Failed to create session")
	}

	utils.LogFromRequest(app, re, "attendee_login", utils.CollectionContacts, contact.Id, "success", nil, "")

	return re.JSON(http.StatusOK, map[string]any{
		"session_token": sessionToken,
		"expires_in":    attendeeSessionTTL,
		"contact": map[string]any{
			"id":         contact.Id,
			"first_name": contact.GetString("first_name"),
			"last_name":  contact.GetString("last_name"),
		},
	})
}

// handleAttendeeProfile returns the authenticated attendee's profile.
func handleAttendeeProfile(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	claims, err := extractAttendeeClaims(re)
	if err != nil {
		return re.JSON(http.StatusUnauthorized, map[string]string{"error": err.Error()})
	}

	contact, err := app.FindRecordById(utils.CollectionContacts, claims.ContactID)
	if err != nil {
		return utils.NotFoundResponse(re, "Contact not found")
	}

	baseURL := getBaseURL()

	return utils.DataResponse(re, map[string]any{
		"id":                            contact.Id,
		"first_name":                    contact.GetString("first_name"),
		"last_name":                     contact.GetString("last_name"),
		"email":                         utils.DecryptField(contact.GetString("email")),
		"phone":                         utils.DecryptField(contact.GetString("phone")),
		"pronouns":                      contact.GetString("pronouns"),
		"bio":                           utils.DecryptField(contact.GetString("bio")),
		"job_title":                     contact.GetString("job_title"),
		"linkedin":                      contact.GetString("linkedin"),
		"location":                      utils.DecryptField(contact.GetString("location")),
		"avatar_url":                    resolveAvatarURL(contact, baseURL, "avatar_url"),
		"avatar_thumb_url":              resolveAvatarURL(contact, baseURL, "avatar_thumb_url"),
		"dietary_requirements":          contact.Get("dietary_requirements"),
		"dietary_requirements_other":    contact.GetString("dietary_requirements_other"),
		"accessibility_requirements":       contact.Get("accessibility_requirements"),
		"accessibility_requirements_other": contact.GetString("accessibility_requirements_other"),
	})
}

// handleAttendeeProfileUpdate allows the attendee to update their own profile.
func handleAttendeeProfileUpdate(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	claims, err := extractAttendeeClaims(re)
	if err != nil {
		return re.JSON(http.StatusUnauthorized, map[string]string{"error": err.Error()})
	}

	contact, err := app.FindRecordById(utils.CollectionContacts, claims.ContactID)
	if err != nil {
		return utils.NotFoundResponse(re, "Contact not found")
	}

	var input map[string]any
	if err := json.NewDecoder(re.Request.Body).Decode(&input); err != nil {
		return utils.BadRequestResponse(re, "Invalid request body")
	}

	// Allowlisted fields that attendees can update
	allowedFields := map[string]bool{
		"first_name":                       true,
		"last_name":                        true,
		"phone":                            true,
		"pronouns":                         true,
		"bio":                              true,
		"job_title":                        true,
		"linkedin":                         true,
		"location":                         true,
		"dietary_requirements":             true,
		"dietary_requirements_other":       true,
		"accessibility_requirements":       true,
		"accessibility_requirements_other": true,
	}

	updatedFields := 0
	for field, value := range input {
		if !allowedFields[field] {
			continue
		}
		contact.Set(field, value)
		updatedFields++
	}

	if updatedFields == 0 {
		return utils.BadRequestResponse(re, "No valid fields to update")
	}

	// Decrypt email before save so encryption hooks can re-encrypt
	contact.Set("email", utils.DecryptField(contact.GetString("email")))

	if err := app.Save(contact); err != nil {
		return utils.InternalErrorResponse(re, "Failed to update profile")
	}

	utils.LogFromRequest(app, re, "attendee_profile_update", utils.CollectionContacts, contact.Id, "success", nil, "")

	return utils.SuccessResponse(re, "Profile updated")
}

// handleAttendeeActivities returns activities for the authenticated attendee.
func handleAttendeeActivities(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	claims, err := extractAttendeeClaims(re)
	if err != nil {
		return re.JSON(http.StatusUnauthorized, map[string]string{"error": err.Error()})
	}

	records, err := app.FindRecordsByFilter(
		utils.CollectionActivities,
		"contact = {:cid}",
		"-occurred_at", 50, 0,
		map[string]any{"cid": claims.ContactID},
	)
	if err != nil {
		return utils.DataResponse(re, []any{})
	}

	activities := make([]map[string]any, 0, len(records))
	for _, r := range records {
		activities = append(activities, map[string]any{
			"id":          r.Id,
			"type":        r.GetString("type"),
			"title":       r.GetString("title"),
			"source_app":  r.GetString("source_app"),
			"source_url":  r.GetString("source_url"),
			"occurred_at": r.GetString("occurred_at"),
			"created":     r.GetString("created"),
		})
	}

	return utils.DataResponse(re, activities)
}

// handleAttendeeEmailLink allows an attendee to link an additional email to their contact.
// This creates a contact link between the current contact and any contact matching the new email.
func handleAttendeeEmailLink(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	claims, err := extractAttendeeClaims(re)
	if err != nil {
		return re.JSON(http.StatusUnauthorized, map[string]string{"error": err.Error()})
	}

	var input struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(re.Request.Body).Decode(&input); err != nil {
		return utils.BadRequestResponse(re, "Invalid request body")
	}

	email := strings.ToLower(strings.TrimSpace(input.Email))
	if email == "" {
		return utils.BadRequestResponse(re, "Email required")
	}

	// Check the target email exists as a different contact
	blindIndex := utils.BlindIndex(email)
	targetContacts, _ := app.FindRecordsByFilter(
		utils.CollectionContacts,
		"email_index = {:idx}",
		"", 1, 0,
		map[string]any{"idx": blindIndex},
	)

	if len(targetContacts) == 0 {
		return utils.NotFoundResponse(re, "No contact found with that email")
	}

	targetContact := targetContacts[0]
	if targetContact.Id == claims.ContactID {
		return utils.BadRequestResponse(re, "That's already your email")
	}

	// Normalise pair order for dedup
	a, b := claims.ContactID, targetContact.Id
	if a > b {
		a, b = b, a
	}

	// Check for existing link
	existing, _ := app.FindRecordsByFilter(
		utils.CollectionContactLinks,
		"contact_a = {:a} && contact_b = {:b}",
		"", 1, 0,
		map[string]any{"a": a, "b": b},
	)
	if len(existing) > 0 {
		return re.JSON(http.StatusConflict, map[string]string{"error": "These contacts are already linked"})
	}

	// Create unverified link — admin can verify later
	collection, err := app.FindCollectionByNameOrId(utils.CollectionContactLinks)
	if err != nil {
		return utils.InternalErrorResponse(re, "Failed to find contact_links collection")
	}

	record := core.NewRecord(collection)
	record.Set("contact_a", a)
	record.Set("contact_b", b)
	record.Set("verified", false)
	record.Set("source", "attendee")
	record.Set("notes", fmt.Sprintf("Linked by attendee via email: %s", email))

	if err := app.Save(record); err != nil {
		return utils.InternalErrorResponse(re, "Failed to create link")
	}

	utils.LogFromRequest(app, re, "attendee_email_link", utils.CollectionContactLinks, record.Id, "success", nil, "")

	return re.JSON(http.StatusCreated, map[string]any{
		"id":      record.Id,
		"message": "Link created. An admin will verify it.",
	})
}

// extractAttendeeClaims extracts and validates the attendee session from the Authorization header.
func extractAttendeeClaims(re *core.RequestEvent) (*utils.AttendeeSessionClaims, error) {
	authHeader := re.Request.Header.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("authorization required")
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		return nil, fmt.Errorf("invalid authorization format")
	}

	claims, err := utils.ValidateAttendeeSession(token)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired session")
	}

	return claims, nil
}

// resolveAvatarURL returns the avatar URL from the contact record.
func resolveAvatarURL(contact *core.Record, baseURL, field string) string {
	url := contact.GetString(field)
	if url != "" {
		return url
	}
	return ""
}
