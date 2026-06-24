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

**Stack:** Hadoop 3.3.6 + Hive 4.0.0 (imágenes `apache/hadoop`, `apache/hive`)

#### 1.1 Infraestructura — Hive + Hadoop en Docker
- [x] Agregar servicios al `docker-compose.yml`:
  - `hadoop-namenode` (HDFS NameNode, puerto 9870, vol. `hdfs_data`)
  - `hadoop-datanode` (HDFS DataNode, depende de namenode)
  - `hive-metastore` (PostgreSQL como backend, vol. `hive_metastore_data`)
  - `hive-server` (HiveServer2, puerto 10000, depende de metastore)
- [x] Conectar servicios a la red `default` existente para comunicación con API/MongoDB
- [x] Agregar healthchecks, dependencias (`depends_on`) y volúmenes persistentes (`hdfs_data`, `hive_metastore_data`)
- [x] Crear `warehouse/.env` con variables de configuración de Hive (HIVE_URL, METASTORE_DB_URL, HADOOP_FS)

#### 1.2 Esquema Estrella — Tablas Hive (DDL)
- [x] Crear `warehouse/ddl/dim_date.hql` — dimensión temporal (date_key, full_date, year, quarter, month, day, weekday, is_weekend)
- [x] Crear `warehouse/ddl/dim_user.hql` — (user_key, user_id, nombre, email, role_name)
- [x] Crear `warehouse/ddl/dim_restaurant.hql` — (restaurant_key, restaurant_id, nombre, direccion, ciudad, telefono)
- [x] Crear `warehouse/ddl/dim_product.hql` — (product_key, product_id, nombre, categoria, precio)
- [x] Crear `warehouse/ddl/dim_order_status.hql` — (status_key, status_name)
- [x] Crear `warehouse/ddl/dim_reservation_status.hql` — (status_key, status_name)
- [x] Crear `warehouse/ddl/fact_orders.hql` — (order_id, date_key, user_key, restaurant_key, status_key, total_amount, item_count)
- [x] Crear `warehouse/ddl/fact_order_details.hql` — (detail_id, order_id, product_key, quantity, unit_price, subtotal)
- [x] Crear `warehouse/ddl/fact_reservations.hql` — (reservation_id, date_key, user_key, restaurant_key, status_key, party_size)
- [x] Crear `warehouse/init_hive.sh` — script que ejecuta DDLs en orden (dimensiones → hechos) contra HiveServer2

#### 1.3 Pipeline ETL — Extraer, Transformar y Cargar

**Nota:** La transformación a modelo estrella se realiza con **Spark** (job `spark/etl_to_star.py`), no con Python puro. El script Python solo extrae los datos crudos a CSV. Spark lee los CSVs, transforma a estrella y escribe directamente en Hive.

- [x] Refactorizar `warehouse/etl/export_from_mongo.py` → `export_data.py`:
  - Detectar `DB_ENGINE` del .env para elegir fuente
  - Rama MongoDB: conectar via PyMongo, extraer `usuarios`, `restaurantes`, `platos`, `pedidos` (+ detalles), `reservas`
  - Rama PostgreSQL: conectar via `psycopg2`, extraer mismas tablas con `SELECT *`
  - Convertir `_id`/UUID → string, fechas ISO, manejar campos anidados (detalles del pedido)
  - Exportar a CSV temporales en `warehouse/etl/staging/`
  - Eliminado `export_from_mongo.py` original (reemplazado)

- [x] ~~Crear `warehouse/etl/transform_to_star.py`~~ → **Reemplazado por `spark/etl_to_star.py`** (ver punto 2.2). Archivo eliminado.

- [x] ~~Crear `warehouse/etl/load_to_hive.py`~~ → **Ya no es necesario. Spark escribe directo en Hive con `df.write.saveAsTable()`**. Archivo eliminado.

- [x] Actualizar `warehouse/etl/run_etl.py`:
  - Orquestador local que ejecuta: export_data.py (Python) → spark-submit etl_to_star.py (Spark → Hive, si SPARK_HOME está disponible)
  - Mantener logging con timestamp y exit codes
  - En producción, Airflow reemplaza a este orquestador

#### 1.4 Cubos/Vistas OLAP (6 vistas + mejoras)

**Mejoras al esquema base (soporte para hora):**
- [x] Agregar columna `hora INT` a `warehouse/ddl/fact_orders.hql` — necesaria para análisis de horarios pico
- [x] `spark/etl_to_star.py` extrae `HOUR(timestamp)` del pedido y puebla `fact_orders.hora`

**Vistas OLAP (6):**
- [x] Crear `warehouse/olap/v_ingresos_mensuales.hql` — Ingresos, pedidos y ticket promedio por mes + restaurante (JOIN fact_orders + dim_date + dim_restaurant)
- [x] Crear `warehouse/olap/v_ventas_por_categoria.hql` — Unidades vendidas, ingresos y ranking por categoría (RANK() OVER ingresos DESC)
- [x] Crear `warehouse/olap/v_reservas_por_ubicacion.hql` — Reservas y comensales por ciudad + mes (JOIN fact_reservations + dim_date + dim_restaurant)
- [x] Crear `warehouse/olap/v_tendencias_mensuales.hql` — Cancelaciones, usuarios activos y crecimiento % mensual con LAG()
- [x] Crear `warehouse/olap/v_horarios_pico.hql` — Pedidos agrupados por hora real y día de semana (usa columna `hora` de fact_orders)
- [x] Crear `warehouse/olap/v_productividad_usuarios.hql` — Top usuarios con segmentación: VIP (top 20%), Frecuente, Ocasional por percentil de gasto

**Script de inicialización:**
- [x] Crear `warehouse/init_olap_views.sh` — ejecuta los 6 .hql contra HiveServer2 después del esquema base

---

### 2. Procesamiento con Apache Spark (20%)

**Stack:** PySpark (Python) — integración natural con scripts ETL ya planeados. Imágenes `bitnami/spark:3.5`.

#### 2.1 Infraestructura Spark en Docker
- [x] Agregar servicios al `docker-compose.yml`:
  - `spark-master` (imagen `bitnami/spark:3.5`, modo master, puertos 8080 UI, 7077 workers)
  - `spark-worker` (imagen `bitnami/spark:3.5`, modo worker, 2+ réplicas escalables)
- [x] Copiar `hive-site.xml` a Spark conf para integración Hive (acceder a tablas del warehouse)
- [x] Conectar a la red `default` para acceso a Hive Metastore
- [x] Healthcheck en spark-master (`spark-submit --version`) para esperar a que esté listo
- **⚠️ Nota al probar:** Usar `--scale spark-worker=2` para levantar 2 workers (sin swarm, `deploy.replicas` no funciona automáticamente). Ej: `docker-compose up -d --scale spark-worker=2 spark-master spark-worker`

#### 2.2 Jobs PySpark

##### Job 1: Transformación ETL — `spark/etl_to_star.py`
- [x] Crear **`spark/etl_to_star.py`** (reemplaza a `transform_to_star.py` y `load_to_hive.py`):
  - Leer CSVs de `warehouse/etl/staging/` con `spark.read.csv()`
  - Generar dimensiones con surrogate keys (`row_number()`)
  - Construir mapeo `original_id → surrogate_key` con broadcast joins
  - Generar `dim_date` desde fechas en los datos usando SparkSQL
  - Transformar fact tables mapeando FKs con joins
  - Calcular métricas: `total_amount`, `item_count`, `subtotal`
  - **Escribir directamente en Hive** con `df.write.mode("overwrite").saveAsTable("nombre_tabla")`
  - Reparar particiones con `MSCK REPAIR TABLE` vía SparkSQL
  - SCD Tipo 0 (no historizar)

##### Job 2: Análisis de Tendencias — `spark/analisis_tendencias.py`
- [x] Crear **`spark/analisis_tendencias.py`** — Tendencias de consumo:
  - Cargar `fact_orders` + `dim_date` + `dim_product` vía SparkSQL/Hive
  - DataFrames: ingresos por mes, top 10 productos más vendidos, categoría más popular por trimestre
  - Output: CSV en `spark/output/tendencias/`

##### Job 3: Análisis de Horarios — `spark/analisis_horarios.py`
- [x] Crear **`spark/analisis_horarios.py`** — Horarios pico:
  - SparkSQL: extraer `HOUR(fecha)` de pedidos, agrupar por hora y día de semana
  - Identificar ventanas de alta demanda (peak hours)
  - Output: CSV en `spark/output/horarios/`

##### Job 4: Análisis de Crecimiento — `spark/analisis_crecimiento.py`
- [x] Crear **`spark/analisis_crecimiento.py`** — Crecimiento mensual:
  - Window functions de SparkSQL: `LAG()` para mes anterior, cálculo de % de crecimiento
  - Métricas: crecimiento en ingresos, crecimiento en número de pedidos, tasa de cancelación mensual
  - Output: CSV en `spark/output/crecimiento/`

#### 2.3 Script de envío
- [x] Crear `spark/submit_jobs.sh` — script que envía los 4 jobs a `spark-submit` vía `docker exec`:
  - ETL secuencial → 3 análisis en paralelo
  - Usa volumen `./spark:/opt/scripts` en los contenedores Spark

---

### 3. Visualización de Datos con Apache Superset (15%)

**Stack:** Apache Superset (imagen `apache/superset`, puerto 8088). Conexión a Hive y SparkSQL como fuentes de datos.

#### 3.1 Infraestructura
- [x] Agregar servicios al `docker-compose.yml`:
  - `superset-db` (PostgreSQL 16 para metadata de Superset)
  - `superset-init` (one-shot: crea admin, migra BD, inicializa roles)
  - `superset` (imagen extendida `apache/superset:4.1.1`, puerto 8088)
  - Dependencias: superset necesita hive-server (consultas) y superset-db (metadata)
- [x] Crear `superset/Dockerfile.superset`:
  - FROM `apache/superset:4.1.1`
  - Instalar drivers: `pyhive`, `thrift`, `thrift-sasl` para Hive + `psycopg2-binary` para PostgreSQL metadata
  - Configurar `SUPERSET_SECRET_KEY`
- [x] Crear `superset/superset_config.py`:
  - PostgreSQL metadata URI (`postgresql://superset:superset@superset-db:5432/superset`)
  - Conexión a Hive documentada (se agrega vía UI): `hive://hive-server:10000/default`
  - Conexión a SparkSQL comentada (futuro): `hive://spark-master:10000/default`
- [x] Crear `superset/init_superset.sh`:
  - `superset fab create-admin` — usuario admin/admin
  - `superset db upgrade`
  - `superset init`
  - ⚠️ **Post-instalación:** Después de `docker-compose up -d`, ejecutar una vez:
    ```bash
    docker-compose run --rm superset-init
    ```

#### 3.2 Dashboards
- [x] **Dashboard 1:** Ingresos por mes y categoría de producto
  - Gráfico de barras: ingresos mensuales agrupados por categoría
  - Gráfico de líneas: tendencia de ingresos totales (12 meses)
  - Tabla de resumen: top 5 categorías por ingresos
- [x] **Dashboard 2:** Actividad de clientes por zona geográfica
  - Gráfico de barras: reservas/pedidos por ciudad
  - Barras apiladas: distribución de clientes por ciudad
  - Big Number: total clientes activos
- [x] **Dashboard 3:** Estadísticas de pedidos completados vs cancelados
  - Time-series: completados vs cancelados por mes
  - Pie chart: proporción general completado/cancelado
  - Tabla: tasa de cancelación por restaurante
- [x] **Script de creación:** `superset/setup_dashboards.py` — crea todo vía API Superset:
  - Conexión Hive, 9 datasets virtuales, 9 charts, 3 dashboards, export JSON
  - Ejecutar después de tener Superset arriba e inicializado:
    ```bash
    docker exec superset pip install -r /app/requirements.txt
    docker exec superset python /app/setup_dashboards.py
    ```

---

### 4. Orquestación con Apache Airflow (15%)

**Stack:** Apache Airflow 2.8 (imagen `apache/airflow:2.8`). Scheduler + Webserver + Worker. DAG ETL diario.

#### 4.1 Infraestructura
- [x] Agregar servicios al `docker-compose.yml`:
  - `airflow-db` (PostgreSQL 16 para metadatos de Airflow)
  - `airflow-init` (one-shot: migrate, create admin, import connections)
  - `airflow-webserver` (puerto 8085, imagen extendida con dependencias)
  - `airflow-scheduler` (misma imagen, comando scheduler)
  - `airflow-worker` (misma imagen, comando celery worker)
- [x] Crear `airflow/Dockerfile`:
  - FROM `apache/airflow:2.8`
  - Instalar: Spark 3.5.0 + `apache-airflow-providers-apache-spark`, `apache-airflow-providers-celery`, `pymongo`, `pyhive`, `thrift`, `thrift-sasl`, `elasticsearch`, `psycopg2-binary`, `requests`
- [x] Crear conexiones pre-configuradas en `airflow/config/connections.yaml`:
  - `mongo_default`, `hive_default`, `spark_default`, `elasticsearch_default`

#### 4.2 DAG de ETL Completo
- [x] Crear `airflow/dags/etl_pipeline.py` — DAG con ejecución diaria:
  1. **`extract_data`** → BashOperator: ejecuta `warehouse/etl/export_data.py` (extrae de MongoDB o PostgreSQL según `DB_ENGINE`)
  2. **`etl_to_star`** → SparkSubmitOperator: envía `spark/etl_to_star.py` (Spark lee CSVs de staging/, transforma a estrella y escribe en Hive)
  3. **`analisis_tendencias`** → SparkSubmitOperator: envía `spark/analisis_tendencias.py` (en paralelo con los otros análisis)
  4. **`analisis_horarios`** → SparkSubmitOperator: envía `spark/analisis_horarios.py` (en paralelo)
  5. **`analisis_crecimiento`** → SparkSubmitOperator: envía `spark/analisis_crecimiento.py` (en paralelo)
  6. **`reindex_elasticsearch`** → BranchPythonOperator + PythonOperator: reindexado condicional vía MD5 hash
     - Calcular MD5 de `warehouse/etl/staging/platos.csv` después de `extract_data`
     - Comparar con hash almacenado en `warehouse/etl/staging/.last_platos_hash`
     - Si cambió → ejecutar `POST /search/reindex` via requests, persistir nuevo hash en `.last_platos_hash`
     - Si no cambió → skipear la tarea (BranchPythonOperator que la omite)
   - Dependencias: extract_data → etl_to_star → (tendencias, horarios, crecimiento en paralelo) → reindex
   - Retry: 2 reintentos con backoff de 5 min
   - CeleryExecutor con Redis como broker

---

### 5. Neo4J para Análisis de Grafos y Rutas (15%)

**Stack:** Neo4j 5 Community + plugin APOC (imagen `neo4j:5-community`, puertos 7474 HTTP, 7687 Bolt).

#### 5.1 Infraestructura
- [x] Agregar servicio `neo4j` al `docker-compose.yml`:
  - Imagen: `neo4j:5-community`
  - Puertos: 7474:7474 (HTTP Browser), 7687:7687 (Bolt)
  - Variables de entorno: `NEO4J_PLUGINS=["apoc"]`, `NEO4J_AUTH=neo4j/password`
  - Volumen: `neo4j_data` para persistencia
  - Healthcheck con cypher-shell
  - Volumen `./neo4j/staging:/var/lib/neo4j/import:ro` para CSVs

#### 5.2 Modelo de Grafo
- [x] Crear `neo4j/` directorio con:
  - `import_data.py` — script Python que exporta desde MongoDB/PostgreSQL a CSVs de nodos y relaciones
  - `load_schema.cypher` — crea constraints, índices y carga CSVs
  - `init_neo4j.sh` — orquestador que ejecuta import_data.py + load_schema.cypher vía cypher-shell
- [x] **Nodos:** Users, Products, Restaurants, Orders, Reservations (import_data.py) + Locations (generate_geo.py)
- [x] **Relaciones:** PLACED, INCLUDES, AT, MADE_RESERVATION (import_data.py) + LOCATED_AT, ROAD (generate_geo.py)

#### 5.3 Consultas Cypher
- [x] **Top 5 productos más comprados juntos (co-compra):** `neo4j/consultas.cypher` — Co-compra con frecuencia por pares de productos en mismos pedidos
- [x] **Usuarios que recomiendan a otros (afinidad por restaurante + categoría):** `neo4j/consultas.cypher` — Usuarios con preferencias similares
- [x] **Caminos mínimos entre ubicaciones para reparto eficiente:** `neo4j/consultas.cypher` — ShortestPath con parámetros `$origen` y `$destino`
- [x] **Usuarios influyentes (PageRank con GDS / Centralidad por grado):** `neo4j/consultas.cypher` — Ambas variantes documentadas: GDS PageRank (requiere plugin) y Centralidad por grado como alternativa

---

### 6. Asignación de Rutas de Entrega (10%)

#### 6.1 Datos Geoespaciales
- [x] Generar coordenadas ficticias para restaurantes en San José, Costa Rica:
  - Crear `scripts/generate_geo.py` — asigna lat/lon determinísticos vía MD5 hash (9.92-9.94, -84.08--84.10) a cada restaurante
  - Exporta CSVs a `neo4j/staging/`: `locations.csv`, `located_at.csv`, `roads.csv`
  - Almacena en Neo4j como nodos `(:Location)` y en MongoDB como campo `ubicacion`
- [x] Crear relación `[:ROAD]` entre ubicaciones cercanas (< 5km) con distancia Haversine calculada:
  - Generada en `scripts/generate_geo.py` con Haversine + CSVs para LOAD CSV
  - Cargada vía `neo4j/load_schema.cypher` con `MERGE (a)-[:ROAD {distance_km}]->(b)`

#### 6.2 Heurística de Enrutamiento
- [x] Implementar endpoint `GET /api/delivery/routes` (handler en `handlers/delivery.go`):
  - Query params: `desde=restaurant_id`, `hasta=order_id`, `algoritmo=nearest|cypher`
- [x] **Algoritmo Vecino Más Cercano (Go puro):**
  - Obtiene todas las Location desde Neo4j vía `GetAllLocations()`
  - TSP greedy: desde origen, siempre visita la Location más cercana no visitada
  - Retorna `RouteResponse` con ruta ordenada + distancia total
- [x] **Algoritmo Cypher ShortestPath:**
  - Delega a `neo4j.GetShortestPath()` que ejecuta `shortestPath` con `[:ROAD*]`
  - Retorna misma estructura `RouteResponse`
- [x] Respuesta JSON: `{ruta: [{direccion, lat, lon, orden}], distancia_total_km, algoritmo_usado}`
- [x] Asignación por repartidor: función `assignZones()` agrupa ubicaciones por grilla de 2x2km

#### 6.3 Integración con Neo4j desde Go
- [x] Agregar driver Neo4j a `go.mod`: `github.com/neo4j/neo4j-go-driver/v5 v5.27.0`
- [x] Crear `repository/neo4j.go` — cliente singleton con conexión Bolt (GetAllLocations, GetLocationByRestaurant, GetLocationByOrder, GetShortestPath)
- [x] Crear `repository/interface.go` — interfaz `DeliveryRepository` con los 4 métodos
- [x] Crear `handlers/delivery.go` — handler del endpoint `/api/delivery/routes`
- [x] Registrar ruta en `api/main.go` — `GET /delivery/routes` en grupo admin (JWT + RequireAdmin)
- [x] Graceful degradation: si Neo4j no contesta, `NewRepos()` setea `Neo4j = nil` y el endpoint responde 503

---

### 7. Pruebas y Validaciones OLAP

#### 7.1 Validación de Datos en Warehouse
- [x] **`scripts/verify_warehouse.py`** — 4 checks automáticos:
  - `SELECT COUNT(*)` en BD operacional vs Hive (usuarios→dim_user, restaurantes→dim_restaurant, platos→dim_product, pedidos→fact_orders, reservas→fact_reservations)
  - Surrogate keys no nulas y sin huérfanos (9 pares fact→dim)
  - `fact_orders.total_amount = SUM(fact_order_details.subtotal)` por order_id
  - Unicidad de surrogate keys en 6 dimensiones

#### 7.2 Validación del Pipeline ETL
- [x] **`scripts/verify_etl.sh`** — 3 checks:
  - Ejecución completa: `python warehouse/etl/export_data.py` sin errores
  - Idempotencia: 2 corridas seguidas, mismos archivos y filas
  - Graceful degradation: detiene BD, verifica error descriptivo (no crash silencioso)

#### 7.3 Validación de Análisis Spark
- [x] **`scripts/verify_spark.sh`** — 3 checks:
  - Los 3 scripts PySpark se ejecutan sin excepción vía `docker exec spark-submit`
  - 7 outputs CSV existen con header y datos no vacíos
  - Validación cruzada: consulta Hive directo vía beeline para comparación manual

#### 7.4 Validación de Neo4j
- [x] **`scripts/verify_neo4j.sh`** — 4 checks:
  - Co-compra: top 5 pares con productos distintos validados
  - Afinidad entre usuarios por restaurante + categoría
  - ShortestPath: ruta válida entre 2 Location nodes reales
  - Centralidad por grado: top usuarios influyentes

#### 7.5 Validación de Airflow
- [x] **`scripts/verify_airflow.sh`** — 3 checks:
  - Trigger DAG + polling hasta completar (timeout 5 min)
  - Logs de 5 tasks sin errores/exception/traceback
  - Árbol de dependencias: extract → etl → análisis en paralelo

#### 7.6 Prueba de Integración Completa
- [x] **`scripts/verify_all.sh`** — Orquestador maestro que ejecuta las 5 validaciones secuencialmente, tabla resumen PASS/FAIL, exit 0 solo si todas pasan
- [x] **`warehouse/requirements.txt`** — Agregadas dependencias: pyhive, thrift, thrift-sasl, requests

---

### 8. Entregables

| Entregable | Descripción | Ubicación |
|---|---|---|
| Código fuente completo | Todo el código de la Fase 2 integrado al repositorio | `warehouse/`, `spark/`, `superset/`, `airflow/`, `neo4j/` |
| DAG de Airflow | Definición del DAG ETL con 7 tareas | `airflow/dags/etl_pipeline.py` |
| Scripts PySpark | 4 scripts: etl_to_star + 3 análisis con DataFrames + SparkSQL | `spark/` |
| Dashboards Superset | 3 dashboards exportados como JSON para importación | `superset/dashboards/` |
| Consultas Cypher | 4 consultas documentadas con resultados esperados | `neo4j/consultas.cypher` |
| Rutas de Entrega | Endpoint `/api/delivery/routes` con 2 algoritmos | `handlers/delivery.go`, `repository/neo4j.go` |
| Scripts de validación OLAP | 6 scripts automáticos con 20+ checks PASS/FAIL | `scripts/verify_*.py`, `scripts/verify_*.sh` |
| Capturas de pantalla / video | Demostración del sistema funcionando | `docs/capturas/` o externo |
| Documentación técnica PDF | Flujo de datos, decisiones arquitectónicas, ejemplos de uso | `docs/`

---

## 🛠 Cambios Pendientes pre-Ejecución

### 1. Agregar columna `ciudad` a `restaurantes`

**Archivo:** `init.sql` — tabla `restaurantes`
- Agregar `ciudad VARCHAR(100) DEFAULT ''` después del campo `direccion`
- Poblar en seed data: `'San José'`, `'Cartago'`, `'Heredia'`

**Archivo:** `initMongo.js` — documentos de restaurantes
- Agregar campo `ciudad` a cada documento (`"San José"`, `"Cartago"`, `"Heredia"`)

**Motivo:** El ETL (`export_data.py`) y las vistas OLAP (`v_reservas_por_ubicacion`) requieren `ciudad` para el análisis geográfico. No afecta al código Go (ver análisis más abajo para confirmación).

---

### 2. Reducir estados de pedido de 4 a 2 (completado vs cancelado)

**Objetivo:** Simplificar el modelo unificando los 4 estados actuales (`pendiente`, `en_preparacion`, `completado`, `entregado`) en solo 2: `completado` y `cancelado`. Esto además arregla el bug donde `analisis_crecimiento.py` busca `'Cancelado'/'Cancelled'` que no existen en `dim_order_status`, resultando en tasa de cancelación siempre 0.

**Archivos a modificar:**

| Capa | Archivo | Cambio |
|---|---|---|
| **OLTP** | `init.sql:93` | Seed: `('completado'), ('cancelado')` |
| **Generación datos** | `scripts/generate_oltp_data.py:48,200` | Mapa `{1: "completado", 2: "cancelado"}` con pesos 75/25 |
| **ETL** | `spark/etl_to_star.py:94` | `data = [(1, "completado"), (2, "cancelado")]` |
| **Warehouse DDL** | `warehouse/ddl/dim_order_status.hql` | Solo `(1, 'completado'), (2, 'cancelado')` |
| **Análisis** | `spark/analisis_crecimiento.py:39-44` | Cambiar `IN ('Cancelado','Cancelled')` → `= 'cancelado'` (arregla bug) |
| **Dashboards Superset** | `superset/setup_dashboards.py:214` | Misma corrección de filtro de cancelación |
| **Export dashboard** | `spark/exports/dashboard_estadisticas.json` | Re-exportar después del cambio |

**Post-ejecución:**
1. Recrear datos OLTP: `docker exec api python scripts/generate_oltp_data.py`
2. Re-ejecutar ETL completo: extraer (`export_data.py`) → Spark (`etl_to_star.py`) → Hive
3. Re-crear dashboards en Superset: `docker exec superset python /app/fix_dashboards_v2.py`

---

### 3. Corregir mapeo de estados de reserva en Warehouse

**Archivo:** `spark/etl_to_star.py` — función `build_dim_reservation_status()`

| Actual (incorrecto) | Correcto |
|---|---|
| 1=Confirmada | 1=pendiente |
| 2=Cancelada | 2=confirmada |
| — | 3=cancelada |

**Archivo:** `warehouse/ddl/dim_reservation_status.hql`
- Agregar estado `"pendiente"` (status_key=1)

---

### 4. Crear generador de datos masivos OLTP

**Archivo:** `scripts/generate_oltp_data.py` **(NUEVO)**

Script que debe generar datos tanto para PostgreSQL como para MongoDB:

- **500+ pedidos** con detalles (líneas de pedido) distribuidos en 6–12 meses de historia
- **200+ reservas** distribuidas en el mismo período
- **10+ usuarios** adicionales
- **8–10 restaurantes** en distintas ciudades (San José, Alajuela, Cartago, Heredia, Escazú)
- Variar horas del día (8:00–22:00) para que `v_horarios_pico` tenga datos útiles
- Incluir estados variados (pendiente, completado, cancelado, etc.)

---

### 5. Verificar `dim_date` poblada

El Spark job `etl_to_star.py` construye `dim_date` a partir de las fechas de pedidos y reservas. Con el generador masivo (paso 4) esto se resuelve automáticamente. Sin datos históricos suficientes, `dim_date` tendrá 1 sola fila y los joins temporales no funcionarán.

---

### 6. Verificar ruta de scripts dentro de contenedores — ✅ Resuelto

Se descubrió que `hive-server` en `docker-compose.yml` **no tenía volumen montado** para los scripts de warehouse. Se agregó:
```yaml
volumes:
  - ./warehouse:/opt/warehouse:ro
```
Ahora los scripts están disponibles en `/opt/warehouse/init_hive.sh` y `/opt/warehouse/init_olap_views.sh`. El executionPlan.md fue actualizado con las rutas correctas.

---

### Impacto en código Go — ✅ Ninguno

Revisión exhaustiva de cada archivo Go que toca la tabla `restaurantes`:

| Archivo | Línea | ¿Menciona `ciudad`? |
|---|---|---|
| `models/models.go` (struct `Restaurant`) | 16–23 | ❌ No |
| `handlers/restaurant.go` (CreateRestaurant) | 29–34 | ❌ No |
| `repository/interface.go` (RestaurantRepository) | 20–24 | ❌ No |
| `repository/postgres/restaurant.go` (List) | 99–100 | ❌ No — `SELECT id, nombre, direccion, ...` |
| `repository/postgres/restaurant.go` (GetByID) | 121–123 | ❌ No — `SELECT id, nombre, direccion, ...` |
| `repository/postgres/restaurant.go` (Create) | 132–136 | ❌ No — `INSERT (nombre, direccion, ...)` |
| `repository/mongo/restaurant.go` (List) | 127–148 | ❌ No — solo accede a campos del modelo |
| `repository/mongo/restaurant.go` (GetByID) | 150–164 | ❌ No |
| `repository/mongo/restaurant.go` (Create) | 166–203 | ❌ No |
| `scripts/generate_geo.py` (va directo a BD) | 138–203 | ❌ No — `SELECT id, nombre, direccion` |

Todas las queries SQL en Go usan columnas explícitas (nunca `SELECT *`). Los scripts Python que leen `ciudad` (`export_data.py`, `setup_dashboards.py`) van directo a la BD sin pasar por la API de Go.

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
| Data Warehouse y OLAP (Hive, esquema estrella, 6 vistas OLAP) | ✅ Completado |
| Procesamiento con Spark (DataFrames, SparkSQL, 4 jobs) | ✅ Completado |
| Visualización (Superset, infraestructura + setup_dashboards.py) | ✅ Completado |
| Dashboards Superset (3 dashboards, 9 charts, Fase 3.2) | ✅ Completado |
| Orquestación Airflow (DAG ETL + reindexado ES) | ✅ Completado |
| Neo4J (servicio Docker, modelo grafo, import_data, consultas Cypher) | ✅ Completado |
| Rutas de Entrega (generate_geo, TSP greedy, Cypher ShortestPath, endpoint Go) | ✅ Completado |
| Pruebas y Validaciones OLAP (scripts/verify_*.py/.sh con 20+ checks) | ✅ Completado |
| Entregables (código, DAG, notebooks, dashboards, PDF) | ⬜ Pendiente |
| Agregar `ciudad` a `restaurantes` (init.sql + initMongo.js) | ✅ Completado |
| Reducir estados de pedido de 4 a 2 (completado vs cancelado) | 🔄 Pendiente (7 archivos) |
| Corregir mapeo estados de reserva en Warehouse | ✅ Completado |
| Crear generador de datos masivos `generate_oltp_data.py` | ✅ Completado |
| Verificar ruta de scripts init_hive / init_olap_views en contenedores | ✅ Completado |
| Ejecutar plan completo (executionPlan.md) | ⬜ Pendiente |
