# Respaldo y recuperación

Añejo guarda copias completas de la base de datos y las restaura desde la pantalla **Sistema > Respaldos**, disponible solo para el Administrador.

## Qué se respalda

| Dato | Dónde vive | Cómo se protege |
| --- | --- | --- |
| Base de datos (ventas, inventario, clientes, usuarios, bitácora) | PostgreSQL | Archivo `pg_dump` comprimido con suma SHA-256 |
| Imágenes de productos | Supabase Storage | Fuera del servidor. Supabase las conserva aparte; para una copia propia, descarga el bucket `productos`. La base solo guarda sus URL. |
| Configuración del servidor (`.env`) | El servidor | No entra en el respaldo. Guárdala en un gestor de contraseñas. |

Un respaldo contiene hashes de contraseñas y datos de clientes: trátalo como información confidencial y nunca lo subas a un bucket público.

## Tipos de respaldo

- **Manual:** botón **Crear respaldo**.
- **Automático:** cada día a las 2:00 a. m. (hora de Managua). Si el servidor estaba apagado, se genera al encenderlo. Conserva 7 diarios y 4 semanales; se ajusta con la clave `backups` de la tabla `settings`.
- **Antes de restaurar:** se crea solo, justo antes de cada restauración, y no se borra de forma automática.

Los archivos quedan en el volumen `backup-data`. Para tener una copia fuera del servidor hay dos opciones:

1. Descargarlos desde la pantalla.
2. Definir `SUPABASE_BACKUP_BUCKET` con un bucket **privado**. Cada respaldo se copia ahí y, si el archivo local se pierde, se recupera de ese bucket.

## Simulacro

Un respaldo que nunca se probó no cuenta. **Verificar** carga el archivo en una base temporal, cuenta las tablas clave y borra esa base, sin tocar los datos reales. También se puede ejecutar sobre el último respaldo desde la terminal:

```sh
podman compose exec api api verify-backup
docker compose exec api api verify-backup
```

## Restaurar

1. En **Respaldos**, elige el archivo y pulsa **Restaurar**.
2. Escribe tu contraseña y la palabra `RESTAURAR`.

El sistema entonces:

1. Rechaza las demás peticiones (modo mantenimiento) y espera a que terminen las que están en curso.
2. Guarda una copia del estado actual.
3. Carga el respaldo en una sola transacción: si algo falla, los datos quedan como estaban.
4. Aplica las migraciones pendientes, si el respaldo es de una versión anterior.
5. Cierra todas las sesiones y cancela los correos pendientes.

La bitácora y el historial de respaldos **no retroceden**: una restauración no puede ocultar lo que pasó después de la fecha del respaldo.

Los números de factura vuelven al punto del respaldo. Si se emitieron facturas después de esa fecha, revisa la numeración antes de seguir vendiendo.

## Objetivos de recuperación

- **Pérdida máxima de datos (RPO):** 24 horas con el respaldo automático. Crea uno manual antes de cualquier operación delicada.
- **Tiempo de recuperación (RTO):** minutos con la base actual; la restauración de un archivo de 150 KB tarda alrededor de un segundo.

## Si se pierde el servidor

Sin la aplicación no hay pantalla de restauración. En una máquina nueva, con un archivo `.dump` descargado:

```sh
# Levanta solo la base de datos
podman compose up -d db        # o: docker compose up -d db

# Carga el respaldo
podman compose cp anejo.dump db:/tmp/anejo.dump        # o: docker compose cp ...
podman compose exec db pg_restore -U licoreria -d licoreria --no-owner --no-privileges /tmp/anejo.dump

# Levanta el resto; la API aplica las migraciones pendientes
podman compose up -d           # o: docker compose up -d
```

El historial de respaldos aparece vacío en la nueva instalación, porque describe los archivos del servidor anterior.
