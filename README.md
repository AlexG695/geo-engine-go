# 📦 Geo-Engine Go SDK

[![Go Reference](https://pkg.go.dev/badge/github.com/AlexG695/geo-engine-go.svg)](https://pkg.go.dev/github.com/AlexG695/geo-engine-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/AlexG695/geo-engine-go)](https://goreportcard.com/report/github.com/AlexG695/geo-engine-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Tests](https://github.com/AlexG695/geo-engine-go/actions/workflows/main.yml/badge.svg)](https://github.com/AlexG695/geo-engine-go/actions/workflows/main.yml)

> *Read this in Spanish: [README.es.md](./README.es.md)*


Official idiomatic **Go** client for the **Geo-Engine** platform.  
Designed for **high performance**, thread safety, and easy integration into backend services.

## ✨ Features

- 📍 Real-time location ingestion
- 🚧 Dynamic geofence creation and management
- ⚡ Native `context.Context` support for timeouts and cancellation

---

## 🚀 Installation

Use `go get` to install the SDK:

```bash
go get github.com/AlexG695/geo-engine-go
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

    geoengine "github.com/AlexG695/geo-engine-go"
)

func main() {
    // 1. Initialize client
    client := geoengine.New("sk_live_123456")

    // 2. Create a context with timeout (Best Practice)
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // 3. Send location
    // ID, Latitude, Longitude
    err := client.SendLocation(ctx, "truck-01", 19.4326, -99.1332)
    if err != nil {
        log.Fatalf("Error sending data: %v", err)
    }

    log.Println("✅ Location sent successfully")
}
```

---

## 🔧 Advanced Configuration

The client uses the **Functional Options** pattern for clean configuration:

```go
client := geoengine.New(
    "sk_test_123456",
    geoengine.WithIngestURL("http://localhost:8080"), // For local dev
    geoengine.WithTimeout(2 * time.Second),           // Aggressive timeout
)
```

### Available Options

| Option                          | Description                       |
| ------------------------------- | --------------------------------- |
| `WithIngestURL(url string)`     | Overrides the ingestion endpoint  |
| `WithManagementURL(url string)` | Overrides the management endpoint |
| `WithTimeout(d time.Duration)`  | Sets the HTTP client timeout      |

---

## 🧪 Testing

Run the tests using standard Go tools:

```bash
go test -v ./...
```

---

## 📄 License

MIT © Geo-Engine Team
