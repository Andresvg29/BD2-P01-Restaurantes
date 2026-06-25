---
title: "Sistema de Análisis OLAP — Restaurantes API"
subtitle: "Arquitectura de Data Warehouse, Procesamiento Spark, Visualización y Grafos"
author: "BD2-P01"
date: "Junio 2026"
toc: true
toc-depth: 3
numbersections: true
---

# Resumen Ejecutivo

Este documento describe la arquitectura OLAP implementada para el sistema de gestión de restaurantes
**BD2-P01-Restaurantes**. El pipeline transforma datos operacionales (desde MongoDB o PostgreSQL)
en un modelo analítico de esquema estrella en **Apache Hive**, utilizando **Apache Spark** para el
procesamiento ETL y análisis avanzados. La visualización se realiza mediante **Apache Superset**
(3 dashboards), la orquestación del pipeline diario con **Apache Airflow**, y el análisis de grafos
con **Neo4j** para recomendaciones y rutas de entrega.

**Componentes clave:**

| Componente | Propósito |
|---|---|
| Hadoop HDFS 3.3.6 | Almacenamiento distribuido del Data Warehouse |
| Apache Hive 4.0.0 | SQL sobre Hadoop, esquema estrella |
| Apache Spark 3.5.0 | Transformación ETL y análisis (PySpark) |
| Apache Superset 4.1.1 | BI y visualización (3 dashboards) |
| Apache Airflow 2.8 | Orquestación del pipeline ETL diario |
| Neo4j 5 | Grafo de conocimiento y rutas de entrega |
| Redis | Caché y broker de Airflow |

# Arquitectura General del Sistema

```
┌──────────────────────────────────────────────────────────────────┐
│                        CAPA OLTP                                 │
│                                                                  │
│   ┌──────────┐    ┌──────────┐    ┌──────────┐                 │
│   │ API REST │    │  Search  │    │ Keycloak │                 │
│   │ (Gin)    │    │ (Gin)    │    │  (Auth)  │                 │
│   └────┬─────┘    └────┬─────┘    └──────────┘                 │
│        │               │                                        │
│   ┌────▼─────┐   ┌────▼─────┐    ┌──────────┐                 │
│   │ PostgreSQL│   │Elasticsearch│  │  Redis   │                 │
│   │ O MongoDB │   │  (Búsqueda) │  │  (Caché) │                 │
│   └──────────┘   └──────────┘    └──────────┘                 │
└──────────────────────────────────────────────────────────────────┘
                            │
                    (Airflow Orchestrator)
                            │
┌──────────────────────────────────────────────────────────────────┐
│                        CAPA OLAP                                 │
│                                                                  │
│   ┌──────────┐    ┌──────────┐    ┌──────────┐                 │
│   │   HDFS   │◄───│  Spark   │◄───│  Airflow │                 │
│   │(Namenode │    │(Master + │    │ (DAG ETL)│                 │
│   │ +Datanode)│    │  Worker) │    └──────────┘                 │
│   └────┬─────┘    └──────────┘                                 │
│        │                                                        │
│   ┌────▼─────┐    ┌──────────┐    ┌──────────┐                 │
│   │   Hive   │◄───│ Superset │    │  Neo4j   │                 │
│   │(Esquema  │    │ (3 Dash- │    │  (Grafos, │                 │
│   │ Estrella)│    │  boards) │    │   Rutas)  │                 │
│   └──────────┘    └──────────┘    └──────────┘                 │
└──────────────────────────────────────────────────────────────────┘
```

## Flujo de Datos de Extremo a Extremo

1. Las operaciones diarias (pedidos, reservas, usuarios) se registran en **MongoDB** o **PostgreSQL**
   a través de la API REST
2. **Airflow** ejecuta el DAG `etl_pipeline` diariamente (o manualmente)
3. **`export_data.py`** extrae datos crudos a CSVs de staging
4. **`export_neo4j.py`** y **`load_neo4j.py`** sincronizan el grafo Neo4j
5. **`etl_to_star.py`** (Spark) transforma CSVs a modelo estrella y escribe en Hive
6. **3 análisis Spark** se ejecutan en paralelo sobre las tablas Hive
7. Si `platos.csv` cambió, se reindexa **Elasticsearch**
8. **Superset** consulta Hive para visualizar los dashboards
9. **Neo4j** permite consultas de co-compra, afinidad, rutas y centralidad

# Data Warehouse — Esquema Estrella

## Justificación: Estrella vs Copo de Nieve

Se optó por **esquema estrella** porque:

- Las dimensiones son planas y sin jerarquías profundas (no hay subcategorías de productos,
  ni regiones geográficas jerárquicas)
- Mejor rendimiento en consultas: menos JOINs, tablas desnormalizadas
- Simplicidad de entendimiento para los dashboards de Superset
- Las tablas de dimensión son pequeñas (cientos de filas vs millones de hechos)

**SCD:** Se implementó **Tipo 0** (no historizar cambios) porque los datos provienen de un sistema
transaccional donde las dimensiones representan el estado actual. En un sistema productivo con
requerimientos de historización, se migraría a SCD Tipo 2.

## Tablas de Dimensión

### dim_date

```sql
CREATE TABLE IF NOT EXISTS dim_date (
    date_key    INT            -- Formato YYYYMMDD, ej: 20250315
  , full_date   DATE
  , year        INT
  , quarter     INT            -- 1 a 4
  , month       INT            -- 1 a 12
  , day         INT            -- 1 a 31
  , weekday     STRING         -- "Monday", "Tuesday", etc.
  , is_weekend  BOOLEAN
)
ROW FORMAT DELIMITED FIELDS TERMINATED BY ','
STORED AS TEXTFILE
LOCATION 'hdfs://hadoop-namenode:9000/user/hive/warehouse/dim_date';
```

Poblada automáticamente por Spark a partir de las fechas de pedidos y reservas.

### dim_user

```sql
CREATE TABLE IF NOT EXISTS dim_user (
    user_key   INT       -- Surrogate key
  , user_id    STRING    -- ID original (UUID de Keycloak)
  , nombre     STRING
  , email      STRING
  , role_name  STRING    -- 'admin' o 'client'
)
ROW FORMAT DELIMITED FIELDS TERMINATED BY ','
STORED AS TEXTFILE
LOCATION 'hdfs://hadoop-namenode:9000/user/hive/warehouse/dim_user';
```

### dim_restaurant

```sql
CREATE TABLE IF NOT EXISTS dim_restaurant (
    restaurant_key   INT
  , restaurant_id    STRING
  , nombre           STRING
  , direccion        STRING
  , ciudad           STRING
  , telefono         STRING
)
ROW FORMAT DELIMITED FIELDS TERMINATED BY ','
STORED AS TEXTFILE
LOCATION 'hdfs://hadoop-namenode:9000/user/hive/warehouse/dim_restaurant';
```

### dim_product

```sql
CREATE TABLE IF NOT EXISTS dim_product (
    product_key   INT
  , product_id    STRING
  , nombre        STRING
  , categoria     STRING
  , precio        DECIMAL(10,2)
)
ROW FORMAT DELIMITED FIELDS TERMINATED BY ','
STORED AS TEXTFILE
LOCATION 'hdfs://hadoop-namenode:9000/user/hive/warehouse/dim_product';
```

### dim_order_status (sembrada)

```sql
CREATE TABLE IF NOT EXISTS dim_order_status (
    status_key  INT
  , status_name STRING
)
ROW FORMAT DELIMITED FIELDS TERMINATED BY ','
STORED AS TEXTFILE
LOCATION 'hdfs://hadoop-namenode:9000/user/hive/warehouse/dim_order_status';

-- Datos sembrados:
-- 1, completado
-- 2, cancelado
```

### dim_reservation_status (sembrada)

```sql
CREATE TABLE IF NOT EXISTS dim_reservation_status (
    status_key  INT
  , status_name STRING
)
ROW FORMAT DELIMITED FIELDS TERMINATED BY ','
STORED AS TEXTFILE
LOCATION 'hdfs://hadoop-namenode:9000/user/hive/warehouse/dim_reservation_status';

-- Datos sembrados:
-- 1, pendiente
-- 2, confirmada
-- 3, cancelada
```

## Tablas de Hechos

### fact_orders (particionada por year, month)

```sql
CREATE TABLE IF NOT EXISTS fact_orders (
    order_id        STRING
  , date_key        INT
  , hora            INT           -- Hora extraída del timestamp (0-23)
  , user_key        INT
  , restaurant_key  INT
  , status_key      INT
  , total_amount    DECIMAL(10,2)
  , item_count      INT
)
PARTITIONED BY (year INT, month INT)
ROW FORMAT DELIMITED FIELDS TERMINATED BY ','
STORED AS TEXTFILE
LOCATION 'hdfs://hadoop-namenode:9000/user/hive/warehouse/fact_orders';
```

### fact_order_details (particionada por year, month)

```sql
CREATE TABLE IF NOT EXISTS fact_order_details (
    detail_id    STRING
  , order_id     STRING
  , product_key  INT
  , quantity     INT
  , unit_price   DECIMAL(10,2)
  , subtotal     DECIMAL(10,2)   -- quantity * unit_price
)
PARTITIONED BY (year INT, month INT)
ROW FORMAT DELIMITED FIELDS TERMINATED BY ','
STORED AS TEXTFILE
LOCATION 'hdfs://hadoop-namenode:9000/user/hive/warehouse/fact_order_details';
```

### fact_reservations (particionada por year, month)

```sql
CREATE TABLE IF NOT EXISTS fact_reservations (
    reservation_id  STRING
  , date_key        INT
  , user_key        INT
  , restaurant_key  INT
  , status_key      INT
  , party_size      INT
)
PARTITIONED BY (year INT, month INT)
ROW FORMAT DELIMITED FIELDS TERMINATED BY ','
STORED AS TEXTFILE
LOCATION 'hdfs://hadoop-namenode:9000/user/hive/warehouse/fact_reservations';
```

## Diagrama del Esquema Estrella

```
┌─────────────────┐     ┌─────────────────────────────────────────────┐     ┌──────────────────┐
│   dim_date      │     │              fact_orders                   │     │  dim_user        │
├─────────────────┤     ├─────────────────────────────────────────────┤     ├──────────────────┤
│ date_key (PK)   │◄────┤► date_key (FK)                             │────►│ user_key (PK)    │
│ full_date       │     │ order_id                    (PK)           │     │ user_id          │
│ year            │     │ hora                                        │     │ nombre           │
│ quarter         │     │ user_key (FK)                    ──────────│─────│ email            │
│ month           │     │ restaurant_key (FK)               ─────────│──┐  │ role_name        │
│ day             │     │ status_key (FK) ──────────────┐            │  │  └──────────────────┘
│ weekday         │     │ total_amount                  │            │  │
│ is_weekend      │     │ item_count                    │            │  │  ┌──────────────────┐
└─────────────────┘     │ year (PARTITION)              │            │  │  │ dim_restaurant   │
                        │ month (PARTITION)             │            │  │  ├──────────────────┤
┌─────────────────┐     └─────────────────────────────────────────────┘  │  │ restaurant_key   │
│ dim_product     │                                                     │  │ (PK)             │
├─────────────────┤                                                     │  │ restaurant_id    │
│ product_key (PK)│──┐                                                  │  │ nombre           │
│ product_id      │  │  ┌─────────────────────────────────────────────┐  │  │ direccion        │
│ nombre          │  │  │        fact_order_details                  │  │  │ ciudad           │
│ categoria       │  │  ├─────────────────────────────────────────────┤  │  │ telefono         │
│ precio          │  │  │ detail_id    (PK)                          │  │  └──────────────────┘
└─────────────────┘  └──┤ product_key (FK)                           │  │
                        │ order_id                                   │  │  ┌──────────────────────┐
┌─────────────────────┐ │ quantity                                   │  │  │ dim_order_status     │
│ dim_reservation_status│ │ unit_price                                │  │  ├──────────────────────┤
├─────────────────────┤ │ subtotal                                   │  │  │ status_key (PK)      │
│ status_key (PK)     │ │ year (PARTITION)                           │  │  │ status_name          │
│ status_name         │ │ month (PARTITION)                          │  │  └──────────────────────┘
└─────────────────────┘ └─────────────────────────────────────────────┘  │
                                                                        │  ┌──────────────────────┐
┌─────────────────┐     ┌─────────────────────────────────────────────┐  │  │ dim_order_status     │
│ dim_user         │     │         fact_reservations                  │  │  ├──────────────────────┤
├─────────────────┤     ├─────────────────────────────────────────────┤  │  │ status_key (PK)      │
│ user_key (PK)   │◄────┤► user_key (FK)                             │  │  │ status_name          │
│ user_id         │     │ reservation_id                (PK)         │  │  └──────────────────────┘
│ nombre          │     │ date_key (FK)                          ────│──┘
│ email           │     │ restaurant_key (FK)              ──────────│────
│ role_name       │     │ status_key (FK) ─────────┐                  │
└─────────────────┘     │ party_size               │                  │
                        │ year (PARTITION)         │                  │
                        │ month (PARTITION)        │                  │
                        └──────────────────────────┴──────────────────┘
                                             │              │
                                             ▼              ▼
                                  ┌─────────────────┐  ┌──────────────────┐
                                  │dim_reservation  │  │  dim_restaurant  │
                                  │_status          │  └──────────────────┘
                                  └─────────────────┘
```

## Vistas OLAP

### v_ingresos_mensuales

Ingresos, pedidos y ticket promedio agrupados por mes y restaurante.

```sql
CREATE VIEW IF NOT EXISTS v_ingresos_mensuales AS
SELECT
    d.year,
    d.month,
    r.restaurant_key,
    r.nombre AS restaurant_nombre,
    COUNT(DISTINCT fo.order_id) AS total_pedidos,
    ROUND(SUM(fo.total_amount), 2) AS ingresos_totales,
    ROUND(AVG(fo.total_amount), 2) AS ticket_promedio
FROM fact_orders fo
JOIN dim_date d ON fo.date_key = d.date_key
JOIN dim_restaurant r ON fo.restaurant_key = r.restaurant_key
GROUP BY d.year, d.month, r.restaurant_key, r.nombre;
```

**Ejemplo de resultado:**

| year | month | restaurant_nombre | total_pedidos | ingresos_totales | ticket_promedio |
|---|---|---|---|---|---|
| 2025 | 3 | La Fonda Criolla | 45 | 157,500.00 | 3,500.00 |
| 2025 | 3 | Sushi Express | 32 | 128,000.00 | 4,000.00 |

### v_ventas_por_categoria

Unidades vendidas, ingresos y ranking por categoría de producto.

```sql
CREATE VIEW IF NOT EXISTS v_ventas_por_categoria AS
SELECT
    dp.categoria,
    COUNT(DISTINCT fod.detail_id) AS total_unidades_vendidas,
    SUM(fod.quantity) AS total_cantidad,
    ROUND(SUM(fod.subtotal), 2) AS ingresos_totales,
    ROUND(AVG(fod.unit_price), 2) AS precio_promedio,
    RANK() OVER (ORDER BY SUM(fod.subtotal) DESC) AS ranking_ingresos
FROM fact_order_details fod
JOIN dim_product dp ON fod.product_key = dp.product_key
GROUP BY dp.categoria;
```

### v_reservas_por_ubicacion

Reservas y comensales por ciudad y mes, con ranking dentro de cada ciudad.

```sql
CREATE VIEW IF NOT EXISTS v_reservas_por_ubicacion AS
SELECT
    r.ciudad,
    r.restaurant_key,
    r.nombre AS restaurant_nombre,
    d.year, d.month,
    COUNT(DISTINCT fr.reservation_id) AS total_reservas,
    SUM(fr.party_size) AS total_comensales,
    ROUND(AVG(fr.party_size), 2) AS promedio_personas,
    RANK() OVER (PARTITION BY r.ciudad
                 ORDER BY COUNT(DISTINCT fr.reservation_id) DESC) AS ranking_ciudad
FROM fact_reservations fr
JOIN dim_date d ON fr.date_key = d.date_key
JOIN dim_restaurant r ON fr.restaurant_key = r.restaurant_key
GROUP BY r.ciudad, r.restaurant_key, r.nombre, d.year, d.month;
```

### v_tendencias_mensuales

Análisis de crecimiento mensual usando funciones de ventana `LAG()`.

```sql
CREATE VIEW IF NOT EXISTS v_tendencias_mensuales AS
SELECT
    d.year, d.month,
    COUNT(DISTINCT fo.order_id) AS total_pedidos,
    ROUND(SUM(fo.total_amount), 2) AS ingresos_totales,
    COUNT(DISTINCT fo.user_key) AS usuarios_activos,
    SUM(CASE WHEN dos.status_name = 'cancelado'
        THEN 1 ELSE 0 END) AS pedidos_cancelados,
    ROUND(
        SUM(CASE WHEN dos.status_name = 'cancelado'
            THEN 1 ELSE 0 END) * 100.0
        / NULLIF(COUNT(DISTINCT fo.order_id), 0), 2
    ) AS tasa_cancelacion_pct,
    ROUND(
        SUM(fo.total_amount)
        - LAG(SUM(fo.total_amount)) OVER (ORDER BY d.year, d.month), 2
    ) AS diferencia_ingresos_mom,
    ROUND(
        (SUM(fo.total_amount) - LAG(SUM(fo.total_amount))
         OVER (ORDER BY d.year, d.month)) * 100.0
        / NULLIF(LAG(SUM(fo.total_amount))
                 OVER (ORDER BY d.year, d.month), 0), 2
    ) AS crecimiento_ingresos_pct
FROM fact_orders fo
JOIN dim_date d ON fo.date_key = d.date_key
JOIN dim_order_status dos ON fo.status_key = dos.status_key
GROUP BY d.year, d.month;
```

### v_horarios_pico

Pedidos agrupados por hora real y día de semana para identificar ventanas de alta demanda.

```sql
CREATE VIEW IF NOT EXISTS v_horarios_pico AS
SELECT
    d.weekday,
    d.is_weekend,
    fo.hora,
    COUNT(DISTINCT fo.order_id) AS total_pedidos,
    ROUND(SUM(fo.total_amount), 2) AS ingresos_totales,
    COUNT(DISTINCT fo.user_key) AS usuarios_unicos,
    RANK() OVER (PARTITION BY d.weekday
                 ORDER BY COUNT(DISTINCT fo.order_id) DESC) AS ranking_hora
FROM fact_orders fo
JOIN dim_date d ON fo.date_key = d.date_key
GROUP BY d.weekday, d.is_weekend, fo.hora;
```

### v_productividad_usuarios

Segmentación de usuarios en VIP, Frecuente y Ocasional basada en percentiles de gasto.

```sql
CREATE VIEW IF NOT EXISTS v_productividad_usuarios AS
SELECT
    du.user_key, du.user_id, du.nombre AS usuario_nombre, du.email,
    COUNT(DISTINCT fo.order_id) AS total_pedidos,
    ROUND(SUM(fo.total_amount), 2) AS gasto_total,
    ROUND(AVG(fo.total_amount), 2) AS ticket_promedio,
    COUNT(DISTINCT fo.restaurant_key) AS restaurantes_visitados,
    NTILE(5) OVER (ORDER BY SUM(fo.total_amount) DESC) AS percentil_gasto,
    CASE
        WHEN NTILE(5) OVER (ORDER BY SUM(fo.total_amount) DESC) <= 1 THEN 'VIP'
        WHEN NTILE(5) OVER (ORDER BY SUM(fo.total_amount) DESC) <= 3 THEN 'Frecuente'
        ELSE 'Ocasional'
    END AS segmento
FROM fact_orders fo
JOIN dim_user du ON fo.user_key = du.user_key
GROUP BY du.user_key, du.user_id, du.nombre, du.email;
```

# Pipeline ETL

## Extracción (export_data.py)

El script `warehouse/etl/export_data.py` detecta automáticamente el motor de base de datos activo
mediante la variable de entorno `DB_ENGINE` y extrae los datos crudos a 6 archivos CSV en
`warehouse/etl/staging/`.

### Dual Source: MongoDB y PostgreSQL

**Rama MongoDB (PyMongo):**

```python
client = MongoClient(MONGO_URI)
db = client[MONGO_DB]

# Extraer usuarios
usuarios = list(db.usuarios.find())
for u in usuarios:
    u["_id"] = str(u["_id"])        # ObjectId → string
    u["fecha_creacion"] = u.get("fecha_creacion", "").isoformat()

# Extraer pedidos con detalles anidados
pedidos = list(db.pedidos.find())
detalles = []
for p in pedidos:
    p["_id"] = str(p["_id"])
    p["fecha"] = p.get("fecha", "").isoformat()
    for d in p.get("detalles", []):
        d["id_pedido"] = str(p["_id"])
        detalles.append(d)
```

**Rama PostgreSQL (psycopg2):**

```python
conn = psycopg2.connect(host=DB_HOST, dbname=DB_NAME, ...)
cur = conn.cursor(cursor_factory=RealDictCursor)

cur.execute("SELECT * FROM usuarios")
usuarios = cur.fetchall()

cur.execute("""
    SELECT dp.*, p.id_restaurante
    FROM detalles_pedido dp
    JOIN pedidos p ON dp.id_pedido = p.id
""")
detalles = cur.fetchall()
```

### Archivos Generados

| Archivo | Origen | Registros típicos |
|---|---|---|
| `usuarios.csv` | `usuarios` | 10-50 |
| `restaurantes.csv` | `restaurantes` | 8-10 |
| `platos.csv` | `platos` | 50-200 |
| `pedidos.csv` | `pedidos` | 500+ |
| `detalles_pedido.csv` | `detalles_pedido` (anidado en MongoDB) | 1500+ |
| `reservas.csv` | `reservas` | 200+ |

## Transformación con Spark (etl_to_star.py)

El job PySpark `spark/etl_to_star.py` es el corazón del pipeline ETL. Reemplaza los scripts
Python `transform_to_star.py` y `load_to_hive.py` que se usaban en versiones anteriores.

### Flujo de Transformación

```
CSVs de staging
    │
    ▼
┌─────────────────────────────────┐
│ spark.read.csv(inferSchema=True)│
└─────────┬───────────────────────┘
          │
          ▼
┌──────────────────────────────────────────────────────────────┐
│ 1. Construir Dimensiones con surrogate keys                 │
│    dim_user:       row_number() OVER (ORDER BY id)          │
│    dim_restaurant: row_number() OVER (ORDER BY id)          │
│    dim_product:    row_number() OVER (ORDER BY id)          │
│    dim_date:       desde fechas de pedidos y reservas       │
│    dim_order_status:        seed estático                   │
│    dim_reservation_status:  seed estático                   │
└──────────────────────┬───────────────────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────────────────┐
│ 2. Crear broadcast mappings (original_id → surrogate_key)   │
│    user_map       = { user_id → user_key }                  │
│    restaurant_map = { restaurant_id → restaurant_key }      │
│    product_map    = { product_id → product_key }            │
│    date_map       = { date_str → date_key }                 │
└──────────────────────┬───────────────────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────────────────┐
│ 3. Construir Fact Tables con broadcast hash joins           │
│    fact_orders:         GROUP BY order_id, mapear FKs       │
│    fact_order_details:  calcular subtotal = qty * price     │
│    fact_reservations:   mapear FKs, cast party_size         │
└──────────────────────┬───────────────────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────────────────┐
│ 4. Escribir en Hive con saveAsTable()                       │
│    df.write.mode("overwrite").saveAsTable("fact_orders")    │
│    MSCK REPAIR TABLE fact_orders                            │
└──────────────────────────────────────────────────────────────┘
```

### Fragmento clave: generación de surrogate keys

```python
from pyspark.sql.window import Window
from pyspark.sql import functions as F

def build_dim_user(spark, df):
    window_spec = Window.orderBy("id")
    role_map = F.create_map(
        F.lit("1"), F.lit("admin"),
        F.lit("2"), F.lit("client")
    )
    return df.select(
        F.row_number().over(window_spec).alias("user_key"),
        F.col("id").alias("user_id"),
        "nombre", "email",
        role_map[F.col("id_rol")].alias("role_name")
    )
```

### Fragmento clave: broadcast join para fact_orders

```python
# Crear broadcast mapping
user_dict = dict(usuarios.select("user_id", "user_key").rdd.map(
    lambda r: (r.user_id, r.user_key)).collect()
)
user_bc = spark.sparkContext.broadcast(user_dict)

# Aplicar mapping via UDF
map_user = F.udf(lambda uid: user_bc.value.get(uid, -1), T.IntegerType())
fact_orders = pedidos_df.select(
    "id", "fecha",
    map_user("id_usuario").alias("user_key"),
    ...
)
```

## Carga a Hive

Spark escribe directamente en Hive usando `saveAsTable()`, lo que elimina la necesidad de scripts
de carga separados:

```python
fact_orders.write \
    .mode("overwrite") \
    .partitionBy("year", "month") \
    .saveAsTable("fact_orders")

# Reparar particiones para que Hive las reconozca
spark.sql("MSCK REPAIR TABLE fact_orders")
```

# Procesamiento con Apache Spark

## Infraestructura Spark

| Servicio | Imagen | Puertos | Rol |
|---|---|---|---|
| spark-master | apache/spark:3.5.0 | 8083 (UI), 7077 (workers) | Coordinador del clúster |
| spark-worker | apache/spark:3.5.0 | — | Ejecutor de tareas (2 cores, 2 GB RAM) |

**Configuración destacada (`spark-defaults.conf`):**

```properties
spark.master                     spark://spark-master:7077
spark.sql.catalogImplementation  hive
spark.hadoop.hive.metastore.uris thrift://hive-metastore:9083
spark.sql.adaptive.enabled       true
spark.sql.adaptive.coalescePartitions.enabled true
spark.serializer                 org.apache.spark.serializer.KryoSerializer
```

## Jobs PySpark

### Job 1: ETL a Esquema Estrella (`etl_to_star.py`)

Descrito en la sección anterior. Transforma 6 CSVs → 6 dimensiones + 3 fact tables en Hive.

### Job 2: Análisis de Tendencias (`analisis_tendencias.py`)

**Consultas SparkSQL ejecutadas:**

```python
# 1. Ingresos por mes
spark.sql("""
    SELECT d.year, d.month,
           COUNT(DISTINCT fo.order_id) AS total_pedidos,
           ROUND(SUM(fo.total_amount), 2) AS ingresos_totales,
           ROUND(AVG(fo.total_amount), 2) AS ticket_promedio
    FROM fact_orders fo
    JOIN dim_date d ON fo.date_key = d.date_key
    GROUP BY d.year, d.month
    ORDER BY d.year, d.month
""").coalesce(1).write.csv("/opt/spark/output/tendencias/ingresos_mensuales",
                           header=True, mode="overwrite")

# 2. Top 10 productos más vendidos
spark.sql("""
    SELECT dp.nombre, dp.categoria,
           SUM(fod.quantity) AS total_unidades,
           ROUND(SUM(fod.subtotal), 2) AS ingresos_totales
    FROM fact_order_details fod
    JOIN dim_product dp ON fod.product_key = dp.product_key
    GROUP BY dp.nombre, dp.categoria
    ORDER BY total_unidades DESC LIMIT 10
""")

# 3. Categoría más popular por trimestre
spark.sql("""
    SELECT d.year, CEIL(d.month / 3.0) AS trimestre,
           dp.categoria, SUM(fod.quantity) AS total_unidades,
           RANK() OVER (PARTITION BY d.year, CEIL(d.month / 3.0)
                        ORDER BY SUM(fod.quantity) DESC) AS ranking
    FROM fact_order_details fod
    JOIN dim_product dp ON fod.product_key = dp.product_key
    JOIN fact_orders fo ON fod.order_id = fo.order_id
    JOIN dim_date d ON fo.date_key = d.date_key
    GROUP BY d.year, CEIL(d.month / 3.0), dp.categoria
""")
```

### Job 3: Análisis de Horarios (`analisis_horarios.py`)

```python
# Peak hours: pedidos por hora y día de semana
spark.sql("""
    SELECT d.weekday, d.is_weekend, fo.hora,
           COUNT(DISTINCT fo.order_id) AS total_pedidos,
           ROUND(SUM(fo.total_amount), 2) AS ingresos_totales,
           RANK() OVER (PARTITION BY d.weekday
                        ORDER BY COUNT(DISTINCT fo.order_id) DESC) AS ranking_hora
    FROM fact_orders fo
    JOIN dim_date d ON fo.date_key = d.date_key
    GROUP BY d.weekday, d.is_weekend, fo.hora
""")
```

### Job 4: Análisis de Crecimiento (`analisis_crecimiento.py`)

```python
# Crecimiento mensual con window functions
spark.sql("""
    SELECT d.year, d.month,
           COUNT(DISTINCT fo.order_id) AS total_pedidos,
           ROUND(SUM(fo.total_amount), 2) AS ingresos_totales,
           ROUND(
               (SUM(fo.total_amount) - LAG(SUM(fo.total_amount))
                OVER (ORDER BY d.year, d.month)) * 100.0
               / NULLIF(LAG(SUM(fo.total_amount))
                        OVER (ORDER BY d.year, d.month), 0), 2
           ) AS crecimiento_ingresos_pct,
           ROUND(
               SUM(CASE WHEN dos.status_name = 'cancelado' THEN 1 ELSE 0 END)
               * 100.0 / NULLIF(COUNT(DISTINCT fo.order_id), 0), 2
           ) AS tasa_cancelacion_pct
    FROM fact_orders fo
    JOIN dim_date d ON fo.date_key = d.date_key
    JOIN dim_order_status dos ON fo.status_key = dos.status_key
    GROUP BY d.year, d.month
    ORDER BY d.year, d.month
""")
```

## Orquestación Local (submit_jobs.sh)

```bash
#!/bin/bash
# submit_jobs.sh — Orquestación de jobs Spark

SPARK_MASTER="spark://spark-master:7077"
SPARK_CMD="docker exec spark-master spark-submit --master $SPARK_MASTER"

# 1. ETL (secuencial)
$SPARK_CMD /opt/scripts/etl_to_star.py

# 2. Análisis (paralelo)
$SPARK_CMD /opt/scripts/analisis_tendencias.py &
$SPARK_CMD /opt/scripts/analisis_horarios.py &
$SPARK_CMD /opt/scripts/analisis_crecimiento.py &
wait  # Espera a que los 3 terminen
```

# Visualización con Apache Superset

## Infraestructura

| Servicio | Imagen | Puerto | Propósito |
|---|---|---|---|
| superset-db | postgres:16-alpine | — | Base de datos de metadata |
| superset | apache/superset:4.1.1 (custom) | 8088 | Web UI de BI |
| superset-init | apache/superset:4.1.1 (custom) | — | Inicialización one-shot |

**Dockerfile.superset — Drivers instalados:**

```dockerfile
FROM apache/superset:4.1.1
USER root
RUN pip install --no-cache-dir \
    pyhive \
    thrift \
    thrift-sasl \
    psycopg2-binary
USER superset
```

**Conexión a Hive (vía UI de Superset):**

```
SQLALCHEMY URI: hive://hive-server:10000/default
```

## Dashboards

### Dashboard 1: Ingresos por Mes y Categoría

**Datasets virtuales:**
- `v_ingresos_mensuales` — ingresos, pedidos, ticket promedio
- `v_ventas_por_categoria` — unidades e ingresos por categoría

**Gráficos:**
1. Barras apiladas: ingresos mensuales agrupados por categoría
2. Línea de tendencia: ingresos totales (12 meses)
3. Tabla resumen: top 5 categorías por ingresos

### Dashboard 2: Actividad de Clientes por Zona Geográfica

**Datasets virtuales:**
- `v_reservas_por_ubicacion` — reservas por ciudad
- `dim_user` (COUNT) — total clientes activos

**Gráficos:**
1. Barras: reservas/pedidos por ciudad
2. Barras apiladas: distribución de clientes por ciudad
3. Big Number: total clientes activos

### Dashboard 3: Pedidos Completados vs Cancelados

**Datasets virtuales:**
- `v_tendencias_mensuales` — tendencia mensual con cancelaciones
- `fact_orders` + `dim_order_status` — proporción general

**Gráficos:**
1. Time-series: completados vs cancelados por mes
2. Pie chart: proporción general completado/cancelado
3. Tabla: tasa de cancelación por restaurante

## Automatización (setup_dashboards.py)

El script `superset/setup_dashboards.py` crea toda la configuración vía API REST de Superset:

```python
# Flujo de creación
1. POST /api/v1/database/       → Conexión Hive
2. POST /api/v1/dataset/        → 9 datasets virtuales (SQL queries)
3. POST /api/v1/chart/          → 9 charts
4. POST /api/v1/dashboard/      → 3 dashboards
5. Exportar dashboards a JSON   → superset/dashboards/
```

# Orquestación con Apache Airflow

## Infraestructura

| Servicio | Imagen | Puerto | Comando |
|---|---|---|---|
| airflow-db | postgres:16-alpine | — | PostgreSQL para metadatos |
| airflow-init | apache/airflow:2.8 (custom) | — | migrate + create admin + import connections |
| airflow-webserver | apache/airflow:2.8 (custom) | 8085 | `airflow webserver` |
| airflow-scheduler | apache/airflow:2.8 (custom) | — | `airflow scheduler` |
| airflow-worker | apache/airflow:2.8 (custom) | — | `celery worker` |

**Dockerfile — Dependencias instaladas:**

```dockerfile
FROM apache/airflow:2.8
USER root
RUN apt-get update && apt-get install -y --no-install-recommends \
    openjdk-17-jre-headless curl && \
    curl -O https://dlcdn.apache.org/spark/spark-3.5.0/spark-3.5.0-bin-hadoop3.tgz && \
    tar -xzf spark-3.5.0-bin-hadoop3.tgz -C /opt/ && \
    ln -s /opt/spark-3.5.0-bin-hadoop3 /opt/spark && \
    rm spark-3.5.0-bin-hadoop3.tgz
USER airflow
RUN pip install --no-cache-dir \
    apache-airflow-providers-apache-spark \
    apache-airflow-providers-celery \
    pymongo psycopg2-binary pyhive thrift thrift-sasl \
    elasticsearch requests
```

**Arquitectura Airflow:**

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   Webserver  │     │  Scheduler   │     │   Worker     │
│   (UI 8085)  │     │              │     │  (Celery)    │
└──────────────┘     └──────┬───────┘     └──────┬───────┘
                            │                    │
                     ┌──────▼────────────────────▼───────┐
                     │           Redis :6379/1            │
                     │           (Celery Broker)          │
                     └───────────────────────────────────┘
                            │                    │
                     ┌──────▼────────────────────▼───────┐
                     │     PostgreSQL (airflow-db)        │
                     │     (Metadata + Result Backend)    │
                     └───────────────────────────────────┘
```

## DAG etl_pipeline

```python
from airflow import DAG
from airflow.operators.bash import BashOperator
from airflow.providers.apache.spark.operators.spark_submit import SparkSubmitOperator
from airflow.operators.python import BranchPythonOperator, PythonOperator
from datetime import datetime, timedelta
import hashlib, os, requests

default_args = {
    "owner": "airflow",
    "retries": 2,
    "retry_delay": timedelta(minutes=5),
}

with DAG(
    "etl_pipeline",
    default_args=default_args,
    schedule="@daily",
    start_date=datetime(2024, 1, 1),
    catchup=False,
    tags=["etl", "restaurantes"],
) as dag:

    extract_data = BashOperator(
        task_id="extract_data",
        bash_command="python /opt/warehouse/etl/export_data.py",
    )

    export_neo4j = BashOperator(
        task_id="export_neo4j",
        bash_command="python /opt/warehouse/etl/export_neo4j.py",
    )

    load_neo4j = BashOperator(
        task_id="load_neo4j",
        bash_command="python /opt/warehouse/etl/load_neo4j.py",
    )

    etl_to_star = SparkSubmitOperator(
        task_id="etl_to_star",
        application="/opt/scripts/etl_to_star.py",
        conn_id="spark_default",
    )

    analisis_tendencias = SparkSubmitOperator(
        task_id="analisis_tendencias",
        application="/opt/scripts/analisis_tendencias.py",
        conn_id="spark_default",
    )

    analisis_horarios = SparkSubmitOperator(
        task_id="analisis_horarios",
        application="/opt/scripts/analisis_horarios.py",
        conn_id="spark_default",
    )

    analisis_crecimiento = SparkSubmitOperator(
        task_id="analisis_crecimiento",
        application="/opt/scripts/analisis_crecimiento.py",
        conn_id="spark_default",
    )

    # Dependencias
    extract_data >> export_neo4j >> load_neo4j >> etl_to_star
    etl_to_star >> [analisis_tendencias, analisis_horarios, analisis_crecimiento]
    [analisis_tendencias, analisis_horarios, analisis_crecimiento] >> check_reindex
    check_reindex >> [reindex_elasticsearch, skip_reindex]
```

## Reindexación Condicional de Elasticsearch

Para evitar llamadas innecesarias al servicio de reindexación, el DAG calcula el hash MD5 de
`platos.csv` después de la extracción y lo compara con el hash almacenado en `.last_platos_hash`:

```python
def _should_reindex(**context):
    if not os.path.exists(PLATOS_CSV):
        return "skip_reindex"

    md5 = hashlib.md5()
    with open(PLATOS_CSV, "rb") as f:
        for chunk in iter(lambda: f.read(8192), b""):
            md5.update(chunk)
    current_hash = md5.hexdigest()

    previous_hash = ""
    if os.path.exists(HASH_FILE):
        with open(HASH_FILE) as f:
            previous_hash = f.read().strip()

    if current_hash == previous_hash:
        return "skip_reindex"

    with open(HASH_FILE, "w") as f:
        f.write(current_hash)
    return "reindex_elasticsearch"
```

# Análisis de Grafos con Neo4j

## Infraestructura

| Servicio | Imagen | Puertos | Plugins |
|---|---|---|---|
| neo4j | neo4j:5-community | 7474 (HTTP), 7687 (Bolt) | APOC |

```yaml
neo4j:
  image: neo4j:5-community
  ports: ["7474:7474", "7687:7687"]
  environment:
    NEO4J_PLUGINS: '["apoc"]'
    NEO4J_AUTH: neo4j/password
  volumes:
    - neo4j_data:/data
    - ./neo4j/staging:/var/lib/neo4j/import
```

**Carga de datos:** `neo4j/load_schema.cypher` crea constraints, índices y carga CSVs vía `LOAD CSV`.

## Modelo de Grafo

```
(:User)────[:PLACED]────>(:Order)────[:INCLUDES]────>(:Product)
                                  │
                                  └────[:AT]────>(:Restaurant)────[:LOCATED_AT]────>(:Location)
                                                                                        │
(:Reservation)────[:MADE_RESERVATION]────>(:User)                                     [:ROAD]
                                                                                        │
(:Reservation)────[:AT]────>(:Restaurant)                                     (:Location)
```

**Nodos (6 tipos):** User, Product, Restaurant, Order, Reservation, Location
**Relaciones (6 tipos):** PLACED, INCLUDES, AT, MADE_RESERVATION, LOCATED_AT, ROAD

## Consultas Cypher

### Consulta 1: Co-compra (Frequently Bought Together)

Identifica pares de productos que se compran juntos con mayor frecuencia, útil para
recomendaciones cruzadas.

```cypher
MATCH (p1:Product)<-[:INCLUDES]-(o:Order)-[:INCLUDES]->(p2:Product)
WHERE id(p1) < id(p2)
RETURN p1.nombre AS producto_a,
       p2.nombre AS producto_b,
       count(DISTINCT o) AS veces_juntos
ORDER BY veces_juntos DESC
LIMIT 5;
```

**Resultado esperado:** Pares como ("Casado", "Jugo Natural", 120 veces juntos).

### Consulta 2: Afinidad entre usuarios

Recomendación social: usuarios que compran en los mismos restaurantes y categorías de producto.

```cypher
MATCH (u1:User)-[:PLACED]->(o1:Order)-[:AT]->(r:Restaurant),
      (u2:User)-[:PLACED]->(o2:Order)-[:AT]->(r)
WHERE u1 <> u2
WITH u1, u2, count(DISTINCT r) AS restaurantes_compartidos
OPTIONAL MATCH (u1)-[:PLACED]->(:Order)-[:INCLUDES]->(p:Product)
              <-[:INCLUDES]-(:Order)<-[:PLACED]-(u2)
WITH u1, u2, restaurantes_compartidos,
     count(DISTINCT p) AS productos_compartidos
WHERE restaurantes_compartidos > 0 OR productos_compartidos > 0
RETURN u1.nombre AS usuario_a, u2.nombre AS usuario_b,
       restaurantes_compartidos, productos_compartidos,
       3*restaurantes_compartidos + 1*productos_compartidos AS afinidad_score
ORDER BY afinidad_score DESC LIMIT 10;
```

**Score de afinidad:** `3 × restaurantes_compartidos + 1 × productos_compartidos`
(los restaurantes compartidos tienen mayor peso).

### Consulta 3: Camino mínimo entre ubicaciones (ShortestPath)

Calcula la ruta más corta entre dos ubicaciones usando relaciones `:ROAD` con distancia en km.

```cypher
// Variante con parámetros
// :param origen => "loc_7faf45d6";
// :param destino => "loc_13573677";

MATCH (a:Location {location_id: $origen}),
      (b:Location {location_id: $destino}),
      p = shortestPath((a)-[:ROAD*]-(b))
RETURN [n IN nodes(p) | n.address] AS ruta,
       reduce(acc = 0.0, r IN relationships(p) | acc + r.distance_km) AS distancia_total_km;
```

**Variante con nombre de restaurante:**

```cypher
MATCH (r1:Restaurant {nombre: "La Fonda Criolla"})-[:LOCATED_AT]->(l1:Location),
      (r2:Restaurant)-[:LOCATED_AT]->(l2:Location)
WHERE r1 <> r2
MATCH p = shortestPath((l1)-[:ROAD*]-(l2))
RETURN r1.nombre AS origen, r2.nombre AS destino,
       [n IN nodes(p) | n.address] AS ruta,
       reduce(acc = 0.0, r IN relationships(p) | acc + r.distance_km) AS distancia_km
ORDER BY distancia_km ASC;
```

### Consulta 4: Usuarios influyentes

Identificación de usuarios VIP basada en su actividad en el grafo.

```cypher
// PageRank con GDS (requiere plugin)
// CALL gds.graph.project('user-product-graph',
//     ['User', 'Order', 'Product'], ['PLACED', 'INCLUDES']);
// CALL gds.pageRank.stream('user-product-graph')
// YIELD nodeId, score
// RETURN gds.util.asNode(nodeId).nombre AS usuario, score
// ORDER BY score DESC LIMIT 5;

// Alternativa: Centralidad por grado
MATCH (u:User)-[:PLACED]->(o:Order)-[:INCLUDES]->(p:Product)
RETURN u.nombre AS usuario,
       count(DISTINCT o) AS total_pedidos,
       count(DISTINCT p) AS productos_distintos,
       count(DISTINCT o)*1.0 + count(DISTINCT p)*0.5 AS influencia_score
ORDER BY influencia_score DESC LIMIT 5;
```

# Rutas de Entrega

## Datos Geoespaciales

El script `scripts/generate_geo.py` genera coordenadas ficticias para restaurantes en
San José, Costa Rica:

- Rango de latitud: 9.92° — 9.94° (Norte)
- Rango de longitud: -84.08° — -84.10° (Oeste)
- Asignación determinística vía MD5 hash del ID del restaurante
- Relaciones `[:ROAD]` entre ubicaciones cercanas (< 5 km)
- Distancia calculada con la fórmula de Haversine

**Fórmula Haversine implementada:**

```python
def haversine(lat1, lon1, lat2, lon2):
    R = 6371  # Radio de la Tierra en km
    dlat = math.radians(lat2 - lat1)
    dlon = math.radians(lon2 - lon1)
    a = (math.sin(dlat/2)**2 +
         math.cos(math.radians(lat1)) *
         math.cos(math.radians(lat2)) *
         math.sin(dlon/2)**2)
    c = 2 * math.asin(math.sqrt(a))
    return R * c
```

## Endpoint REST

```
GET /api/delivery/routes?desde={restaurant_id}&hasta={order_id}&algoritmo=nearest|cypher
```

**Headers:** `Authorization: Bearer <JWT>` (requiere rol admin)

### Algoritmo 1: Vecino Más Cercano (TSP Greedy)

Implementado en Go puro en `handlers/delivery.go`:

```go
func nearestNeighbor(routes []types.Location, origen, destino string) RouteResponse {
    visited := make(map[string]bool)
    path := []types.Location{}
    totalDist := 0.0
    current := findByLocationID(routes, origen)
    visited[current.LocationID] = true
    path = append(path, current)

    for len(visited) < len(routes) {
        nearest, dist := findClosestUnvisited(current, routes, visited)
        visited[nearest.LocationID] = true
        path = append(path, nearest)
        totalDist += dist
        current = nearest
    }
    return RouteResponse{Ruta: path, DistanciaTotal: totalDist, Algoritmo: "nearest"}
}
```

### Algoritmo 2: Cypher ShortestPath

Delega a Neo4j para encontrar la ruta más corta en el grafo de ubicaciones:

```go
func (c *Neo4jClient) GetShortestPath(origenID, destinoID string) (*RouteResponse, error) {
    query := `
        MATCH (a:Location {location_id: $origen}),
              (b:Location {location_id: $destino}),
              p = shortestPath((a)-[:ROAD*]-(b))
        RETURN [n IN nodes(p) | n.address] AS addresses,
               [n IN nodes(p) | n.lat] AS lats,
               [n IN nodes(p) | n.lon] AS lons,
               reduce(acc = 0.0, r IN relationships(p) | acc + r.distance_km) AS total_km
    `
    // Ejecuta query con parámetros, parsea resultado
}
```

### Zonificación

`assignZones()` agrupa ubicaciones en una grilla de 2×2 km para asignación eficiente por
repartidor:

```go
func assignZones(locations []types.Location) map[string][]types.Location {
    zones := make(map[string][]types.Location)
    for _, loc := range locations {
        zoneX := int(loc.Lat * 100) / 2   // Grilla de ~2km
        zoneY := int(loc.Lon * 100) / 2
        zoneKey := fmt.Sprintf("%d-%d", zoneX, zoneY)
        zones[zoneKey] = append(zones[zoneKey], loc)
    }
    return zones
}
```

## Graceful Degradation

Si Neo4j no está disponible, `NewRepos()` setea `Neo4j = nil` y el endpoint responde con
HTTP 503:

```json
{
    "error": "Servicio de enrutamiento no disponible"
}
```

# Pruebas y Validaciones

La carpeta `scripts/` contiene 6 scripts de validación automatizada que verifican cada capa
del sistema OLAP.

## verify_warehouse.py

**5 checks de integridad de datos:**

| # | Check | Método |
|---|---|---|
| 1 | Conteo de registros: BD operacional vs Hive | `SELECT COUNT(*)` en ambas fuentes para usuarios, restaurantes, platos, pedidos, reservas |
| 2 | Surrogate keys no nulas en fact tables | `SELECT COUNT(*) FROM fact_orders WHERE date_key IS NULL OR user_key IS NULL OR ...` |
| 3 | Sin llaves huérfanas en facts | 9 combinaciones: `fact_orders.date_key NOT IN (SELECT date_key FROM dim_date)` |
| 4 | Consistencia de total_amount | `fact_orders.total_amount = SUM(fact_order_details.subtotal)` por order_id |
| 5 | Unicidad de surrogate keys | `SELECT user_key, COUNT(*) FROM dim_user GROUP BY user_key HAVING COUNT(*) > 1` |

## verify_etl.sh

**3 checks del pipeline de extracción:**

| # | Check |
|---|---|
| 1 | `export_data.py` se ejecuta sin errores (exit code 0) |
| 2 | Idempotencia: 2 corridas consecutivas producen mismos archivos y conteo de filas |
| 3 | Graceful degradation: al detener la BD, el script produce error descriptivo (no crash silencioso) |

## verify_spark.sh

**3 checks de los análisis Spark:**

| # | Check |
|---|---|
| 1 | Los 3 scripts PySpark se ejecutan sin excepción vía `docker exec spark-submit` |
| 2 | 7 outputs CSV existen con header y datos no vacíos |
| 3 | Validación cruzada: consulta Hive directo vía beeline para comparación manual |

## verify_neo4j.sh

**4 checks del grafo Neo4j:**

| # | Check |
|---|---|
| 1 | Co-compra: top 5 pares con productos distintos y `veces_juntos > 0` |
| 2 | Afinidad: pares de usuarios con `afinidad_score > 0` |
| 3 | ShortestPath: ruta válida entre 2 Location nodes reales con distancia > 0 |
| 4 | Centralidad: top usuarios con `influencia_score > 0` |

## verify_airflow.sh

**4 checks de orquestación:**

| # | Check |
|---|---|
| 1 | DAG `etl_pipeline` existe en Airflow |
| 2 | Trigger manual + polling hasta completar (timeout 5 min) |
| 3 | Logs de 5 tasks sin ERROR/EXCEPTION/TRACEBACK |
| 4 | Árbol de dependencias correcto: extract → etl → análisis en paralelo |

## verify_all.sh

Orquestador maestro que ejecuta las 5 validaciones secuencialmente:

```bash
#!/bin/bash
# verify_all.sh — Integración completa OLAP

echo "=== OLAP Verification Suite ==="
./scripts/verify_etl.sh;       etl_result=$?
python scripts/verify_warehouse.py; warehouse_result=$?
./scripts/verify_spark.sh;     spark_result=$?
./scripts/verify_neo4j.sh;     neo4j_result=$?
./scripts/verify_airflow.sh;   airflow_result=$?

echo ""
echo "=== Resumen ==="
echo "ETL:        $([ $etl_result -eq 0 ] && echo 'PASS' || echo 'FAIL')"
echo "Warehouse:  $([ $warehouse_result -eq 0 ] && echo 'PASS' || echo 'FAIL')"
echo "Spark:      $([ $spark_result -eq 0 ] && echo 'PASS' || echo 'FAIL')"
echo "Neo4J:      $([ $neo4j_result -eq 0 ] && echo 'PASS' || echo 'FAIL')"
echo "Airflow:    $([ $airflow_result -eq 0 ] && echo 'PASS' || echo 'FAIL')"

exit $((etl_result | warehouse_result | spark_result | neo4j_result | airflow_result))
```

# Decisiones Técnicas

## Esquema Estrella vs Copo de Nieve

Se eligió **esquema estrella** porque las dimensiones son planas sin jerarquías profundas
(no hay subcategorías de productos, ni regiones geográficas anidadas). Esto simplifica las
consultas (menos JOINs) y mejora el rendimiento en Hive, que está optimizado para lecturas
secuenciales sobre tablas desnormalizadas.

## Spark vs Python Puro para Transformación

La transformación a modelo estrella se realiza con **PySpark** en lugar de Python+pandas por
tres razones:

1. **Procesamiento distribuido**: Spark puede escalar horizontalmente añadiendo workers
2. **Escritura directa a Hive**: `df.write.saveAsTable()` evita la necesidad de scripts de
   carga intermedios
3. **Broadcast joins**: Las dimensiones pequeñas se transmiten a todos los nodos, evitando
   shuffles costosos

## Broadcast Hash Joins vs Shuffle Joins

Las tablas de dimensión son pequeñas (cientos de filas) comparadas con las fact tables
(miles/millones de filas). Spark realiza **broadcast hash joins** automáticamente, donde la
dimensión se copia a cada nodo ejecutor, eliminando la necesidad de shuffle en el JOIN.

## SCD Tipo 0 (No Historizar)

Se implementó **Slowly Changing Dimension Tipo 0** porque:

- Los datos provienen de un sistema transaccional (OLTP) donde las dimensiones representan
  el estado actual
- El pipeline ETL es diario y reemplaza completamente el contenido de las tablas
- Para un sistema en producción con requerimientos de historización, se migraría a SCD Tipo 2
  agregando `valid_from` y `valid_to`

## Particionamiento de Fact Tables

Las tablas de hechos se particionan por `year, month` para:

- **Pruning de particiones**: Las consultas con filtros de tiempo solo escanean las particiones
  relevantes
- **Mantenibilidad**: Facilita la carga incremental y la purga de datos antiguos
- **Rendimiento**: Reduce significativamente los datos escaneados en consultas temporales

## Reindexación Condicional de Elasticsearch

En lugar de reindexar Elasticsearch en cada ejecución del DAG, se calcula el **hash MD5** del
archivo `platos.csv` y solo se reindexa si el hash cambió. Esto evita llamadas innecesarias a
la API de reindexación cuando no hay modificaciones en el catálogo de platos.

## Graceful Degradation en Toda la Arquitectura

Cada componente del sistema tiene un plan de fallo controlado:

| Componente | Fallo | Comportamiento |
|---|---|---|
| Redis | Caído | Middleware HTTP omite caché, pasa al handler |
| Neo4j | Caído | Endpoint de rutas responde 503 |
| Elasticsearch | Caído | Búsqueda responde 500 con mensaje claro |
| Hive | Caído | Spark jobs fallan con error descriptivo |
| Airflow worker | Caído | Celery reasigna tareas a otros workers |

# Ejemplos de Uso

## Ejecutar ETL Manualmente

```bash
# 1. Extraer datos (MongoDB o PostgreSQL según DB_ENGINE)
python warehouse/etl/export_data.py

# 2. Transformar y cargar a Hive con Spark
docker exec spark-master spark-submit \
    --master spark://spark-master:7077 \
    /opt/scripts/etl_to_star.py

# 3. Ejecutar análisis en paralelo
docker exec spark-master spark-submit \
    --master spark://spark-master:7077 \
    /opt/scripts/analisis_tendencias.py &
docker exec spark-master spark-submit \
    --master spark://spark-master:7077 \
    /opt/scripts/analisis_horarios.py &
docker exec spark-master spark-submit \
    --master spark://spark-master:7077 \
    /opt/scripts/analisis_crecimiento.py &
wait
```

## Consultar una Vista OLAP en Hive

```bash
docker exec hive-server beeline -u jdbc:hive2://localhost:10000/default \
    -e "SELECT * FROM v_ingresos_mensuales ORDER BY year, month LIMIT 10;" \
    --outputformat=csv2
```

## Disparar el DAG de Airflow

```bash
docker exec airflow-webserver airflow dags trigger etl_pipeline

# Ver progreso
docker exec airflow-webserver airflow dags list-runs -d etl_pipeline

# Ver logs de una tarea específica
docker exec airflow-webserver airflow tasks logs \
    etl_pipeline extract_data 2025-03-15
```

## Acceder a Superset

```
URL: http://localhost:8088
Usuario: admin
Contraseña: admin
```

Crear una conexión a Hive con: `hive://hive-server:10000/default`

## Ejecutar Consultas Cypher en Neo4j

```bash
# Co-compra: top 5 productos más comprados juntos
docker exec neo4j cypher-shell -u neo4j -p password \
    "MATCH (p1:Product)<-[:INCLUDES]-(o:Order)-[:INCLUDES]->(p2:Product)
     WHERE id(p1) < id(p2)
     RETURN p1.nombre, p2.nombre, count(DISTINCT o) AS veces_juntos
     ORDER BY veces_juntos DESC LIMIT 5;"

# ShortestPath entre dos restaurantes
docker exec neo4j cypher-shell -u neo4j -p password \
    "MATCH (r1:Restaurant {nombre: 'La Fonda Criolla'})-[:LOCATED_AT]->(l1:Location),
           (r2:Restaurant)-[:LOCATED_AT]->(l2:Location)
     WHERE r1 <> r2
     MATCH p = shortestPath((l1)-[:ROAD*]-(l2))
     RETURN r1.nombre, r2.nombre,
            [n IN nodes(p) | n.address] AS ruta,
            reduce(acc=0.0, r IN relationships(p) | acc + r.distance_km) AS distancia_km
     ORDER BY distancia_km ASC LIMIT 5;"
```

## Probar el Endpoint de Rutas de Entrega

```bash
# Obtener token JWT
TOKEN=$(curl -s -X POST http://localhost/api/auth/login \
    -H 'Content-Type: application/json' \
    -d '{"email":"admin@mail.com","password":"admin"}' | jq -r '.access_token')

# Algoritmo Vecino Más Cercano
curl -s "http://localhost/api/delivery/routes?desde=rest-001&hasta=order-050&algoritmo=nearest" \
    -H "Authorization: Bearer $TOKEN" | jq

# Algoritmo Cypher ShortestPath
curl -s "http://localhost/api/delivery/routes?desde=rest-001&hasta=order-050&algoritmo=cypher" \
    -H "Authorization: Bearer $TOKEN" | jq
```

## Validar el Sistema Completo

```bash
# Ejecutar todas las validaciones
bash scripts/verify_all.sh

# Ejecutar validación individual
python scripts/verify_warehouse.py
bash scripts/verify_spark.sh
```

# Apéndice A: Infraestructura Docker

## Servicios OLAP

| Servicio | Imagen | Puerto | Dependencias |
|---|---|---|---|
| hadoop-namenode | apache/hadoop:3.3.6 | 9870 (HTTP), 9000 (RPC) | — |
| hadoop-datanode | apache/hadoop:3.3.6 | — | namenode |
| hive-metastore-db | postgres:16-alpine | — | — |
| hive-metastore | apache/hive:4.0.0 | 9083 | metastore-db |
| hive-server | apache/hive:4.0.0 | 10000 | metastore, namenode |
| spark-master | apache/spark:3.5.0 | 8083 (UI), 7077 (workers) | — |
| spark-worker | apache/spark:3.5.0 | — | spark-master |
| superset-db | postgres:16-alpine | — | — |
| superset | apache/superset:4.1.1 | 8088 | superset-db, hive-server |
| superset-init | apache/superset:4.1.1 | — | superset-db |
| airflow-db | postgres:16-alpine | — | — |
| airflow-init | apache/airflow:2.8 | — | airflow-db |
| airflow-webserver | apache/airflow:2.8 | 8085 | airflow-db |
| airflow-scheduler | apache/airflow:2.8 | — | airflow-db |
| airflow-worker | apache/airflow:2.8 | — | airflow-db |
| neo4j | neo4j:5-community | 7474 (HTTP), 7687 (Bolt) | — |

## Volúmenes Persistentes

| Volumen | Montaje | Propósito |
|---|---|---|
| hdfs_data | `/data` | Datos HDFS (NameNode + DataNode) |
| hive_metastore_data | `/var/lib/postgresql/data` | Metadata de Hive |
| superset_db_data | `/var/lib/postgresql/data` | Metadata de Superset |
| superset_data | `/app/data` | Datos de Superset |
| airflow_db_data | `/var/lib/postgresql/data` | Metadata de Airflow |
| airflow_logs | `/opt/airflow/logs` | Logs de Airflow |
| neo4j_data | `/data` | Base de datos Neo4j |

# Apéndice B: Comandos Útiles

## Inicialización

```bash
# Iniciar todos los servicios OLAP
docker-compose up -d hadoop-namenode hadoop-datanode
docker-compose up -d hive-metastore-db hive-metastore hive-server
docker-compose up -d spark-master spark-worker
docker-compose up -d superset-db
docker-compose run --rm superset-init
docker-compose up -d superset
docker-compose up -d airflow-db
docker-compose run --rm airflow-init
docker-compose up -d airflow-webserver airflow-scheduler airflow-worker
docker-compose up -d neo4j

# Inicializar esquema Hive
docker exec hive-server bash /opt/warehouse/init_hive.sh
docker exec hive-server bash /opt/warehouse/init_olap_views.sh

# Generar datos de prueba
docker exec api python scripts/generate_oltp_data.py
docker exec api python scripts/generate_geo.py

# Cargar Neo4j
bash neo4j/init_neo4j.sh

# Configurar Superset
docker exec superset python /app/setup_dashboards.py
```

## Monitoreo

```bash
# Ver logs de Airflow
docker-compose logs --timestamps -f airflow-webserver

# Ver Spark UI
open http://localhost:8083

# Ver estado de HDFS
curl http://localhost:9870/jmx

# Ver logs de Hive
docker logs hive-server

# Ver logs de Spark
docker logs spark-master

# Consultar estado de Neo4j
docker exec neo4j cypher-shell -u neo4j -p password "CALL dbms.listConfig();"
```

## Troubleshooting

```bash
# Hive no responde
docker exec hive-server beeline -u jdbc:hive2://localhost:10000/default -e "SHOW TABLES;"

# Spark no conecta a Hive
docker exec spark-master spark-sql \
    --conf spark.hadoop.hive.metastore.uris=thrift://hive-metastore:9083 \
    -e "SHOW TABLES;"

# Airflow: reiniciar scheduler
docker-compose restart airflow-scheduler

# Neo4j: verificar conectividad
docker exec neo4j cypher-shell -u neo4j -p password \
    "MATCH (n) RETURN count(n) AS total_nodos;"

# Reindexar Elasticsearch forzado
curl -X POST http://localhost/search/reindex
```
