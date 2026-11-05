# API (Go)

API REST bajo `/api/v1`. Go 1.27, chi, pgx y golang-migrate.

## Estructura

```text
apps/api/
├── cmd/api/              # ejecutable: `serve` (por defecto) y `migrate up|down|version`
├── internal/
│   ├── apperr/           # errores de aplicación independientes de HTTP
│   ├── httpx/            # respuestas JSON, sobre de error, decodificación, validación, paginación, middleware
│   ├── health/           # GET /api/v1/health
│   ├── server/           # router y servidor con apagado ordenado
│   └── platform/
│       ├── config/       # variables de entorno, validadas al arrancar
│       ├── database/     # pool de PostgreSQL, migraciones, traducción de errores SQL
│       └── logx/         # logger estructurado por request
└── migrations/           # esquema SQL versionado (embebido en el binario)
```

Cada módulo de negocio sigue `handler → service → repository`.

## Ejecutar en local

Con la base de datos levantada (`podman compose up -d db` o `docker compose up -d db` desde la raíz):

```bash
export DATABASE_URL="postgres://licoreria:<password>@localhost:5432/licoreria?sslmode=disable"
go run ./cmd/api              # aplica migraciones pendientes y sirve en :8080
go run ./cmd/api migrate version
go test ./...
```

## Variables de entorno

| Variable | Por defecto | Descripción |
| --- | --- | --- |
| `DATABASE_URL` | (obligatoria) | Cadena de conexión a PostgreSQL |
| `HTTP_ADDR` | `:8080` | Dirección de escucha |
| `APP_ENV` | `development` | `development` o `production` |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | `text` en desarrollo, `json` en producción | Formato de los logs |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:3000` | Orígenes permitidos, separados por coma |
| `SHUTDOWN_TIMEOUT_SECONDS` | `10` | Tiempo para terminar requests al apagar |

## Convenciones

**Errores.** Toda respuesta de error usa el mismo sobre:

```json
{
  "error": {
    "code": "validation_failed",
    "message": "Revisa los datos ingresados.",
    "requestId": "3f0c2a9e-8b1d-4c55-9d0e-2a71f3c4b6d1",
    "details": {
      "fields": [{ "field": "email", "code": "email", "message": "Ingresa un correo electrónico válido." }]
    }
  }
}
```

`code` es estable y en inglés; `message` está en español y se puede mostrar al usuario. Los errores internos nunca exponen detalles: el cliente recibe `internal_error` y el `requestId` para rastrearlo en los logs.

**Logs.** Una línea por request con `request_id`, método, ruta, status y duración. Texto legible en desarrollo y JSON en producción. Contraseñas, PIN, tokens y cédulas se ocultan automáticamente.

**Listas.** `{"data": [...], "meta": {"page", "pageSize", "total"}}`, con `?page=` y `?pageSize=` (máximo 100).
