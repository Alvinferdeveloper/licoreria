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

## Primer administrador

```bash
go run ./cmd/api create-admin -name "Lucía Barberena" -email lucia@example.com -username lucia
# en contenedor: podman compose exec api api create-admin ...  (o docker compose exec ...)
```

Imprime una contraseña temporal una sola vez; el sistema obliga a cambiarla en el primer inicio de sesión.

## Endpoints

| Método y ruta | Permiso | Descripción |
| --- | --- | --- |
| `POST /auth/login` | público | Usuario o correo y contraseña. Devuelve el token de acceso y fija la cookie de refresco |
| `POST /auth/refresh` | cookie | Rota la cookie de refresco y emite un token nuevo |
| `POST /auth/logout` | cookie | Cierra la sesión |
| `POST /auth/forgot-password` | público | Envía un enlace de un solo uso (30 min) |
| `POST /auth/reset-password` | público | Define la contraseña nueva con el enlace |
| `GET /auth/me` | sesión | Usuario actual, rol y permisos |
| `POST /auth/change-password` | sesión | Cambio de contraseña (obligatorio con contraseña temporal) |
| `GET/POST /users`, `GET/PUT /users/{id}` | `users:*` | Usuarios; al crear se envía la contraseña temporal por correo |
| `PATCH /users/{id}/status` | `users:delete` | Activar o desactivar; desactivar cierra sus sesiones |
| `POST /users/{id}/resend-access` | `users:update` | Nueva contraseña temporal |
| `GET /roles`, `GET /roles/{id}`, `GET /permissions` | `roles:view` | Roles y catálogo de permisos |
| `PUT /roles/{id}/permissions` | `roles:update` | Reemplaza los permisos de un rol (el de Administrador no se edita) |

Seguridad: contraseñas con Argon2id; token de acceso JWT de 15 min y cookie de refresco `HttpOnly` rotativa, cuya reutilización cierra todas las sesiones del usuario; bloqueo tras 5 intentos fallidos y límite de intentos por IP; los permisos se verifican en cada request contra la base de datos.

En desarrollo los correos llegan a Mailpit: http://localhost:8025.

## Variables de entorno

| Variable | Por defecto | Descripción |
| --- | --- | --- |
| `DATABASE_URL` | (obligatoria) | Cadena de conexión a PostgreSQL |
| `HTTP_ADDR` | `:8080` | Dirección de escucha |
| `APP_ENV` | `development` | `development` o `production` |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | `text` en desarrollo, `json` en producción | Formato de los logs |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:3000` | Orígenes permitidos, separados por coma |
| `SHUTDOWN_TIMEOUT` | `10s` | Tiempo para terminar requests al apagar |
| `JWT_SECRET` | (obligatoria, 32+ bytes) | Firma de los tokens de acceso |
| `ACCESS_TOKEN_TTL` / `REFRESH_TOKEN_TTL` / `RESET_TOKEN_TTL` | `15m` / `168h` / `30m` | Vigencias |
| `COOKIE_SECURE` | `true` en producción | Cookie de refresco solo por HTTPS |
| `APP_BASE_URL` | `http://localhost:3000` | URL de la web para los enlaces de los correos |
| `SMTP_HOST` / `SMTP_PORT` | `localhost` / `1025` | Servidor de correo (Mailpit en desarrollo) |
| `SMTP_USERNAME` / `SMTP_PASSWORD` / `MAIL_FROM` | | Credenciales y remitente |
| `TEST_DATABASE_URL` | | Base separada para las pruebas de integración |
| `SUPABASE_URL` / `SUPABASE_SERVICE_KEY` | | Supabase Storage para imágenes de productos; sin ellas la subida de imágenes responde `storage_not_configured` |
| `SUPABASE_BUCKET` | `productos` | Bucket público donde se guardan las imágenes |

## Datos de demostración

```bash
go run ./cmd/api seed-demo   # 200+ productos con stock inicial, clientes y proveedores (requiere un administrador)
```

## Imágenes de productos (Supabase)

1. En Supabase, crea un bucket **público** llamado `productos` (Storage > New bucket > Public).
2. En Project Settings > API copia la URL del proyecto y la clave `service_role`.
3. Agrégalas a `.env` como `SUPABASE_URL` y `SUPABASE_SERVICE_KEY` y reinicia la API.

La clave `service_role` solo vive en el servidor; el navegador sube la imagen a la API (`POST /products/{id}/image`, JPG/PNG/WebP hasta 5 MB) y la API la envía a Supabase.

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
