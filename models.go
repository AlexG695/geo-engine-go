package geoengine

import (
	"errors"
	"fmt"
)

var (
	// ErrClientClosed is returned when an operation is attempted on a closed client.
	ErrClientClosed = errors.New("geoengine: client is closed")

	// ErrDeviceIDRequired is returned when a device ID is missing.
	ErrDeviceIDRequired = errors.New("geoengine: device_id is required")

	// ErrInvalidCoordinates is returned when coordinate polygon validation fails.
	ErrInvalidCoordinates = errors.New("geoengine: at least 3 coordinate pairs are required for a polygon")
)

// APIError represents an error returned by the GeoEngine REST or Management API.
type APIError struct {
	StatusCode int
	Message    string
}

// Error formats the APIError into a human-readable string.
func (e *APIError) Error() string {
	return fmt.Sprintf("geoengine: API error (status %d): %s", e.StatusCode, e.Message)
}

// LocationPing represents a single telemetry payload matching PostGIS location_history.
type LocationPing struct {
	DeviceID  string  `json:"device_id"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Accuracy  float64 `json:"accuracy,omitempty"`
	Speed     float64 `json:"speed,omitempty"`
	Heading   float64 `json:"heading,omitempty"`
	Timestamp int64   `json:"timestamp"`
	IsMocked  bool    `json:"is_mocked,omitempty"`
}

// IngestResponse represents the status returned by the ingestion service.
type IngestResponse struct {
	Success        bool   `json:"success"`
	ProcessedCount int64  `json:"processed_count"`
	ErrorMessage   string `json:"error_message,omitempty"`
}

// GeofenceRequest represents a spatial polygon payload targeting PostGIS geofences.
type GeofenceRequest struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Color       string         `json:"color,omitempty"`
	WebhookURL  string         `json:"webhook_url,omitempty"`
	GeoJSON     map[string]any `json:"geojson"`
}
