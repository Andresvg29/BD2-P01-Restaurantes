# BD2-P01-Restaurantes

API REST para gestión de restaurantes, menús, reservas y pedidos. Construida en Go 1.24 con soporte para PostgreSQL y MongoDB, caché Redis como middleware HTTP, búsqueda con Elasticsearch, autenticación Keycloak y Data Warehouse OLAP con Hadoop HDFS, Hive, Spark, Superset, Airflow y Neo4j.

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
- [Arquitectura OLAP](#arquitectura-olap)
- [Pipeline ETL](#pipeline-etl)
- [Procesamiento con Spark](#procesamiento-con-spark)
- [Visualización con Superset](#visualización-con-superset)
- [Orquestación con Airflow](#orquestación-con-airflow)
- [Análisis de Grafos con Neo4j](#análisis-de-grafos-con-neo4j)
- [Rutas de Entrega](#rutas-de-entrega)
- [Suite de Validación](#suite-de-validación)
- [Plan de Ejecución](#plan-de-ejecución)

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

### Arquitectura Completa (OLTP + OLAP)

```
                                    ┌──────────────────────────────────────────┐
                                    │           CAPA OLTP                      │
                                    │                                          │
                                    │  ┌──────────┐  ┌──────────┐  ┌────────┐ │
                                    │  │ Keycloak  │  │  Redis   │  │  Nginx │ │
                                    │  │ (Auth)    │  │ (Cache)  │  │ (Proxy)│ │
                                    │  └─────┬────┘  └────┬─────┘  └───┬────┘ │
                                    │        │            │             │      │
                                    │  ┌─────▼────────────▼─────────────▼────┐ │
                                    │  │       API Principal (Gin)          │ │
                                    │  │         + Search MS (Gin)          │ │
                                    │  └──┬──────────────┬──────────────┬───┘ │
                                    │     │              │              │     │
                                    │  ┌──▼──┐      ┌────▼────┐  ┌────▼───┐ │
                                    │  │PG/MG│      │Elastic  │  │ Neo4j  │ │
                                    │  │(OLTP)│      │search   │  │(Grafos)│ │
                                    │  └─────┘      └─────────┘  └────────┘ │
                                    └────────────────┬───────────────────────┘
                                                     │
                                            (Airflow Orchestrator)
                                                     │
                                    ┌────────────────▼───────────────────────┐
                                    │           CAPA OLAP                    │
                                    │                                       │
                                    │  ┌──────────┐    ┌──────────┐        │
                                    │  │   HDFS   │◄───│  Spark   │        │
                                    │  │(Namenode │    │(Master + │        │
                                    │  │+Datanode)│    │  Worker) │        │
                                    │  └────┬─────┘    └────┬─────┘        │
                                    │       │               │              │
                                    │  ┌────▼─────┐   ┌────▼─────┐        │
                                    │  │   Hive   │   │ Superset │        │
                                    │  │(Esquema  │   │ (3 Dash- │        │
                                    │  │ Estrella)│   │  boards) │        │
                                    │  └──────────┘   └──────────┘        │
                                    └──────────────────────────────────────┘
```

### Flujo de Datos Extremo a Extremo

```
Operaciones Diarias (OLTP)
       │
       ▼
  export_data.py ───→ CSVs de staging
       │
       ├──→ export_neo4j.py + load_neo4j.py ───→ Neo4j (Grafos)
       │
       └──→ etl_to_star.py (Spark) ───→ Hive (Esquema Estrella)
                                                │
                                          ┌─────┼─────┐
                                          ▼     ▼     ▼
                                    Tendencias  Horarios  Crecimiento
                                          │
                                    ┌─────┘
                                    ▼
                              Superset Dashboards
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

### Esquema Estrella vs Copo de Nieve (OLAP)

Se optó por **esquema estrella** porque las dimensiones son planas sin jerarquías profundas. Esto simplifica las consultas (menos JOINs) y mejora el rendimiento en Hive, optimizado para lecturas secuenciales sobre tablas desnormalizadas.

### Spark vs Python Puro para Transformación

La transformación a modelo estrella se realiza con **PySpark** por tres razones:
1. **Procesamiento distribuido**: Escala horizontalmente añadiendo workers
2. **Escritura directa a Hive**: `df.write.saveAsTable()` evita scripts de carga intermedios
3. **Broadcast hash joins**: Dimensiones pequeñas se transmiten a todos los nodos, evitando shuffles

### SCD Tipo 0 (No Historizar)

Se implementó Slowly Changing Dimension Tipo 0 porque los datos provienen de un sistema transaccional donde las dimensiones representan el estado actual. Para un sistema en producción con historización, se migraría a SCD Tipo 2 agregando `valid_from` y `valid_to`.

### Particionamiento de Fact Tables

Las tablas de hechos se particionan por `year, month` para pruning de particiones, mantenibilidad (carga incremental, purga) y rendimiento en consultas temporales.

### Graceful Degradation Generalizada

| Componente | Fallo | Comportamiento |
|---|---|---|
| Redis | Caído | Middleware HTTP omite caché, pasa al handler |
| Neo4j | Caído | Endpoint de rutas responde 503 |
| Elasticsearch | Caído | Búsqueda responde error descriptivo |
| Hive | Caído | Spark jobs fallan con error controlado |
| Airflow worker | Caído | Celery reasigna tareas a otros workers |

## Características

- **API REST** construida en Go 1.24 con Gin framework
- **Múltiples Motores de BD**: PostgreSQL o MongoDB (intercambiables vía `DB_ENGINE`)
- **MongoDB Sharding**: 3 shards con replicación
- **Redis**: Middleware HTTP de caché con graceful degradation
- **Elasticsearch**: Búsqueda de texto completo e indexación
- **Autenticación**: Keycloak con OAuth2/OpenID Connect
- **Autorización**: RBAC (Admin/Client)
- **Nginx**: Proxy inverso para enrutamiento
- **Docker Compose**: Orquestación completa de servicios
- **Hadoop HDFS**: Almacenamiento distribuido para el Data Warehouse
- **Apache Hive 4.0**: SQL sobre Hadoop con esquema estrella
- **Apache Spark 3.5**: Transformación ETL y análisis distribuido (PySpark)
- **Apache Superset 4.1**: BI y visualización con 3 dashboards
- **Apache Airflow 2.8**: Orquestación del pipeline ETL diario (CeleryExecutor)
- **Neo4j 5**: Grafo de conocimiento, co-compra y rutas de entrega
- **Rutas de Entrega**: Endpoint con TSP greedy y Cypher ShortestPath
- **Datos Geoespaciales**: Coordenadas Haversine para ubicaciones
- **Suite de Validación**: 6 scripts verify_* con 20+ checks automáticos

## Configuración Inicial

### 1. Prerrequisitos

- Docker y Docker Compose
- Go 1.24+ (para desarrollo local)
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

# Hive (Data Warehouse)
HIVE_URL=jdbc:hive2://hive-server:10000/default
HIVE_HOST=hive-server
HIVE_PORT=10000

# Hive Metastore
METASTORE_DB_TYPE=postgres
METASTORE_DB_HOST=hive-metastore-db
METASTORE_DB_PORT=5432
METASTORE_DB_DATABASE=metastore
METASTORE_DB_USER=hive
METASTORE_DB_PASSWORD=hive

# Hadoop HDFS
HADOOP_FS=hdfs://hadoop-namenode:9000
HADOOP_NAMENODE_HOST=hadoop-namenode
HADOOP_NAMENODE_PORT=9000
HADOOP_NAMENODE_HTTP_PORT=9870

# Hive Metastore Thrift
HIVE_METASTORE_HOST=hive-metastore
HIVE_METASTORE_PORT=9083

# Neo4j (Grafos)
NEO4J_URI=bolt://neo4j:7687
NEO4J_USER=neo4j
NEO4J_PASSWORD=password
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
- MongoDB Sharded (puerto 27017, 3 shards + configsvr + mongos)
- Keycloak (puerto 8080)
- Redis (puerto 6379)
- Elasticsearch (puerto 9200)
- Kibana (puerto 5601)
- Adminer (puerto 8081)
- Nginx Proxy (puerto 80)
- Hadoop HDFS (Namenode puerto 9870/9000, Datanode)
- Hive Metastore (puerto 9083) + HiveServer2 (puerto 10000)
- Spark Master (UI puerto 8083, workers puerto 7077)
- Superset (puerto 8088)
- Airflow Webserver (puerto 8085) + Scheduler + Worker (Celery)
- Neo4j (HTTP puerto 7474, Bolt puerto 7687)

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
| `GET` | `/api/delivery/routes` | Admin | Ruta de entrega (TSP greedy o Cypher) |

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

### Rutas de Entrega

#### GET /api/delivery/routes

Calcula la ruta óptima de entrega desde un restaurante hasta la ubicación de un pedido.

- **Auth:** JWT + Rol Admin
- **Parámetros de consulta:**

| Parámetro | Tipo | Requerido | Descripción |
|-----------|------|-----------|-------------|
| `desde` | string | sí | ID del restaurante (origen) |
| `hasta` | string | sí | ID del pedido (destino) |
| `algoritmo` | string | no | `nearest` (TSP greedy) o `cypher` (ShortestPath en Neo4j) |

**Respuesta 200 OK:**
```json
{
  "ruta": [
    {"direccion": "San José", "lat": 9.93, "lon": -84.09, "orden": 0},
    {"direccion": "Escazú", "lat": 9.94, "lon": -84.14, "orden": 1}
  ],
  "distancia_total_km": 5.7,
  "algoritmo_usado": "nearest"
}
```

**Errores:**

| Código | Cuerpo | Causa |
|--------|--------|-------|
| `400` | `{"error": "..."}` | Parámetros faltantes o inválidos |
| `503` | `{"error": "Servicio de enrutamiento no disponible"}` | Neo4j no disponible |

**Ejemplo:**
```bash
TOKEN=$(curl -s -X POST http://localhost/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@mail.com","password":"admin"}' | jq -r '.access_token')

# Algoritmo Vecino Más Cercano
curl -s "http://localhost/api/delivery/routes?desde=rest-001&hasta=order-050&algoritmo=nearest" \
  -H "Authorization: Bearer $TOKEN"

# Algoritmo Cypher ShortestPath
curl -s "http://localhost/api/delivery/routes?desde=rest-001&hasta=order-050&algoritmo=cypher" \
  -H "Authorization: Bearer $TOKEN"
```

---

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
| Hadoop HDFS | http://localhost:9870 | - | - |
| Spark Master UI | http://localhost:8083 | - | - |
| Superset | http://localhost:8088 | admin | admin |
| Airflow Webserver | http://localhost:8085 | admin | admin |
| Neo4j Browser | http://localhost:7474 | neo4j | password |
| Neo4j Bolt | bolt://localhost:7687 | neo4j | password |

## Estructura del Proyecto

```
.
├── api/
│   └── main.go                          # Servidor API (Gin)
├── handlers/
│   ├── auth.go                          # Login, RegisterUser
│   ├── restaurant.go                    # ListRestaurants, CreateRestaurant
│   ├── menu.go                          # CRUD menú
│   ├── reservation.go                   # CreateReservation, CancelReservation
│   ├── order.go                         # CreateOrder, GetOrder
│   ├── user.go                          # GetCurrentUser, UpdateUser, DeleteUser
│   ├── search.go                        # Búsqueda Elasticsearch
│   ├── delivery.go                      # Rutas de entrega (TSP, Cypher)
│   ├── helpers.go                       # respondError, respondJSON, helpers auth
│   └── *_test.go                        # Tests (mock repos)
├── repository/
│   ├── interface.go                     # Interfaces (User, Restaurant, Menu, etc.)
│   ├── factory.go                       # Factory Pattern (postgres vs mongo)
│   ├── redis.go                         # Conexión Redis + constantes de claves
│   ├── neo4j.go                         # Cliente Neo4j (Bolt)
│   ├── postgres/                        # Implementación PostgreSQL
│   │   └── restaurant.go
│   ├── mongo/                           # Implementación MongoDB
│   │   ├── restaurant.go                # (incluye helpers toString, toFloat64, toInt)
│   │   └── repository_test.go           # 55 tests, 94.3% cobertura
│   └── factory_test.go
├── models/
│   └── models.go                        # User, Restaurant, MenuItem, etc.
├── middleware/
│   ├── auth.go                          # Auth + RequireAdmin (Keycloak JWT)
│   ├── redis_cache.go                   # Cache middleware (GET + invalidation)
│   ├── redis_cache_test.go              # Tests (miniredis, 15 tests)
│   └── logger.go                        # Request logger (ID, método, path, status)
├── elasticsearch/
│   ├── client.go                        # Conexión ES + creación índice
│   ├── indexer.go                       # Indexación de platos
│   └── mock.go                          # Mock para tests
├── keycloak/
│   ├── interfaces.go                    # ClientInterface (para mockear)
│   └── setup.go                         # Configuración realm, client, roles
├── mocks/
│   └── mock.go                          # Mocks testify de todos los repositorios
├── search/
│   └── main.go                          # Microservicio de búsqueda
├── nginx/
│   └── nginx.conf                       # Configuración Nginx
├── hadoop/
│   └── core-site.xml                    # Configuración HDFS para Hive
├── hive/
│   └── hive-site.xml                    # Configuración Hive Metastore
├── airflow/
│   ├── Dockerfile                       # Airflow 2.8 + Spark + providers
│   ├── config/
│   │   └── connections.yaml             # Conexiones pre-configuradas
│   └── dags/
│       └── etl_pipeline.py              # DAG diario (7 tareas, CeleryExecutor)
├── spark/
│   ├── etl_to_star.py                   # Transformación a esquema estrella
│   ├── analisis_tendencias.py           # Tendencias de consumo
│   ├── analisis_horarios.py             # Horarios pico
│   ├── analisis_crecimiento.py          # Crecimiento mensual (LAG, %)
│   ├── submit_jobs.sh                   # Orquestación local de jobs
│   ├── hive-site.xml                    # Config Hive para Spark
│   ├── spark-defaults.conf              # Config Spark (Kryo, adaptive)
│   └── exports/                         # Dashboards Superset exportados (3 JSON)
├── superset/
│   ├── Dockerfile.superset              # Imagen con drivers pyhive, thrift
│   ├── superset_config.py               # Config PostgreSQL metadata
│   ├── init_superset.sh                 # One-shot: create admin, db upgrade
│   ├── setup_dashboards.py              # Crea 3 dashboards vía API REST
│   ├── requirements.txt                 # Dependencias Python
│   └── dashboards/                      # Dashboard JSON exports
├── neo4j/
│   ├── import_data.py                   # Exporta OLTP → CSVs de nodos/relaciones
│   ├── load_schema.cypher               # Constraints, índices, LOAD CSV
│   ├── consultas.cypher                 # Co-compra, afinidad, shortestPath
│   ├── init_neo4j.sh                    # Orquestador import + load
│   └── staging/                         # CSVs para LOAD CSV
├── warehouse/
│   ├── ddl/                             # DDLs del esquema estrella (9 tablas)
│   │   ├── dim_date.hql
│   │   ├── dim_user.hql
│   │   ├── dim_restaurant.hql
│   │   ├── dim_product.hql
│   │   ├── dim_order_status.hql
│   │   ├── dim_reservation_status.hql
│   │   ├── fact_orders.hql
│   │   ├── fact_order_details.hql
│   │   └── fact_reservations.hql
│   ├── olap/                            # Vistas OLAP (6 vistas)
│   │   ├── v_ingresos_mensuales.hql
│   │   ├── v_ventas_por_categoria.hql
│   │   ├── v_reservas_por_ubicacion.hql
│   │   ├── v_tendencias_mensuales.hql
│   │   ├── v_horarios_pico.hql
│   │   └── v_productividad_usuarios.hql
│   ├── etl/
│   │   ├── export_data.py               # Extrae datos (MongoDB/PostgreSQL → CSVs)
│   │   ├── export_neo4j.py              # Exporta CSVs para Neo4j
│   │   ├── load_neo4j.py                # Carga en Neo4j vía HTTP API
│   │   ├── run_etl.py                   # Orquestador local
│   │   └── staging/                     # CSVs temporales de staging
│   ├── init_hive.sh                     # Ejecuta DDLs contra HiveServer2
│   ├── init_olap_views.sh               # Crea las 6 vistas OLAP
│   ├── requirements.txt                 # Dependencias Python
│   └── .env                             # Variables de Hive/Hadoop
├── scripts/
│   ├── generate_geo.py                  # Coordenadas Haversine para restaurantes
│   ├── generate_oltp_data.py            # Generador de datos masivos (500+ pedidos)
│   ├── verify_all.sh                    # Orquestador maestro de validación
│   ├── verify_warehouse.py              # 5 checks: conteos, orphans, consistencia
│   ├── verify_etl.sh                    # 3 checks: ejecución, idempotencia, degradation
│   ├── verify_spark.sh                  # 3 checks: jobs, CSV outputs, validación cruzada
│   ├── verify_neo4j.sh                  # 4 checks: co-compra, afinidad, shortestPath
│   └── verify_airflow.sh               # 4 checks: DAG, trigger, logs, dependencias
├── docs/
│   └── documentacion_olap.md            # Documentación técnica OLAP
├── docker-compose.yml                   # Orquestación (30+ servicios)
├── Dockerfile                           # Build API
├── Dockerfile.search                    # Build search microservice
├── init.sql                             # Inicialización PostgreSQL
├── initMongo.js                         # Inicialización MongoDB sharding
├── init-mongo.sh                        # Script bash init MongoDB
├── init-mongo.ps1                       # Script PowerShell init MongoDB
└── .env                                 # Variables de entorno
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

### Data Warehouse (Hive)

```bash
# Inicializar esquema estrella (9 tablas DDL)
docker exec hive-server bash /opt/warehouse/init_hive.sh

# Crear vistas OLAP (6 vistas)
docker exec hive-server bash /opt/warehouse/init_olap_views.sh

# Consultar una vista
docker exec hive-server beeline -u jdbc:hive2://localhost:10000/default \
  -e "SELECT * FROM v_ingresos_mensuales ORDER BY year, month LIMIT 10;" \
  --outputformat=csv2

# Ver tablas en Hive
docker exec hive-server beeline -u jdbc:hive2://localhost:10000/default \
  -e "SHOW TABLES;"
```

### Spark

```bash
# Enviar todos los jobs
bash spark/submit_jobs.sh

# Job individual
docker exec spark-master spark-submit \
  --master spark://spark-master:7077 \
  /opt/scripts/etl_to_star.py

# Ver resultados de análisis
ls spark/output/tendencias/
ls spark/output/horarios/
ls spark/output/crecimiento/
```

### Superset

```bash
# Inicializar
docker-compose run --rm superset-init

# Crear dashboards
docker exec superset pip install -r /app/requirements.txt
docker exec superset python /app/setup_dashboards.py

# Acceder: http://localhost:8088 (admin/admin)
```

### Airflow

```bash
# Trigger manual del DAG
docker exec airflow-webserver airflow dags trigger etl_pipeline

# Ver ejecuciones
docker exec airflow-webserver airflow dags list-runs -d etl_pipeline

# Ver logs de una tarea
docker exec airflow-webserver airflow tasks logs etl_pipeline extract_data 2025-03-15

# Acceder: http://localhost:8085 (admin/admin)
```

### Neo4j

```bash
# Cargar datos
bash neo4j/init_neo4j.sh

# Consultas Cypher
docker exec neo4j cypher-shell -u neo4j -p password \
  "MATCH (p1:Product)<-[:INCLUDES]-(o:Order)-[:INCLUDES]->(p2:Product)
   WHERE id(p1) < id(p2)
   RETURN p1.nombre, p2.nombre, count(DISTINCT o) AS veces_juntos
   ORDER BY veces_juntos DESC LIMIT 5;"

# Acceder: http://localhost:7474 (neo4j/password)
```

### Población de Datos

```bash
# Generar datos masivos (500+ pedidos, 200+ reservas)
docker exec api python scripts/generate_oltp_data.py

# Generar coordenadas geoespaciales
docker exec api python scripts/generate_geo.py
```

### Validación Completa

```bash
# Suite completa
bash scripts/verify_all.sh

# Validaciones individuales
python scripts/verify_warehouse.py
bash scripts/verify_etl.sh
bash scripts/verify_spark.sh
bash scripts/verify_neo4j.sh
bash scripts/verify_airflow.sh
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

### HiveServer2 no abre puerto 10000
```bash
# Verificar que HDFS no esté en safemode
docker exec hadoop-namenode hdfs dfsadmin -safemode leave

# Verificar hive-site.xml tiene metastore.uris correcto
docker exec hive-metastore cat /opt/hive/conf/hive-site.xml | grep metastore.uris

# Ver logs
docker-compose logs hive-server
```

### Hive no encuentra datos (queries vacías)
```bash
# Verificar core-site.xml con fs.defaultFS
docker exec hive-server cat /opt/hadoop/etc/hadoop/core-site.xml

# Verificar tablas en HDFS
docker exec hadoop-namenode hdfs dfs -ls /user/hive/warehouse/

# Reparar particiones
docker exec hive-server beeline -u jdbc:hive2://localhost:10000/default \
  -e "MSCK REPAIR TABLE fact_orders;"
```

### Spark no escribe en Hive
```bash
# Verificar hive-site.xml en Spark
docker exec spark-master cat /opt/spark/conf/hive-site.xml

# Verificar conectividad con metastore
docker exec spark-master spark-sql \
  --conf spark.hadoop.hive.metastore.uris=thrift://hive-metastore:9083 \
  -e "SHOW TABLES;"
```

### Superset: Error "CSRF token missing"
Agregar en `superset/superset_config.py`:
```python
WTF_CSRF_ENABLED = False
```

### Superset: "Missing label" en charts
Los parámetros `groupby` deben ser array de strings (`["year"]`), no objetos anidados. Los `metrics` no deben incluir campo `"type"` dentro de `column`.

### Superset: Hive no se conecta (ParseException SHOW CATALOGS)
```bash
# Instalar parche para HiveEngineSpec
docker exec superset pip install pyhive thrift thrift-sasl
```

### Neo4j no carga CSVs
```bash
# Verificar permisos de staging
docker exec neo4j ls -la /var/lib/neo4j/import/

# Verificar conectividad Bolt
docker exec neo4j cypher-shell -u neo4j -p password "RETURN 1;"
```

### Airflow DAG falla
```bash
# Verificar conexiones configuradas
docker exec airflow-webserver airflow connections list

# Reimportar conexiones
docker-compose run --rm airflow-init

# Ver logs del scheduler
docker-compose logs airflow-scheduler

# Reiniciar scheduler
docker-compose restart airflow-scheduler
```

### Spark Thrift Server: UNRESOLVED_COLUMN en Superset
Verificar que los nombres de columna en las SQL queries de los datasets virtuales coincidan exactamente con los nombres en las tablas Hive (ej: `status_name` en vez de `descripcion`).

## Arquitectura OLAP

### Data Warehouse — Esquema Estrella

Se implementó un Data Warehouse en **Apache Hive 4.0** sobre **Hadoop HDFS** con modelo de esquema estrella (6 dimensiones + 3 tablas de hechos). Las tablas se particionan por `year, month`.

**Tablas de Dimensión:**

| Tabla | Columnas | Población |
|-------|----------|-----------|
| `dim_date` | date_key, full_date, year, quarter, month, day, weekday, is_weekend | Automática desde fechas en datos |
| `dim_user` | user_key, user_id, nombre, email, role_name | Desde OLTP |
| `dim_restaurant` | restaurant_key, restaurant_id, nombre, direccion, ciudad, telefono | Desde OLTP |
| `dim_product` | product_key, product_id, nombre, categoria, precio | Desde OLTP |
| `dim_order_status` | status_key, status_name | Sembrada: completado, cancelado |
| `dim_reservation_status` | status_key, status_name | Sembrada: pendiente, confirmada, cancelada |

**Tablas de Hechos:**

| Tabla | Particiones | Métricas |
|-------|------------|----------|
| `fact_orders` | year, month | total_amount, item_count, hora |
| `fact_order_details` | year, month | quantity, unit_price, subtotal |
| `fact_reservations` | year, month | party_size |

**6 Vistas OLAP:** `v_ingresos_mensuales`, `v_ventas_por_categoria`, `v_reservas_por_ubicacion`, `v_tendencias_mensuales`, `v_horarios_pico`, `v_productividad_usuarios`.

## Pipeline ETL

El pipeline ETL diario transforma datos operacionales (MongoDB o PostgreSQL) en el modelo analítico.

```
export_data.py (Python)
      │
      │  Detecta DB_ENGINE, conecta a MongoDB/PostgreSQL
      │  Extrae: usuarios, restaurantes, platos, pedidos, detalles, reservas
      ▼
  CSVs de staging (warehouse/etl/staging/)
      │
      │  usuarios.csv, restaurantes.csv, platos.csv,
      │  pedidos.csv, detalles_pedido.csv, reservas.csv
      ▼
  etl_to_star.py (Spark)
      │
      │  1. spark.read.csv() con inferSchema
      │  2. Genera surrogate keys (row_number)
      │  3. Broadcast mappings (original_id → surrogate_key)
      │  4. Joins para fact tables
      │  5. Escribe en Hive con saveAsTable()
      ▼
  Hive (Esquema Estrella) → Vistas OLAP
```

**Exportación a Neo4j** (paralelo): `export_neo4j.py` + `load_neo4j.py` transforman los mismos datos en CSVs de nodos/relaciones y los cargan vía LOAD CSV + Cypher.

## Procesamiento con Spark

**Infraestructura:** Spark Master (UI puerto 8083) + Workers escalables (2 cores, 2 GB RAM c/u). Configuración con Kryo serializer, adaptive query execution y conexión a Hive Metastore.

**4 Jobs PySpark:**

| Job | Script | Descripción | Output |
|-----|--------|-------------|--------|
| ETL Estrella | `etl_to_star.py` | Transforma CSVs → esquema estrella en Hive | Tablas Hive |
| Tendencias | `analisis_tendencias.py` | Ingresos por mes, top 10 productos, categoría por trimestre | CSV en `spark/output/tendencias/` |
| Horarios | `analisis_horarios.py` | Pedidos por hora y día de semana, ranking de demanda | CSV en `spark/output/horarios/` |
| Crecimiento | `analisis_crecimiento.py` | Crecimiento % mensual con LAG(), tasa de cancelación | CSV en `spark/output/crecimiento/` |

```bash
# Orquestación local
bash spark/submit_jobs.sh

# O manualmente
docker exec spark-master spark-submit --master spark://spark-master:7077 /opt/scripts/etl_to_star.py
docker exec spark-master spark-submit --master spark://spark-master:7077 /opt/scripts/analisis_tendencias.py &
docker exec spark-master spark-submit --master spark://spark-master:7077 /opt/scripts/analisis_horarios.py &
docker exec spark-master spark-submit --master spark://spark-master:7077 /opt/scripts/analisis_crecimiento.py &
wait
```

## Visualización con Superset

**Infraestructura:** Apache Superset 4.1.1 (imagen extendida con drivers `pyhive`, `thrift`, `thrift-sasl`). Conexión a Hive vía `hive://hive-server:10000/default`.

```bash
# Inicializar
docker-compose up -d superset-db
docker-compose run --rm superset-init
docker-compose up -d superset

# Crear dashboards
docker exec superset pip install -r /app/requirements.txt
docker exec superset python /app/setup_dashboards.py
```

**3 Dashboards (9 charts, creados vía API REST):**

| Dashboard | Datasets | Charts |
|-----------|----------|--------|
| **Ingresos por Mes y Categoría** | `v_ingresos_mensuales`, `v_ventas_por_categoria` | Barras apiladas, línea tendencia, top 5 categorías |
| **Actividad por Zona Geográfica** | `v_reservas_por_ubicacion`, `dim_user` | Barras por ciudad, distribución, Big Number |
| **Pedidos Completados vs Cancelados** | `v_tendencias_mensuales`, `fact_orders` | Time-series, pie chart, tasa cancelación |

## Orquestación con Airflow

**Infraestructura:** Apache Airflow 2.8 con CeleryExecutor, Redis como broker. Imagen extendida con Spark 3.5.0 y providers para Spark, Celery, MongoDB, Hive, Elasticsearch.

```bash
# Inicializar
docker-compose up -d airflow-db
docker-compose run --rm airflow-init
docker-compose up -d airflow-webserver airflow-scheduler airflow-worker
```

**DAG `etl_pipeline`** (ejecución diaria):

```
extract_data (export_data.py)
    │
    ├──→ export_neo4j → load_neo4j (CSVs → Neo4j)
    │
    └──→ etl_to_star (Spark → Hive)
              │
        ┌─────┼─────┐
        ▼     ▼     ▼
   Tendencias  Horarios  Crecimiento
   (Spark)    (Spark)    (Spark)
```

7 tareas con CeleryExecutor, 2 reintentos con backoff de 5 min, reindexación condicional de Elasticsearch vía MD5 hash del archivo `platos.csv`.

```bash
# Trigger manual
docker exec airflow-webserver airflow dags trigger etl_pipeline
```

## Análisis de Grafos con Neo4j

**Infraestructura:** Neo4j 5 Community con plugin APOC, puertos 7474 (HTTP Browser) y 7687 (Bolt).

**Modelo de Grafo (6 tipos de nodos, 6 relaciones):**

```
(:User)────[:PLACED]────>(:Order)────[:INCLUDES]────>(:Product)
                                            │
                                            └────[:AT]────>(:Restaurant)────[:LOCATED_AT]────>(:Location)
                                                                                                       │
                                                                                                     [:ROAD]
                                                                                                       │
                                                                                               (:Location)
```

**Consultas Cypher implementadas:**

| Consulta | Descripción | Algoritmo |
|----------|-------------|-----------|
| Co-compra | Top 5 pares de productos comprados juntos | `MATCH` con `WHERE id(p1) < id(p2)` |
| Afinidad | Usuarios que recomendar a otros | Score: `3×restaurantes + 1×productos` compartidos |
| ShortestPath | Ruta más corta entre ubicaciones | `shortestPath((a)-[:ROAD*]-(b))` con Haversine |
| Influencia | Usuarios más activos (PageRank) | Centralidad por grado (alternativa GDS) |

## Rutas de Entrega

**Datos Geoespaciales:** El script `scripts/generate_geo.py` genera coordenadas Haversine ficticias para restaurantes en San José, Costa Rica (9.92-9.94°N, -84.08--84.10°O), relaciones `[:ROAD]` entre ubicaciones cercanas (< 5 km) con distancia calculada, y zonificación por grilla de 2×2 km.

**Endpoint REST:** `GET /api/delivery/routes?desde={id}&hasta={id}&algoritmo=nearest|cypher`

- **Vecino Más Cercano (TSP Greedy):** Implementado en Go puro en `handlers/delivery.go`. Desde el origen, visita siempre la ubicación no visitada más cercana.
- **Cypher ShortestPath:** Delega a Neo4j con `shortestPath((a)-[:ROAD*]-(b))` para la ruta óptima en el grafo.

Graceful degradation: si Neo4j no responde, el endpoint retorna HTTP 503.

## Suite de Validación

La carpeta `scripts/` contiene 6 scripts de validación automatizada con 20+ checks:

| Script | Checks | Descripción |
|--------|--------|-------------|
| `verify_warehouse.py` | 5 | Conteos OLTP vs Hive, orphans, consistencia total_amount, unicidad surrogate keys |
| `verify_etl.sh` | 3 | Ejecución sin errores, idempotencia, graceful degradation |
| `verify_spark.sh` | 3 | 3 jobs sin excepción, 7 outputs CSV, validación cruzada |
| `verify_neo4j.sh` | 4 | Co-compra, afinidad, shortestPath, centralidad |
| `verify_airflow.sh` | 4 | DAG existe, trigger + polling, logs sin errores, árbol dependencias |
| `verify_all.sh` | Orquestador | Ejecuta los 5 anteriores, tabla resumen PASS/FAIL |

```bash
# Suite completa
bash scripts/verify_all.sh

# Validación individual
python scripts/verify_warehouse.py
bash scripts/verify_spark.sh
```

## Plan de Ejecución

El despliegue se realiza en 8 fases secuenciales respetando el árbol de dependencias:

```
Fase 0: Preparación (mkdir, .env, docker info)
Fase 1: Infraestructura Base (Postgres/Mongo, Keycloak, Redis, ES, Kibana)
Fase 2: Infraestructura OLAP (HDFS, Hive, Spark, esquema estrella, vistas)
Fase 3: BI y Orquestación (Superset, Airflow)
Fase 4: Grafos y Geo (Neo4j, generate_geo, init_neo4j)
Fase 5: API, Search y Nginx
Fase 6: Población Masiva (generate_oltp_data)
Fase 7: ETL y Análisis (export_data → Spark → Hive)
Fase 8: Verificación (verify_all.sh)
```

**Orden de arranque (dependencias entre servicios):**

```
db ──→ keycloak-db ──→ keycloak
  │
  ├──→ elasticsearch ──→ kibana ──→ api ──→ nginx
  │                              └── search
  ├──→ redis
  │
  ├──→ hadoop-namenode ──→ hadoop-datanode ──→ hive-metastore ──→ hive-server
  │                                                                     │
  │                                                     spark-master ──→┘
  │                                                         │
  │                                                     spark-worker
  │
  ├──→ superset-db ──→ superset-init ──→ superset
  │
  ├──→ airflow-db ──→ airflow-init ──→ airflow-webserver
  │                                    ├── airflow-scheduler
  │                                    └── airflow-worker
  │
  └──→ neo4j
```

## Documentación Adicional
- [REDIS_CONFIG.md](REDIS_CONFIG.md) — Configuración avanzada de Redis como middleware
- [planRedis.md](planRedis.md) — Plan de migración de decorators a middleware HTTP
