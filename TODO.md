# TODO — Restaurantes API (BD2-P01)

## 📋 Fase 1: Refactorización (Feedback Recibido)

### 1. Redis como Middleware HTTP (en lugar de Decorator)

**Objetivo:** Migrar del patrón Decorator en repositorios a un middleware Gin que intercepte requests HTTP. Cachear responses GET completas. **Graceful degradation:** si Redis falla, la API sigue funcionando sin interrupción.

- [x] **Crear `middleware/redis_cache.go`** — Middleware que:
  - Intercepte requests GET, guarde/recupere responses completas en Redis
  - Invalide caché en POST/PUT/DELETE según la ruta
  - Wrap `gin.ResponseWriter` para capturar el body de la response
  - Timeout corto (500ms) para no bloquear el handler

- [x] **Eliminar decorators actuales** (`repository/restaurant_redis.go`, `user_redis.go`, `menu_redis.go`) — La lógica de caché pasa al middleware

- [x] **Simplificar `repository/redis.go`** — Mantener solo conexión Redis y constantes de claves

- [x] **Actualizar `repository/factory.go`** — Ya no aplicar decorators sobre repositorios

- [x] **Registrar middleware en `api/main.go`**

- [x] **Tests:** `middleware/redis_cache_test.go` con cliente Redis mockeado (cache hit, cache miss, Redis caído, invalidación)

---

### 2. Código más Estructurado y Profesional

#### 2.1 Limpieza de Legacy

- [x] Eliminar `repository/plato_repository.go` (tiene tipos duplicados `PlatoRepository` y `Plato` que chocan con `MenuRepository`)
- [x] Eliminar `repository/plato_redis.go` (decorator legacy que usa `PlatoRepository`)
- [x] Eliminar `repository/plato_postgres.go` (implementación independiente, no se usa en factory)
- [x] Eliminar `models/plato.go` (estructura `Plato` duplicada de `models.MenuItem`)
- [x] Eliminar `handlers/reservations.go` (archivo vacío, solo `package handlers`)

#### 2.2 Consistencia

- [x] Renombrar paquete `repository/mongodb` → `repository/mongo` (en `factory.go` se importa como `mongoRepo "restaurante/repository/mongodb"` pero el `package` dentro declara `mongo`)
- [x] ~~Corregir keys de tests: en `restaurant_redis_test.go` se usa `restaurants:all` pero la constante real es `KeyRestaurantList = "restaurants:list"`~~ *(archivo eliminado en 2.1)*

#### 2.3 Separación de Responsabilidades

- [x] Mover handlers a archivos por dominio:
  - `handlers/restaurant.go` → solo `ListRestaurants`, `CreateRestaurant`
  - `handlers/menu.go` → `GetMenu`, `CreateMenu`, `UpdateMenu`, `DeleteMenu`
  - `handlers/reservation.go` → `CreateReservation`, `CancelReservation`
  - `handlers/order.go` → `CreateOrder`, `GetOrder`
  - `handlers/user.go` → `GetCurrentUser`, `UpdateUser`, `DeleteUser`
  - `handlers/auth.go` → `Login`, `RegisterUser`

#### 2.4 Mejoras Arquitectónicas

- [x] Estandarizar manejo de errores: crear helpers `respondError(c, status, msg)` y `respondJSON(c, status, data)`
- [x] Extraer lógica repetitiva de autorización (`adminID != userID`) a helpers (`requireSameUser`, `requireAdminOfRestaurant`, `requireAdminOfMenuItem`, `requireOwnerOfReservation`)
- [x] Agregar logs estructurados con request ID y contexto (`middleware/logger.go` + RequestLogger en `api/main.go`)

---

### 3. Tests Unitarios

#### 3.1 Tests de Redis Middleware

- [x] Tests en `middleware/redis_cache_test.go` con `miniredis` (15 tests):
  - Cache hit, cache miss, status/content-type preservation
  - Invalidación POST/PUT/DELETE, skip on non-2xx
  - Graceful degradation: Redis caído, cliente nil, entrada corrupta
  - SkipPaths, NonGET, rutas sin regla de invalidación

#### 3.2 Tests de Handlers

- [x] Crear interfaz `KeycloakClient` (`keycloak.ClientInterface`) para poder mockear `Login` y `RegisterUser`
- [x] Agregar `TestLogin_Success`, `TestLogin_Unauthorized`, `TestRegisterUser_Success`, `TestRegisterUser_KeycloakConflict`, `TestRegisterUser_UpsertFails`
- [x] Agregar tests faltantes: `TestCreateReservation_DBError`, `TestCancelReservation_DBError` + fix handler (no chequeaba error de `Cancel`)

#### 3.3 Tests de Repositorios MongoDB

- [x] Crear `repository/mongo/repository_test.go` con 55 tests (94.3% cobertura):
  - `UserRepository` (15 tests), `RestaurantRepository` (7 tests), `MenuRepository` (18 tests)
  - `ReservationRepository` (8 tests), `OrderRepository` (4 tests), `toInt` via `GetByID` (6 tests)

#### 3.4 Tests de Factory

- [x] Refactorizar `repository/factory_test.go` — tests de `getEnv` con casos borde (variable vacía, no definida)
- [ ] NOTA: `NewRepos()` no es unit-testeable directamente porque siempre inicializa Elasticsearch (que requiere conexión real). Separar `InitClient` del constructor permitiría tests unitarios.

---

### 4. Documentación

- [x] **README.md**:
  - Diagrama de arquitectura (componentes y flujo)
  - Documentar nuevo patrón: Redis como middleware HTTP
  - Agregar sección "Decisiones Técnicas"
  - Mejorar ejemplos de API con `curl`
  - Documentar transición OLTP → OLAP
  - Agregar API Reference completa

- [x] **REDIS_CONFIG.md**: Actualizar para reflejar middleware HTTP, nuevo formato de claves, graceful degradation, invalidación

- [x] **GoDoc en funciones exportadas**: `models/` (todos los tipos), `repository/interface.go` (interfaces), `repository/factory.go` (Repos, NewRepos), `middleware/redis_cache.go` (tipos exportados), `middleware/auth.go` (RequireAdmin), `middleware/logger.go` (RequestLogger), `elasticsearch/client.go` (InitClient), `keycloak/interfaces.go` (ClientInterface)

---

## 🚀 Fase 2: Análisis y Procesamiento OLAP

### 1. Arquitectura OLAP y Almacén de Datos (20%)

**Esquema:** Estrella (recomendado vs copo de nieve por dimensiones planas sin jerarquías profundas)

#### 1.1 Infraestructura — Hive + Hadoop en Docker
- [ ] Agregar servicios al `docker-compose.yml`:
  - `hadoop-namenode` (HDFS NameNode, puerto 9870)
  - `hadoop-datanode` (HDFS DataNode)
  - `hive-metastore` (con Postgres como backend)
  - `hive-server` (HiveServer2, puerto 10000)
- [ ] Agregar healthchecks, dependencias y volúmenes persistentes para HDFS/Metastore
- [ ] Crear `warehouse/.env` con variables de configuración de Hive

#### 1.2 Esquema Estrella — Tablas Hive (DDL)
- [ ] Crear `warehouse/ddl/dim_date.hql` — dimensión temporal (date_key, full_date, year, quarter, month, day, weekday, is_weekend)
- [ ] Crear `warehouse/ddl/dim_user.hql` — (user_key, user_id, nombre, email, role_name)
- [ ] Crear `warehouse/ddl/dim_restaurant.hql` — (restaurant_key, restaurant_id, nombre, direccion, ciudad, telefono)
- [ ] Crear `warehouse/ddl/dim_product.hql` — (product_key, product_id, nombre, categoria, precio)
- [ ] Crear `warehouse/ddl/dim_order_status.hql` — (status_key, status_name)
- [ ] Crear `warehouse/ddl/dim_reservation_status.hql` — (status_key, status_name)
- [ ] Crear `warehouse/ddl/fact_orders.hql` — (order_id, date_key, user_key, restaurant_key, status_key, total_amount, item_count)
- [ ] Crear `warehouse/ddl/fact_order_details.hql` — (detail_id, order_id, product_key, quantity, unit_price, subtotal)
- [ ] Crear `warehouse/ddl/fact_reservations.hql` — (reservation_id, date_key, user_key, restaurant_key, status_key, party_size)
- [ ] Crear `warehouse/init_hive.sh` — script que ejecuta todas las DDL en orden contra HiveServer2

#### 1.3 Pipeline ETL — Extraer, Transformar y Cargar
- [ ] Crear `warehouse/etl/export_from_mongo.py` — extrae datos desde MongoDB (usuarios, restaurantes, platos, pedidos, reservas) usando PyMongo
- [ ] Crear `warehouse/etl/transform_to_star.py` — transforma a modelo estrella (genera dimensiones, mapea IDs a surrogate keys, calcula métricas)
- [ ] Crear `warehouse/etl/load_to_hive.py` — inserta datos transformados en Hive vía HiveServer2 (PyHive/impyla)
- [ ] Crear `warehouse/etl/run_etl.py` — orquestador que ejecuta export → transform → load secuencialmente

#### 1.4 Cubos/Vistas OLAP (5+)
- [ ] Crear `warehouse/olap/v_ingresos_mensuales.hql` — Ingresos por mes y restaurante
- [ ] Crear `warehouse/olap/v_ventas_por_categoria.hql` — Ventas por categoría de producto
- [ ] Crear `warehouse/olap/v_reservas_por_ubicacion.hql` — Reservas por ubicación y fecha
- [ ] Crear `warehouse/olap/v_tendencias_mensuales.hql` — Tasa de cancelación y crecimiento mensual
- [ ] Crear `warehouse/olap/v_horarios_pico.hql` — Pedidos agrupados por hora del día
- [ ] Crear `warehouse/olap/v_productividad_usuarios.hql` — Top usuarios por frecuencia y monto

### 2. Procesamiento con Apache Spark (20%)
- [ ] Procesar grandes volúmenes de datos de órdenes, productos y reservas
- [ ] Implementar transformaciones con Spark DataFrames y SparkSQL
- [ ] Análisis: Tendencias de consumo
- [ ] Análisis: Horarios pico
- [ ] Análisis: Crecimiento mensual

### 3. Visualización de Datos (15%)
- [ ] Configurar herramienta libre (Apache Superset / Metabase / Redash)
- [ ] Dashboard 1: Ingresos por mes y categoría de producto
- [ ] Dashboard 2: Actividad de clientes por zona geográfica
- [ ] Dashboard 3: Estadísticas de pedidos completados vs cancelados

### 4. Orquestación con Apache Airflow (15%)
- [ ] Diseñar DAG con flujo completo:
  - Extracción: Obtener datos desde MongoDB o PostgreSQL
  - Transformación: Procesar datos con Spark
  - Carga: Insertar datos transformados en el Data Warehouse
- [ ] Tarea de mantenimiento: Reindexar ElasticSearch si se detectan cambios en el catálogo de productos

### 5. Neo4J para Análisis de Grafos y Rutas (15%)
- [ ] Modelar relación usuarios, productos y pedidos en estructura de grafo
- [ ] Identificar patrones de co-compra y usuarios influyentes en el sistema de recomendaciones
- [ ] Simular y optimizar rutas de entrega usando geonodos y relaciones de distancia/tiempo estimado
- [ ] Consulta Cypher: Top 5 productos más comprados juntos
- [ ] Consulta Cypher: Usuarios que recomiendan a otros
- [ ] Consulta Cypher: Caminos mínimos entre ubicaciones para reparto eficiente

### 6. Asignación de Rutas de Entrega (10%)
- [ ] Diseñar módulo para simular rutas de entrega basadas en geolocalización de clientes
- [ ] Implementar heurística (vecino más cercano / consultas Neo4J para caminos óptimos)
- [ ] Mostrar rutas optimizadas y asignaciones correspondientes por repartidor

### 7. Pruebas y Validaciones
- [ ] Verificar integridad y consistencia de datos almacenados en el warehouse
- [ ] Validar ejecución periódica y robustez del pipeline orquestado con Airflow
- [ ] Validar resultados analíticos de Spark y Neo4J mediante consultas y reportes exportables

### 8. Entregables
- [ ] Código fuente completo y definición del DAG de Airflow
- [ ] Scripts o notebooks desarrollados para Spark
- [ ] Dashboards configurados y listos para exportar
- [ ] Consultas Cypher y documentación de la estructura del grafo
- [ ] Capturas de pantalla o video demostrativo del funcionamiento
- [ ] Documentación técnica en PDF (flujo de datos, decisiones arquitectónicas, ejemplos)

---

## Progreso

| Área | Estado |
|------|--------|
| Redis como Middleware | ✅ Completado |
| Limpieza código legacy (decorators Redis + Plato) | ✅ Completado |
| Consistencia (renombrar mongodb → mongo) | ✅ Completado |
| Separación handlers por dominio | ✅ Completado |
| Estandarización errores/logs | ✅ Completado |
| Tests unitarios (Redis middleware) | ✅ Completado |
| Tests repositorios MongoDB (55 tests, 94.3%) | ✅ Completado |
| Tests factory (getEnv con casos borde) | ✅ Completado |
| Documentación (README, REDIS_CONFIG, GoDoc) | ✅ Completado |
| Data Warehouse y OLAP (Hive, esquema estrella, 5+ cubos) | ⬜ Pendiente |
| Procesamiento con Spark (DataFrames, SparkSQL, 3 análisis) | ⬜ Pendiente |
| Visualización (Superset/Metabase/Redash, 3 dashboards) | ⬜ Pendiente |
| Orquestación Airflow (DAG ETL + reindexado ES) | ⬜ Pendiente |
| Neo4J (grafo, co-compra, rutas, consultas Cypher) | ⬜ Pendiente |
| Rutas de Entrega (simulación, heurística, asignación) | ⬜ Pendiente |
| Pruebas y Validaciones OLAP | ⬜ Pendiente |
| Entregables (código, DAG, notebooks, dashboards, PDF) | ⬜ Pendiente |
