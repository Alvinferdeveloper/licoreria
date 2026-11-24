# Manual de usuario

Añejo es el sistema de punto de venta, inventario y facturación de la licorería. Este manual explica el trabajo diario por rol. Lo que cada persona ve en el menú depende de los permisos de su rol.

| Rol | Qué hace |
| --- | --- |
| **Administrador** | Todo: usuarios, roles, configuración, respaldos y autorizaciones con PIN |
| **Cajero** | Vende, cobra, abre y cierra su propia caja, atiende clientes |
| **Bodeguero** | Recibe compras y ajusta inventario |
| **Consulta** | Solo lectura de reportes y datos |

## 1. Entrar al sistema

1. Escribe tu usuario o correo y tu contraseña.
2. La primera vez, activa tu cuenta eligiendo tu propia contraseña:
   - **Con correo:** abre el enlace de la invitación que te llegó y elige tu contraseña.
   - **Sin correo:** el administrador te entrega un código de acceso. En la pantalla de inicio toca **Activa tu cuenta**, escribe tu usuario, el código y tu contraseña nueva.
   El enlace y el código sirven una sola vez y vencen en 72 horas.
3. Si la olvidaste y tienes correo, usa **¿La olvidaste?** y sigue el enlace del correo. Si no tienes correo, pide al administrador un código de acceso nuevo.

Tras varios intentos fallidos la cuenta se bloquea unos minutos. La sesión se cierra sola si queda inactiva. Si el negocio activó un horario de uso, fuera de él solo entran quienes tengan permiso; el mensaje dice a qué hora se habilita de nuevo.

Si tienes correo, el sistema te avisa cuando tu contraseña cambia, cuando tu cuenta se bloquea y cuando entras fuera de horario. Si no fuiste tú, avisa al administrador de inmediato. Los administradores reciben aviso de cada cuenta bloqueada, de los intentos de entrar fuera de horario (uno por persona cada hora) y de cualquier fallo del respaldo automático.

El ícono de la luna o el sol, junto al nombre del sistema, cambia entre tema claro y oscuro.

## 2. Empezar el día

Para vender hacen falta dos cosas: la tasa del día y una caja abierta. Sin ellas el punto de venta se bloquea y explica qué falta.

**Tasa del día (Administrador):** en **Vender > Tipo de cambio**, registra cuántos córdobas vale un dólar. Cada venta guarda la tasa que usó, así que corregirla no altera ventas anteriores.

**Abrir caja (Cajero):** en **Vender > Mi caja**, elige tu caja, escribe el fondo inicial en córdobas y en dólares, y confirma.

## 3. Vender

Abre **Vender > Punto de venta**. Todo el flujo se puede hacer solo con teclado:

| Tecla | Acción |
| --- | --- |
| `F2` | Ir al buscador |
| `F4` | Cobrar |
| `F8` | Poner la venta en espera |
| `Enter` | Confirmar el cobro, o iniciar la venta siguiente |
| `Esc` | Volver del cobro al carrito sin perder nada |

**Paso a paso:**

1. Escanea el código de barras o busca por nombre, marca o SKU. La búsqueda ignora acentos.
   - **Con lector:** un lector USB o Bluetooth funciona sin configurar nada; el producto entra directo a la cuenta.
   - **Con la cámara:** pulsa el icono de código de barras del buscador. En teléfonos y tablets usa la cámara trasera y en laptops la webcam. Cada código leído suena y se agrega; la cámara sigue abierta para el siguiente. Pulsa **Listo** al terminar. El navegador solo permite la cámara en una dirección segura (https) o en el mismo equipo del servidor.
2. Ajusta las cantidades en la cuenta.
3. Si la factura va a nombre de alguien, elige el cliente o créalo ahí mismo. Por defecto es Consumidor final.
4. Marca **Cliente mayor de 18 años verificado**. Es obligatorio.
5. Pulsa `F4`, elige método y moneda, y registra lo entregado. Se pueden combinar pagos, por ejemplo dólares y córdobas.
6. El sistema muestra el vuelto, siempre en córdobas. Confirma con `Enter`.
7. Elige **Ticket**, **Factura PDF** o **Correo**, y pulsa `Enter` para la venta siguiente.

**Descuentos.** Aplica sobre el total de la venta, en porcentaje o monto. Sobre el límite definido en Configuración, pide el usuario y PIN de un administrador.

**Ventas en espera.** `F8` guarda el carrito para atender a otro cliente. Retómalo desde **En espera**.

**Tarjeta y transferencia** no pueden cobrar más que el saldo. Solo el efectivo genera vuelto. El sistema nunca pide ni guarda datos de la tarjeta:

- **Tarjeta:** cobra el monto en el datáfono del banco. El cliente inserta o acerca su tarjeta en el datáfono, que imprime un voucher. Escribe en el sistema el **número de autorización** del voucher.
- **Transferencia:** el cliente transfiere desde la app de su banco. Confirma que llegó y escribe el **número de referencia**.

Sin ese número no se registra el pago. Sale en el ticket, la factura y el detalle de la venta.

**Sin conexión.** Si el punto de venta pierde la conexión con el servidor, aparece una franja ámbar y puedes seguir vendiendo:

- Se busca y escanea con el catálogo guardado en el equipo, que se actualiza solo cada 10 minutos mientras hay conexión.
- Solo se cobra en efectivo, y los descuentos deben estar dentro de tu límite, porque no se puede validar el PIN de un administrador.
- Cada venta entrega un **comprobante provisional** (código `P-…`). Al volver la conexión, las ventas se envían solas en orden y reciben su número de factura; en la píldora del encabezado ves cuántas faltan y cuál factura recibió cada una.
- **No cierres la pestaña ni recargues la página** mientras no haya conexión: la app necesita el servidor para abrir de nuevo.
- Si alguna venta deja el stock en negativo, se registra igual (ya ocurrió) y los administradores reciben un aviso.
- No se puede cerrar la caja mientras haya ventas sin enviar. Las ventas sin conexión con más de 48 horas ya no se aceptan.

## 4. Anular una venta

En **Vender > Ventas**, abre la venta y pulsa **Anular**, con un motivo. Sin el permiso de anular, pide el PIN de un administrador. El stock vuelve solo y la factura queda marcada como anulada.

Solo se puede anular mientras el turno de esa venta siga abierto. El sistema no registra devoluciones de ventas de turnos ya cerrados.

## 5. Cerrar la caja

En **Vender > Mi caja** el desglose muestra lo que debería haber por moneda. Para movimientos ajenos a las ventas, como pagar hielo o un flete, usa **Entrada o salida de efectivo**, siempre con motivo.

Tarjeta y transferencia no están en la gaveta. Debajo del desglose, **Tarjeta y transferencias del turno** lista cada cobro con su autorización o referencia: compáralo con el cierre de lote del datáfono y con el estado de la cuenta.

Al final del turno, pulsa **Cerrar caja** y escribe lo que contaste en cada moneda. El sistema calcula la diferencia. Si no cuadra, queda registrada y se avisa por correo a los supervisores.

El historial de todas las cajas está en **Vender > Turnos de caja**.

## 6. Productos y catálogo

- **Inventario > Productos:** crear, editar, buscar y desactivar productos. Un producto puede tener presentaciones, como six-pack o caja, con su propio código y precio.
- **SKU automático:** al crear un producto, el SKU se sugiere solo con las siglas de la marca, el inicio del nombre y el volumen ("Flor de Caña 7 años 750 ml" → `FDC-7AN-750`). Si ya existe, se le agrega `-2`, `-3`… Puedes escribir el tuyo.
- **Importar:** carga masiva desde Excel o CSV. Descarga la plantilla, revisa la vista previa de errores y confirma. Solo entran las filas válidas.
- **Categorías y marcas:** los datos maestros, incluidas las unidades de medida.

Solo el Administrador ve los costos y cambia precios.

## 7. Compras e inventario

**Registrar una compra (Inventario > Compras):**

1. Crea la compra con proveedor, fecha, número de factura y productos. Queda en borrador. Para cerveza, vino y mezcladores anota la fecha de vencimiento de cada producto (columna **Vence**, opcional).
2. Al llegar la mercadería, pulsa **Recibir**. El stock sube y se actualiza el costo promedio.

Una compra recibida se puede anular mientras su stock siga disponible.

**Existencias y kardex (Inventario > Existencias y kardex):**

- **Pronóstico:** con lo vendido en los últimos 28 días (la última semana pesa más), calcula cuántos días le quedan a cada producto y cuánto pedir para cubrir 7, 14 o 30 días más su mínimo. La cantidad se redondea a la presentación más pequeña (por ejemplo, six-packs completos). Cada producto se marca como **Agotado**, **Crítico** (3 días o menos), **Pronto** o **Bajo mínimo**, con un mini gráfico de sus ventas diarias. Los productos se agrupan por el proveedor de su última compra, y **Crear borrador de compra** arma la compra con esas cantidades y el último costo para revisarla antes de recibirla.
- **Por pedir:** los productos en su punto de reorden, agrupados por proveedor, con un botón para registrar la compra.
- **Vencimientos:** lo vencido y lo que vence en 30, 60 o 90 días, con cuántas unidades quedan de cada compra y su valor. Se asume que primero sale lo más antiguo, así que el stock actual pertenece a las compras más recientes. Lo vencido se da de baja como merma con **Dar de baja** (dos clics para confirmar). Los administradores reciben un correo una sola vez por cada lote que entra en los últimos 30 días antes de vencer.
- **Kardex:** cada entrada y salida con su saldo, responsable y origen. No se puede editar: una corrección siempre es un movimiento nuevo.
- **Registrar ajuste:** un conteo físico o una merma, con motivo obligatorio.

Cuando un producto cruza el mínimo, se envía un correo a quienes compran.

## 8. Panel y reportes

**Negocio > Panel** muestra las ventas del día, ticket promedio, utilidad, productos bajo mínimo y cinco gráficos: ventas por día, productos más vendidos, ventas por categoría, cobros por método y horas pico. Cada gráfico tiene **Ver tabla**. El cajero solo ve sus propias ventas.

**Negocio > Reportes** ofrece 11 reportes con filtros, vista previa y exportación a Excel y PDF:

- **Ventas:** por período, por producto y categoría, por cajero y cierres de caja, por caja y moneda, anuladas, utilidad y margen por producto.
- **Inventario:** inventario valorizado, productos bajo mínimo, kardex por producto, compras por proveedor.
- **Control:** bitácora de auditoría.

**Negocio > Riesgos de caja** compara a cada cajero con el resto de la tienda en los últimos 7, 30 o 90 días: anulaciones, descuentos cerca del límite o autorizados con PIN, cierres con faltante, PIN rechazados y entradas fuera de horario. Quien lo hace notablemente más que los demás se marca **Revisar** (una señal) o **Revisar ya** (dos o más). Comparar con la tienda evita castigar a quien simplemente vende más. Debajo están los últimos hechos a revisar, con factura, motivo o caja. Lo ven el administrador y el rol de consulta (permiso de bitácora). Una marca no acusa a nadie: indica qué revisar primero.

## 9. Clientes y proveedores

En **Personas** se administran clientes y proveedores. Consumidor final es el cliente por defecto y no se puede modificar. Los registros no se borran: se desactivan, para conservar el historial.

## 10. Usuarios y roles (Administrador)

- **Personas > Usuarios:** al crear un usuario con correo, le llega una invitación para elegir su contraseña; sin correo, el sistema muestra una sola vez un código de acceso para entregárselo en persona. Mientras no la elija, aparece como **Invitación pendiente**. Desde la fila se reenvía la invitación (avión de papel), se genera un código nuevo (llave), útil también si alguien sin correo olvida su contraseña, y se desactiva una cuenta.
- **Sistema > Roles y permisos:** marca qué puede hacer cada rol. El cambio aplica de inmediato.
- **PIN de autorización:** quien autoriza descuentos o anulaciones define su PIN en **Cambiar contraseña**, debajo de su nombre.

## 11. Configuración (Administrador)

En **Sistema > Configuración** se definen:

- Nombre, RUC, dirección, teléfono y correo, que salen en facturas y tickets.
- Logo de la factura, en PNG o JPG de hasta 2 MB.
- IVA para productos nuevos. No cambia productos existentes ni facturas emitidas.
- Porcentaje de descuento permitido sin autorización.
- Formato de impresión por defecto y nota al pie.
- **Horario de uso:** qué días y entre qué horas se puede usar el sistema, con una tolerancia (30 minutos por defecto) antes de abrir y después de cerrar. Si el cierre es igual o anterior a la apertura, el turno termina al día siguiente. Fuera del horario, quien no tenga el permiso **Fuera de horario** no puede entrar ni registrar ventas nuevas, pero sí cerrar su caja. El Administrador siempre tiene ese permiso, y puede darlo a otros roles. Cada vez que alguien entra fuera de horario con permiso, queda en la bitácora.
- **Extensiones de hoy:** para un inventario, un evento o un cierre tarde, extiende el horario hasta una hora, para todos o para una persona. La extensión vence sola, se puede terminar antes y queda en la bitácora.

La numeración se lleva por caja. Cada una tiene su serie, que se administra en **Turnos de caja** y se bloquea tras la primera factura.

## 12. Bitácora, respaldos y estado del sistema (Administrador)

**Sistema > Bitácora** registra quién hizo qué, cuándo y desde dónde. Nadie puede modificarla.

**Sistema > Respaldos** crea, descarga, verifica y restaura copias de la base de datos. La guía completa está en [respaldos.md](respaldos.md).

**Sistema > Estado del sistema** revisa en vivo, cada 15 segundos, lo que el negocio necesita para vender y no perder datos. Cada tarjeta dice **Bien**, **Revisar** o **Falla**:

- **Base de datos:** si responde y en cuánto tiempo, su tamaño y las conexiones en uso.
- **Respaldos:** cuándo fue el último, a qué hora corre el automático y si hay copia externa. Avisa si pasó más de un día sin respaldo o si el último intento falló.
- **Tasa del día:** si falta, el punto de venta no puede cobrar; la tarjeta trae el enlace para registrarla.
- **Correo:** enviados, en cola y fallidos en las últimas 24 horas.
- **Disco del servidor:** espacio libre; avisa por debajo del 15 % porque los respaldos podrían fallar.
- **Imágenes y logo:** si la conexión con Supabase Storage está configurada.
- **Servidor y actividad:** tiempo encendido, memoria, sesiones abiertas, cajas abiertas y ventas sin conexión sincronizadas.
- **Tráfico de la API:** peticiones por hora de las últimas 24 horas y los últimos errores del servidor con su código de solicitud, que sirve para encontrar el detalle en los registros.

## Mensajes frecuentes

| Mensaje | Qué hacer |
| --- | --- |
| Falta la tasa del día | Un administrador la registra en Tipo de cambio |
| Sin caja abierta | Abre tu turno en Mi caja |
| No queda más stock | Revisa el kardex o registra la compra pendiente |
| Este descuento supera tu límite | Pide el usuario y PIN de un administrador |
| El turno de esa venta ya cerró | La venta ya no se puede anular |
| Estás fuera del horario de uso / El horario de ventas terminó | Cierra tu caja; si hay que seguir, el administrador extiende el horario en Configuración |
| El almacenamiento de imágenes no está configurado | Faltan las credenciales de Supabase en el servidor |
