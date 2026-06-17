# Proyecto Restaurante - API REST

## Descripción

API REST para gestión de restaurantes, menús, reservas y pedidos.
Construida en Go con Gin framework, soporte PostgreSQL/MongoDB, Redis como middleware de caché,
Elasticsearch para búsqueda, y Keycloak para autenticación OAuth2/JWT.

## Stack

- Go 1.24, Gin Web Framework
- PostgreSQL / MongoDB (seleccionable via DB_ENGINE)
- Redis (caché HTTP middleware)
- Elasticsearch (búsqueda full-text)
- Keycloak (autenticación OIDC/OAuth2)
- Nginx (reverse proxy)

## Endpoints

### Públicos (sin auth)
- `GET  /api/restaurants` — Listar restaurantes
- `GET  /api/menus/:id` — Obtener menú de un restaurante
- `POST /api/auth/login` — Iniciar sesión (body: email, password)
- `POST /api/auth/register` — Registrarse (body: nombre, apellido, email, password)

### Admin (JWT + rol admin)
- `POST   /api/restaurants` — Crear restaurante (body: nombre, direccion, telefono, capacidad_total)
- `POST   /api/menus` — Agregar platos (body: id_restaurante, platos[{nombre, descripcion, precio, categoria}])
- `PUT    /api/menus/:id` — Actualizar plato (body parcial: nombre, descripcion, precio, esta_disponible, categoria)
- `DELETE /api/menus/:id` — Eliminar plato

### Usuarios autenticados (JWT)
- `GET   /api/users/me` — Perfil actual
- `PUT   /api/users/:id` — Actualizar perfil propio
- `DELETE /api/users/:id` — Eliminar cuenta propia
- `POST  /api/reservations` — Crear reserva (body: id_restaurante, fecha_reserva, cantidad_personas)
- `DELETE /api/reservations/:id` — Cancelar reserva propia
- `POST  /api/orders` — Crear pedido (body: id_restaurante, es_recogida, detalles[{id_plato, cantidad}])
- `GET   /api/orders/:id` — Obtener pedido propio

### Búsqueda (microservicio, público)
- `GET  /search/products?q=` — Búsqueda difusa por texto en Elasticsearch
- `GET  /search/products/category/:categoria` — Filtrar por categoría exacta
- `POST /search/reindex` — Reindexar todos los platos en Elasticsearch

## Forma de probar

Tienes dos opciones para hacer requests HTTP a la API:

### Opción 1: rest-api_rest_api_request (MCP tool)
Usa la herramienta `rest-api_rest_api_request` del MCP server `rest-api`. Soporta GET, POST, PUT, PATCH, DELETE con headers personalizados, auth y auto-detección de content-type.

Ejemplos:
- `rest-api_rest_api_request(url="http://localhost/api/restaurants", method="GET")`
- `rest-api_rest_api_request(url="http://localhost/api/auth/login", method="POST", body={"email":"test@mail.com","password":"123"}, headers={"Content-Type": "application/json"})`
- `rest-api_rest_api_request(url="http://localhost/api/restaurants", method="POST", body={"nombre":"Mi Rest","direccion":"San José","capacidad_total":30}, bearerToken="eyJ...")`
- `rest-api_rest_api_request(url="http://localhost/api/users/me", method="GET", bearerToken="eyJ...")`

### Opción 2: curl vía Bash
Usa herramientas Bash para ejecutar curl.

La API corre en http://localhost/api (via Nginx).
El microservicio de búsqueda corre en http://localhost/search.

Para endpoints que requieren JWT, primero haz login a /api/auth/login
y usa el access_token en el header Authorization: Bearer <token>.

---

## Airflow (Orquestación ETL)

Airflow orquesta el pipeline ETL diario. Stack: Airflow 2.8 + CeleryExecutor + Redis (broker).

### Servicios en docker-compose
| Servicio | Puerto | Descripción |
|---|---|---|
| `airflow-db` | — | PostgreSQL 16 para metadatos |
| `airflow-init` | — | One-shot: migra BD, crea admin, importa conexiones |
| `airflow-webserver` | 8085 | UI de Airflow |
| `airflow-scheduler` | — | Scheduler de tareas |
| `airflow-worker` | — | Ejecutor Celery de tareas |

### Inicialización
```bash
docker-compose up -d airflow-db
docker-compose run --rm airflow-init
docker-compose up -d airflow-webserver airflow-scheduler airflow-worker
```

### Acceso
- **UI:** http://localhost:8085
- **Usuario:** `admin` / `admin`

### DAG: `etl_pipeline`
Ejecución diaria con 6 tareas:
```
extract_data (export_data.py)
    ↓
etl_to_star (Spark → Hive)
    ↓
tendencias, horarios, crecimiento (Spark, en paralelo)
    ↓
check_reindex → reindex_elasticsearch (si platos.csv cambió)
             └→ skip_reindex (si no cambió)
```

### Trigger manual
```bash
docker exec airflow-webserver airflow dags trigger etl_pipeline
```

### Ver logs de tareas
```bash
docker exec airflow-webserver airflow tasks logs etl_pipeline <task_id> <execution_date>
```
