package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// ProjectionCallbackPayload represents the callback from consumers.
type ProjectionCallbackPayload struct {
	ProjectionID     string `json:"projection_id"`
	Consumer         string `json:"consumer"`
	Status           string `json:"status"`
	Message          string `json:"message"`
	RecordsProcessed int    `json:"records_processed"`
}

// ProjectionConsumerStatus represents a consumer's status for a projection.
type ProjectionConsumerStatus struct {
	Name             string `json:"name"`
	AppId            string `json:"appId,omitempty"`
	Status           string `json:"status"`
	Message          string `json:"message,omitempty"`
	RecordsProcessed int    `json:"records_processed,omitempty"`
	ReceivedAt       string `json:"received_at,omitempty"`
}

// ProjectionLogEntry represents a single projection log.
type ProjectionLogEntry struct {
	ID          string                     `json:"id"`
	CreatedAt   string                     `json:"created_at"`
	RecordCount int                        `json:"record_count"`
	Consumers   []ProjectionConsumerStatus `json:"consumers"`
}

// handleProjectionCallback handles callback from consumers reporting projection status.
func handleProjectionCallback(app core.App, re *core.RequestEvent) error {
	var payload ProjectionCallbackPayload
	if err := re.BindBody(&payload); err != nil {
		return re.JSON(400, map[string]string{"error": "invalid payload"})
	}

	if payload.ProjectionID == "" || payload.Consumer == "" {
		return re.JSON(400, map[string]string{"error": "projection_id and consumer are required"})
	}

	callbacksCollection, err := app.FindCollectionByNameOrId("projection_callbacks")
	if err != nil {
		log.Printf("[ProjectionCallback] Collection not found: %v", err)
		return re.JSON(500, map[string]string{"error": "callbacks collection not found"})
	}

	record := core.NewRecord(callbacksCollection)
	record.Set("projection_id", payload.ProjectionID)
	record.Set("consumer", payload.Consumer)
	record.Set("status", payload.Status)
	record.Set("message", payload.Message)
	record.Set("records_processed", payload.RecordsProcessed)

	if err := app.Save(record); err != nil {
		log.Printf("[ProjectionCallback] Failed to save callback: %v", err)
		return re.JSON(500, map[string]string{"error": "failed to save callback"})
	}

	// Update the consumer's last_* fields
	consumer, err := app.FindFirstRecordByData("projection_consumers", "app_id", payload.Consumer)
	if err == nil && consumer != nil {
		consumer.Set("last_consumption", time.Now().UTC())
		consumer.Set("last_status", payload.Status)
		consumer.Set("last_message", payload.Message)
		if err := app.Save(consumer); err != nil {
			log.Printf("[ProjectionCallback] Warning: failed to update consumer: %v", err)
		}
	}

	log.Printf("[ProjectionCallback] Received from %s: %s (projection: %s)", payload.Consumer, payload.Status, payload.ProjectionID)
	return re.JSON(200, map[string]string{"status": "ok"})
}

// handleProjectionLogs returns all projection logs with their callback statuses.
func handleProjectionLogs(app core.App, re *core.RequestEvent) error {
	logs, err := app.FindRecordsByFilter(
		"projection_logs",
		"",
		"-created",
		50, 0,
	)
	if err != nil {
		return re.JSON(500, map[string]string{"error": "failed to fetch logs"})
	}

	callbacks, _ := app.FindRecordsByFilter(
		"projection_callbacks",
		"",
		"-received_at",
		500, 0,
	)

	callbacksByProjection := make(map[string][]*core.Record)
	for _, cb := range callbacks {
		projID := cb.GetString("projection_id")
		callbacksByProjection[projID] = append(callbacksByProjection[projID], cb)
	}

	entries := make([]ProjectionLogEntry, len(logs))
	for i, logRecord := range logs {
		projectionID := logRecord.Id

		var consumerNames []string
		if raw := logRecord.GetString("consumers"); raw != "" {
			json.Unmarshal([]byte(raw), &consumerNames)
		}

		callbackMap := make(map[string]*core.Record)
		for _, cb := range callbacksByProjection[projectionID] {
			consumer := cb.GetString("consumer")
			callbackMap[consumer] = cb
		}

		consumerStatuses := make([]ProjectionConsumerStatus, len(consumerNames))
		for j, name := range consumerNames {
			cs := ProjectionConsumerStatus{
				Name:   name,
				Status: "pending",
			}
			if cb, ok := callbackMap[name]; ok {
				cs.Status = cb.GetString("status")
				cs.Message = cb.GetString("message")
				cs.RecordsProcessed = int(cb.GetFloat("records_processed"))
				cs.ReceivedAt = cb.GetDateTime("received_at").String()
			}
			consumerStatuses[j] = cs
		}

		entries[i] = ProjectionLogEntry{
			ID:          projectionID,
			CreatedAt:   logRecord.GetDateTime("created").String(),
			RecordCount: int(logRecord.GetFloat("record_count")),
			Consumers:   consumerStatuses,
		}
	}

	return re.JSON(200, map[string]any{
		"logs": entries,
	})
}

// handleProjectionProgress returns the progress of a specific projection.
func handleProjectionProgress(app core.App, re *core.RequestEvent) error {
	id := re.Request.PathValue("id")
	if id == "" {
		return re.JSON(http.StatusBadRequest, map[string]string{"error": "id is required"})
	}

	logRecord, err := app.FindRecordById("projection_logs", id)
	if err != nil {
		return re.JSON(http.StatusNotFound, map[string]string{"error": "projection not found"})
	}

	expectedConsumers := logRecord.Get("consumers")
	var consumerNames []string
	if consumers, ok := expectedConsumers.([]any); ok {
		for _, c := range consumers {
			if name, ok := c.(string); ok {
				consumerNames = append(consumerNames, name)
			}
		}
	}

	callbacks, _ := app.FindRecordsByFilter(
		"projection_callbacks",
		"projection_id = {:projectionId}",
		"-received_at",
		100, 0,
		map[string]any{"projectionId": id},
	)

	callbackMap := make(map[string]*core.Record)
	for _, cb := range callbacks {
		consumer := cb.GetString("consumer")
		callbackMap[consumer] = cb
	}

	consumerStatuses := make([]ProjectionConsumerStatus, len(consumerNames))
	completedCount := 0
	for i, name := range consumerNames {
		cs := ProjectionConsumerStatus{
			Name:   name,
			AppId:  name,
			Status: "pending",
		}
		if cb, ok := callbackMap[name]; ok {
			cs.Status = cb.GetString("status")
			cs.Message = cb.GetString("message")
			cs.RecordsProcessed = int(cb.GetFloat("records_processed"))
			cs.ReceivedAt = cb.GetDateTime("received_at").String()
			completedCount++
		}
		consumerStatuses[i] = cs
	}

	return re.JSON(http.StatusOK, map[string]any{
		"projection_id": id,
		"total":         len(consumerNames),
		"completed":     completedCount,
		"consumers":     consumerStatuses,
	})
}

// handleListProjectionConsumers returns all consumers.
func handleListProjectionConsumers(app core.App, re *core.RequestEvent) error {
	consumers, err := app.FindRecordsByFilter(
		"projection_consumers",
		"",
		"name",
		0, 0,
	)
	if err != nil {
		return re.JSON(500, map[string]string{"error": "failed to fetch consumers"})
	}

	type ConsumerResponse struct {
		ID              string `json:"id"`
		Name            string `json:"name"`
		AppID           string `json:"app_id"`
		EndpointURL     string `json:"endpoint_url"`
		Enabled         bool   `json:"enabled"`
		LastConsumption string `json:"last_consumption,omitempty"`
		LastStatus      string `json:"last_status,omitempty"`
		LastMessage     string `json:"last_message,omitempty"`
	}

	result := make([]ConsumerResponse, len(consumers))
	for i, c := range consumers {
		lastConsumption := ""
		if lc := c.GetDateTime("last_consumption"); !lc.IsZero() {
			lastConsumption = lc.String()
		}

		result[i] = ConsumerResponse{
			ID:              c.Id,
			Name:            c.GetString("name"),
			AppID:           c.GetString("app_id"),
			EndpointURL:     c.GetString("endpoint_url"),
			Enabled:         c.GetBool("enabled"),
			LastConsumption: lastConsumption,
			LastStatus:      c.GetString("last_status"),
			LastMessage:     c.GetString("last_message"),
		}
	}

	return re.JSON(200, map[string]any{"consumers": result})
}

// handleToggleProjectionConsumer toggles the enabled status of a consumer.
func handleToggleProjectionConsumer(app core.App, re *core.RequestEvent) error {
	id := re.Request.PathValue("id")
	if id == "" {
		return re.JSON(400, map[string]string{"error": "id is required"})
	}

	record, err := app.FindRecordById("projection_consumers", id)
	if err != nil {
		return re.JSON(404, map[string]string{"error": "consumer not found"})
	}

	currentEnabled := record.GetBool("enabled")
	record.Set("enabled", !currentEnabled)

	if err := app.Save(record); err != nil {
		log.Printf("[ProjectionConsumer] Failed to toggle: %v", err)
		return re.JSON(500, map[string]string{"error": "failed to toggle consumer"})
	}

	return re.JSON(200, map[string]any{
		"message": "Consumer toggled",
		"enabled": !currentEnabled,
	})
}
