# Añejo: guía de instalación

Añejo es un sistema de punto de venta e inventario para licorerías. Todo corre en contenedores, así que no hace falta instalar Go, Node.js ni PostgreSQL: solo un motor de contenedores.

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

## Más documentación

- [Manual de usuario](docs/manual-usuario.md)
- [Diagrama entidad-relación](docs/diagrama-er.md) (también en [PDF](docs/diagrama-er.pdf))
- [Instalación en un servidor público con HTTPS](docs/instalacion.md)
- [Respaldo y recuperación](docs/respaldos.md)
