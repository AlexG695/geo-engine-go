package geoengine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSendLocation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ingest" {
			t.Errorf("Expected path /ingest, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("Expected method POST, got %s", r.Method)
		}

		if r.Header.Get("X-API-Key") != "test-api-key" {
			t.Errorf("Expected API Key header")
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client := New("test-api-key", WithBaseURL(server.URL))

	ctx := context.Background()
	err := client.SendLocation(ctx, "device-123", 19.4326, -99.1332)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestSendLocation_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := New("test-api-key", WithBaseURL(server.URL), WithTimeout(1*time.Millisecond))

	ctx := context.Background()
	err := client.SendLocation(ctx, "device-123", 0, 0)

	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
}
