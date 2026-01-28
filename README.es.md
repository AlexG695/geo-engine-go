# 📦 Geo-Engine Go SDK

[![Go Reference](https://pkg.go.dev/badge/github.com/AlexG695/geo-engine-go.svg)](https://pkg.go.dev/github.com/AlexG695/geo-engine-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/AlexG695/geo-engine-go)](https://goreportcard.com/report/github.com/AlexG695/geo-engine-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Tests](https://github.com/AlexG695/geo-engine-go/.github/workflows/test.yml/badge.svg)](https://github.com/AlexG695/geo-engine-go/.github/workflows/test.yml)
> *Lee esto en Inglés: [README.md](./README.md)*

Cliente oficial e idiomático en **Go** para interactuar con la plataforma **Geo-Engine**.
Diseñado para **alto rendimiento**, seguridad en concurrencia (thread-safe) y fácil integración en microservicios.

Características:
- 📍 Ingestión de ubicaciones en tiempo real.
- 🚧 Creación y gestión dinámica de geocercas.
- ⚡ Soporte nativo de `context.Context` para timeouts y cancelación.

---

## 🚀 Instalación

Usa `go get` para instalar el SDK:

```bash
go get github.com/AlexG695/geo-engine-go

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
	
    geoengine "github.com/AlexG695/geo-engine-go"
)

func main() {
    // 1. Inicializar cliente
    client := geoengine.New("sk_live_123456")

    // 2. Crear un contexto con timeout (Buena práctica en Go)
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // 3. Enviar ubicación
    // ID, Latitud, Longitud
    err := client.SendLocation(ctx, "camion-01", 19.4326, -99.1332)
    if err != nil {
        log.Fatalf("Error enviando datos: %v", err)
    }

    log.Println("✅ Ubicación enviada correctamente")
}

```

---

## 🔧 Configuración Avanzada

El cliente utiliza el patrón de **Opciones Funcionales** (Functional Options) para una configuración limpia y mantenible.

```go
client := geoengine.New(
    "sk_test_123456",
    geoengine.WithIngestURL("http://localhost:8080"), // Para desarrollo local
    geoengine.WithTimeout(2 * time.Second),           // Timeout agresivo
)

```

### Opciones disponibles

| Opción | Descripción |
| --- | --- |
| `WithIngestURL(url string)` | Sobrescribe la URL de ingestión |
| `WithManagementURL(url string)` | Sobrescribe la URL de gestión |
| `WithTimeout(d time.Duration)` | Define el timeout del cliente HTTP |

---

## 🧪 Testing

Para correr las pruebas:

```bash
go test -v ./...

```

---

## 📄 Licencia

MIT © Geo-Engine Team
