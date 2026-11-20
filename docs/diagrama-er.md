# Diagrama entidad-relación

Base de datos PostgreSQL 16 con 33 tablas, creadas por las migraciones de `apps/api/migrations`. El modelo se divide por dominio para que cada diagrama se pueda leer; las tablas que se repiten son el punto de unión entre dominios. Solo se muestran las columnas clave.

Decisiones de diseño:

- **El stock es un resultado.** `products.stock` lo actualiza un trigger al insertar en `inventory_movements`, y ese kardex no admite `UPDATE` ni `DELETE`.
- **La venta guarda una foto.** `sales` y `sale_items` copian nombre, precio, costo e IVA del momento, para que cambiar el catálogo no altere facturas pasadas.
- **Todo monto en córdobas.** Cada pago conserva su moneda original, la tasa usada y el equivalente en `amount_nio`.
- **La bitácora es de solo agregar.** Un trigger impide modificar o borrar `audit_log`.

## Seguridad y usuarios

```mermaid
erDiagram
    roles ||--o{ users : "asigna"
    roles ||--o{ role_permissions : "tiene"
    permissions ||--o{ role_permissions : "se otorga en"
    users ||--o{ refresh_sessions : "abre"
    users ||--o{ auth_tokens : "recibe"
    users ||--o{ password_history : "acumula"
    users ||--o{ audit_log : "genera"

    users {
        uuid id PK
        citext username UK
        citext email UK
        text password_hash
        text pin_hash
        uuid role_id FK
        boolean must_change_password
        timestamptz locked_until
    }
    roles {
        uuid id PK
        text code UK
        text name
        boolean is_system
    }
    permissions {
        uuid id PK
        text module
        text action
    }
    role_permissions {
        uuid role_id PK
        uuid permission_id PK
    }
    refresh_sessions {
        uuid id PK
        uuid user_id FK
        text token_hash
        timestamptz expires_at
        timestamptz revoked_at
    }
    auth_tokens {
        uuid id PK
        uuid user_id FK
        text token_hash
        text purpose
        timestamptz expires_at
    }
    password_history {
        uuid id PK
        uuid user_id FK
        text password_hash
    }
    audit_log {
        uuid id PK
        uuid user_id FK
        text action
        text entity
        jsonb before_data
        jsonb after_data
    }
```

## Catálogo e inventario

```mermaid
erDiagram
    categories ||--o{ categories : "agrupa"
    categories ||--o{ products : "clasifica"
    brands |o--o{ products : "marca"
    units ||--o{ products : "mide"
    products ||--o{ product_presentations : "se vende como"
    products ||--o{ inventory_movements : "registra"
    suppliers ||--o{ purchases : "surte"
    purchases ||--|{ purchase_items : "contiene"
    products ||--o{ purchase_items : "se compra en"

    products {
        uuid id PK
        citext sku UK
        text barcode UK
        text name
        uuid category_id FK
        uuid brand_id FK
        uuid unit_id FK
        numeric price
        numeric average_cost
        numeric tax_percent
        integer stock
        integer min_stock
        integer reorder_point
    }
    product_presentations {
        uuid id PK
        uuid product_id FK
        text name
        integer factor
        text barcode UK
        numeric price
    }
    categories {
        uuid id PK
        text name
        uuid parent_id FK
    }
    brands {
        uuid id PK
        text name
    }
    units {
        uuid id PK
        text name
        text abbreviation
    }
    inventory_movements {
        uuid id PK
        uuid product_id FK
        text type
        integer quantity
        integer stock_after
        numeric unit_cost
        text source_type
        uuid source_id
    }
    suppliers {
        uuid id PK
        text name
        text tax_id
        integer credit_days
    }
    purchases {
        uuid id PK
        uuid supplier_id FK
        text status
        date purchase_date
        numeric total
        uuid received_by FK
    }
    purchase_items {
        uuid id PK
        uuid purchase_id FK
        uuid product_id FK
        integer quantity
        numeric unit_cost
    }
```

## Caja y ventas

```mermaid
erDiagram
    registers ||--o{ cash_shifts : "abre"
    users ||--o{ cash_shifts : "atiende"
    cash_shifts ||--o{ cash_movements : "registra"
    cash_shifts ||--o{ sales : "cobra"
    registers ||--o{ sales : "numera"
    customers ||--o{ sales : "compra"
    sales ||--|{ sale_items : "detalla"
    sales ||--|{ sale_payments : "se paga con"
    products ||--o{ sale_items : "se vende en"
    product_presentations |o--o{ sale_items : "presentación"
    currencies ||--o{ sale_payments : "moneda"
    currencies ||--o{ exchange_rates : "cotiza"
    sales ||--o{ returns : "devuelve"
    returns ||--|{ return_items : "contiene"
    sale_items ||--o{ return_items : "origen"

    registers {
        uuid id PK
        text name UK
        text invoice_series UK
        integer next_invoice_number
    }
    cash_shifts {
        uuid id PK
        uuid register_id FK
        uuid user_id FK
        text status
        numeric opening_nio
        numeric opening_usd
        numeric expected_nio
        numeric counted_nio
        numeric difference_nio
    }
    cash_movements {
        uuid id PK
        uuid shift_id FK
        text type
        char currency_code FK
        numeric amount
        text reason
    }
    sales {
        uuid id PK
        text invoice_series UK
        integer invoice_number UK
        uuid shift_id FK
        uuid register_id FK
        uuid user_id FK
        uuid customer_id FK
        numeric subtotal
        numeric discount
        numeric tax
        numeric total
        numeric exchange_rate
        text status
        uuid idempotency_key UK
    }
    sale_items {
        uuid id PK
        uuid sale_id FK
        uuid product_id FK
        uuid presentation_id FK
        integer quantity
        integer units_per_item
        numeric unit_price
        numeric unit_cost
        numeric tax_percent
    }
    sale_payments {
        uuid id PK
        uuid sale_id FK
        text method
        char currency_code FK
        numeric amount
        numeric exchange_rate
        numeric amount_nio
    }
    customers {
        uuid id PK
        text full_name
        text tax_id
        boolean is_walk_in
    }
    currencies {
        char code PK
        text symbol
        boolean is_base
    }
    exchange_rates {
        uuid id PK
        char currency_code FK
        numeric rate
        date effective_date
    }
    returns {
        uuid id PK
        uuid sale_id FK
        uuid shift_id FK
        numeric refund_total
    }
    return_items {
        uuid id PK
        uuid return_id FK
        uuid sale_item_id FK
        integer quantity
    }
```

Las tablas `returns` y `return_items` existen en el esquema, pero el sistema no registra devoluciones: quedaron fuera del alcance de esta versión.

## Sistema

```mermaid
erDiagram
    users ||--o{ backups : "crea"
    users |o--o{ settings : "actualiza"
    users |o--o{ schedule_extensions : "recibe o autoriza"
    email_templates ||--o{ email_queue : "origina"

    settings {
        text key PK
        jsonb value
        uuid updated_by FK
    }
    schedule_extensions {
        uuid id PK
        uuid user_id FK "nulo: todos"
        timestamptz ends_at
        uuid created_by FK
    }
    backups {
        uuid id PK
        text file_name
        text kind
        text status
        text checksum_sha256
        timestamptz verified_at
    }
    email_templates {
        text key PK
        text subject
        text html_body
    }
    email_queue {
        uuid id PK
        text template_key FK
        text to_address
        text status
        integer attempts
    }
```
