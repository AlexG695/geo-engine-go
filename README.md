# 📦 Geo-Engine Go SDK

[![Go Reference](https://pkg.go.dev/badge/github.com/AlexG695/geo-engine-go/v2.svg)](https://pkg.go.dev/github.com/AlexG695/geo-engine-go/v2)
[![Go Report Card](https://goreportcard.com/badge/github.com/AlexG695/geo-engine-go/v2)](https://goreportcard.com/report/github.com/AlexG695/geo-engine-go/v2)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Tests](https://github.com/AlexG695/geo-engine-go/actions/workflows/main.yml/badge.svg)](https://github.com/AlexG695/geo-engine-go/actions/workflows/main.yml)

> *Read this in Spanish: [README.es.md](./README.es.md)*

Official idiomatic **Go** client for the **Geo-Engine** platform.  
Designed for **high performance**, thread safety, and seamless integration into backend microservices over HTTP REST and gRPC.

## ✨ Features

- 📍 Real-time sub-10ms location ingestion (HTTP REST & gRPC).
- 🗺️ High-performance Spatial Validation module for point-in-polygon queries (single & general).
- 👥 Users module to manage organization members, roles, and email invitations.
- 🛡️ GeoIdentity module to create access rules (IP lists, geographic limits) and assign them to users or the whole organization.
- 🚧 Dynamic geofence creation and management backed by PostGIS spatial queries.
- ⚡ Native `context.Context` support for timeouts, deadlines, and cancellation.
- 🔄 Thread-safe asynchronous worker pool for high-throughput batching.
- 🛡️ Built-in HMAC-SHA256 signature generation for anti-replay protection.
- 🧩 Minimal dependencies: only standard library and official gRPC/Protobuf packages.

---

## 🚀 Installation

Use `go get` to install the SDK:

```bash
go get github.com/AlexG695/geo-engine-go/v2
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

    geoengine "github.com/AlexG695/geo-engine-go/v2"
)

func main() {
    client := geoengine.New("geo_live_123456")
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

## 🗺️ Spatial Module & Point Validation

The `client.Spatial` module connects directly to the GeoEngine API service to perform point-in-polygon evaluations with the best DX:

### Single Geofence Validation
```go
res, err := client.Spatial.ValidatePoint(ctx, "geofence_123", 19.4326, -99.1332)
if err != nil {
    if errors.Is(err, geoengine.ErrGeofenceNotFound) {
        log.Println("Geofence not found")
        return
    }
    log.Fatalf("Spatial error: %v", err)
}

if res.IsInside() {
    log.Printf("Device is inside %s (%s)!", res.GeofenceName, res.GeofenceID)
} else {
    log.Println("Device is outside the geofence")
}
```

### General Geofence Validation (All Geofences)
```go
res, err := client.Spatial.ValidateGeneral(ctx, 19.4326, -99.1332)
if err != nil {
    log.Fatalf("General validation failed: %v", err)
}

if res.HasMatches() {
    log.Printf("Matched %d geofences: %v", res.TotalMatches, res.GeofenceNames())
    if res.ContainsGeofence("CDMX Downtown") {
        log.Println("Device is in CDMX Downtown")
    }
}
```

### Fluent Query Builder
```go
// Fluent syntax for maximum readability
res, err := client.Spatial.Check(19.4326, -99.1332).
    InGeofence("geofence_123").
    ExecuteSingle(ctx)

generalRes, err := client.Spatial.Check(19.4326, -99.1332).
    InAllGeofences().
    ExecuteGeneral(ctx)
```

---

## 👥 Users Module

The `client.Users` module manages the members of the organization that owns the API key:

```go
// List members and available roles
users, err := client.Users.List(ctx)
roles, err := client.Users.ListRoles(ctx)

// Invite a new member (GeoEngine sends the invitation email)
inv, err := client.Users.Invite(ctx, geoengine.InviteUserRequest{
    Email: "ana@acme.com",
    Role:  geoengine.RoleViewer,
    Lang:  "en",
})
if errors.Is(err, geoengine.ErrInvitationPending) {
    log.Println("There is already a pending invitation for this email")
}

// Change a member's role or revoke their access
user, err := client.Users.UpdateRole(ctx, "user_123", geoengine.RoleDispatcher)
err = client.Users.Remove(ctx, "user_123")
if errors.Is(err, geoengine.ErrLastAdmin) {
    log.Println("The organization must keep at least one admin")
}
```

---

## 🛡️ GeoIdentity Module

The `client.GeoIdentity` module creates access rules and assigns them to a user or to the whole organization:

```go
area, err := geoengine.PolygonFromLatLng([][]float64{
    {19.40, -99.20}, {19.40, -99.10}, {19.50, -99.10}, // [lat, lng]
})
if err != nil {
    log.Fatal(err)
}

rule, err := client.GeoIdentity.CreateRule(ctx, geoengine.CreateRuleRequest{
    Name:        "CDMX only",
    Action:      geoengine.RuleActionBlock,
    BlackIPList: []string{"203.0.113.0/24"},
    BlockTor:    true,
    GeoLimit:    area,
    AssignedSubjects: []geoengine.SubjectAssignment{
        {SubjectType: geoengine.SubjectUser, SubjectID: "user_123"},
    },
})

// Assign or unassign the rule
_, err = client.GeoIdentity.AssignToUser(ctx, rule.ID, "user_456")
_, err = client.GeoIdentity.AssignToOrganization(ctx, rule.ID)
_, err = client.GeoIdentity.UnassignFromUser(ctx, rule.ID, "user_456")

// Rules that apply to a user (direct, by role, or organization-wide)
rules, err := client.GeoIdentity.ListUserRules(ctx, "user_123")
```

> A rule created without `AssignedSubjects` applies to the whole organization. A rule must always keep at least one subject: removing the last one returns `ErrLastSubject` (deactivate or delete the rule instead).

---

## 🔧 Advanced Configuration

The client uses the **Functional Options** pattern for clean configuration:

```go
client := geoengine.New(
    "sk_test_123456",
    geoengine.WithEnvironment("test"),
    geoengine.WithAPIServiceURL("https://api.geoengine.dev"),
    geoengine.WithTimeout(2 * time.Second),
)
```

### Available Options

| Option | Description |
| --- | --- |
| `WithAPIKey(key string)` | Sets the API key for execution |
| `WithJWTToken(token string)` | Sets the Bearer JWT token for authenticated batch sessions |
| `WithEnvironment(env string)` | Configures the execution target (`"live"` or `"test"`) |
| `WithAPIServiceURL(url string)` | Overrides the API service URL for spatial operations |
| `WithIngestURL(url string)` | Overrides the ingestion REST endpoint |
| `WithGRPCAddress(addr string)` | Overrides the default gRPC server target address |
| `WithManagementURL(url string)` | Overrides the control plane management endpoint |
| `WithBaseURL(url string)` | Overrides ingest, management, and API service endpoints (for testing) |
| `WithTimeout(d time.Duration)` | Sets the HTTP client timeout |
| `WithHTTPClient(client *http.Client)` | Configures a custom `*http.Client` (tracing, proxies) |
| `WithBatchConfig(size, interval)` | Configures buffer limits for the background async worker |

---

## 🔄 High-Throughput Async Batch Ingestion

For high-volume fleet tracking or IoT telemetry, use the background worker:

```go
package main

import (
    "time"

    geoengine "github.com/AlexG695/geo-engine-go/v2"
    geopb "github.com/AlexG695/geo-engine-go/v2/proto/geopb"
)


func main() {
    client := geoengine.New("geo_live_123456")
    defer client.Close()

    // Buffer: 5000 pings, Batch size: 100, Flush interval: 500ms
    ingester := geoengine.NewAsyncIngester(client, 5000, 100, 500*time.Millisecond)
    defer ingester.Stop()

    ok := ingester.Enqueue(&geopb.LocationPing{
        DeviceId:  "vehicle-scooter-09",
        Latitude:  19.4326,
        Longitude: -99.1332,
        Timestamp: time.Now().UnixMilli(),
    })
    if !ok {
        // Handle buffer queue full or ingester stopped
    }
}
```

---

## 🧪 Testing

Run the unit tests with race detection using standard Go tooling:

```bash
go test -v -race -cover ./...
```

---

## 📄 License

MIT © Geo-Engine Team