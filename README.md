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
| Despliegue | Docker Compose |

## Estructura

```text
licoreria/
├── apps/
│   ├── web/        # Frontend Next.js
│   └── api/        # API REST en Go
├── deploy/         # docker-compose, proxy, scripts de respaldo
├── docs/           # requisitos, diagrama ER, manual de usuario
└── .github/        # CI
```

## Requisitos de desarrollo

- Node.js 22+ y pnpm 10+
- Go 1.23+
- Docker y Docker Compose

## Puesta en marcha

```bash
pnpm install
pnpm dev:web        # http://localhost:3000/pos
pnpm test           # pruebas del dominio (dinero, carrito, cobro)
pnpm typecheck
pnpm lint
```

La API en Go y Docker Compose se agregan en la fase 0.
