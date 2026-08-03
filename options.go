package geoengine

import "time"

// Options holds configuration parameters for the Client.
type Options struct {
	APIKey         string
	JWTToken       string
	Environment    string // "live" or "test"
	ManagementURL  string
	IngestURL      string
	GRPCAddress    string
	Timeout        time.Duration
	BatchSize      int
	FlushInterval  time.Duration
	WorkerPoolSize int
}

// Option modifies Options using functional parameters.
type Option func(*Options)

// WithAPIKey sets the X-API-Key used against the api_keys table.
func WithAPIKey(key string) Option {
	return func(o *Options) {
		o.APIKey = key
	}
}

// WithJWTToken sets the Bearer token for authenticated batch operations.
func WithJWTToken(token string) Option {
	return func(o *Options) {
		o.JWTToken = token
	}
}

// WithEnvironment overrides the runtime environment ("live" or "test").
func WithEnvironment(env string) Option {
	return func(o *Options) {
		o.Environment = env
	}
}

// WithIngestURL overrides the HTTP ingestion endpoint.
func WithIngestURL(url string) Option {
	return func(o *Options) {
		o.IngestURL = url
	}
}

// WithGRPCAddress overrides the default gRPC server target host.
func WithGRPCAddress(addr string) Option {
	return func(o *Options) {
		o.GRPCAddress = addr
	}
}

// WithManagementURL overrides the control plane URL for geofence/trip operations.
func WithManagementURL(url string) Option {
	return func(o *Options) {
		o.ManagementURL = url
	}
}

// WithBaseURL overrides both ingestion and management endpoints for local mock testing.
func WithBaseURL(url string) Option {
	return func(o *Options) {
		o.IngestURL = url
		o.ManagementURL = url
	}
}

// WithTimeout sets maximum HTTP request duration.
func WithTimeout(d time.Duration) Option {
	return func(o *Options) {
		o.Timeout = d
	}
}

// WithBatchConfig configures the buffer size and flush interval for AsyncIngester.
func WithBatchConfig(batchSize int, flushInterval time.Duration) Option {
	return func(o *Options) {
		o.BatchSize = batchSize
		o.FlushInterval = flushInterval
	}
}
