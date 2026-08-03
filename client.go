package geoengine

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	geopb "github.com/AlexG695/geo-engine-go/proto/geopb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

const (
	defaultManagementURL = "https://api.geoengine.dev"
	defaultIngestURL     = "https://ingest.geoengine.dev"
	defaultGRPCAddr      = "geo-ingestion-api-757071746002.us-central1.run.app:443"
	defaultTimeout       = 10 * time.Second
	userAgent            = "GeoEngineGoSDK/2.0.0"
)

// Client interacts with GeoEngine API over HTTP REST and gRPC.
// Safe for concurrent access across multiple goroutines.
type Client struct {
	opts Options
	http *http.Client

	grpcConn   *grpc.ClientConn
	grpcClient geopb.GeoIngestServiceClient
	grpcOnce   sync.Once
	grpcErr    error
	mu         sync.RWMutex
	closed     bool
}

// New creates a new GeoEngine client instance.
func New(apiKey string, opts ...Option) *Client {
	options := Options{
		APIKey:         apiKey,
		Environment:    "live",
		ManagementURL:  defaultManagementURL,
		IngestURL:      defaultIngestURL,
		GRPCAddress:    defaultGRPCAddr,
		Timeout:        defaultTimeout,
		BatchSize:      100,
		FlushInterval:  500 * time.Millisecond,
		WorkerPoolSize: 4,
	}

	for _, opt := range opts {
		opt(&options)
	}

	return &Client{
		opts: options,
		http: &http.Client{Timeout: options.Timeout},
	}
}

// --- gRPC Transport Layer ---

func (c *Client) initGRPC() error {
	c.grpcOnce.Do(func() {
		creds := credentials.NewTLS(&tls.Config{InsecureSkipVerify: false})

		authInterceptor := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			now := time.Now().UnixMilli()
			md := metadata.New(map[string]string{
				"x-geo-environment": c.opts.Environment,
				"x-request-time":    strconv.FormatInt(now, 10),
			})

			if c.opts.APIKey != "" {
				md.Set("x-api-key", c.opts.APIKey)
				md.Set("x-signature", c.generateHMACSignature(c.opts.APIKey, now))
			}

			if c.opts.JWTToken != "" {
				md.Set("authorization", "Bearer "+c.opts.JWTToken)
			}

			ctx = metadata.NewOutgoingContext(ctx, md)
			return invoker(ctx, method, req, reply, cc, opts...)
		}

		conn, err := grpc.Dial(
			c.opts.GRPCAddress,
			grpc.WithTransportCredentials(creds),
			grpc.WithUnaryInterceptor(authInterceptor),
		)
		if err != nil {
			c.grpcErr = fmt.Errorf("geoengine: failed to connect to gRPC server: %w", err)
			return
		}

		c.grpcConn = conn
		c.grpcClient = geopb.NewGeoIngestServiceClient(conn)
	})

	return c.grpcErr
}

func (c *Client) generateHMACSignature(key string, timestamp int64) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(strconv.FormatInt(timestamp, 10)))
	return hex.EncodeToString(h.Sum(nil))
}

// SendSingleLocationGRPC streams a single location ping via gRPC.
func (c *Client) SendSingleLocationGRPC(ctx context.Context, ping *geopb.LocationPing) (*geopb.IngestResponse, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, errors.New("geoengine: client is closed")
	}
	c.mu.RUnlock()

	if err := c.initGRPC(); err != nil {
		return nil, err
	}

	return c.grpcClient.SendSingleLocation(ctx, ping)
}

// SendBatchLocationGRPC sends a batch of telemetry pings via gRPC.
func (c *Client) SendBatchLocationGRPC(ctx context.Context, pings []*geopb.LocationPing) (*geopb.IngestResponse, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, errors.New("geoengine: client is closed")
	}
	c.mu.RUnlock()

	if err := c.initGRPC(); err != nil {
		return nil, err
	}

	req := &geopb.LocationBatchRequest{Pings: pings}
	return c.grpcClient.SendBatchLocation(ctx, req)
}

// --- HTTP REST Layer ---

type locationPayload struct {
	DeviceID  string  `json:"device_id"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Timestamp int64   `json:"timestamp"`
}

// SendLocation sends device coordinates over HTTP REST.
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

	return c.doRequest(ctx, http.MethodPost, c.opts.IngestURL+"/ingest/", payload)
}

// CreateGeofence creates a spatial geofence polygon in PostGIS via management REST API.
func (c *Client) CreateGeofence(ctx context.Context, name string, coordinates [][]float64, webhookURL string) error {
	if len(coordinates) < 3 {
		return fmt.Errorf("at least 3 coordinate pairs are required for a polygon")
	}

	var polygon [][]float64
	for _, p := range coordinates {
		if len(p) != 2 {
			return fmt.Errorf("invalid coordinate format: %v (expected [lat, lng])", p)
		}
		polygon = append(polygon, []float64{p[1], p[0]})
	}

	first := polygon[0]
	last := polygon[len(polygon)-1]
	if first[0] != last[0] || first[1] != last[1] {
		polygon = append(polygon, first)
	}

	payload := GeofenceRequest{
		Name:       name,
		WebhookURL: webhookURL,
		GeoJSON: map[string]any{
			"type":        "Polygon",
			"coordinates": [][][]float64{polygon},
		},
	}

	return c.doRequest(ctx, http.MethodPost, c.opts.ManagementURL+"/geofences", payload)
}

func (c *Client) doRequest(ctx context.Context, method, url string, payload interface{}) error {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(payload); err != nil {
		return fmt.Errorf("failed to encode json: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, &body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	now := time.Now().UnixMilli()
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.opts.APIKey)
	req.Header.Set("X-Geo-Environment", c.opts.Environment)
	req.Header.Set("X-Request-Time", strconv.FormatInt(now, 10))
	req.Header.Set("X-Signature", c.generateHMACSignature(c.opts.APIKey, now))
	req.Header.Set("User-Agent", userAgent)

	if c.opts.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.opts.JWTToken)
	}

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

// Close gracefully releases gRPC network channels.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	if c.grpcConn != nil {
		return c.grpcConn.Close()
	}
	return nil
}
