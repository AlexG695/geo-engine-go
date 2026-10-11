package geoengine

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

var (
	// ErrClientClosed is returned when an operation is attempted on a closed client.
	ErrClientClosed = errors.New("geoengine: client is closed")

	// ErrDeviceIDRequired is returned when a device ID is missing.
	ErrDeviceIDRequired = errors.New("geoengine: device_id is required")

	// ErrInvalidCoordinates is returned when coordinate polygon validation fails.
	ErrInvalidCoordinates = errors.New("geoengine: at least 3 coordinate pairs are required for a polygon")

	// ErrGeofenceNotFound is returned when the specified geofence is not found.
	ErrGeofenceNotFound = errors.New("geoengine: geofence not found")

	// ErrGeofenceIDRequired is returned when geofence ID is missing or empty.
	ErrGeofenceIDRequired = errors.New("geoengine: geofence_id is required")

	// ErrInvalidLatitude is returned when latitude is outside the valid range [-90, 90].
	ErrInvalidLatitude = errors.New("geoengine: latitude must be between -90 and 90 degrees")

	// ErrInvalidLongitude is returned when longitude is outside the valid range [-180, 180].
	ErrInvalidLongitude = errors.New("geoengine: longitude must be between -180 and 180 degrees")

	// ErrQuotaExceeded is returned when the account request quota is exceeded.
	ErrQuotaExceeded = errors.New("geoengine: request quota exceeded")

	// ErrUnauthorized is returned when API authentication fails.
	ErrUnauthorized = errors.New("geoengine: unauthorized API request")

	// ErrForbidden is returned when access to the requested resource is forbidden.
	ErrForbidden = errors.New("geoengine: access forbidden")
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

// ValidateCoordinates verifies that latitude is between -90 and 90 and longitude is between -180 and 180 degrees.
func ValidateCoordinates(lat, lng float64) error {
	if lat < -90 || lat > 90 {
		return ErrInvalidLatitude
	}
	if lng < -180 || lng > 180 {
		return ErrInvalidLongitude
	}
	return nil
}

// SpatialValidationRequest defines parameters for validating a point against a specific geofence.
type SpatialValidationRequest struct {
	GeofenceID string  `json:"geofence_id"`
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
}

// Validate verifies request parameters before network transmission.
func (r SpatialValidationRequest) Validate() error {
	if strings.TrimSpace(r.GeofenceID) == "" {
		return ErrGeofenceIDRequired
	}
	return ValidateCoordinates(r.Latitude, r.Longitude)
}

// SpatialGeneralRequest defines parameters for validating a point against all organization geofences.
type SpatialGeneralRequest struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// Validate verifies coordinate parameters before network transmission.
func (r SpatialGeneralRequest) Validate() error {
	return ValidateCoordinates(r.Latitude, r.Longitude)
}

// SpatialValidationResult represents the validation result for a single geofence.
type SpatialValidationResult struct {
	Inside       bool    `json:"inside"`
	GeofenceID   string  `json:"geofence_id"`
	GeofenceName string  `json:"geofence_name,omitempty"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
}

// IsInside returns true if the coordinate is inside the geofence.
func (r *SpatialValidationResult) IsInside() bool {
	return r != nil && r.Inside
}

// IsOutside returns true if the coordinate is outside the geofence.
func (r *SpatialValidationResult) IsOutside() bool {
	return r == nil || !r.Inside
}

// Coordinates returns latitude and longitude as a tuple.
func (r *SpatialValidationResult) Coordinates() (float64, float64) {
	if r == nil {
		return 0, 0
	}
	return r.Latitude, r.Longitude
}

// String implements fmt.Stringer for clear and clean logging.
func (r *SpatialValidationResult) String() string {
	if r == nil {
		return "<nil SpatialValidationResult>"
	}
	name := r.GeofenceName
	if name == "" {
		name = "unnamed"
	}
	status := "OUTSIDE"
	if r.Inside {
		status = "INSIDE"
	}
	return fmt.Sprintf("[%s] geofence=%s (%s) at (%.6f, %.6f)", status, r.GeofenceID, name, r.Latitude, r.Longitude)
}

// GeofenceMatch represents a matched geofence in general validation.
type GeofenceMatch struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// SpatialGeneralResult represents the result of validating a point against all active geofences.
type SpatialGeneralResult struct {
	Inside           bool            `json:"inside"`
	TotalMatches     int             `json:"total_matches"`
	MatchedGeofences []GeofenceMatch `json:"matched_geofences"`
	Latitude         float64         `json:"latitude"`
	Longitude        float64         `json:"longitude"`
}

// IsInside returns true if the coordinate falls inside at least one geofence.
func (r *SpatialGeneralResult) IsInside() bool {
	return r != nil && r.Inside
}

// HasMatches returns true if one or more geofences matched the point.
func (r *SpatialGeneralResult) HasMatches() bool {
	return r != nil && r.TotalMatches > 0
}

// FirstMatch returns the first matched geofence, or nil if no matches found.
func (r *SpatialGeneralResult) FirstMatch() *GeofenceMatch {
	if r == nil || len(r.MatchedGeofences) == 0 {
		return nil
	}
	return &r.MatchedGeofences[0]
}

// GeofenceIDs returns all matched geofence IDs.
func (r *SpatialGeneralResult) GeofenceIDs() []string {
	if r == nil || len(r.MatchedGeofences) == 0 {
		return []string{}
	}
	ids := make([]string, len(r.MatchedGeofences))
	for i, m := range r.MatchedGeofences {
		ids[i] = m.ID
	}
	return ids
}

// GeofenceNames returns all matched geofence names.
func (r *SpatialGeneralResult) GeofenceNames() []string {
	if r == nil || len(r.MatchedGeofences) == 0 {
		return []string{}
	}
	names := make([]string, len(r.MatchedGeofences))
	for i, m := range r.MatchedGeofences {
		names[i] = m.Name
	}
	return names
}

// ContainsGeofence checks whether a specific geofence ID or name is among the matches.
func (r *SpatialGeneralResult) ContainsGeofence(idOrName string) bool {
	_, found := r.FindMatch(idOrName)
	return found
}

// FindMatch searches for a matched geofence by ID or name and returns it if found.
func (r *SpatialGeneralResult) FindMatch(idOrName string) (*GeofenceMatch, bool) {
	if r == nil {
		return nil, false
	}
	for i := range r.MatchedGeofences {
		if r.MatchedGeofences[i].ID == idOrName || r.MatchedGeofences[i].Name == idOrName {
			return &r.MatchedGeofences[i], true
		}
	}
	return nil, false
}

// Coordinates returns latitude and longitude as a tuple.
func (r *SpatialGeneralResult) Coordinates() (float64, float64) {
	if r == nil {
		return 0, 0
	}
	return r.Latitude, r.Longitude
}

// String implements fmt.Stringer for clear and clean logging.
func (r *SpatialGeneralResult) String() string {
	if r == nil {
		return "<nil SpatialGeneralResult>"
	}
	status := "OUTSIDE"
	if r.Inside {
		status = "INSIDE"
	}
	names := r.GeofenceNames()
	return fmt.Sprintf("[%s] matches=%d (%s) at (%.6f, %.6f)", status, r.TotalMatches, strings.Join(names, ", "), r.Latitude, r.Longitude)
}

// SpatialErrorDetail represents detailed error metadata from api-service.
type SpatialErrorDetail struct {
	Type     string            `json:"@type"`
	Reason   string            `json:"reason"`
	Domain   string            `json:"domain"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// SpatialAPIError represents a structured error returned by api-service.
type SpatialAPIError struct {
	StatusCode int                  `json:"status_code"`
	Code       string               `json:"code"`
	Message    string               `json:"message"`
	StatusText string               `json:"status_text,omitempty"`
	Details    []SpatialErrorDetail `json:"details,omitempty"`
}

// Error formats the error into a readable string with a standard prefix.
func (e *SpatialAPIError) Error() string {
	if e == nil {
		return "geoengine: <nil SpatialAPIError>"
	}
	if e.Code != "" {
		return fmt.Sprintf("geoengine: spatial API error (status %d, code %s): %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("geoengine: spatial API error (status %d): %s", e.StatusCode, e.Message)
}

// Unwrap returns the corresponding sentinel error for use with errors.Is and errors.As.
func (e *SpatialAPIError) Unwrap() error {
	if e == nil {
		return nil
	}
	switch {
	case e.IsNotFound():
		return ErrGeofenceNotFound
	case e.IsQuotaExceeded():
		return ErrQuotaExceeded
	case e.IsUnauthorized():
		return ErrUnauthorized
	case e.StatusCode == http.StatusForbidden:
		return ErrForbidden
	default:
		return nil
	}
}

// IsNotFound returns true if the error indicates the geofence or resource was not found.
func (e *SpatialAPIError) IsNotFound() bool {
	if e == nil {
		return false
	}
	return e.StatusCode == http.StatusNoContent || e.StatusCode == http.StatusNotFound || e.Code == "NOT_FOUND"
}

// IsQuotaExceeded returns true if the error indicates quota limit was hit.
func (e *SpatialAPIError) IsQuotaExceeded() bool {
	if e == nil {
		return false
	}
	return e.StatusCode == http.StatusPaymentRequired || e.Code == "QUOTA_EXCEEDED"
}

// IsUnauthorized returns true if authentication failed.
func (e *SpatialAPIError) IsUnauthorized() bool {
	if e == nil {
		return false
	}
	return e.StatusCode == http.StatusUnauthorized || e.Code == "MISSING_API_KEY" || e.Code == "MISSING_ORGANIZATION"
}

// Is enables standard errors.Is matching against sentinel errors.
func (e *SpatialAPIError) Is(target error) bool {
	if e == nil {
		return false
	}
	if target == ErrGeofenceNotFound && e.IsNotFound() {
		return true
	}
	if target == ErrQuotaExceeded && e.IsQuotaExceeded() {
		return true
	}
	if target == ErrUnauthorized && e.IsUnauthorized() {
		return true
	}
	if target == ErrForbidden && e.StatusCode == http.StatusForbidden {
		return true
	}
	return false
}
