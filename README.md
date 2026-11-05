# Añejo · Sistema de inventario y facturación para licorería

Punto de venta, inventario (kardex), compras, caja por turnos y facturación para una licorería en Nicaragua, con cobro en córdobas y dólares.

> Proyecto de examen de grado. En construcción.

## Stack

| Capa | Tecnología |
| --- | --- |
| Frontend | Next.js (App Router), TypeScript, Tailwind CSS, shadcn/ui |
| Backend | Go (API REST `/api/v1`, Chi) |
| Base de datos | PostgreSQL 16 |
| Archivos | Supabase Storage |
| Correo | Resend / SMTP |
| Contenedores | Compose (Podman o Docker) |

## Estructura

```text
licoreria/
├── apps/
│   ├── web/        # Frontend Next.js
│   └── api/        # API REST en Go (ver apps/api/README.md)
├── compose.yaml    # base de datos y API (Podman o Docker)
├── deploy/         # proxy y scripts de respaldo
├── docs/           # requisitos, diagrama ER, manual de usuario
└── .github/        # CI
```

## Requisitos de desarrollo

- Node.js 22+ y pnpm 10+
- Go 1.27+
- Podman 5+ (`podman compose`) o Docker (`docker compose`)

## Puesta en marcha

Copia `.env.example` a `.env` y define `DB_PASSWORD`.

**Todo en contenedores** (base de datos y API en http://localhost:8080):

```bash
podman compose up -d --build   # o: docker compose up -d --build
curl http://localhost:8080/api/v1/health
```

La documentación interactiva de la API queda en http://localhost:8080/api/v1/docs (contrato en `apps/api/openapi.yaml`).

**Desarrollo** (base de datos en contenedor, API y web en local):

```bash
podman compose up -d db        # o: docker compose up -d db
export DATABASE_URL="postgres://licoreria:<DB_PASSWORD>@localhost:5432/licoreria?sslmode=disable"
pnpm dev:api                   # API en :8080; aplica las migraciones al arrancar
pnpm install && pnpm dev:web   # web en http://localhost:3000
```

## Calidad

```bash
pnpm test:api      # pruebas de la API
pnpm test          # pruebas del frontend
pnpm typecheck
pnpm lint
```

El CI (GitHub Actions) ejecuta formato, `go vet`, pruebas, el ciclo completo de migraciones contra PostgreSQL y el build del frontend.
