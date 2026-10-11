package geoengine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// SpatialService handles point-in-polygon and spatial validation against the GeoEngine api-service.
// Safe for concurrent use across multiple goroutines.
type SpatialService struct {
	client *Client
}

func newSpatialService(client *Client) *SpatialService {
	return &SpatialService{client: client}
}

// ValidatePoint is a convenience method to check if coordinates fall within a specific geofence.
func (s *SpatialService) ValidatePoint(ctx context.Context, geofenceID string, lat, lng float64) (*SpatialValidationResult, error) {
	req := SpatialValidationRequest{
		GeofenceID: strings.TrimSpace(geofenceID),
		Latitude:   lat,
		Longitude:  lng,
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	return s.ValidateLocation(ctx, req)
}

// ValidateLocation checks if coordinates fall inside the specified geofence using the api-service.
func (s *SpatialService) ValidateLocation(ctx context.Context, req SpatialValidationRequest) (*SpatialValidationResult, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	endpoint := strings.TrimRight(s.client.opts.APIServiceURL, "/") + "/api/spatial/validate"

	var result SpatialValidationResult
	if err := s.doSpatialRequest(ctx, endpoint, req, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// ValidateGeneral checks whether coordinates intersect any geofence belonging to the organization.
func (s *SpatialService) ValidateGeneral(ctx context.Context, lat, lng float64) (*SpatialGeneralResult, error) {
	req := SpatialGeneralRequest{
		Latitude:  lat,
		Longitude: lng,
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	return s.ValidateGeneralLocation(ctx, req)
}

// ValidateGeneralLocation checks whether coordinates intersect any geofence using the api-service.
func (s *SpatialService) ValidateGeneralLocation(ctx context.Context, req SpatialGeneralRequest) (*SpatialGeneralResult, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	endpoint := strings.TrimRight(s.client.opts.APIServiceURL, "/") + "/api/spatial/validate-general"

	var result SpatialGeneralResult
	if err := s.doSpatialRequest(ctx, endpoint, req, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// Check initializes a fluent builder for spatial coordinate validation.
func (s *SpatialService) Check(lat, lng float64) *SpatialCheckBuilder {
	return &SpatialCheckBuilder{
		service: s,
		lat:     lat,
		lng:     lng,
	}
}

// SpatialCheckBuilder provides a fluent query builder for spatial operations.
type SpatialCheckBuilder struct {
	service *SpatialService
	lat     float64
	lng     float64
}

// InGeofence configures validation targeting a specific geofence ID.
func (b *SpatialCheckBuilder) InGeofence(geofenceID string) *SingleGeofenceCheck {
	return &SingleGeofenceCheck{
		service:    b.service,
		geofenceID: geofenceID,
		lat:        b.lat,
		lng:        b.lng,
	}
}

// InAllGeofences configures validation against all organization geofences.
func (b *SpatialCheckBuilder) InAllGeofences() *GeneralGeofenceCheck {
	return &GeneralGeofenceCheck{
		service: b.service,
		lat:     b.lat,
		lng:     b.lng,
	}
}

// SingleGeofenceCheck evaluates coordinate containment inside a specific geofence.
type SingleGeofenceCheck struct {
	service    *SpatialService
	geofenceID string
	lat        float64
	lng        float64
}

// Execute performs the single geofence spatial query.
func (c *SingleGeofenceCheck) Execute(ctx context.Context) (*SpatialValidationResult, error) {
	return c.service.ValidatePoint(ctx, c.geofenceID, c.lat, c.lng)
}

// ExecuteSingle is an alias for Execute.
func (c *SingleGeofenceCheck) ExecuteSingle(ctx context.Context) (*SpatialValidationResult, error) {
	return c.Execute(ctx)
}

// Validate is an alias for Execute.
func (c *SingleGeofenceCheck) Validate(ctx context.Context) (*SpatialValidationResult, error) {
	return c.Execute(ctx)
}

// GeneralGeofenceCheck evaluates coordinate containment across all organization geofences.
type GeneralGeofenceCheck struct {
	service *SpatialService
	lat     float64
	lng     float64
}

// Execute performs the general spatial query against all organization geofences.
func (c *GeneralGeofenceCheck) Execute(ctx context.Context) (*SpatialGeneralResult, error) {
	return c.service.ValidateGeneral(ctx, c.lat, c.lng)
}

// ExecuteGeneral is an alias for Execute.
func (c *GeneralGeofenceCheck) ExecuteGeneral(ctx context.Context) (*SpatialGeneralResult, error) {
	return c.Execute(ctx)
}

// Validate is an alias for Execute.
func (c *GeneralGeofenceCheck) Validate(ctx context.Context) (*SpatialGeneralResult, error) {
	return c.Execute(ctx)
}

type apiErrorContainer struct {
	Code    int                  `json:"code"`
	Message string               `json:"message"`
	Status  string               `json:"status"`
	Details []SpatialErrorDetail `json:"details,omitempty"`
}

func (s *SpatialService) doSpatialRequest(ctx context.Context, endpoint string, payload any, target any) error {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	if err := json.NewEncoder(buf).Encode(payload); err != nil {
		return fmt.Errorf("geoengine: failed to encode request payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, buf)
	if err != nil {
		return fmt.Errorf("geoengine: failed to create request: %w", err)
	}

	now := time.Now().UnixMilli()
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", s.client.opts.APIKey)
	req.Header.Set("X-Geo-Environment", s.client.opts.Environment)
	req.Header.Set("X-Request-Time", strconv.FormatInt(now, 10))
	req.Header.Set("X-Signature", s.client.generateHMACSignature(s.client.opts.APIKey, now))
	req.Header.Set("User-Agent", userAgent)

	if s.client.opts.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.client.opts.JWTToken)
	}

	resp, err := s.client.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("geoengine: request canceled or deadline exceeded: %w", ctx.Err())
		}
		return fmt.Errorf("geoengine: network error: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusNoContent {
		return &SpatialAPIError{
			StatusCode: http.StatusNoContent,
			Code:       "NOT_FOUND",
			Message:    "geofence not found",
			StatusText: http.StatusText(http.StatusNoContent),
		}
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return fmt.Errorf("geoengine: failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return parseSpatialError(resp.StatusCode, bodyBytes)
	}

	var envelope struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(bodyBytes, &envelope); err != nil {
		return fmt.Errorf("geoengine: failed to parse response JSON: %w", err)
	}

	if !envelope.Success {
		return parseSpatialError(resp.StatusCode, bodyBytes)
	}

	if err := json.Unmarshal(envelope.Data, target); err != nil {
		return fmt.Errorf("geoengine: failed to decode response data: %w", err)
	}

	return nil
}

func parseSpatialError(statusCode int, body []byte) *SpatialAPIError {
	apiErr := &SpatialAPIError{
		StatusCode: statusCode,
		StatusText: http.StatusText(statusCode),
	}

	var parsed struct {
		Error *apiErrorContainer `json:"error"`
	}

	if err := json.Unmarshal(body, &parsed); err == nil && parsed.Error != nil {
		apiErr.Code = parsed.Error.Status
		if parsed.Error.Code != 0 {
			apiErr.StatusCode = parsed.Error.Code
		}
		apiErr.Message = parsed.Error.Message
		apiErr.Details = parsed.Error.Details

		for _, detail := range parsed.Error.Details {
			if detail.Reason != "" {
				apiErr.Code = detail.Reason
				break
			}
		}
	} else {
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = http.StatusText(statusCode)
		}
		apiErr.Message = msg
	}

	if apiErr.Code == "" {
		switch statusCode {
		case http.StatusNotFound, http.StatusNoContent:
			apiErr.Code = "NOT_FOUND"
		case http.StatusPaymentRequired:
			apiErr.Code = "QUOTA_EXCEEDED"
		case http.StatusUnauthorized:
			apiErr.Code = "UNAUTHORIZED"
		case http.StatusForbidden:
			apiErr.Code = "FORBIDDEN"
		}
	}

	return apiErr
}
