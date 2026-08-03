# 📦 Geo-Engine Go SDK

[![Go Reference](https://pkg.go.dev/badge/github.com/AlexG695/geo-engine-go.svg)](https://pkg.go.dev/github.com/AlexG695/geo-engine-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/AlexG695/geo-engine-go)](https://goreportcard.com/report/github.com/AlexG695/geo-engine-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Tests](https://github.com/AlexG695/geo-engine-go/actions/workflows/main.yml/badge.svg)](https://github.com/AlexG695/geo-engine-go/actions/workflows/main.yml)

> *Lee esto en Inglés: [README.md](./README.md)*

Cliente oficial e idiomático en **Go** para interactuar con la plataforma **Geo-Engine**.  
Diseñado para **alto rendimiento**, seguridad en concurrencia (thread-safe) y fácil integración en microservicios mediante HTTP REST y gRPC.

## ✨ Características

- 📍 Ingestión de ubicaciones en tiempo real con latencia sub-10ms (HTTP REST & gRPC).
- 🚧 Creación y gestión dinámica de geocercas respaldadas por consultas espaciales en PostGIS.
- ⚡ Soporte nativo de `context.Context` para timeouts, deadlines y cancelación.
- 🔄 Procesador asíncrono sin bloqueos para transmisión eficiente por lotes (batching).
- 🛡️ Generación automática de firmas criptográficas HMAC-SHA256 para protección anti-replay.

---

## 🚀 Instalación

Usa `go get` para instalar el SDK:

```bash
go get [github.com/AlexG695/geo-engine-go](https://github.com/AlexG695/geo-engine-go)

```

---

## ⚡ Inicio Rápido

Envía la ubicación de un dispositivo en pocas líneas:

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

    err := client.SendLocation(ctx, "camion-01", 19.4326, -99.1332)
    if err != nil {
        log.Fatalf("Error enviando datos: %v", err)
    }

    log.Println("Ubicación enviada correctamente")
}

```

---

## 🔧 Configuración Avanzada

El cliente utiliza el patrón de **Opciones Funcionales** (Functional Options) para una configuración limpia y mantenible:

```go
client := geoengine.New(
    "sk_test_123456",
    geoengine.WithEnvironment("test"),
    geoengine.WithIngestURL("http://localhost:8080"), // Para desarrollo local
    geoengine.WithTimeout(2 * time.Second),
)

```

### Opciones disponibles

| Opción | Descripción |
| --- | --- |
| `WithAPIKey(key string)` | Configura la clave API de ejecución |
| `WithJWTToken(token string)` | Configura el token JWT Bearer para operaciones por lote |
| `WithEnvironment(env string)` | Define el entorno objetivo (`"live"` o `"test"`) |
| `WithIngestURL(url string)` | Sobrescribe la URL del pipeline de ingestión REST |
| `WithGRPCAddress(addr string)` | Sobrescribe la dirección objetivo del servidor gRPC |
| `WithManagementURL(url string)` | Sobrescribe la URL del plano de gestión |
| `WithTimeout(d time.Duration)` | Define el timeout límite del cliente HTTP |
| `WithBatchConfig(size, interval)` | Configura el tamaño y frecuencia del búfer asíncrono |

---

## 🔄 Ingestión Asíncrona por Lotes de Alto Rendimiento

Para telemetría de flotas masivas o dispositivos IoT, utiliza el worker en segundo plano:

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

    // Búfer: 5000 pings, Tamaño de lote: 100, Frecuencia de envío: 500ms
    ingester := geoengine.NewAsyncIngester(client, 5000, 100, 500*time.Millisecond)
    defer ingester.Stop()

    ingester.Enqueue(&geopb.LocationPing{
        DeviceId:  "scooter-01",
        Latitude:  19.4326,
        Longitude: -99.1332,
        Timestamp: time.Now().UnixMilli(),
    })
}

```

---

## 🧪 Testing

Para correr las pruebas:

```bash
go test -v ./...

```

---

## 📄 Licencia

MIT © Geo-Engine Team