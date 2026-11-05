# Instalación y despliegue

Todo corre en contenedores, con **Podman** o **Docker**, sin cambiar nada. Cada comando se muestra en ambas variantes.

Solo necesitas Git y uno de los dos motores. La primera construcción descarga imágenes y dependencias, así que requiere internet y tarda varios minutos.

## Instalación local

**1. Clona el repositorio y crea la configuración:**

```sh
git clone <url-del-repositorio> licoreria
cd licoreria
cp .env.example .env
```

En `.env`, define al menos estas dos variables. El secreto debe tener 32 bytes o más; puedes generarlo con `openssl rand -base64 48`.

```sh
DB_PASSWORD=una-contraseña-larga
JWT_SECRET=un-secreto-largo-y-aleatorio
```

**2. Levanta todo con un comando:**

```sh
podman compose up -d --build
docker compose up -d --build
```

Las migraciones se aplican solas al arrancar la API.

**3. Crea el primer administrador:**

```sh
podman compose exec api api create-admin -name "Tu nombre" -email tu@correo.com -username admin
docker compose exec api api create-admin -name "Tu nombre" -email tu@correo.com -username admin
```

El comando muestra una contraseña temporal una sola vez. Se cambia en el primer inicio de sesión.

**4. Carga los datos de demostración (opcional):**

```sh
podman compose exec api api seed-demo
docker compose exec api api seed-demo
```

Carga 203 productos, clientes, proveedores, los datos del negocio y 90 días de ventas, turnos y compras hasta ayer. Solo funciona con el catálogo vacío. No la uses en un negocio real.

Crea además los cajeros `ana`, `carlos` y `yader`, sin contraseña conocida. Para usarlos, reenvía su acceso desde **Usuarios**.

**5. Entra:**

| Servicio | Dirección |
| --- | --- |
| Aplicación | http://localhost:3000 |
| Documentación de la API | http://localhost:8080/api/v1/docs |
| Correos de prueba (Mailpit) | http://localhost:8025 |

Si un puerto está ocupado, cámbialo en `.env` con `WEB_PORT`, `API_PORT`, `DB_PORT` o `MAILPIT_UI_PORT`.

Los puertos solo escuchan en la propia máquina. Para usar una tablet en la misma red, define `BIND_ADDR=0.0.0.0` y entra con la IP del equipo.

## Servidor público

Necesitas un servidor Linux con los puertos 80 y 443 abiertos y un dominio que apunte a él. Agrega esto a `.env`:

```sh
DOMAIN=tu-dominio.com
APP_ENV=production
APP_BASE_URL=https://tu-dominio.com
CORS_ALLOWED_ORIGINS=https://tu-dominio.com

# Correo real, por ejemplo Resend
SMTP_HOST=smtp.resend.com
SMTP_PORT=587
SMTP_USERNAME=resend
SMTP_PASSWORD=tu-clave
MAIL_FROM=Tu negocio <ventas@tu-dominio.com>

# Imágenes y logo: bucket público de Supabase
SUPABASE_URL=https://tu-proyecto.supabase.co
SUPABASE_SERVICE_KEY=tu-clave-service-role
SUPABASE_BUCKET=productos
```

Luego levanta el sistema con el proxy, que obtiene el certificado HTTPS solo:

```sh
podman compose --profile production up -d --build
docker compose --profile production up -d --build
```

Sigue con los pasos 3 y 4 de la instalación local.

La clave `service_role` de Supabase da acceso total al almacenamiento: va solo en el `.env` del servidor. Guarda ese archivo en un gestor de contraseñas, porque los respaldos no lo incluyen.

## Operación

```sh
# Ver el estado y los registros
podman compose ps
podman compose logs -f api

# Actualizar a una versión nueva
git pull
podman compose up -d --build

# Detener sin perder datos
podman compose down
```

Con Docker son los mismos comandos, cambiando `podman` por `docker`. `down -v` borra también los volúmenes, es decir, **toda la base de datos y los respaldos**.

Para el respaldo y la restauración, sigue [respaldos.md](respaldos.md).

## Solución de problemas

| Síntoma | Causa y solución |
| --- | --- |
| `Set DB_PASSWORD in .env` | Falta el archivo `.env` o esa variable |
| La API no arranca y `logs` muestra un error de configuración | El mensaje indica qué variable corregir |
| `podman` no conecta en Windows o macOS | Inicia la máquina virtual: `podman machine start` |
| Un puerto ya está en uso | Cámbialo en `.env` y repite `up -d` |
| Subir una imagen responde que el almacenamiento no está configurado | Faltan las variables de Supabase |
| En producción no se puede iniciar sesión | La cookie exige HTTPS: entra por el dominio, no por la IP |
