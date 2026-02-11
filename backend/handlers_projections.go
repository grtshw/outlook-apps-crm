package main

import (
	"net/http"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// handleListEventProjections returns event projections for admin UI (searchable dropdown).
func handleListEventProjections(re *core.RequestEvent, app *pocketbase.PocketBase) error {
	search := re.Request.URL.Query().Get("search")

	filter := "orphaned_at = '' || orphaned_at = null"
	params := map[string]any{}

	if search != "" {
		filter += " && name ~ {:search}"
		params["search"] = search
	}

	records, err := app.FindRecordsByFilter(
		"event_projections",
		filter,
		"-date",
		100, 0,
		params,
	)
	if err != nil {
		return re.JSON(http.StatusOK, map[string]any{"items": []any{}})
	}

	items := make([]map[string]any, len(records))
	for i, r := range records {
		items[i] = map[string]any{
			"id":           r.Id,
			"event_id":     r.GetString("event_id"),
			"name":         r.GetString("name"),
			"slug":         r.GetString("slug"),
			"date":         r.GetString("date"),
			"edition_year": r.GetInt("edition_year"),
			"venue":        r.GetString("venue"),
			"venue_city":   r.GetString("venue_city"),
			"event_type":   r.GetString("event_type"),
			"status":       r.GetString("status"),
			"format":       r.GetString("format"),
		}
	}

	return re.JSON(http.StatusOK, map[string]any{"items": items})
}
