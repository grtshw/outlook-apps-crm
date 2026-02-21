package main

import (
	"encoding/json"
	"net/http"

	"github.com/grtshw/outlook-apps-crm/utils"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func handleThemesList(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	records, err := app.FindRecordsByFilter(utils.CollectionThemes, "", "sort_order", 0, 0, nil)
	if err != nil {
		return utils.InternalErrorResponse(re, "Failed to fetch themes")
	}

	items := make([]map[string]any, len(records))
	for i, r := range records {
		items[i] = buildThemeResponse(r)
	}

	return re.JSON(http.StatusOK, map[string]any{"items": items})
}

func handleThemeGet(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	id := re.Request.PathValue("id")
	record, err := app.FindRecordById(utils.CollectionThemes, id)
	if err != nil {
		return utils.NotFoundResponse(re, "Theme not found")
	}

	return re.JSON(http.StatusOK, buildThemeResponse(record))
}

func handleThemeCreate(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	var input map[string]any
	if err := json.NewDecoder(re.Request.Body).Decode(&input); err != nil {
		return utils.BadRequestResponse(re, "Invalid JSON")
	}

	collection, err := app.FindCollectionByNameOrId(utils.CollectionThemes)
	if err != nil {
		return utils.InternalErrorResponse(re, "Collection not found")
	}

	record := core.NewRecord(collection)
	applyThemeFields(record, input)

	if err := app.Save(record); err != nil {
		return utils.InternalErrorResponse(re, "Failed to create theme")
	}

	utils.LogFromRequest(app, re, "create", utils.CollectionThemes, record.Id, "success", nil, "")

	return re.JSON(http.StatusCreated, buildThemeResponse(record))
}

func handleThemeUpdate(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	id := re.Request.PathValue("id")
	record, err := app.FindRecordById(utils.CollectionThemes, id)
	if err != nil {
		return utils.NotFoundResponse(re, "Theme not found")
	}

	var input map[string]any
	if err := json.NewDecoder(re.Request.Body).Decode(&input); err != nil {
		return utils.BadRequestResponse(re, "Invalid JSON")
	}

	applyThemeFields(record, input)

	if err := app.Save(record); err != nil {
		return utils.InternalErrorResponse(re, "Failed to update theme")
	}

	utils.LogFromRequest(app, re, "update", utils.CollectionThemes, record.Id, "success", nil, "")

	return re.JSON(http.StatusOK, buildThemeResponse(record))
}

func handleThemeDelete(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	id := re.Request.PathValue("id")
	record, err := app.FindRecordById(utils.CollectionThemes, id)
	if err != nil {
		return utils.NotFoundResponse(re, "Theme not found")
	}

	// Prevent deletion if any guest lists use this theme
	count := countRecords(app, utils.CollectionGuestLists, "theme = {:id}", id)
	if count > 0 {
		return utils.BadRequestResponse(re, "Cannot delete theme: it is used by guest lists")
	}

	if err := app.Delete(record); err != nil {
		return utils.InternalErrorResponse(re, "Failed to delete theme")
	}

	utils.LogFromRequest(app, re, "delete", utils.CollectionThemes, id, "success", nil, "")

	return utils.SuccessResponse(re, "Theme deleted")
}

// buildThemeResponse builds the API response for a theme record.
func buildThemeResponse(r *core.Record) map[string]any {
	return map[string]any{
		"id":               r.Id,
		"name":             r.GetString("name"),
		"slug":             r.GetString("slug"),
		"color_primary":    r.GetString("color_primary"),
		"color_secondary":  r.GetString("color_secondary"),
		"color_background": r.GetString("color_background"),
		"color_surface":    r.GetString("color_surface"),
		"color_text":       r.GetString("color_text"),
		"color_text_muted": r.GetString("color_text_muted"),
		"color_border":     r.GetString("color_border"),
		"color_button":     r.GetString("color_button"),
		"logo_url":         r.GetString("logo_url"),
		"logo_light_url":   r.GetString("logo_light_url"),
		"hero_image_url":   r.GetString("hero_image_url"),
		"is_dark":          r.GetBool("is_dark"),
		"sort_order":       r.GetInt("sort_order"),
		"created":          r.GetString("created"),
		"updated":          r.GetString("updated"),
	}
}

// buildThemePublicResponse builds the public-facing theme object (no timestamps).
func buildThemePublicResponse(r *core.Record) map[string]any {
	return map[string]any{
		"name":             r.GetString("name"),
		"slug":             r.GetString("slug"),
		"color_primary":    r.GetString("color_primary"),
		"color_secondary":  r.GetString("color_secondary"),
		"color_background": r.GetString("color_background"),
		"color_surface":    r.GetString("color_surface"),
		"color_text":       r.GetString("color_text"),
		"color_text_muted": r.GetString("color_text_muted"),
		"color_border":     r.GetString("color_border"),
		"color_button":     r.GetString("color_button"),
		"logo_url":         r.GetString("logo_url"),
		"logo_light_url":   r.GetString("logo_light_url"),
		"hero_image_url":   r.GetString("hero_image_url"),
		"is_dark":          r.GetBool("is_dark"),
	}
}

// fetchThemeForGuestList fetches and returns the theme for a guest list, or nil if not set.
func fetchThemeForGuestList(app *pocketbase.PocketBase, guestList *core.Record) map[string]any {
	themeID := guestList.GetString("theme")
	if themeID == "" {
		return nil
	}
	theme, err := app.FindRecordById(utils.CollectionThemes, themeID)
	if err != nil {
		return nil
	}
	return buildThemePublicResponse(theme)
}

func applyThemeFields(record *core.Record, input map[string]any) {
	if v, ok := input["name"].(string); ok {
		record.Set("name", v)
	}
	if v, ok := input["slug"].(string); ok {
		record.Set("slug", v)
	}
	if v, ok := input["color_primary"].(string); ok {
		record.Set("color_primary", v)
	}
	if v, ok := input["color_secondary"].(string); ok {
		record.Set("color_secondary", v)
	}
	if v, ok := input["color_background"].(string); ok {
		record.Set("color_background", v)
	}
	if v, ok := input["color_surface"].(string); ok {
		record.Set("color_surface", v)
	}
	if v, ok := input["color_text"].(string); ok {
		record.Set("color_text", v)
	}
	if v, ok := input["color_text_muted"].(string); ok {
		record.Set("color_text_muted", v)
	}
	if v, ok := input["color_border"].(string); ok {
		record.Set("color_border", v)
	}
	if v, ok := input["color_button"].(string); ok {
		record.Set("color_button", v)
	}
	if v, ok := input["logo_url"].(string); ok {
		record.Set("logo_url", v)
	}
	if v, ok := input["logo_light_url"].(string); ok {
		record.Set("logo_light_url", v)
	}
	if v, ok := input["hero_image_url"].(string); ok {
		record.Set("hero_image_url", v)
	}
	if v, ok := input["is_dark"].(bool); ok {
		record.Set("is_dark", v)
	}
	if v, ok := input["sort_order"].(float64); ok {
		record.Set("sort_order", int(v))
	}
}
