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
- [ ] Actualizar `spark/etl_to_star.py` para extraer `HOUR(timestamp)` del pedido y poblar `fact_orders.hora` *(pendiente hasta crear spark/etl_to_star.py en 2.2)*

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
- [ ] Agregar servicios al `docker-compose.yml`:
  - `spark-master` (imagen `bitnami/spark:3.5`, modo master, puertos 8080 UI, 7077 workers)
  - `spark-worker` (imagen `bitnami/spark:3.5`, modo worker, 2+ réplicas escalables)
- [ ] Copiar `hive-site.xml` a Spark conf para integración Hive (acceder a tablas del warehouse)
- [ ] Conectar a la red `default` para acceso a Hive Metastore

#### 2.2 Jobs PySpark

##### Job 1: Transformación ETL — `spark/etl_to_star.py`
- [ ] Crear **`spark/etl_to_star.py`** (reemplaza a `transform_to_star.py` y `load_to_hive.py`):
  - Leer CSVs de `warehouse/etl/staging/` con `spark.read.csv()`
  - Generar dimensiones con surrogate keys (`monotonically_increasing_id` o `row_number()`)
  - Construir mapeo `original_id → surrogate_key` con broadcast joins
  - Generar `dim_date` desde fechas en los datos usando SparkSQL
  - Transformar fact tables mapeando FKs con joins
  - Calcular métricas: `total_amount`, `item_count`, `subtotal`
  - **Escribir directamente en Hive** con `df.write.mode("overwrite").saveAsTable("nombre_tabla")`
  - Reparar particiones con `MSCK REPAIR TABLE` vía SparkSQL
  - SCD Tipo 0 (no historizar)

##### Job 2: Análisis de Tendencias — `spark/analisis_tendencias.py`
- [ ] Crear **`spark/analisis_tendencias.py`** — Tendencias de consumo:
  - Cargar `fact_orders` + `dim_date` + `dim_product` vía SparkSQL/Hive
  - DataFrames: ingresos por mes, top 10 productos más vendidos, categoría más popular por trimestre
  - Output: CSV/JSON en `spark/output/tendencias/`

##### Job 3: Análisis de Horarios — `spark/analisis_horarios.py`
- [ ] Crear **`spark/analisis_horarios.py`** — Horarios pico:
  - SparkSQL: extraer `HOUR(fecha)` de pedidos, agrupar por hora y día de semana
  - Identificar ventanas de alta demanda (peak hours)
  - Output: CSV/JSON en `spark/output/horarios/`

##### Job 4: Análisis de Crecimiento — `spark/analisis_crecimiento.py`
- [ ] Crear **`spark/analisis_crecimiento.py`** — Crecimiento mensual:
  - Window functions de SparkSQL: `LAG()` para mes anterior, cálculo de % de crecimiento
  - Métricas: crecimiento en ingresos, crecimiento en número de pedidos, tasa de cancelación mensual
  - Output: CSV/JSON en `spark/output/crecimiento/`

#### 2.3 Script de envío
- [ ] Crear `spark/submit_jobs.sh` — script que envía los 4 jobs a `spark-submit`:
  ```bash
  spark-submit --master spark://spark-master:7077 spark/etl_to_star.py
  spark-submit --master spark://spark-master:7077 spark/analisis_tendencias.py
  ...

---

### 3. Visualización de Datos con Apache Superset (15%)

**Stack:** Apache Superset (imagen `apache/superset`, puerto 8088). Conexión a Hive y SparkSQL como fuentes de datos.

#### 3.1 Infraestructura
- [ ] Agregar servicio `superset` al `docker-compose.yml`:
  - Imagen: `apache/superset`
  - Puerto: 8088
  - Depende de: hive-server (para consultas)
- [ ] Crear `superset/Dockerfile.superset`:
  - FROM `apache/superset`
  - Instalar drivers: `pyhive`, `thrift`, `thrift-sasl` para Hive
  - Configurar `SUPERSET_SECRET_KEY`
- [ ] Crear `superset/superset_config.py`:
  - Conexiones a Hive (hive://hive-server:10000/default)
  - Conexión a SparkSQL (hive://spark-master:10000/default) — si Spark Thrift Server está disponible
- [ ] Crear `superset/init_superset.sh`:
  - `superset fab create-admin` — usuario admin
  - `superset db upgrade`
  - `superset init`
  - Importar dashboards pre-definidos

#### 3.2 Dashboards
- [ ] **Dashboard 1:** Ingresos por mes y categoría de producto
  - Gráfico de barras: ingresos mensuales agrupados por categoría
  - Gráfico de líneas: tendencia de ingresos totales (12 meses)
  - Tabla de resumen: top 5 categorías por ingresos
- [ ] **Dashboard 2:** Actividad de clientes por zona geográfica
  - Mapa de calor (deck.gl): reservas y pedidos por ubicación de restaurante
  - Barras apiladas: distribución de clientes por ciudad
  - Big Number: total clientes activos
- [ ] **Dashboard 3:** Estadísticas de pedidos completados vs cancelados
  - Time-series: completados vs cancelados por mes
  - Pie chart: proporción general completado/cancelado
  - Tabla: tasa de cancelación por restaurante

---

### 4. Orquestación con Apache Airflow (15%)

**Stack:** Apache Airflow 2.8 (imagen `apache/airflow:2.8`). Scheduler + Webserver + Worker. DAG ETL diario.

#### 4.1 Infraestructura
- [ ] Agregar servicios al `docker-compose.yml`:
  - `airflow-db` (PostgreSQL 16 para metadatos de Airflow)
  - `airflow-webserver` (puerto 8080, imagen extendida con dependencias)
  - `airflow-scheduler` (misma imagen, comando scheduler)
  - `airflow-worker` (misma imagen, comando celery worker)
- [ ] Crear `airflow/Dockerfile`:
  - FROM `apache/airflow:2.8`
  - Instalar: `apache-airflow-providers-apache-spark`, `pymongo`, `pyhive`, `thrift`, `elasticsearch`
- [ ] Crear conexiones pre-configuradas en `airflow/connections.yaml`:
  - `mongo_default`, `hive_default`, `spark_default`, `elasticsearch_default`

#### 4.2 DAG de ETL Completo
- [ ] Crear `airflow/dags/etl_pipeline.py` — DAG con ejecución diaria:
  1. **`extract_data`** → BashOperator: ejecuta `warehouse/etl/export_data.py` (extrae de MongoDB o PostgreSQL según `DB_ENGINE`)
  2. **`etl_to_star`** → SparkSubmitOperator: envía `spark/etl_to_star.py` (Spark lee CSVs de staging/, transforma a estrella y escribe en Hive)
  3. **`analisis_tendencias`** → SparkSubmitOperator: envía `spark/analisis_tendencias.py` (en paralelo con los otros análisis)
  4. **`analisis_horarios`** → SparkSubmitOperator: envía `spark/analisis_horarios.py` (en paralelo)
  5. **`analisis_crecimiento`** → SparkSubmitOperator: envía `spark/analisis_crecimiento.py` (en paralelo)
  6. **`reindex_elasticsearch`** → PythonOperator: reindexado condicional vía MD5 hash
     - Calcular MD5 de `warehouse/etl/staging/platos.csv` después de `extract_data`
     - Comparar con hash almacenado en `warehouse/etl/staging/.last_platos_hash`
     - Si cambió → ejecutar `POST /search/reindex` via HttpHook, persistir nuevo hash en `.last_platos_hash`
     - Si no cambió → skipear la tarea (BranchPythonOperator que la omite)
  - Dependencias: extract_data → etl_to_star → (tendencias, horarios, crecimiento en paralelo) → reindex
  - Retry: 2 reintentos con backoff de 5 min
  - Alertas: email/slack opcional

---

### 5. Neo4J para Análisis de Grafos y Rutas (15%)

**Stack:** Neo4j 5 Community + plugin APOC (imagen `neo4j:5-community`, puertos 7474 HTTP, 7687 Bolt).

#### 5.1 Infraestructura
- [ ] Agregar servicio `neo4j` al `docker-compose.yml`:
  - Imagen: `neo4j:5-community`
  - Puertos: 7474:7474 (HTTP Browser), 7687:7687 (Bolt)
  - Variables de entorno: `NEO4J_PLUGINS=["apoc"]`, `NEO4J_AUTH=neo4j/password`
  - Volumen: `neo4j_data` para persistencia

#### 5.2 Modelo de Grafo
- [ ] Crear `neo4j/` directorio con:
  - `import_data.py` — script Python que exporta desde MongoDB/PostgreSQL a CSVs de nodos y relaciones
  - `load_schema.cypher` — crea constraints, índices y carga CSVs
- [ ] **Nodos:**
  ```cypher
  (:User {user_id, nombre, email})
  (:Product {product_id, nombre, categoria, precio})
  (:Restaurant {restaurant_id, nombre, direccion, lat, lon})
  (:Order {order_id, total_amount, fecha})
  (:Reservation {reservation_id, fecha, party_size})
  (:Location {location_id, lat, lon, address})
  ```
- [ ] **Relaciones:**
  ```cypher
  (u:User)-[:PLACED {fecha}]->(o:Order)
  (o:Order)-[:INCLUDES {cantidad, subtotal}]->(p:Product)
  (o:Order)-[:AT]->(r:Restaurant)
  (u:User)-[:MADE_RESERVATION]->(res:Reservation)
  (res:Reservation)-[:AT]->(r:Restaurant)
  (r:Restaurant)-[:LOCATED_AT]->(l:Location)
  (l1:Location)-[:ROAD {distance_km}]->(l2:Location)
  ```

#### 5.3 Consultas Cypher
- [ ] **Top 5 productos más comprados juntos (co-compra):**
  ```cypher
  MATCH (p1:Product)<-[:INCLUDES]-(o:Order)-[:INCLUDES]->(p2:Product)
  WHERE p1 <> p2
  RETURN p1.nombre, p2.nombre, COUNT(*) AS frecuencia
  ORDER BY frecuencia DESC LIMIT 5
  ```
- [ ] **Usuarios que recomiendan a otros (compra misma categoría + mismo restaurante):**
  ```cypher
  MATCH (u1:User)-[:PLACED]->(:Order)-[:AT]->(r:Restaurant),
        (u2:User)-[:PLACED]->(:Order)-[:AT]->(r)
  WHERE u1 <> u2
  RETURN u1.nombre, u2.nombre, COUNT(*) AS afinidad
  ORDER BY afinidad DESC LIMIT 10
  ```
- [ ] **Caminos mínimos entre ubicaciones para reparto eficiente:**
  ```cypher
  MATCH (a:Location {location_id: $origen}),
        (b:Location {location_id: $destino}),
        p = shortestPath((a)-[:ROAD*]-(b))
  RETURN [n IN nodes(p) | n.address] AS ruta,
         reduce(acc = 0, r IN relationships(p) | acc + r.distance_km) AS distancia_total
  ```
- [ ] **Usuarios influyentes (PageRank con GDS opcional):**
  ```cypher
  CALL gds.pageRank.stream('user-product-graph')
  YIELD nodeId, score
  RETURN gds.util.asNode(nodeId).nombre AS usuario, score
  ORDER BY score DESC LIMIT 5
  ```

---

### 6. Asignación de Rutas de Entrega (10%)

#### 6.1 Datos Geoespaciales
- [ ] Generar coordenadas ficticias para restaurantes en San José, Costa Rica:
  - Crear `scripts/generate_geo.py` — asigna lat/lon aleatorios pero realistas (9.92-9.94, -84.08--84.10) a cada restaurante basado en su dirección
  - Almacenar en Neo4j como nodos `(:Location)` y en MongoDB como campo `ubicacion`
- [ ] Crear relación `[:ROAD]` entre ubicaciones cercanas (< 5km) con distancia Haversine calculada
  ```cypher
  MATCH (a:Location), (b:Location)
  WHERE a <> b AND point.distance(a.point, b.point) < 5000
  MERGE (a)-[:ROAD {distance_km: round(point.distance(a.point, b.point)/1000, 1)}]->(b)
  ```

#### 6.2 Heurística de Enrutamiento
- [ ] Implementar endpoint `GET /api/delivery/routes` (handler en Go):
  - Query params: `desde=restaurant_id`, `hasta=order_id` (obtiene dirección del cliente)
  - Parámetro opcional: `algoritmo=nearest|cypher`
- [ ] **Algoritmo Vecino Más Cercano (Go puro):**
  - Obtener lista de Location IDs desde Neo4j
  - Implementar TSP greedy: desde el origen, visitar siempre la Location más cercana no visitada
  - Retornar ruta ordenada + distancia total
- [ ] **Algoritmo Cypher ShortestPath:**
  - Delegar a la consulta Neo4j de camino mínimo
  - Usar driver Go `neo4j/neo4j-go-driver` para ejecutar la consulta
- [ ] Respuesta JSON: `{ruta: [{direccion, lat, lon, orden}], distancia_total_km, algoritmo_usado}`
- [ ] Asignación por repartidor: agrupar rutas por zona (clustering simple por grilla de 2x2km)

#### 6.3 Integración con Neo4j desde Go
- [ ] Agregar driver Neo4j a `go.mod`: `github.com/neo4j/neo4j-go-driver/v5`
- [ ] Crear `repository/neo4j.go` — cliente Neo4j singleton
- [ ] Crear `handlers/delivery.go` — handler del endpoint `/api/delivery/routes`
- [ ] Registrar ruta en `api/main.go` (requiere JWT + rol admin)

---

### 7. Pruebas y Validaciones OLAP

#### 7.1 Validación de Datos en Warehouse
- [ ] Verificar integridad: `SELECT COUNT(*)` en MongoDB vs `SELECT COUNT(*)` en Hive para cada tabla
- [ ] Validar que surrogate keys en fact tables tengan correspondencia en dimensiones (no nulos, no huérfanos)
- [ ] Verificar que `fact_orders.total_amount = SUM(fact_order_details.subtotal)` (por order_id)
- [ ] Validar unicidad de surrogate keys en cada dimensión

#### 7.2 Validación del Pipeline ETL
- [ ] Ejecución completa: `python warehouse/etl/run_etl.py` sin errores
- [ ] Prueba de idempotencia: ejecutar ETL 2 veces seguidas (debe truncar y recargar sin duplicados)
- [ ] Prueba de graceful degradation: si MongoDB no responde, script falla con mensaje claro (no crash silencioso)

#### 7.3 Validación de Análisis Spark
- [ ] Los 3 scripts PySpark se ejecutan sin excepción contra datos reales
- [ ] Outputs contienen datos no vacíos con formato esperado (CSV con cabeceras)
- [ ] Validación manual: cruzar resultados Spark contra queries SQL directas en Hive

#### 7.4 Validación de Neo4j
- [ ] Las 3 consultas Cypher devuelven resultados no vacíos
- [ ] La consulta de camino mínimo retorna una ruta válida (secuencia de nodos conectados)
- [ ] Prueba de co-compra: verificar que los pares devueltos tienen sentido lógico

#### 7.5 Validación de Airflow
- [ ] Trigger manual del DAG: todas las tareas completan en verde
- [ ] Logs de cada tarea sin errores
- [ ] Validar que el DAG respeta dependencias (no ejecuta transform antes de extract)

#### 7.6 Prueba de Integración Completa
- [ ] Crear `scripts/verify_all.sh` — script que ejecuta validaciones secuencialmente y reporta PASS/FAIL

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
| Capturas de pantalla / video | Demostración del sistema funcionando | `docs/capturas/` o externo |
| Documentación técnica PDF | Flujo de datos, decisiones arquitectónicas, ejemplos de uso | `docs/`

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
