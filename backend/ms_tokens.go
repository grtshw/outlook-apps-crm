package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/grtshw/outlook-apps-crm/utils"
	"github.com/pocketbase/pocketbase"
)

// persistMicrosoftTokens encrypts and stores OAuth tokens on the user record.
func persistMicrosoftTokens(app *pocketbase.PocketBase, userID, accessToken, refreshToken string, expiry time.Time) {
	record, err := app.FindRecordById("users", userID)
	if err != nil {
		log.Printf("[MSTokens] Failed to find user %s: %v", userID, err)
		return
	}

	encAccess, err := utils.Encrypt(accessToken)
	if err != nil {
		log.Printf("[MSTokens] Failed to encrypt access token: %v", err)
		return
	}

	encRefresh, err := utils.Encrypt(refreshToken)
	if err != nil {
		log.Printf("[MSTokens] Failed to encrypt refresh token: %v", err)
		return
	}

	record.Set("ms_access_token", encAccess)
	record.Set("ms_refresh_token", encRefresh)
	record.Set("ms_token_expires_at", expiry.UTC().Format(time.RFC3339))

	if err := app.Save(record); err != nil {
		log.Printf("[MSTokens] Failed to save tokens for user %s: %v", userID, err)
		return
	}

	log.Printf("[MSTokens] Persisted tokens for user %s", userID)
}

// getValidMicrosoftToken returns a valid access token for the given user,
// refreshing it if expired. Returns an error if no token is available or refresh fails.
func getValidMicrosoftToken(app *pocketbase.PocketBase, userID string) (string, error) {
	record, err := app.FindRecordById("users", userID)
	if err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}

	encAccess := record.GetString("ms_access_token")
	encRefresh := record.GetString("ms_refresh_token")
	expiresAtStr := record.GetString("ms_token_expires_at")

	if encAccess == "" || encRefresh == "" {
		return "", fmt.Errorf("no Microsoft tokens stored for user %s — they need to log in again", userID)
	}

	accessToken := utils.DecryptField(encAccess)
	if accessToken == "" {
		return "", fmt.Errorf("failed to decrypt access token for user %s", userID)
	}

	// Check if token is still valid (with 5-minute buffer)
	if expiresAtStr != "" {
		expiresAt, err := time.Parse(time.RFC3339, expiresAtStr)
		if err == nil && time.Now().UTC().Add(5*time.Minute).Before(expiresAt) {
			return accessToken, nil
		}
	}

	// Token expired — refresh it
	refreshToken := utils.DecryptField(encRefresh)
	if refreshToken == "" {
		return "", fmt.Errorf("failed to decrypt refresh token for user %s", userID)
	}

	newAccess, newRefresh, newExpiry, err := refreshMicrosoftToken(refreshToken)
	if err != nil {
		return "", fmt.Errorf("token refresh failed for user %s: %w", userID, err)
	}

	// Persist the new tokens
	persistMicrosoftTokens(app, userID, newAccess, newRefresh, newExpiry)

	return newAccess, nil
}

// refreshMicrosoftToken exchanges a refresh token for new access and refresh tokens.
func refreshMicrosoftToken(refreshToken string) (accessToken, newRefreshToken string, expiry time.Time, err error) {
	clientID := os.Getenv("MS_OAUTH_CLIENT_ID")
	clientSecret := os.Getenv("MS_OAUTH_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		return "", "", time.Time{}, fmt.Errorf("MS_OAUTH_CLIENT_ID and MS_OAUTH_CLIENT_SECRET must be set")
	}

	data := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"scope":         {"openid profile email offline_access Calendars.ReadWrite"},
	}

	resp, err := http.PostForm("https://login.microsoftonline.com/common/oauth2/v2.0/token", data)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Error        string `json:"error"`
		ErrorDesc    string `json:"error_description"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", time.Time{}, fmt.Errorf("failed to decode token response: %w", err)
	}

	if result.Error != "" {
		return "", "", time.Time{}, fmt.Errorf("microsoft token error: %s — %s", result.Error, result.ErrorDesc)
	}

	if result.AccessToken == "" {
		return "", "", time.Time{}, fmt.Errorf("empty access token in response")
	}

	exp := time.Now().UTC().Add(time.Duration(result.ExpiresIn) * time.Second)

	// Microsoft may or may not return a new refresh token
	rt := result.RefreshToken
	if rt == "" {
		rt = refreshToken
	}

	return result.AccessToken, rt, exp, nil
}

// hasMicrosoftTokens checks if a user has stored Microsoft tokens.
func hasMicrosoftTokens(app *pocketbase.PocketBase, userID string) bool {
	record, err := app.FindRecordById("users", userID)
	if err != nil {
		return false
	}
	return record.GetString("ms_access_token") != "" && record.GetString("ms_refresh_token") != ""
}

// userCalendarStatus returns the calendar readiness status for a user.
func userCalendarStatus(app *pocketbase.PocketBase, userID string) string {
	record, err := app.FindRecordById("users", userID)
	if err != nil {
		return "not_found"
	}

	if record.GetString("ms_access_token") == "" {
		return "no_tokens"
	}

	expiresAtStr := record.GetString("ms_token_expires_at")
	if expiresAtStr == "" {
		return "ready"
	}

	expiresAt, err := time.Parse(time.RFC3339, expiresAtStr)
	if err != nil {
		return "ready"
	}

	// If token expired but we have refresh token, still "ready" (will auto-refresh)
	if time.Now().UTC().After(expiresAt) && record.GetString("ms_refresh_token") != "" {
		return "ready"
	}

	if strings.TrimSpace(record.GetString("ms_refresh_token")) == "" {
		return "expired"
	}

	return "ready"
}
