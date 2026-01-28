package geoengine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	defaultManagementURL = "https://api.geoengine.dev"
	defaultIngestURL     = "https://ingest.geoengine.dev"
	defaultTimeout       = 10 * time.Second
	userAgent            = "GeoEngineGo/1.0.0"
)

// Client interacts with the Geo-Engine API.
// It is safe for concurrent use by multiple goroutines.
type Client struct {
	apiKey        string
	managementURL string
	ingestURL     string
	http          *http.Client
}

// Option defines the functional configuration for the Client.
type Option func(*Client)

// WithIngestURL overrides the default ingestion URL.
// Useful for local development or proxy testing.
func WithIngestURL(url string) Option {
	return func(c *Client) {
		c.ingestURL = url
	}
}

// WithManagementURL overrides the default management URL.
func WithManagementURL(url string) Option {
	return func(c *Client) {
		c.managementURL = url
	}
}

// WithBaseURL overrides both the management and ingestion URLs.
// This is primarily useful for testing with a mock server.
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.ingestURL = url
		c.managementURL = url
	}
}

// WithTimeout sets the maximum duration for HTTP requests.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.http.Timeout = d
	}
}

// New creates a new Geo-Engine client.
// It accepts optional configuration functions.
func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:        apiKey,
		managementURL: defaultManagementURL,
		ingestURL:     defaultIngestURL,
		http:          &http.Client{Timeout: defaultTimeout},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

type locationPayload struct {
	DeviceID  string  `json:"device_id"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Timestamp int64   `json:"timestamp"`
}

type geofencePayload struct {
	Name       string         `json:"name"`
	WebhookURL string         `json:"webhook_url"`
	GeoJSON    map[string]any `json:"geojson"`
}

// SendLocation sends a device's location to the ingestion engine.
// It returns an error if the context is canceled or the API rejects the data.
func (c *Client) SendLocation(ctx context.Context, deviceID string, lat, lng float64) error {
	if deviceID == "" {
		return fmt.Errorf("device_id is required")
	}

	payload := locationPayload{
		DeviceID:  deviceID,
		Latitude:  lat,
		Longitude: lng,
		Timestamp: time.Now().Unix(),
	}

	return c.doRequest(ctx, http.MethodPost, c.ingestURL+"/ingest", payload)
}

// CreateGeofence creates a new monitoring zone programmatically.
// Coordinates must be a slice of [latitude, longitude] pairs.
// The polygon will be automatically closed if the last point does not match the first.
func (c *Client) CreateGeofence(ctx context.Context, name string, coordinates [][]float64, webhookURL string) error {
	if len(coordinates) < 3 {
		return fmt.Errorf("at least 3 coordinates are required for a polygon")
	}

	var polygon [][]float64
	for _, p := range coordinates {
		if len(p) != 2 {
			return fmt.Errorf("invalid coordinate format: %v (expected [lat, lng])", p)
		}
		// GeoJSON standard expects [Longitude, Latitude]
		polygon = append(polygon, []float64{p[1], p[0]})
	}

	// Automatically close the polygon ring
	first := polygon[0]
	last := polygon[len(polygon)-1]
	if first[0] != last[0] || first[1] != last[1] {
		polygon = append(polygon, first)
	}

	payload := geofencePayload{
		Name:       name,
		WebhookURL: webhookURL,
		GeoJSON: map[string]any{
			"type":        "Polygon",
			"coordinates": [][][]float64{polygon},
		},
	}

	return c.doRequest(ctx, http.MethodPost, c.managementURL+"/geofences", payload)
}

func (c *Client) doRequest(ctx context.Context, method, url string, payload interface{}) error {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(payload); err != nil {
		return fmt.Errorf("failed to encode json: %w", err)
	}

	// NewRequestWithContext ensures we respect timeouts and cancellations
	req, err := http.NewRequestWithContext(ctx, method, url, &body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		select {
		case <-ctx.Done():
			return fmt.Errorf("request canceled: %w", ctx.Err())
		default:
			return fmt.Errorf("network error: %w", err)
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("api error: status %d", resp.StatusCode)
	}

	return nil
}
