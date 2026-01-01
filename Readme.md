# 📦 Geo-Engine Go SDK

Cliente oficial en **Go** para interactuar con la plataforma **Geo-Engine**.  
Diseñado para **alto rendimiento**, simplicidad y fácil integración en servicios de backend.

Permite:
- 📍 Enviar ubicaciones en tiempo real
- 🚚 Identificar dispositivos (vehículos, usuarios, activos)
- ⚡ Integrarse fácilmente en microservicios y APIs en Go

---

## 🚀 Instalación

Usa `go get` para instalar el SDK:

```bash
go get github.com/tu-usuario/geo-engine-go
````

---

## ⚡ Uso Rápido

Envía la ubicación de un dispositivo en pocos pasos:

```go
package main

import (
    "log"

    "github.com/tu-usuario/geo-engine-go"
)

func main() {
    // 1. Inicializar cliente
    // Por defecto conecta a la nube de producción
    client := geoengine.New("sk_live_123456")

    // 2. Enviar ubicación
    err := client.SendLocation("camion-01", 19.4326, -99.1332)
    if err != nil {
        log.Fatalf("Error enviando datos: %v", err)
    }

    log.Println("✅ Ubicación enviada correctamente")
}
```

---

## 🔧 Configuración Avanzada

Puedes personalizar el cliente usando **opciones funcionales**, por ejemplo para conectar a un entorno local o ajustar timeouts:

```go
client := geoengine.New(
    "sk_test_123456",
    geoengine.WithIngestURL("http://localhost:8080"),
    geoengine.WithTimeout(5 * time.Second),
)
```

### Opciones disponibles

| Opción                         | Descripción                          |
| ------------------------------ | ------------------------------------ |
| `WithIngestURL(url string)`    | Cambia el endpoint de ingestión      |
| `WithTimeout(d time.Duration)` | Define el timeout de las solicitudes |

---

## 🔐 Autenticación

El SDK utiliza **API Keys** para autenticación.

* Producción: `sk_live_...`
* Pruebas: `sk_test_...`

👉 Mantén tus claves seguras y **no las incluyas en el código fuente**.

---

## 🧪 Testing

Para correr las pruebas:

```bash
go test ./...
```

---

## 📄 Licencia

Este proyecto está bajo la licencia **MIT**.
Consulta el archivo [LICENSE](LICENSE) para más detalles.

---

## 🤝 Contribuciones

¡Las contribuciones son bienvenidas!

1. Haz un fork del proyecto
2. Crea una rama (`feature/nueva-funcionalidad`)
3. Envía un Pull Request 🚀

```


