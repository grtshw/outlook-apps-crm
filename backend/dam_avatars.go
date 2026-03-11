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

// damLogoCache stores org_id → logo URLs fetched from DAM
var damLogoCache struct {
	sync.RWMutex
	data map[string][]map[string]any
}

// GetDAMAvatarURLs returns cached DAM avatar URLs for a CRM contact ID
func GetDAMAvatarURLs(crmID string) (DAMAvatarURLs, bool) {
	damAvatarCache.RLock()
	defer damAvatarCache.RUnlock()
	urls, ok := damAvatarCache.data[crmID]
	return urls, ok
}

// GetDAMLogoURLs returns cached DAM logo URLs for a CRM organisation ID
func GetDAMLogoURLs(orgID string) ([]map[string]any, bool) {
	damLogoCache.RLock()
	defer damLogoCache.RUnlock()
	urls, ok := damLogoCache.data[orgID]
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
			ID          string `json:"id"`
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
		if item.ID == "" || (item.SmallURL == "" && item.ThumbURL == "") {
			continue
		}
		// Use DAM proxy URLs (Tigris URLs 403 — must go through DAM's proxy handler)
		urls := DAMAvatarURLs{
			ThumbURL:    damURL + "/api/people/" + item.ID + "/avatar/thumb",
			SmallURL:    damURL + "/api/people/" + item.ID + "/avatar/small",
			OriginalURL: damURL + "/api/people/" + item.ID + "/avatar/original",
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

// RefreshDAMLogoCache fetches all organisations from DAM and updates the logo cache.
func RefreshDAMLogoCache() {
	damURL := os.Getenv("DAM_PUBLIC_URL")
	if damURL == "" {
		damURL = "https://outlook-apps-dam.fly.dev"
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(damURL + "/api/public/organisations")
	if err != nil {
		log.Printf("[DAMLogos] Failed to fetch from DAM: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[DAMLogos] DAM returned status %d", resp.StatusCode)
		return
	}

	var damResp struct {
		Items []struct {
			ID       string           `json:"id"`
			OrgID    string           `json:"org_id"`
			LogoURLs []map[string]any `json:"logo_urls"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&damResp); err != nil {
		log.Printf("[DAMLogos] Failed to parse DAM response: %v", err)
		return
	}

	newCache := make(map[string][]map[string]any, len(damResp.Items))
	populated := 0
	for _, item := range damResp.Items {
		if item.OrgID == "" || len(item.LogoURLs) == 0 {
			continue
		}
		newCache[item.OrgID] = item.LogoURLs
		populated++
	}

	damLogoCache.Lock()
	damLogoCache.data = newCache
	damLogoCache.Unlock()

	log.Printf("[DAMLogos] Cache refreshed: %d orgs with logos (from %d total)", populated, len(damResp.Items))
}
