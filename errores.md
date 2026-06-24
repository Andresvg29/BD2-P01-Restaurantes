# Registro de Errores

## Formato

```
## [YYYY-MM-DD] - Título del error

**Error:**
Mensaje de error exacto

**Endpoint / Ruta:**
`GET /api/...`

**Causa:**
Explicación de la causa raíz

**Solución:**
Descripción de la solución aplicada

**Archivos afectados:**
- `ruta/al/archivo.go:linea`
```

---

## [2026-06-01] - interface conversion: interface {} is primitive.ObjectID, not string

**Error:**
```
interface conversion: interface {} is primitive.ObjectID, not string
```

**Endpoint / Ruta:**
`GET /restaurants`

**Causa:**
En `RestaurantRepository.Create()` no se asignaba un `_id` explícito al documento antes de insertarlo en MongoDB. MongoDB generaba automáticamente un `ObjectID` (tipo `primitive.ObjectID`), pero al leer los documentos en `List()` y `GetByID()` se intentaba hacer `doc["_id"].(string)`, causando un panic.

Los demás repositorios (`UserRepository`, `MenuRepository`, `OrderRepository`, `ReservationRepository`) sí asignaban `"_id": primitive.NewObjectID().Hex()` explícitamente.

**Solución:**
Agregar `"_id": primitive.NewObjectID().Hex()` al `bson.M` del nuevo restaurante en `RestaurantRepository.Create()`, de forma consistente con el resto de repositorios.

```go
id := primitive.NewObjectID().Hex()
nuevoRestaurante := bson.M{
    "_id":              id,
    "nombre":           nombre,
    "direccion":        direccion,
    "telefono":         telefono,
    "id_administrador": adminID,
    "capacidad_total":  capacidad,
    "fecha_creacion":   time.Now(),
}
```

**Archivos afectados:**
- `repository/mongo/restaurant.go:139` — `List()` ahora usa `objectIDToString()` para `_id` y `toInt()` para `capacidad_total`
- `repository/mongo/restaurant.go:162` — `GetByID()` ahora usa `toInt()` para `capacidad_total`
- `repository/mongo/restaurant.go:167` — `Create()` ahora asigna `_id` explícito como string
- `repository/mongo/restaurant.go:452` — Nueva función helper `objectIDToString()`

---

## [2026-06-01] - interface conversion: interface {} is int32, not float64

**Error:**
```
interface conversion: interface {} is int32, not float64
```

**Endpoint / Ruta:**
`POST /search/reindex`

**Causa:**
En `MenuRepository.List()` se hacía `doc["precio"].(float64)`, pero MongoDB puede almacenar números sin decimales como `int32`. Esto ocurría al reindexar en ElasticSearch o al listar platos desde la colección `platos`.

**Solución:**
Se creó la función helper `toFloat64()` que maneja `float64`, `int32`, `int`, `int64`, y nil. Se aplicó en:
- `MenuRepository.List()`
- `MenuRepository.GetByRestaurant()`
- `OrderRepository.Create()` (lectura de precio del plato)
- `OrderRepository.GetByID()` (para `total`)

Además se reemplazaron casteos directos `.(int32)` con `toInt()` en:
- `OrderRepository.GetByID()` (`id_estado_pedido`)
- `ReservationRepository.GetRestaurantCapacity()` (`capacidad_total`)

**Archivos afectados:**
- `repository/mongo/restaurant.go:235` — `List()` usa `toFloat64()` para `precio`
- `repository/mongo/restaurant.go:259` — `GetByRestaurant()` usa `toFloat64()` para `precio`
- `repository/mongo/restaurant.go:400` — `Create()` usa `toFloat64()` para `precio`
- `repository/mongo/restaurant.go:436` — `GetByID()` usa `toInt()` y `toFloat64()`
- `repository/mongo/restaurant.go:338` — `GetRestaurantCapacity()` usa `toInt()`
- `repository/mongo/restaurant.go:442` — Nueva función helper `toFloat64()`

---

## [2026-06-01] - Corrección masiva de type assertions inseguras

**Problema:**
25 type assertions directas en todo el proyecto que podían panicear si el valor era `nil` o de un tipo inesperado.

**Solución:**

**Nuevos helpers en `repository/mongo/restaurant.go`:**
- `toString()` — maneja `string`, `primitive.ObjectID`, `nil`
- `toBool()` — maneja `bool`, `nil`

**Archivos afectados:**

| Archivo | Cambio |
|---|---|
| `repository/mongo/restaurant.go` | `toString()` aplicado en todos los campos string de `UserRepository.GetByID`, `UserRepository.GetByEmail`, `UserRepository.Create`, `RestaurantRepository.List`, `RestaurantRepository.GetByID`, `MenuRepository.List`, `MenuRepository.GetByRestaurant`, `MenuRepository.GetAdminID`, `ReservationRepository.GetOwnerID`, `OrderRepository.GetByID` |
| `repository/mongo/restaurant.go` | `toBool()` aplicado en `MenuRepository.List`, `MenuRepository.GetByRestaurant`, `OrderRepository.GetByID` |
| `repository/mongo/restaurant.go` | `UserRepository.GetByID`: se eliminó la aserción directa de `fecha_creacion` en el literal del struct (línea 37) que paniqueaba antes de llegar al safe-check existente |
| `handlers/search.go` | Parseo de respuesta de Elasticsearch con `comma-ok` en lugar de aserciones encadenadas |
| `middleware/auth.go` | `RequireRole` y `RequireAdmin` ahora usan `comma-ok` en lugar de aserciones directas |

---

## [2026-06-23] - HiveServer2: Spark ETL escribe en ruta local en vez de HDFS

**Error:**
El ETL a star schema escribía las tablas en `file:/opt/hive/data/warehouse/fact_orders` en lugar de `hdfs://hadoop-namenode:9000/user/hive/warehouse/fact_orders`.

**Causa:**
El `hive.metastore.warehouse.dir` dentro del contenedor `hive-metastore` apuntaba a una ruta local (`/opt/hive/data/warehouse`). No existía un `hive-site.xml` personalizado montado que apuntara a HDFS.

**Solución:**
Crear `hive/hive-site.xml` con `hive.metastore.warehouse.dir=hdfs://hadoop-namenode:9000/user/hive/warehouse` y montarlo como volumen de solo lectura en `/opt/hive/conf/hive-site.xml` del contenedor `hive-metastore`. Al reiniciar, el metastore re-creó el schema en PostgreSQL y las tablas siguientes se crearon en HDFS.

**Archivos afectados:**
- `hive/hive-site.xml` — nuevo archivo con configuración corregida
- `docker-compose.yml` — volumen montado para hive-metastore

---

## [2026-06-23] - Superset 4.1.1: SQL Lab muestra "Unable to migrate query editor state to backend" (KeyError: 'dbId')

**Error:**
```
KeyError: 'dbId'
```
Al abrir SQL Lab en Superset, aparece el mensaje "Unable to migrate query editor state to backend" en la consola del navegador.

**Causa:**
Error en `superset/views/sql_lab/views.py` línea 94. El código usa `database_id=query_editor['dbId']` con acceso directo por clave, pero `query_editor` puede no contener `'dbId'` en algunos estados, lanzando `KeyError`.

**Solución:**
Reemplazar `query_editor['dbId']` por `query_editor.get('dbId')` para que retorne `None` en lugar de lanzar excepción cuando la clave no existe.

**Archivos afectados:**
- `superset/views/sql_lab/views.py:94` — cambio de `['dbId']` a `.get('dbId')`

---

## [2026-06-23] - HiveServer2: "The query returned no data" en Superset SQL Lab

**Error:**
Superset SQL Lab conectado a HiveServer2 (`hive://hive-server:10000/default`) ve las tablas pero al ejecutar queries retorna "The query returned no data".

**Causa:**
El archivo `/opt/hadoop/etc/hadoop/core-site.xml` dentro del contenedor `hive-server` estaba vacío (sin `fs.defaultFS`). HiveServer2 no sabía a qué sistema de archivos apuntar para leer los datos (Parquet en HDFS), por lo que las queries no encontraban datos.

**Solución:**
Crear `hadoop/core-site.xml` con `<property><name>fs.defaultFS</name><value>hdfs://hadoop-namenode:9000</value></property>` y montarlo como volumen de solo lectura en `/opt/hadoop/etc/hadoop/core-site.xml` del contenedor `hive-server`.

**Archivos afectados:**
- `hadoop/core-site.xml` — nuevo archivo con configuración de HDFS
- `docker-compose.yml` — volumen montado para hive-server

---

## [2026-06-23] - HiveServer2: No abre puerto 10000 (Tez se cuelga sin YARN)

**Error:**
HiveServer2 no acepta conexiones en el puerto 10000. `Connection refused` desde cualquier contenedor. El log muestra `Hive Session ID = ...` pero nunca reporta el inicio del Thrift listener.

**Causa:**
Hive 4.0.0 usa Tez como motor de ejecución por defecto (`hive.execution.engine=tez`). Tez intenta crear una sesión con YARN al iniciar HiveServer2, pero no hay un clúster YARN en el entorno, por lo que la inicialización se cuelga antes de abrir el puerto Thrift. Aunque `tez.local.mode=true` y `mapreduce.framework.name=local` están configurados, la sesión de Tez local aún se traba.

**Solución:**
Reemplazar HiveServer2 con **Spark Thrift Server** (`start-thriftserver.sh`) en el contenedor `spark-master`. Spark Thrift Server se conecta al mismo Hive Metastore remoto (`thrift://hive-metastore:9083`), usa el warehouse HDFS y no requiere YARN (usa el cluster Spark standalone existente).

**Archivos afectados:**
- Ninguno (comando manual: `start-thriftserver.sh --master spark://spark-master:7077`)
- La conexión en Superset cambia de `hive://hive-server:10000/default` a `hive://spark-master:10000/default`

---

## [2026-06-23] - Superset charts: "Columns missing in dataset" + "Found invalid orderby options"

**Error:**
```
DB engine Error
Missing label
```
Al abrir los charts en Superset aparece el mensaje de error del chart, acompañado en consola de "Columns missing in dataset" y "Found invalid orderby options".

**Causa raíz:**
Los virtual datasets creados vía SQL no tenían columnas registradas en la tabla `table_columns`. Los charts usaban `all_columns` y `order_by_cols` referenciando nombres de columna que Superset no reconocía.

**Solución (3 iteraciones):**

1. **Primera iteración (fallida):** Uso de `all_columns + order_by_cols` en parámetros de chart sin columnas registradas → "Columns missing in dataset" + "invalid orderby options".

2. **Segunda iteración (fallida):** Se registraron las columnas en `table_columns` pero los charts usaban `groupby` con formato de objetos (`{"column_type": "BASE", "column_name": "year"}`) y `metrics` con `"type"` extra en la columna → "Missing label".

3. **Tercera iteración (correcta):** Formato correcto:
   - `groupby` como array de strings: `["year", "month", "categoria"]`
   - `metrics` sin `"type"` en `column`: `{"expressionType": "SIMPLE", "column": {"column_name": "total"}, "aggregate": "SUM", "label": "SUM(total)"}`

**Archivos afectados:**
- `spark/create_dashboards_v5.py` y `spark/fix_dashboards.py` — scripts de creación de dashboards

---

## [2026-06-23] - Superset charts: "DB engine Error / Missing label" (Issue 1011)

**Error:**
```
DB engine Error
Missing label
```
Referencia: Issue 1011 de Superset.

**Causa:**
Los parámetros de los charts usaban `groupby` en formato de objetos anidados (`[{"column_type": "BASE", "column_name": "year"}]`) en lugar del formato plano de strings (`["year"]`). Además, los objetos `metrics` incluían un campo `"type"` dentro de `"column"` que no es parte del contrato del SimpleMetric en Superset 4.

**Solución:**
Corregir los parámetros de los 3 charts:
- `groupby`: de array de objetos a array de strings
- `metrics`: eliminar `"type"` del objeto `column`

Formato final correcto:
```json
{
  "groupby": ["estado"],
  "metrics": [
    {
      "expressionType": "SIMPLE",
      "column": {"column_name": "cantidad"},
      "aggregate": "SUM",
      "label": "SUM(cantidad)"
    }
  ]
}
```

**Archivos afectados:**
- `spark/fix_dashboards.py` — script de corrección

---

## [2026-06-23] - Spark Thrift Server: UNRESOLVED_COLUMN en virtual datasets de Superset

**Error:**
```
A column or function parameter with name `s`.`descripcion` cannot be resolved.
Did you mean one of the following?
[`fo`.`hora`, `fo`.`month`, `fo`.`order_id`, `fo`.`user_key`, `fo`.`year`].
```

**Endpoint / Ruta:**
Charts de Superset contra Spark Thrift Server (`hive://spark-master:10000/default`), virtual datasets de los dashboards.

**Causa raíz:**
Las SQL queries de los virtual datasets referencian columnas que no existen en las tablas Parquet reales del star schema:

| Dataset | Columna usada | Columna real | Estado |
|---|---|---|---|
| `estado_pedidos` | `s.descripcion` | `dim_order_status.status_name` | ❌ |
| `actividad_ciudad` | `u.ciudad` | `dim_user` NO tiene `ciudad` | ❌ |
| `actividad_ciudad` | `u.id_usuario` | `dim_user.user_id` | ❌ |
| `ingresos_mes_cat` | `p.categoria`, `d.year`, `d.month` | correctas | ✅ |

Además, `dim_user` generada por el ETL no incluye datos de ubicación (solo `user_key`, `user_id`, `nombre`, `email`, `role_name`). La ciudad está únicamente en `dim_restaurant`.

**Solución:**
1. `estado_pedidos`: cambiar `s.descripcion` → `s.status_name`
2. `actividad_ciudad`: reescribir query para unir `fact_orders → dim_restaurant` en vez de `fact_orders → dim_user`:
   ```sql
   SELECT r.ciudad,
          COUNT(DISTINCT fo.order_id) as pedidos,
          ROUND(SUM(fo.total_amount), 2) as total_gastado,
          COUNT(DISTINCT fo.user_key) as clientes_unicos
   FROM fact_orders fo
   JOIN dim_restaurant r ON fo.restaurant_key = r.restaurant_key
   GROUP BY r.ciudad
   ORDER BY total_gastado DESC
   ```
3. Limpiar y recrear los 3 virtual datasets con SQL corregidas + columnas en `table_columns`
4. Re-crear charts, dashboards y re-exportar a `spark/exports/`

**Archivos afectados:**
- `spark/fix_dashboards_v2.py` — SQL queries incorrectas en líneas 63-76
- `spark/etl_to_star.py` — `dim_user` no incluye columna `ciudad` (podría agregarse si es necesario)
