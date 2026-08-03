package geoengine

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
