package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

// syncMicrosoftProfilePhoto fetches the user's profile photo from Microsoft Graph API
// and saves it as their avatar in PocketBase
func syncMicrosoftProfilePhoto(app *pocketbase.PocketBase, record *core.Record, accessToken string) {
	if record == nil {
		return
	}

	// Fetch photo from Microsoft Graph API
	req, err := http.NewRequest("GET", "https://graph.microsoft.com/v1.0/me/photo/$value", nil)
	if err != nil {
		log.Printf("[OAuth] Failed to create photo request: %v", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[OAuth] Failed to fetch Microsoft photo: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// 404 means no photo set - not an error
		if resp.StatusCode != http.StatusNotFound {
			log.Printf("[OAuth] Microsoft photo request returned status %d", resp.StatusCode)
		}
		return
	}

	// Read the image data
	photoData, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[OAuth] Failed to read photo data: %v", err)
		return
	}

	// Determine content type from response header
	contentType := resp.Header.Get("Content-Type")
	ext := ".jpg" // default
	if strings.Contains(contentType, "png") {
		ext = ".png"
	}

	// Create a file from the photo data
	fileName := fmt.Sprintf("avatar_%s%s", record.Id, ext)
	file, err := filesystem.NewFileFromBytes(photoData, fileName)
	if err != nil {
		log.Printf("[OAuth] Failed to create file from photo: %v", err)
		return
	}

	// Re-fetch the record to avoid stale data
	freshRecord, err := app.FindRecordById("users", record.Id)
	if err != nil {
		log.Printf("[OAuth] Failed to find user for avatar update: %v", err)
		return
	}

	// Set the avatar file
	freshRecord.Set("avatar", file)

	// Save the record
	if err := app.Save(freshRecord); err != nil {
		log.Printf("[OAuth] Failed to save user avatar: %v", err)
		return
	}

	log.Printf("[OAuth] Successfully synced Microsoft profile photo for user %s", record.Id)
}
