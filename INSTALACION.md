# Añejo: guía de instalación

Añejo es un sistema de punto de venta e inventario para licorerías. La forma recomendada es con contenedores: no hace falta instalar Go, Node.js ni PostgreSQL, solo un motor de contenedores. Si prefieres instalar cada pieza a mano, sigue la sección [Instalación sin Docker](#instalación-sin-docker-manual) al final.

## Requisitos

- **Docker Desktop** (Windows, macOS o Linux): <https://www.docker.com/products/docker-desktop/>. Ábrelo y espera a que diga que está en ejecución antes de seguir.
  - También funciona con **Podman Desktop**. Si usas Podman, cambia `docker` por `podman` en cada comando.
- Unos 4 GB de RAM libres y 5 GB de disco.
- Internet la primera vez: se descargan las imágenes y las dependencias.
- **Git**, solo para la opción B.

## 1. Obtener el proyecto

Elige una de las dos opciones.

### Opción A: desde el archivo comprimido

1. Descomprime el archivo en una carpeta, por ejemplo `C:\anejo`.
2. Abre una terminal **dentro de esa carpeta**, la que contiene `compose.yaml`. En Windows: clic derecho dentro de la carpeta → **Abrir en Terminal**.

### Opción B: desde GitHub

```sh
git clone https://github.com/Alvinferdeveloper/licoreria.git anejo
cd anejo
```

## 2. Crear la configuración

Copia el archivo de ejemplo:

```sh
# Windows (PowerShell o CMD)
copy .env.example .env

# macOS o Linux
cp .env.example .env
```

Abre `.env` con cualquier editor de texto (por ejemplo, el Bloc de notas) y cambia estas dos líneas:

```sh
DB_PASSWORD=una-contraseña-larga-para-la-base
JWT_SECRET=escribe-aqui-un-texto-aleatorio-de-al-menos-32-caracteres
```

`JWT_SECRET` debe tener 32 caracteres o más. Cualquier texto largo sirve para probar el sistema.

No hace falta tocar nada más. Los correos que envía el sistema llegan a un buzón de prueba incluido (Mailpit), no a correos reales. La subida de imágenes de productos se activa solo si se configura Supabase; sin eso, el resto del sistema funciona completo.

## 3. Levantar el sistema

```sh
docker compose up -d --build
```

La primera vez tarda entre 5 y 10 minutos. Las siguientes, segundos. La base de datos se crea y se prepara sola.

Para comprobar que todo está arriba:

```sh
docker compose ps
```

Los servicios `db`, `api`, `web` y `mailpit` deben aparecer como *running* o *healthy*.

## 4. Crear el administrador

```sh
docker compose exec api api create-admin -name "Administrador" -email admin@anejo.local -username admin
```

El comando muestra una **contraseña temporal una sola vez**. Cópiala: el sistema pide cambiarla en el primer inicio de sesión.

## 5. Cargar los datos de demostración (recomendado)

```sh
docker compose exec api api seed-demo
```

Carga en unos 15 segundos un negocio completo:
- 203 productos con sus presentaciones (six-pack, caja), clientes y proveedores;
- 90 días de ventas, turnos de caja, compras, vencimientos y mermas;
- la bitácora de esos días.

Así el panel, los reportes, el pronóstico y los riesgos de caja tienen datos que mostrar.

Para volver a cargarla desde cero con la fecha de hoy:

```sh
docker compose exec api api seed-demo -reset
```

## 6. Entrar

| Qué | Dirección |
| --- | --- |
| **Aplicación** | <http://localhost:3000> |
| Correos de prueba (Mailpit) | <http://localhost:8025> |
| Documentación de la API (Swagger) | <http://localhost:8080/api/v1/docs> |

Entra con el usuario `admin` y la contraseña temporal del paso 4.

### Primeros pasos sugeridos

- **Define tu PIN** en **Cambiar contraseña**. Se usa para autorizar descuentos mayores al 5 % y anulaciones hechas por un cajero.
- **Crea un cajero** en **Usuarios** con un correo cualquiera. La invitación llega a Mailpit con un enlace para elegir la contraseña.
- **Vende**: en **Mi caja** abre un turno y luego entra a **Punto de venta**. Se puede buscar por nombre o escribir un código de barras, por ejemplo `7431002058943`.
- Los cajeros de la demo (`ana`, `carlos`, `yader`) no tienen contraseña conocida; para usarlos, reenvía su acceso desde **Usuarios**.

## Detener y volver a iniciar

```sh
# Detener (los datos se conservan)
docker compose down

# Volver a iniciar
docker compose up -d
```

`docker compose down -v` borra también la base de datos y los respaldos.

## Problemas comunes

| Síntoma | Solución |
| --- | --- |
| `Cannot connect to the Docker daemon` o el comando no responde | Abre Docker Desktop y espera a que esté en ejecución |
| `Set DB_PASSWORD in .env` | Falta el archivo `.env`; repite el paso 2 en la carpeta del proyecto |
| `no configuration file provided` | La terminal no está en la carpeta que contiene `compose.yaml` |
| `port is already allocated` | Otro programa usa ese puerto. En `.env` cambia `WEB_PORT=3000` por otro, por ejemplo `3002`, y también `APP_BASE_URL` y `CORS_ALLOWED_ORIGINS` a `http://localhost:3002`. Luego repite el paso 3 |
| La aplicación no carga justo después de levantar | La primera vez la API tarda unos segundos en preparar la base; espera y recarga |
| Con Podman en Windows: `cannot connect to Podman` | Inicia la máquina virtual con `podman machine start` |
| Sin Docker: la API muestra `DATABASE_URL is required` o `JWT_SECRET must be at least 32 bytes` | Las variables se definen en la misma terminal donde corres `go run`; si abriste otra terminal, defínelas de nuevo |
| Sin Docker: `backup failed` con `pg_dump: executable file not found` | Agrega la carpeta `bin` de PostgreSQL 16 al `PATH` y abre una terminal nueva. El resto del sistema funciona igual |
| Sin Docker: `permission denied to create extension` | Crea las extensiones como el usuario `postgres`, como en el paso 1 |

## Instalación sin Docker (manual)

Corre las tres piezas directamente en tu computadora: PostgreSQL, la API en Go y la web en Next.js.

### Requisitos

| Programa | Versión | Descarga |
| --- | --- | --- |
| PostgreSQL | **16** | <https://www.postgresql.org/download/> |
| Go | 1.27 o superior | <https://go.dev/dl/> |
| Node.js | 22 o superior | <https://nodejs.org/> |
| pnpm | 10 | En una terminal: `npm install -g pnpm@10` |

Al instalar PostgreSQL en Windows, anota la contraseña del usuario `postgres`. Para que funcionen los respaldos, la carpeta `bin` de PostgreSQL (por ejemplo `C:\Program Files\PostgreSQL\16\bin`) debe estar en el `PATH`, porque la API usa `pg_dump`, `pg_restore` y `psql`.

### 1. Crear la base de datos

Abre una terminal y entra a PostgreSQL como administrador:

```sh
psql -U postgres
```

Dentro de `psql`, crea el usuario y la base de datos (cambia la contraseña por una tuya):

```sql
CREATE USER licoreria WITH PASSWORD 'UnaClaveSegura2026';
CREATE DATABASE licoreria OWNER licoreria;
\c licoreria
CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS unaccent;
\q
```

Las tablas no se crean a mano: la API aplica las migraciones sola al arrancar. Las extensiones se crean aquí porque requieren permisos de administrador.

### 2. Arrancar la API

Abre una terminal en la carpeta `apps/api` del proyecto y define las variables. La API no lee el archivo `.env`, así que van en la terminal.

**Windows (PowerShell):**

```powershell
$env:DATABASE_URL = "postgres://licoreria:UnaClaveSegura2026@localhost:5432/licoreria?sslmode=disable"
$env:JWT_SECRET   = "escribe-aqui-un-texto-aleatorio-de-al-menos-32-caracteres"
$env:BACKUP_DIR   = "./backups"
go run ./cmd/api
```

**macOS o Linux:**

```sh
export DATABASE_URL="postgres://licoreria:UnaClaveSegura2026@localhost:5432/licoreria?sslmode=disable"
export JWT_SECRET="escribe-aqui-un-texto-aleatorio-de-al-menos-32-caracteres"
export BACKUP_DIR="./backups"
go run ./cmd/api
```

La primera vez descarga las dependencias y tarda un poco. Está lista cuando muestra `api listening` con `addr=:8080`. Deja esa terminal abierta.

### 3. Crear el administrador y cargar la demo

En **otra** terminal, también en `apps/api` y con las mismas dos variables (`DATABASE_URL` y `JWT_SECRET`) definidas:

```sh
go run ./cmd/api create-admin -name "Administrador" -email admin@anejo.local -username admin
go run ./cmd/api seed-demo
```

El primero muestra la contraseña temporal del administrador una sola vez. El segundo carga los datos de demostración (opcional).

### 4. Arrancar la web

En una tercera terminal, en la carpeta **raíz** del proyecto (la que contiene `package.json`):

```sh
pnpm install
pnpm dev:web
```

Entra a <http://localhost:3000> con el usuario `admin` y la contraseña temporal.

La web le pasa las peticiones a la API en `http://localhost:8080`. Si la API corre en otro puerto, define antes `API_ORIGIN` (por ejemplo, en PowerShell: `$env:API_ORIGIN = "http://localhost:8090"`) y, en la API, `HTTP_ADDR=":8090"`. Si el puerto 3000 está ocupado, define `PORT` antes de `pnpm dev:web` (por ejemplo, `$env:PORT = "3005"`).

### Correo sin Docker

Sin Docker no está el buzón de prueba Mailpit, así que las invitaciones y alertas no llegan a ningún lado. El sistema funciona igual:

- Crea los usuarios **sin correo**: el sistema muestra un código de acceso para activarlos desde **Activa tu cuenta**.
- O instala Mailpit (<https://mailpit.axllent.org/>, un solo ejecutable); la API le envía los correos en `localhost:1025` y se ven en <http://localhost:8025>.
- O configura un correo real con `RESEND_API_KEY` o las variables `SMTP_*` (ver `.env.example`).

### Alternativa: crear la base con un script

Si prefieres crear todas las tablas con SQL en lugar de dejar que la API aplique las migraciones, ejecuta `base-de-datos.sql` (incluido en la entrega) sobre la base vacía, después del paso 1:

```sh
psql -U licoreria -d licoreria -f base-de-datos.sql
```

Crea la estructura completa, los datos base y la versión de las migraciones, así que la API la reconoce y no vuelve a crear nada.

## Más documentación

- [Manual de usuario](docs/manual-usuario.md)
- [Diagrama entidad-relación](docs/diagrama-er.md) (también en [PDF](docs/diagrama-er.pdf))
- [Instalación en un servidor público con HTTPS](docs/instalacion.md)
- [Respaldo y recuperación](docs/respaldos.md)
