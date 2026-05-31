# BD2-P01-Restaurantes

API REST para gestión de restaurantes, menús, reservas y pedidos. Construida en Go con soporte para PostgreSQL y MongoDB, caché Redis como middleware HTTP, búsqueda con Elasticsearch y autenticación Keycloak.

## Tabla de Contenidos

- [Arquitectura](#arquitectura)
- [Decisiones Técnicas](#decisiones-técnicas)
- [Características](#características)
- [Configuración Inicial](#configuración-inicial)
- [Ejecución del Proyecto](#ejecución-del-proyecto)
- [API Reference](#api-reference)
- [Cambiar Entre Motores de BD](#cambiar-entre-motores-de-bd)
- [URLs de Acceso](#urls-de-acceso)
- [Estructura del Proyecto](#estructura-del-proyecto)
- [Comandos Útiles](#comandos-útiles)
- [Troubleshooting](#troubleshooting)
- [OLTP → OLAP: Transición a Analítica](#oltp--olap-transición-a-analítica)

## Arquitectura

```
                                          ┌─────────────┐
                                          │   Keycloak   │
                                          │  (Auth/OIDC) │
                                          └──────┬──────┘
                                                 │ JWT
                                    ┌────────────▼──────────┐
                                    │   Nginx (Reverse      │
                                    │    Proxy, puerto 80)  │
                                    └───┬────────────┬──────┘
                                        │            │
                               ┌────────▼──┐  ┌─────▼────────┐
                               │ API Principal│  │ Search MS  │
                               │  (Gin, :8000)│  │ (Gin, :8001)│
                               └──┬──┬──┬──┬─┘  └──────┬──────┘
                                  │  │  │  │            │
                    ┌─────────────┘  │  │  └────────────┐
                    ▼                ▼  ▼               ▼
            ┌──────────┐     ┌────────┐          ┌──────────┐
            │ Postgres │     │ Redis  │          │Elasticsearch
            │  MongoDB │     │(Cache  │          │ (Search  │
            │  (según  │     │ Middle-│          │  Index)  │
            │ DB_ENGINE│     │ ware)  │          └──────────┘
            └──────────┘     └────────┘
```

### Flujo de una Request GET (con caché)

```
1. Request → Nginx → API (Gin)
2. Redis Middleware intercepta
3. ¿Path en skipPaths? → Sí → pasa al handler
4. ¿Cache hit en Redis? → Sí → response directa (c.Abort())
5. Cache miss → handler ejecuta → response se guarda en Redis
```

### Flujo de una Request POST/PUT/DELETE

```
1. Request → Nginx → API (Gin)
2. Redis Middleware: pasa al handler
3. Handler ejecuta (persiste en BD, indexa en ES)
4. ¿Response 2xx? → Redis Middleware invalida claves relacionadas
```

## Decisiones Técnicas

### Redis como Middleware HTTP (en lugar de Decorator)

**Problema original:** Redis se implementaba como decorators sobre los repositorios (`UserRedisDecorator`, `RestaurantRedisDecorator`, `MenuRedisDecorator`). Esto duplicaba la lógica de caché en cada repositorio y acoplaba la capa de persistencia con la de caché.

**Solución:** Migrar a middleware Gin que intercepta requests HTTP GET, cachea responses completas en Redis como JSON (status + contentType + body), e invalida en POST/PUT/DELETE según reglas de ruta.

**Ventajas:**
- Centraliza la lógica de caché en un solo lugar
- Los repositorios ya no conocen Redis (separación de concerns)
- Cachea cualquier response, no solo JSON
- Invalidación por ruta (más predecible que por método de repositorio)
- Fácil de deshabilitar quitando una línea en `main.go`

### miniredis para Tests

Se usa `github.com/alicebob/miniredis/v2` en lugar de mockear interfaces de go-redis. Proporciona un servidor Redis real en memoria para tests deterministas sin necesidad de infraestructura.

### SCAN en lugar de KEYS

Para borrado por prefijo se usa `SCAN` (cursor-based, no bloqueante) en lugar de `KEYS` (peligroso en producción con muchas keys).

### skipPaths vs Grupo de Rutas

Se optó por middleware en el router raíz con skipPaths en lugar de un grupo separado. Esto permite que la invalidación corra en todas las rutas mutantes mientras el cacheo GET se salta rutas con datos de usuario.

### Soporte Multi-Motor

`DB_ENGINE=postgres` o `DB_ENGINE=mongodb` intercambian toda la capa de persistencia mediante Factory Pattern. Las interfaces (`UserRepository`, `RestaurantRepository`, etc.) son el contrato común.

## Características

- **API REST** construida en Go con Gin framework
- **Múltiples Motores de BD**: PostgreSQL o MongoDB (intercambiables vía `DB_ENGINE`)
- **MongoDB Sharding**: 3 shards con replicación
- **Redis**: Middleware HTTP de caché con graceful degradation
- **Elasticsearch**: Búsqueda de texto completo e indexación
- **Autenticación**: Keycloak con OAuth2/OpenID Connect
- **Autorización**: RBAC (Admin/Client)
- **Nginx**: Proxy inverso para enrutamiento
- **Docker Compose**: Orquestación completa de servicios

## Configuración Inicial

### 1. Prerrequisitos

- Docker y Docker Compose
- Go 1.21+ (para desarrollo local)
- Make (opcional)

### 2. Variables de Entorno

**Archivo `.env`:**
```env
# Motor de base de datos activo
DB_ENGINE=mongodb              # Cambiar a 'postgres' o 'mongodb'

# Configuración de la API
PORT=8000

# PostgreSQL
POSTGRES_USER=admin
POSTGRES_PASSWORD=password
POSTGRES_DB=restaurante_db
POSTGRES_HOST=db
POSTGRES_PORT=5432

# MongoDB
MONGO_URI=mongodb://mongos:27017/restaurante_db
MONGO_DB=restaurante_db

# Keycloak (Autenticación)
KEYCLOAK_ADMIN=admin
KEYCLOAK_ADMIN_PASSWORD=admin
KEYCLOAK_PORT=8080
KEYCLOAK_URL=http://keycloak:8080
KEYCLOAK_REALM=restaurante-api
KEYCLOAK_CLIENT_ID=backend-api
KEYCLOAK_CLIENT_SECRET=CozoA7kBJYwEtR4gL2m5bJ6gSCUVC32E

# Elasticsearch (Búsqueda)
ELASTICSEARCH_URL=http://elasticsearch:9200

# Redis (Caché - Middleware HTTP)
REDIS_HOST=redis
REDIS_PORT=6379
REDIS_PASSWORD=
ENABLE_REDIS=true
```

### 3. Crear Archivo `.env`
```bash
cp .env.example .env
```

## Ejecución del Proyecto

### Opción 1: Todo Completo (Recomendado)

```bash
# Iniciar todos los servicios
docker-compose up --build -d
```

**Servicios:**
- API Principal (puerto 8000, vía Nginx puerto 80)
- Microservicio Search (puerto 8001, vía Nginx)
- PostgreSQL (puerto 5432)
- MongoDB Sharded (puerto 27017)
- Keycloak (puerto 8080)
- Redis (puerto 6379)
- Elasticsearch (puerto 9200)
- Kibana (puerto 5601)
- Adminer (puerto 8081)
- Nginx Proxy (puerto 80)

### Desarrollo Local

```bash
# Solo servicios de soporte
docker-compose up -d db keycloak redis elasticsearch

# Compilar y ejecutar API
go run api/main.go
```

### Detener y Limpiar

```bash
docker-compose down          # Detener contenedores
docker-compose down -v       # Detener y eliminar volúmenes
```

## API Reference

### Resumen de Endpoints

| Método | Ruta (a través de Nginx) | Auth | Descripción |
|--------|--------------------------|------|-------------|
| `POST` | `/api/auth/login` | Pública | Iniciar sesión y obtener JWT |
| `POST` | `/api/auth/register` | Pública | Registrar un nuevo usuario |
| `GET` | `/api/restaurants` | Pública | Listar todos los restaurantes |
| `POST` | `/api/restaurants` | Admin | Crear un nuevo restaurante |
| `GET` | `/api/menus/:id` | Pública | Obtener menú de un restaurante |
| `POST` | `/api/menus` | Admin | Agregar platos a un menú |
| `PUT` | `/api/menus/:id` | Admin | Actualizar un plato |
| `DELETE` | `/api/menus/:id` | Admin | Eliminar un plato |
| `GET` | `/api/users/me` | JWT | Obtener perfil del usuario autenticado |
| `PUT` | `/api/users/:id` | JWT | Actualizar perfil propio |
| `DELETE` | `/api/users/:id` | JWT | Eliminar cuenta propia |
| `POST` | `/api/reservations` | JWT | Crear una reserva |
| `DELETE` | `/api/reservations/:id` | JWT | Cancelar reserva propia |
| `POST` | `/api/orders` | JWT | Crear un pedido |
| `GET` | `/api/orders/:id` | JWT | Obtener pedido propio |
| `GET` | `/search/products?q=` | Pública | Búsqueda difusa de platos |
| `GET` | `/search/products/category/:categoria` | Pública | Filtrar platos por categoría |
| `POST` | `/search/reindex` | Pública | Reindexar todos los platos en ES |

---

### Autenticación

#### POST /api/auth/login

Inicia sesión con credenciales y devuelve tokens JWT.

- **Auth:** Pública
- **Content-Type:** `application/json`

**Request Body:**

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| `email` | string | sí | Correo electrónico del usuario |
| `password` | string | sí | Contraseña |

**Respuesta 200 OK:**
```json
{
  "access_token": "eyJhbGciOiJSUzI1NiIs...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
  "expires_in": 3600,
  "token_type": "Bearer"
}
```

**Errores:**

| Código | Cuerpo | Causa |
|--------|--------|-------|
| `400` | `{"error": "..."}` | Campo requerido faltante o inválido |
| `401` | `{"error": "Credenciales invalidas"}` | Email o contraseña incorrectos |

**Ejemplo:**
```bash
curl -X POST http://localhost/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"usuario@mail.com","password":"secreta"}'
```

---

#### POST /api/auth/register

Registra un nuevo usuario en Keycloak y crea su registro local en la BD.

- **Auth:** Pública
- **Content-Type:** `application/json`

**Request Body:**

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| `nombre` | string | sí | Nombre del usuario |
| `apellido` | string | sí | Apellido del usuario |
| `email` | string | sí | Correo electrónico |
| `password` | string | sí | Contraseña |

**Respuesta 201 Created:**
```json
{
  "id": "uuid-de-keycloak",
  "nombre": "Ana",
  "email": "ana@mail.com",
  "message": "Usuario registrado exitosamente"
}
```

**Errores:**

| Código | Cuerpo | Causa |
|--------|--------|-------|
| `400` | `{"error": "..."}` | Campo requerido faltante |
| `500` | `{"error": "..."}` | Error al crear en Keycloak o BD |

**Ejemplo:**
```bash
curl -X POST http://localhost/api/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"nombre":"Ana","apellido":"López","email":"ana@mail.com","password":"secreta"}'
```

---

### Restaurantes

#### GET /api/restaurants

Obtiene la lista completa de restaurantes registrados.

- **Auth:** Pública
- **Cache:** Respuesta cacheada en Redis (clave `GET:/restaurants`)

**Respuesta 200 OK:**
```json
[
  {
    "id": "rest-123",
    "nombre": "La Trattoria",
    "direccion": "San José",
    "telefono": "8888-1234",
    "id_administrador": "user-uuid-del-admin",
    "capacidad_total": 50
  }
]
```

**Ejemplo:**
```bash
curl http://localhost/api/restaurants
```

---

#### POST /api/restaurants

Crea un nuevo restaurante. El usuario autenticado queda como administrador del restaurante.

- **Auth:** JWT + Rol Admin (`user_role_id == 1`)
- **Content-Type:** `application/json`
- **Header requerido:** `Authorization: Bearer <token>`

**Request Body:**

| Campo | Tipo | Requerido | Validación | Descripción |
|-------|------|-----------|------------|-------------|
| `nombre` | string | sí | — | Nombre del restaurante |
| `direccion` | string | sí | — | Dirección física |
| `telefono` | string | no | — | Número de teléfono |
| `capacidad_total` | int | sí | `gt=0` | Capacidad máxima de comensales |

**Respuesta 201 Created:**
```json
{
  "id": "rest-456",
  "nombre": "La Trattoria",
  "direccion": "San José",
  "telefono": "8888-1234",
  "id_administrador": "user-uuid",
  "capacidad_total": 50
}
```

**Errores:**

| Código | Cuerpo | Causa |
|--------|--------|-------|
| `400` | `{"error": "..."}` | Campo requerido faltante o `capacidad_total <= 0` |
| `401` | `{"error": "No autorizado"}` | Token no provisto o inválido |
| `403` | `{"error": "Se requiere rol de administrador"}` | El usuario no es admin |

**Ejemplo:**
```bash
curl -X POST http://localhost/api/restaurants \
  -H 'Authorization: Bearer eyJ...' \
  -H 'Content-Type: application/json' \
  -d '{"nombre":"La Trattoria","direccion":"San José","telefono":"8888-1234","capacidad_total":50}'
```

---

### Menús

#### GET /api/menus/:id

Obtiene todos los platos de un restaurante específico.

- **Auth:** Pública
- **Parámetro de ruta:** `id` — ID del restaurante
- **Cache:** Respuesta cacheada en Redis (clave `GET:/menus/:id`)

**Respuesta 200 OK:**
```json
{
  "id_restaurante": "rest-123",
  "platos": [
    {
      "id": "plato-1",
      "id_restaurante": "rest-123",
      "nombre": "Casado",
      "descripcion": "Arroz, frijoles, carne, ensalada y maduro",
      "precio": 3500,
      "esta_disponible": true,
      "categoria": "Almuerzos"
    }
  ]
}
```

**Errores:**

| Código | Cuerpo | Causa |
|--------|--------|-------|
| `500` | `{"error": "Error obteniendo menú"}` | Error interno de BD |

**Ejemplo:**
```bash
curl http://localhost/api/menus/rest-123
```

---

#### POST /api/menus

Agrega uno o más platos al menú de un restaurante. Cada plato se indexa automáticamente en Elasticsearch.

- **Auth:** JWT + Rol Admin (debe ser administrador del restaurante indicado)
- **Content-Type:** `application/json`

**Request Body:**

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| `id_restaurante` | string | sí | ID del restaurante al que pertenecen los platos |
| `platos` | array | sí | Lista de platos a crear |

**Cada objeto en `platos`:**

| Campo | Tipo | Requerido | Validación | Descripción |
|-------|------|-----------|------------|-------------|
| `nombre` | string | sí | — | Nombre del plato |
| `descripcion` | string | no | — | Descripción del plato |
| `precio` | float | sí | `gt=0` | Precio en colones |
| `categoria` | string | no | — | Categoría (ej. `Almuerzos`, `Cenas`, `Bebidas`) |

**Respuesta 201 Created:**
```json
[
  {
    "id": "plato-1",
    "id_restaurante": "rest-123",
    "nombre": "Casado",
    "descripcion": "Arroz, frijoles, carne, ensalada y maduro",
    "precio": 3500,
    "esta_disponible": true,
    "categoria": "Almuerzos"
  }
]
```

**Errores:**

| Código | Cuerpo | Causa |
|--------|--------|-------|
| `400` | `{"error": "..."}` | Validación fallida (ej. `precio` <= 0) |
| `401` | `{"error": "No autorizado"}` | Token no provisto o inválido |
| `403` | `{"error": "No es administrador de este restaurante"}` | El admin no es dueño del restaurante |

**Ejemplo:**
```bash
curl -X POST http://localhost/api/menus \
  -H 'Authorization: Bearer eyJ...' \
  -H 'Content-Type: application/json' \
  -d '{"id_restaurante":"rest-123","platos":[{"nombre":"Casado","descripcion":"Arroz, frijoles, carne","precio":3500,"categoria":"Almuerzos"}]}'
```

---

#### PUT /api/menus/:id

Actualiza los datos de un plato existente. Re-indexa el plato en Elasticsearch automáticamente.

- **Auth:** JWT + Rol Admin (debe ser administrador del restaurante al que pertenece el plato)
- **Content-Type:** `application/json`
- **Parámetro de ruta:** `id` — ID del plato a actualizar

**Request Body (todos los campos son opcionales — actualización parcial):**

| Campo | Tipo | Descripción |
|-------|------|-------------|
| `nombre` | string | Nuevo nombre del plato |
| `descripcion` | string | Nueva descripción |
| `precio` | float | Nuevo precio |
| `esta_disponible` | bool | Disponibilidad del plato |
| `categoria` | string | Nueva categoría |

**Respuesta 200 OK:**
```json
{
  "message": "Actualizado"
}
```

**Errores:**

| Código | Cuerpo | Causa |
|--------|--------|-------|
| `400` | `{"error": "..."}` | Validación fallida |
| `401` | `{"error": "No autorizado"}` | Token no provisto o inválido |
| `403` | `{"error": "..."}` | No es administrador del restaurante |
| `404` | `{"error": "Plato no encontrado"}` | El ID del plato no existe |

**Ejemplo:**
```bash
curl -X PUT http://localhost/api/menus/plato-123 \
  -H 'Authorization: Bearer eyJ...' \
  -H 'Content-Type: application/json' \
  -d '{"nombre":"Casado Especial","precio":4000,"esta_disponible":true}'
```

---

#### DELETE /api/menus/:id

Elimina un plato del menú. Lo remueve también del índice de Elasticsearch.

- **Auth:** JWT + Rol Admin (debe ser administrador del restaurante al que pertenece el plato)
- **Parámetro de ruta:** `id` — ID del plato a eliminar

**Respuesta 200 OK:**
```json
{
  "message": "Eliminado"
}
```

**Errores:**

| Código | Cuerpo | Causa |
|--------|--------|-------|
| `401` | `{"error": "No autorizado"}` | Token no provisto o inválido |
| `403` | `{"error": "..."}` | No es administrador del restaurante |
| `404` | `{"error": "..."}` | Plato no encontrado |

**Ejemplo:**
```bash
curl -X DELETE http://localhost/api/menus/plato-123 \
  -H 'Authorization: Bearer eyJ...'
```

---

### Usuarios

#### GET /api/users/me

Obtiene el perfil del usuario autenticado.

- **Auth:** JWT (cualquier usuario autenticado)
- **Header requerido:** `Authorization: Bearer <token>`

**Respuesta 200 OK:**
```json
{
  "id": "user-uuid",
  "nombre": "Ana López",
  "email": "ana@mail.com",
  "keycloak_id": "kc-uuid",
  "id_rol": 2,
  "fecha_creacion": "2025-01-01T00:00:00Z"
}
```

**Errores:**

| Código | Cuerpo | Causa |
|--------|--------|-------|
| `401` | `{"error": "No autorizado"}` | Token no provisto o inválido |
| `404` | `{"error": "Usuario no encontrado"}` | Usuario no existe en BD local |

**Ejemplo:**
```bash
curl http://localhost/api/users/me \
  -H 'Authorization: Bearer eyJ...'
```

---

#### PUT /api/users/:id

Actualiza el perfil del usuario autenticado. Solo puede actualizar su propio perfil (el `:id` debe coincidir con el `user_id` del token).

- **Auth:** JWT (debe coincidir `:id` con el token)
- **Content-Type:** `application/json`
- **Parámetro de ruta:** `id` — ID del usuario a actualizar

**Request Body:**

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| `nombre` | string | no | Nuevo nombre |
| `email` | string | no | Nuevo correo electrónico |

**Respuesta 200 OK:**
```json
{
  "message": "Usuario actualizado"
}
```

**Errores:**

| Código | Cuerpo | Causa |
|--------|--------|-------|
| `400` | `{"error": "..."}` | Validación fallida |
| `401` | `{"error": "No autorizado"}` | Token no provisto o inválido |
| `403` | `{"error": "No autorizado para modificar este usuario"}` | El `:id` no coincide con el token |

**Ejemplo:**
```bash
curl -X PUT http://localhost/api/users/user-123 \
  -H 'Authorization: Bearer eyJ...' \
  -H 'Content-Type: application/json' \
  -d '{"nombre":"Ana López Actualizado","email":"ana.nuevo@mail.com"}'
```

---

#### DELETE /api/users/:id

Elimina la cuenta del usuario autenticado. Solo puede eliminar su propia cuenta.

- **Auth:** JWT (debe coincidir `:id` con el token)
- **Parámetro de ruta:** `id` — ID del usuario a eliminar

**Respuesta 200 OK:**
```json
{
  "message": "Usuario eliminado"
}
```

**Errores:**

| Código | Cuerpo | Causa |
|--------|--------|-------|
| `401` | `{"error": "No autorizado"}` | Token no provisto o inválido |
| `403` | `{"error": "No autorizado para eliminar este usuario"}` | El `:id` no coincide con el token |

**Ejemplo:**
```bash
curl -X DELETE http://localhost/api/users/user-123 \
  -H 'Authorization: Bearer eyJ...'
```

---

### Reservas

#### POST /api/reservations

Crea una nueva reserva para el usuario autenticado. Valida que la cantidad de personas no exceda la capacidad del restaurante.

- **Auth:** JWT (cualquier usuario autenticado)
- **Content-Type:** `application/json`

**Request Body:**

| Campo | Tipo | Requerido | Validación | Descripción |
|-------|------|-----------|------------|-------------|
| `id_restaurante` | string | sí | — | ID del restaurante |
| `fecha_reserva` | string (RFC3339) | sí | — | Fecha y hora de la reserva |
| `cantidad_personas` | int | sí | `gt=0` | Número de comensales |

**Respuesta 201 Created:**
```json
{
  "id": "res-123",
  "id_usuario": "user-uuid",
  "id_restaurante": "rest-123",
  "fecha_reserva": "2025-06-15T19:00:00Z",
  "cantidad_personas": 4,
  "id_estado": 1,
  "nombre_estado": "Confirmada"
}
```

**Errores:**

| Código | Cuerpo | Causa |
|--------|--------|-------|
| `400` | `{"error": "..."}` | Validación fallida (ej. `cantidad_personas` <= 0, excede capacidad) |
| `401` | `{"error": "No autorizado"}` | Token no provisto o inválido |

**Ejemplo:**
```bash
curl -X POST http://localhost/api/reservations \
  -H 'Authorization: Bearer eyJ...' \
  -H 'Content-Type: application/json' \
  -d '{"id_restaurante":"rest-123","fecha_reserva":"2025-06-15T19:00:00Z","cantidad_personas":4}'
```

---

#### DELETE /api/reservations/:id

Cancela una reserva existente. Solo el usuario que creó la reserva puede cancelarla.

- **Auth:** JWT (debe ser el dueño de la reserva)
- **Parámetro de ruta:** `id` — ID de la reserva a cancelar

**Respuesta 200 OK:**
```json
{
  "message": "Cancelada"
}
```

**Errores:**

| Código | Cuerpo | Causa |
|--------|--------|-------|
| `401` | `{"error": "No autorizado"}` | Token no provisto o inválido |
| `403` | `{"error": "No autorizado"}` | La reserva pertenece a otro usuario |
| `404` | `{"error": "Reserva no encontrada"}` | El ID de reserva no existe |

**Ejemplo:**
```bash
curl -X DELETE http://localhost/api/reservations/res-123 \
  -H 'Authorization: Bearer eyJ...'
```

---

### Pedidos

#### POST /api/orders

Crea un nuevo pedido para el usuario autenticado. Calcula automáticamente el total sumando `precio * cantidad` de cada detalle.

- **Auth:** JWT (cualquier usuario autenticado)
- **Content-Type:** `application/json`

**Request Body:**

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| `id_restaurante` | string | sí | ID del restaurante |
| `es_recogida` | bool | no | `true` para recoger en local, `false` para entrega |
| `detalles` | array | sí | Lista de platos solicitados |

**Cada objeto en `detalles`:**

| Campo | Tipo | Requerido | Validación | Descripción |
|-------|------|-----------|------------|-------------|
| `id_plato` | string | sí | — | ID del plato |
| `cantidad` | int | sí | `gt=0` | Cantidad solicitada |

**Respuesta 201 Created:**
```json
{
  "id": "order-123",
  "id_usuario": "user-uuid",
  "id_restaurante": "rest-123",
  "es_recogida": false,
  "id_estado_pedido": 1,
  "total": 7000,
  "detalles": [
    {
      "id": "det-1",
      "id_plato": "plato-1",
      "cantidad": 2,
      "precio": 3500
    }
  ]
}
```

**Errores:**

| Código | Cuerpo | Causa |
|--------|--------|-------|
| `400` | `{"error": "..."}` | Validación fallida (ej. `cantidad` <= 0, `detalles` vacío) |
| `401` | `{"error": "No autorizado"}` | Token no provisto o inválido |

**Ejemplo:**
```bash
curl -X POST http://localhost/api/orders \
  -H 'Authorization: Bearer eyJ...' \
  -H 'Content-Type: application/json' \
  -d '{"id_restaurante":"rest-123","detalles":[{"id_plato":"plato-1","cantidad":2}]}'
```

---

#### GET /api/orders/:id

Obtiene la información de un pedido específico. Solo el usuario que creó el pedido puede consultarlo.

- **Auth:** JWT (debe ser el dueño del pedido)
- **Parámetro de ruta:** `id` — ID del pedido

**Respuesta 200 OK:**
```json
{
  "id": "order-123",
  "id_usuario": "user-uuid",
  "id_restaurante": "rest-123",
  "es_recogida": false,
  "id_estado_pedido": 1,
  "total": 7000,
  "detalles": [
    {
      "id": "det-1",
      "id_plato": "plato-1",
      "cantidad": 2,
      "precio": 3500
    }
  ]
}
```

**Errores:**

| Código | Cuerpo | Causa |
|--------|--------|-------|
| `401` | `{"error": "No autorizado"}` | Token no provisto o inválido |
| `403` | `{"error": "No autorizado"}` | El pedido pertenece a otro usuario |
| `404` | `{"error": "Pedido no encontrado"}` | El ID de pedido no existe |

**Ejemplo:**
```bash
curl http://localhost/api/orders/order-123 \
  -H 'Authorization: Bearer eyJ...'
```

---

### Búsqueda (Microservicio)

Los endpoints de búsqueda pertenecen a un microservicio independiente (puerto 8001) expuesto a través de Nginx en la ruta `/search/`.

#### GET /search/products?q=

Realiza una búsqueda de texto completo (fuzzy) sobre los platos indexados en Elasticsearch. Busca en los campos `nombre` (con peso 3x) y `descripcion` con análisis de lenguaje español y `fuzziness: AUTO`.

- **Auth:** Pública
- **Parámetro de consulta:** `q` (requerido) — término de búsqueda

**Respuesta 200 OK:**
```json
[
  {
    "id": "plato-1",
    "id_restaurante": "rest-123",
    "nombre": "Casado",
    "descripcion": "Arroz, frijoles, carne, ensalada y maduro",
    "precio": 3500,
    "esta_disponible": true,
    "categoria": "Almuerzos"
  }
]
```

**Errores:**

| Código | Cuerpo | Causa |
|--------|--------|-------|
| `400` | `{"error": "Debe proporcionar un término de búsqueda"}` | Parámetro `q` vacío o ausente |

**Ejemplo:**
```bash
curl "http://localhost/search/products?q=casado"
```

---

#### GET /search/products/category/:categoria

Filtra platos por categoría exacta (coincidencia directa sobre el campo `categoria` en Elasticsearch).

- **Auth:** Pública
- **Parámetro de ruta:** `categoria` — nombre exacto de la categoría (ej. `Almuerzos`, `Cenas`, `Bebidas`)

**Respuesta 200 OK:**
```json
[
  {
    "id": "plato-1",
    "id_restaurante": "rest-123",
    "nombre": "Casado",
    "descripcion": "Arroz, frijoles, carne, ensalada y maduro",
    "precio": 3500,
    "esta_disponible": true,
    "categoria": "Almuerzos"
  }
]
```

**Ejemplo:**
```bash
curl "http://localhost/search/products/category/Almuerzos"
```

---

#### POST /search/reindex

Reconstruye el índice de Elasticsearch desde cero. Obtiene todos los platos desde la base de datos y los indexa en lote. Útil después de una restauración de datos o para sincronizar el índice.

- **Auth:** Pública (idealmente restringir a admin en producción)

**Respuesta 200 OK:**
```json
{
  "message": "Proceso de reindexación completado con éxito",
  "count": 150
}
```

**Errores:**

| Código | Cuerpo | Causa |
|--------|--------|-------|
| `500` | `{"error": "Error al obtener platos de la BD"}` | Error al listar platos desde la BD |
| `500` | `{"error": "Error durante la reindexación en ES"}` | Error al indexar en Elasticsearch |

**Ejemplo:**
```bash
curl -X POST http://localhost/search/reindex
```

## Cambiar Entre Motores de BD

### PostgreSQL vs MongoDB

El proyecto soporta dos motores de base de datos intercambiables mediante `DB_ENGINE` en `.env`.

### PostgreSQL

```bash
# 1. Editar .env
DB_ENGINE=postgres

# 2. Inicializar
docker-compose up -d db
docker exec -it db psql -U admin -d restaurante_db -f /docker-entrypoint-initdb.d/init.sql

# 3. Servicios restantes
docker-compose up -d keycloak redis elasticsearch

# 4. Ejecutar API
go run api/main.go
```

### MongoDB

```bash
# 1. Editar .env
DB_ENGINE=mongodb

# 2. Inicializar sharding
docker-compose up -d configsvr mongo1 mongo2 mongo3 mongos
docker exec mongos mongosh --port 27017 < initMongo.js

# 3. Servicios restantes
docker-compose up -d keycloak redis elasticsearch

# 4. Ejecutar API
go run api/main.go
```

### Comparativa

| Aspecto | PostgreSQL | MongoDB |
|---------|-----------|---------|
| Motor | Relacional | Documentos |
| Variable | `DB_ENGINE=postgres` | `DB_ENGINE=mongodb` |
| Contenedor | `db` | `mongos` (+ configsvr, mongo1-3) |
| Puerto | 5432 | 27017 |
| Admin UI | Adminer (8081) | MongoDB Compass |
| Inicialización | `init.sql` | `initMongo.js` |
| Escalabilidad | Single instance | Sharded (3 shards) |

## URLs de Acceso

| Servicio | URL | Usuario | Contraseña |
|----------|-----|---------|-----------|
| API Principal | http://localhost/api | OAuth2 (Keycloak) | - |
| Microservicio Search | http://localhost/search | OAuth2 (Keycloak) | - |
| Keycloak Admin | http://localhost:8080 | admin | admin |
| Elasticsearch | http://localhost:9200 | - | - |
| Kibana | http://localhost:5601 | - | - |
| Adminer (PostgreSQL) | http://localhost:8081 | admin | password |
| MongoDB (Local) | mongodb://localhost:27017 | - | - |
| Nginx | http://localhost:80 | - | - |

## Estructura del Proyecto

```
.
├── api/
│   └── main.go                    # Servidor API (Gin)
├── handlers/
│   ├── auth.go                    # Login, RegisterUser
│   ├── restaurant.go              # ListRestaurants, CreateRestaurant
│   ├── menu.go                    # CRUD menú
│   ├── reservation.go             # CreateReservation, CancelReservation
│   ├── order.go                   # CreateOrder, GetOrder
│   ├── user.go                    # GetCurrentUser, UpdateUser, DeleteUser
│   ├── search.go                  # Búsqueda Elasticsearch
│   ├── helpers.go                 # respondError, respondJSON, helpers auth
│   └── *_test.go                  # Tests (mock repos)
├── repository/
│   ├── interface.go               # Interfaces (User, Restaurant, Menu, etc.)
│   ├── factory.go                 # Factory Pattern (postgres vs mongo)
│   ├── redis.go                   # Conexión Redis + constantes de claves
│   ├── postgres/                  # Implementación PostgreSQL
│   │   ├── restaurant.go
│   │   └── repository_test.go
│   ├── mongo/                     # Implementación MongoDB
│   │   ├── restaurant.go
│   │   └── repository_test.go
│   └── factory_test.go
├── models/
│   └── models.go                  # User, Restaurant, MenuItem, etc.
├── middleware/
│   ├── auth.go                    # Auth + RequireAdmin (Keycloak JWT)
│   ├── redis_cache.go             # Cache middleware (GET + invalidation)
│   ├── redis_cache_test.go        # Tests (miniredis, 15 tests)
│   └── logger.go                  # Request logger (ID, método, path, status)
├── elasticsearch/
│   ├── client.go                  # Conexión ES + creación índice
│   ├── indexer.go                 # Indexación de platos
│   └── mock.go                    # Mock para tests
├── keycloak/
│   ├── interfaces.go              # ClientInterface (para mockear)
│   └── setup.go                   # Configuración realm, client, roles
├── mocks/
│   └── mock.go                    # Mocks testify de todos los repositorios
├── search/
│   └── main.go                    # Microservicio de búsqueda
├── nginx/
│   └── nginx.conf                 # Configuración Nginx
├── docker-compose.yml             # Orquestación
├── Dockerfile                     # Build API
├── Dockerfile.search              # Build search microservice
├── init.sql                       # Inicialización PostgreSQL
├── initMongo.js                   # Inicialización MongoDB sharding
└── .env                           # Variables de entorno
```

## Comandos Útiles

### Compilación y Tests

```bash
go build -o api_bin ./api                # Compilar API
go test ./...                            # Tests completos
go test -cover ./...                     # Con cobertura
go test -coverprofile=coverage.out ./... # Reporte detallado
go vet ./...                             # Análisis estático
golangci-lint run                        # Linter
go mod tidy                              # Sincronizar módulos
```

### Docker

```bash
docker-compose build api                 # Reconstruir API
docker-compose logs --timestamps -f      # Logs en tiempo real
docker exec -it api bash                 # Shell en contenedor
```

### Redis (Caché)

```bash
docker exec -it redis redis-cli
KEYS *                                   # Ver claves
GET "GET:/restaurants"                   # Ver valor cacheado
FLUSHALL                                 # Limpiar caché
```

### Elasticsearch

```bash
curl -s http://localhost:9200/_cluster/health | jq
curl -s http://localhost:9200/platos/_search | jq
```

## Troubleshooting

### Contenedores no inician
```bash
docker-compose logs api
docker-compose build --no-cache api
docker-compose down -v && docker-compose up --build -d
```

### Connection refused (PostgreSQL)
```bash
docker-compose ps db
docker-compose logs db
docker-compose restart db
```

### MongoDB no inicia
```bash
sleep 60
docker exec mongos mongosh --port 27017 < initMongo.js
docker exec mongo1 mongosh --port 27018 --eval "rs.status()"
```

### Redis no responde
```bash
docker exec redis redis-cli ping
docker exec redis redis-cli FLUSHALL
docker-compose restart redis
```

### Elasticsearch no indexa
```bash
curl -s http://localhost:9200/_cluster/health
docker-compose logs elasticsearch
```

### API devuelve 401/403
```bash
curl -X POST http://localhost:8080/realms/restaurante-api/protocol/openid-connect/token \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  -d 'client_id=backend-api&client_secret=CozoA7kBJYwEtR4gL2m5bJ6gSCUVC32E&grant_type=client_credentials'
```

### Volúmenes llenos
```bash
docker-compose down -v
docker volume prune
```

## OLTP → OLAP: Transición a Analítica

### Estado Actual (OLTP)

La API actual está diseñada para transacciones OLTP:
- **Restaurantes**: CRUD de restaurantes y menús
- **Usuarios**: Registro y autenticación (Keycloak)
- **Reservas**: Creación y cancelación
- **Pedidos**: Creación y consulta
- **Búsqueda**: Indexación en Elasticsearch para búsqueda full-text

### Próximos Pasos (OLAP)

Se planea agregar endpoints analíticos para responder preguntas de negocio:

| Endpoint | Descripción | Métrica |
|----------|-------------|---------|
| `GET /analytics/ventas` | Ingresos totales por período | SUM(total) |
| `GET /analytics/platos-populares` | Platos más pedidos | COUNT(id_plato) |
| `GET /analytics/ingresos-restaurante` | Ingresos por restaurante | SUM(total) GROUP BY restaurante |
| `GET /analytics/tendencias` | Tendencia de ventas semanal/mensual | COUNT, SUM por fecha |
| `GET /analytics/reservas` | Reservas por restaurante/período | COUNT, AVG(personas) |
| `GET /analytics/horas-pico` | Horas con más pedidos | COUNT GROUP BY HOUR |

### Evaluación de Enfoques

| Enfoque | Pros | Contras |
|---------|------|---------|
| **Agregaciones MongoDB** | Sin infraestructura extra | Limitado a MongoDB, sin historical data |
| **Vistas PostgreSQL** | SQL nativo, sin latencia de red | Limitado a PostgreSQL |
| **Elasticsearch aggregations** | Ya usado, rápido para agrupaciones | Datos solo de menús actualmente |
| **Nuevo microservicio OLAP** | Desacoplado, puede usar motor óptimo | Más infraestructura, latencia de sync |
| **ClickHouse / DuckDB** | Optimizado para OLAP | Nueva dependencia, overskill inicial |

### Recomendación

Comenzar con **agregaciones sobre la BD actual** (MongoDB `$group` / PostgreSQL `GROUP BY`) expuestas como nuevos endpoints en la API actual. Esto permite iterar rápido sin nueva infraestructura. Si la carga analítica crece, migrar a un microservicio separado con Elasticsearch aggregations o un motor OLAP dedicado.

## Documentación Adicional
- [REDIS_CONFIG.md](REDIS_CONFIG.md) — Configuración avanzada de Redis como middleware
- [planRedis.md](planRedis.md) — Plan de migración de decorators a middleware HTTP
