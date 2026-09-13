package geoengine

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	geopb "github.com/AlexG695/geo-engine-go/v2/proto/geopb"
)

func TestSendLocation_TableDriven(t *testing.T) {
	tests := []struct {
		name         string
		deviceID     string
		lat          float64
		lng          float64
		handler      http.HandlerFunc
		timeout      time.Duration
		withJWT      string
		expectedErr  bool
		checkErrType func(error) bool
		checkStatus  int
	}{
		{
			name:     "successful location send",
			deviceID: "device-123",
			lat:      19.4326,
			lng:      -99.1332,
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/ingest" {
					t.Errorf("expected path /ingest, got %s", r.URL.Path)
				}
				if r.Method != http.MethodPost {
					t.Errorf("expected POST, got %s", r.Method)
				}
				if r.Header.Get("X-API-Key") != "test-api-key" {
					t.Errorf("missing or invalid X-API-Key")
				}
				if r.Header.Get("X-Signature") == "" {
					t.Errorf("missing HMAC signature header")
				}
				if r.Header.Get("User-Agent") != userAgent {
					t.Errorf("unexpected User-Agent: %s", r.Header.Get("User-Agent"))
				}

				var payload locationPayload
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Errorf("failed to decode request body: %v", err)
				}
				if payload.DeviceID != "device-123" {
					t.Errorf("expected device-123, got %s", payload.DeviceID)
				}

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"success":true,"processed_count":1}`))
			},
			expectedErr: false,
		},
		{
			name:     "with JWT authorization header",
			deviceID: "truck-99",
			lat:      40.7128,
			lng:      -74.0060,
			withJWT:  "jwt.token.abc",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer jwt.token.abc" {
					t.Errorf("expected Authorization Bearer header, got %s", r.Header.Get("Authorization"))
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"success":true}`))
			},
			expectedErr: false,
		},
		{
			name:        "missing device ID returns ErrDeviceIDRequired",
			deviceID:    "",
			lat:         19.4326,
			lng:         -99.1332,
			expectedErr: true,
			checkErrType: func(err error) bool {
				return errors.Is(err, ErrDeviceIDRequired)
			},
		},
		{
			name:     "server returns 400 Bad Request",
			deviceID: "device-err",
			lat:      0,
			lng:      0,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"invalid coordinates"}`))
			},
			expectedErr: true,
			checkErrType: func(err error) bool {
				var apiErr *APIError
				if errors.As(err, &apiErr) {
					return apiErr.StatusCode == http.StatusBadRequest
				}
				return false
			},
		},
		{
			name:     "server returns 500 Internal Server Error",
			deviceID: "device-500",
			lat:      10,
			lng:      20,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`internal database failure`))
			},
			expectedErr: true,
			checkErrType: func(err error) bool {
				var apiErr *APIError
				if errors.As(err, &apiErr) {
					return apiErr.StatusCode == http.StatusInternalServerError
				}
				return false
			},
		},
		{
			name:     "request timeout",
			deviceID: "device-slow",
			lat:      1,
			lng:      1,
			timeout:  10 * time.Millisecond,
			handler: func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(50 * time.Millisecond)
				w.WriteHeader(http.StatusOK)
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var serverURL string
			if tt.handler != nil {
				server := httptest.NewServer(tt.handler)
				defer server.Close()
				serverURL = server.URL
			} else {
				serverURL = "http://localhost:9999"
			}

			opts := []Option{WithIngestURL(serverURL)}
			if tt.timeout > 0 {
				opts = append(opts, WithTimeout(tt.timeout))
			}
			if tt.withJWT != "" {
				opts = append(opts, WithJWTToken(tt.withJWT))
			}

			client := New("test-api-key", opts...)
			defer client.Close()

			ctx := context.Background()
			err := client.SendLocation(ctx, tt.deviceID, tt.lat, tt.lng)

			if (err != nil) != tt.expectedErr {
				t.Fatalf("expected error: %v, got: %v", tt.expectedErr, err)
			}
			if tt.checkErrType != nil && !tt.checkErrType(err) {
				t.Errorf("error failed type check: %v", err)
			}
		})
	}
}

func TestCreateGeofence_TableDriven(t *testing.T) {
	tests := []struct {
		name         string
		fenceName    string
		coords       [][]float64
		webhookURL   string
		handler      http.HandlerFunc
		expectedErr  bool
		checkErrType func(error) bool
	}{
		{
			name:       "successful polygon creation",
			fenceName:  "Downtown Zone",
			coords:     [][]float64{{19.43, -99.13}, {19.44, -99.13}, {19.44, -99.12}},
			webhookURL: "https://example.com/webhook",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/geofences" {
					t.Errorf("expected /geofences, got %s", r.URL.Path)
				}
				if r.Method != http.MethodPost {
					t.Errorf("expected POST, got %s", r.Method)
				}
				var req GeofenceRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Errorf("failed to decode geofence request: %v", err)
				}
				if req.Name != "Downtown Zone" {
					t.Errorf("expected Downtown Zone, got %s", req.Name)
				}
				// Verify polygon auto-closure (4 points returned: 3 + first repeated)
				geojsonCoords := req.GeoJSON["coordinates"].([]any)[0].([]any)
				if len(geojsonCoords) != 4 {
					t.Errorf("expected 4 coordinates (closed polygon), got %d", len(geojsonCoords))
				}
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"id":"geo-123","status":"active"}`))
			},
			expectedErr: false,
		},
		{
			name:        "less than 3 coordinates returns ErrInvalidCoordinates",
			fenceName:   "Line",
			coords:      [][]float64{{19.43, -99.13}, {19.44, -99.13}},
			expectedErr: true,
			checkErrType: func(err error) bool {
				return errors.Is(err, ErrInvalidCoordinates)
			},
		},
		{
			name:        "malformed coordinate point returns error",
			fenceName:   "Bad Points",
			coords:      [][]float64{{19.43, -99.13, 0.0}, {19.44, -99.13}, {19.44, -99.12}},
			expectedErr: true,
		},
		{
			name:       "server returns 422 Unprocessable Entity",
			fenceName:  "Duplicate Polygon",
			coords:     [][]float64{{10.0, 10.0}, {10.0, 20.0}, {20.0, 20.0}},
			webhookURL: "https://example.com/hook",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnprocessableEntity)
				_, _ = w.Write([]byte(`{"error":"geofence name already exists"}`))
			},
			expectedErr: true,
			checkErrType: func(err error) bool {
				var apiErr *APIError
				return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusUnprocessableEntity
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var serverURL string
			if tt.handler != nil {
				server := httptest.NewServer(tt.handler)
				defer server.Close()
				serverURL = server.URL
			} else {
				serverURL = "http://localhost:9999"
			}

			client := New("test-api-key", WithManagementURL(serverURL))
			defer client.Close()

			err := client.CreateGeofence(context.Background(), tt.fenceName, tt.coords, tt.webhookURL)
			if (err != nil) != tt.expectedErr {
				t.Fatalf("expected error: %v, got: %v", tt.expectedErr, err)
			}
			if tt.checkErrType != nil && !tt.checkErrType(err) {
				t.Errorf("error failed type check: %v", err)
			}
		})
	}
}

func TestClientOptions(t *testing.T) {
	customHTTP := &http.Client{Timeout: 3 * time.Second}

	client := New(
		"api-key-xyz",
		WithAPIKey("override-key"),
		WithJWTToken("jwt-token"),
		WithEnvironment("test"),
		WithIngestURL("https://custom-ingest.com"),
		WithManagementURL("https://custom-mgmt.com"),
		WithGRPCAddress("localhost:50051"),
		WithTimeout(4*time.Second),
		WithBatchConfig(200, 250*time.Millisecond),
		WithHTTPClient(customHTTP),
	)
	defer client.Close()

	if client.opts.APIKey != "override-key" {
		t.Errorf("expected APIKey override-key, got %s", client.opts.APIKey)
	}
	if client.opts.JWTToken != "jwt-token" {
		t.Errorf("expected JWTToken jwt-token, got %s", client.opts.JWTToken)
	}
	if client.opts.Environment != "test" {
		t.Errorf("expected Environment test, got %s", client.opts.Environment)
	}
	if client.opts.IngestURL != "https://custom-ingest.com" {
		t.Errorf("expected IngestURL https://custom-ingest.com, got %s", client.opts.IngestURL)
	}
	if client.opts.ManagementURL != "https://custom-ingest.com" && client.opts.ManagementURL != "https://custom-mgmt.com" {
		t.Errorf("expected ManagementURL https://custom-mgmt.com, got %s", client.opts.ManagementURL)
	}
	if client.opts.GRPCAddress != "localhost:50051" {
		t.Errorf("expected GRPCAddress localhost:50051, got %s", client.opts.GRPCAddress)
	}
	if client.opts.BatchSize != 200 || client.opts.FlushInterval != 250*time.Millisecond {
		t.Errorf("unexpected batch config: %v, %v", client.opts.BatchSize, client.opts.FlushInterval)
	}
	if client.http != customHTTP {
		t.Errorf("expected custom HTTP client to be configured")
	}

	// Test WithBaseURL
	client2 := New("test", WithBaseURL("https://mock-base.com"))
	defer client2.Close()
	if client2.opts.IngestURL != "https://mock-base.com" || client2.opts.ManagementURL != "https://mock-base.com" {
		t.Errorf("expected WithBaseURL to set both ingest and management URLs")
	}
}

func TestClientClose(t *testing.T) {
	client := New("test-key")
	if err := client.Close(); err != nil {
		t.Fatalf("failed to close client: %v", err)
	}

	// Idempotent close
	if err := client.Close(); err != nil {
		t.Fatalf("subsequent Close() should succeed, got: %v", err)
	}

	// gRPC operations after Close should return ErrClientClosed
	ctx := context.Background()
	_, err := client.SendSingleLocationGRPC(ctx, &geopb.LocationPing{})
	if !errors.Is(err, ErrClientClosed) {
		t.Errorf("expected ErrClientClosed, got: %v", err)
	}

	_, err = client.SendBatchLocationGRPC(ctx, []*geopb.LocationPing{})
	if !errors.Is(err, ErrClientClosed) {
		t.Errorf("expected ErrClientClosed, got: %v", err)
	}
}

func TestAsyncIngester_TableDriven(t *testing.T) {
	client := New("test-api-key")
	defer client.Close()

	ingester := NewAsyncIngester(client, 50, 5, 50*time.Millisecond)

	// Test nil enqueue
	if ingester.Enqueue(nil) {
		t.Error("enqueuing nil ping should return false")
	}

	// Test valid enqueue
	ping := &geopb.LocationPing{
		DeviceId:  "dev-01",
		Latitude:  19.4326,
		Longitude: -99.1332,
		Timestamp: time.Now().UnixMilli(),
	}
	if !ingester.Enqueue(ping) {
		t.Error("failed to enqueue valid ping")
	}

	ingester.Stop()

	// Subsequent enqueue after Stop must return false without panic
	if ingester.Enqueue(ping) {
		t.Error("enqueuing after Stop should return false")
	}

	// Calling Stop() multiple times should be safe
	ingester.Stop()
}

func TestAsyncIngester_ConcurrentSafety(t *testing.T) {
	client := New("test-api-key")
	defer client.Close()

	ingester := NewAsyncIngester(client, 1000, 10, 20*time.Millisecond)

	var wg sync.WaitGroup
	workers := 10
	iterations := 100

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = ingester.Enqueue(&geopb.LocationPing{
					DeviceId:  "device-concurrent",
					Latitude:  19.0,
					Longitude: -99.0,
					Timestamp: time.Now().UnixMilli(),
				})
				time.Sleep(50 * time.Microsecond)
			}
		}(i)
	}

	time.Sleep(10 * time.Millisecond)
	ingester.Stop()
	wg.Wait()
}

func TestAPIError_Formatting(t *testing.T) {
	err := &APIError{
		StatusCode: 404,
		Message:    `{"error":"not found"}`,
	}
	expected := `geoengine: API error (status 404): {"error":"not found"}`
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestHMACSignature(t *testing.T) {
	client := New("my-secret-key")
	defer client.Close()

	sig1 := client.generateHMACSignature("my-secret-key", 1700000000000)
	sig2 := client.generateHMACSignature("my-secret-key", 1700000000000)
	sig3 := client.generateHMACSignature("other-key", 1700000000000)

	if sig1 == "" || len(sig1) != 64 {
		t.Errorf("expected 64-char hex sha256 signature, got: %s", sig1)
	}
	if sig1 != sig2 {
		t.Errorf("signatures with same key and timestamp should be deterministic")
	}
	if sig1 == sig3 {
		t.Errorf("signatures with different keys should not match")
	}
}
