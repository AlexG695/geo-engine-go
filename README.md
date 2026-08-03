# 📦 Geo-Engine Go SDK

[![Go Reference](https://pkg.go.dev/badge/github.com/AlexG695/geo-engine-go.svg)](https://pkg.go.dev/github.com/AlexG695/geo-engine-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/AlexG695/geo-engine-go)](https://goreportcard.com/report/github.com/AlexG695/geo-engine-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Tests](https://github.com/AlexG695/geo-engine-go/actions/workflows/main.yml/badge.svg)](https://github.com/AlexG695/geo-engine-go/actions/workflows/main.yml)

> *Read this in Spanish: [README.es.md](./README.es.md)*

Official idiomatic **Go** client for the **Geo-Engine** platform.  
Designed for **high performance**, thread safety, and seamless integration into backend microservices over HTTP REST and gRPC.

## ✨ Features

- 📍 Real-time sub-10ms location ingestion (HTTP REST & gRPC).
- 🚧 Dynamic geofence creation and management backed by PostGIS spatial queries.
- ⚡ Native `context.Context` support for timeouts, deadlines, and cancellation.
- 🔄 Lock-free asynchronous worker pool for high-throughput batching.
- 🛡️ Built-in HMAC-SHA256 signature generation for anti-replay protection.

---

## 🚀 Installation

Use `go get` to install the SDK:

```bash
go get [github.com/AlexG695/geo-engine-go](https://github.com/AlexG695/geo-engine-go)

```

---

## ⚡ Quick Start

Send a device location in just a few lines:

```go
package main

import (
    "context"
    "log"
    "time"

    geoengine "[github.com/AlexG695/geo-engine-go](https://github.com/AlexG695/geo-engine-go)"
)

func main() {
    client := geoengine.New("sk_live_123456")
    defer client.Close()

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    err := client.SendLocation(ctx, "truck-01", 19.4326, -99.1332)
    if err != nil {
        log.Fatalf("Error sending location: %v", err)
    }

    log.Println("Location sent successfully")
}

```

---

## 🔧 Advanced Configuration

The client uses the **Functional Options** pattern for clean configuration:

```go
client := geoengine.New(
    "sk_test_123456",
    geoengine.WithEnvironment("test"),
    geoengine.WithIngestURL("http://localhost:8080"), // For local dev
    geoengine.WithTimeout(2 * time.Second),
)

```

### Available Options

| Option | Description |
| --- | --- |
| `WithAPIKey(key string)` | Sets the API key for execution |
| `WithJWTToken(token string)` | Sets the Bearer JWT token for authenticated batch sessions |
| `WithEnvironment(env string)` | Configures the execution target (`"live"` or `"test"`) |
| `WithIngestURL(url string)` | Overrides the ingestion REST endpoint |
| `WithGRPCAddress(addr string)` | Overrides the default gRPC server target address |
| `WithManagementURL(url string)` | Overrides the control plane management endpoint |
| `WithTimeout(d time.Duration)` | Sets the HTTP client timeout |
| `WithBatchConfig(size, interval)` | Configures buffer limits for the background async worker |

---

## 🔄 High-Throughput Async Batch Ingestion

For high-volume fleet tracking or IoT telemetry, use the lock-free background worker:

```go
package main

import (
    "time"

    geoengine "[github.com/AlexG695/geo-engine-go](https://github.com/AlexG695/geo-engine-go)"
    geopb "[github.com/AlexG695/geo-engine-go/proto/geopb](https://github.com/AlexG695/geo-engine-go/proto/geopb)"
)

func main() {
    client := geoengine.New("sk_live_123456")
    defer client.Close()

    // Buffer: 5000 pings, Batch size: 100, Flush interval: 500ms
    ingester := geoengine.NewAsyncIngester(client, 5000, 100, 500*time.Millisecond)
    defer ingester.Stop()

    ingester.Enqueue(&geopb.LocationPing{
        DeviceId:  "vehicle-scooter-09",
        Latitude:  19.4326,
        Longitude: -99.1332,
        Timestamp: time.Now().UnixMilli(),
    })
}

```

---

## 🧪 Testing

Run the unit tests using standard Go tooling:

```bash
go test -v ./...

```

---

## 📄 License

MIT © Geo-Engine Team