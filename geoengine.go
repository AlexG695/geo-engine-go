package geoengine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	defaultManagementURL = "https://api.geoengine.dev"
	defaultIngestURL     = "http://ingest.geoengine.dev"
	defaultTimeout       = 10 * time.Second
	userAgent            = "GeoEngineGo/1.0.0"
)

type Client struct {
	apiKey        string
	managementURL string
	ingestURL     string
	http          *http.Client
}
type Option func(*Client)

func WithIngestURL(url string) Option {
	return func(c *Client) {
		c.ingestURL = url
	}
}
func WithManagementURL(url string) Option {
	return func(c *Client) {
		c.managementURL = url
	}
}
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.http.Timeout = d
	}
}
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
	Name       string                 `json:"name"`
	WebhookURL string                 `json:"webhook_url"`
	GeoJSON    map[string]interface{} `json:"geojson"`
}

func (c *Client) SendLocation(deviceID string, lat, lng float64) error {
	if deviceID == "" {
		return fmt.Errorf("device_id es requerido")
	}

	payload := locationPayload{
		DeviceID:  deviceID,
		Latitude:  lat,
		Longitude: lng,
		Timestamp: time.Now().Unix(),
	}

	return c.doRequest(http.MethodPost, c.ingestURL+"/ingest", payload)
}

func (c *Client) CreateGeofence(name string, coordinates [][]float64, webhookURL string) error {
	if len(coordinates) < 3 {
		return fmt.Errorf("se requieren al menos 3 coordenadas para un polígono")
	}

	var polygon [][]float64
	for _, p := range coordinates {
		if len(p) != 2 {
			return fmt.Errorf("formato de coordenada inválido: %v (se espera [lat, lng])", p)
		}
		polygon = append(polygon, []float64{p[1], p[0]})
	}

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

	return c.doRequest(http.MethodPost, c.managementURL+"/geofences", payload)
}

func (c *Client) doRequest(method, url string, payload interface{}) error {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(payload); err != nil {
		return fmt.Errorf("error codificando json: %w", err)
	}

	req, err := http.NewRequest(method, url, &body)
	if err != nil {
		return fmt.Errorf("error creando request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("error de red: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("error de API: status %d", resp.StatusCode)
	}

	return nil
}
