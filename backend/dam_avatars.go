package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// DAMAvatarURLs holds the avatar variant URLs for a person from DAM
type DAMAvatarURLs struct {
	ThumbURL    string
	SmallURL    string
	OriginalURL string
}

// damAvatarCache stores crm_id → avatar URLs fetched from DAM
var damAvatarCache struct {
	sync.RWMutex
	data map[string]DAMAvatarURLs
}

// GetDAMAvatarURLs returns cached DAM avatar URLs for a CRM contact ID
func GetDAMAvatarURLs(crmID string) (DAMAvatarURLs, bool) {
	damAvatarCache.RLock()
	defer damAvatarCache.RUnlock()
	urls, ok := damAvatarCache.data[crmID]
	return urls, ok
}

// RefreshDAMAvatarCache fetches all people from DAM and updates the cache.
// Called on startup, after project-all, and after receiving avatar URL webhooks.
func RefreshDAMAvatarCache() {
	damURL := os.Getenv("DAM_PUBLIC_URL")
	if damURL == "" {
		damURL = "https://outlook-apps-dam.fly.dev"
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(damURL + "/api/public/people")
	if err != nil {
		log.Printf("[DAMAvatars] Failed to fetch from DAM: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[DAMAvatars] DAM returned status %d", resp.StatusCode)
		return
	}

	var damResp struct {
		Items []struct {
			CrmID       string `json:"crm_id"`
			PresenterID string `json:"presenter_id"`
			ThumbURL    string `json:"avatar_thumb_url"`
			SmallURL    string `json:"avatar_small_url"`
			OriginalURL string `json:"avatar_original_url"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&damResp); err != nil {
		log.Printf("[DAMAvatars] Failed to parse DAM response: %v", err)
		return
	}

	newCache := make(map[string]DAMAvatarURLs, len(damResp.Items))
	populated := 0
	for _, item := range damResp.Items {
		if item.SmallURL == "" && item.ThumbURL == "" {
			continue
		}
		urls := DAMAvatarURLs{
			ThumbURL:    item.ThumbURL,
			SmallURL:    item.SmallURL,
			OriginalURL: item.OriginalURL,
		}
		if item.CrmID != "" {
			newCache[item.CrmID] = urls
			populated++
		} else if item.PresenterID != "" {
			newCache[item.PresenterID] = urls
			populated++
		}
	}

	damAvatarCache.Lock()
	damAvatarCache.data = newCache
	damAvatarCache.Unlock()

	log.Printf("[DAMAvatars] Cache refreshed: %d people with avatars (from %d total)", populated, len(damResp.Items))
}
