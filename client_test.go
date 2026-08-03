package geoengine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	geopb "github.com/AlexG695/geo-engine-go/proto/geopb"
)

func TestSendLocation_HTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ingest/" {
			t.Errorf("expected path /ingest/, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected method POST, got %s", r.Method)
		}
		if r.Header.Get("X-API-Key") != "test-api-key" {
			t.Errorf("missing or invalid X-API-Key header")
		}
		if r.Header.Get("X-Signature") == "" {
			t.Errorf("missing HMAC X-Signature header")
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"queued"}`))
	}))
	defer server.Close()

	client := New("test-api-key", WithBaseURL(server.URL))

	ctx := context.Background()
	err := client.SendLocation(ctx, "device-123", 19.4326, -99.1332)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestSendLocation_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := New("test-api-key", WithBaseURL(server.URL), WithTimeout(1*time.Millisecond))

	ctx := context.Background()
	err := client.SendLocation(ctx, "device-123", 0, 0)

	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

func TestAsyncIngester(t *testing.T) {
	client := New("test-api-key")
	defer client.Close()

	ingester := NewAsyncIngester(client, 10, 2, 100*time.Millisecond)

	ok := ingester.Enqueue(&geopb.LocationPing{
		DeviceId:  "dev-01",
		Latitude:  19.4326,
		Longitude: -99.1332,
		Timestamp: time.Now().UnixMilli(),
	})

	if !ok {
		t.Error("failed to enqueue location ping to async ingester")
	}

	ingester.Stop()
}
