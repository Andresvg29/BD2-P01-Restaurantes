# Plan de Ejecución — BD2-P01-Restaurantes

## Prerrequisitos

- Docker Desktop 24+ con al menos 8 GB de RAM asignados
- Puertos libres: 80, 5432, 6379, 8080, 27017–27019, 9200, 9870, 10000, 8083, 8088, 8085, 7474, 7687
- Git y Python 3.10+ instalados
- `.env` configurado con `DB_ENGINE=postgres` o `DB_ENGINE=mongodb`

---

## Fase 0 — Preparación del Entorno

```bash
# 1. Clonar y preparar
cd BD2-P01-Restaurantes
mkdir -p warehouse/etl/staging neo4j/staging spark/output

# 2. Verificar Docker
docker info

# 3. Revisar configuración
cat .env
# Asegurar: DB_ENGINE, POSTGRES_*, REDIS_HOST, ELASTICSEARCH_URL, etc.
```

---

## Fase 1 — Infraestructura Base

```bash
# 4. Iniciar servicios core (PostgreSQL + Keycloak + Redis)
docker-compose up -d db keycloak-db keycloak redis

# 5. Verificar BD operacional
# PostgreSQL se auto-inicializa con init.sql
docker exec db psql -U admin -d restaurante_db -c "\dt"
docker exec db psql -U admin -d restaurante_db -c "SELECT count(*) FROM restaurantes;"
# Debe mostrar: 3

# 6. (Opcional) Si DB_ENGINE=mongodb, iniciar cluster MongoDB
docker-compose up -d configsvr mongo1 mongo2 mongo3 mongos
docker exec mongo1 mongosh --eval \
  "rs.initiate({_id:'shard0',members:[{_id:0,host:'mongo1:27018'},{_id:1,host:'mongo2:27018'},{_id:2,host:'mongo3:27018'}]})"
docker exec mongos mongosh --eval "sh.addShard('shard0/mongo1:27018,mongo2:27018,mongo3:27018')"
docker exec mongos mongosh /initMongo.js

# 7. Iniciar Elasticsearch + Kibana
docker-compose up -d elasticsearch kibana
```

---

## Fase 2 — Infraestructura OLAP

```bash
# 8. Hadoop HDFS
docker-compose up -d hadoop-namenode hadoop-datanode

# 9. Hive
docker-compose up -d hive-metastore-db hive-metastore hive-server

# 10. Inicializar esquema estrella (9 tablas DDL)
docker exec hive-server /opt/warehouse/init_hive.sh

# 11. Crear vistas OLAP (6 vistas)
docker exec hive-server /opt/warehouse/init_olap_views.sh

# 12. Spark
docker-compose up -d spark-master
docker-compose up -d --scale spark-worker=2 spark-worker
```

---

## Fase 3 — BI y Orquestación

```bash
# 13. Superset
docker-compose up -d superset-db superset-init superset

# 14. Configurar Superset
docker-compose run --rm superset-init
docker exec superset pip install -r /app/requirements.txt
docker exec superset python /app/setup_dashboards.py

# 15. Airflow
docker-compose up -d airflow-db
docker-compose run --rm airflow-init
docker-compose up -d airflow-webserver airflow-scheduler airflow-worker
```

---

## Fase 4 — Grafos y Geolocalización

```bash
# 16. Neo4j
docker-compose up -d neo4j

# 17. Generar coordenadas geoespaciales
cd scripts
pip install -r ../warehouse/requirements.txt
python generate_geo.py
cd ..

# 18. Importar datos a Neo4j (exporta OLTP → CSVs → Neo4j)
cd neo4j && bash init_neo4j.sh && cd ..
```

---

## Fase 5 — API, Search y Nginx

```bash
# 19. Construir y levantar API + Search
docker-compose up -d api search

# 20. Nginx
docker-compose up -d nginx

# 21. Verificar
curl http://localhost/api/restaurants
curl http://localhost/search/products?q=ceviche
```

---

## Fase 6 — Población Masiva de Datos

```bash
# 22. Generar datos OLTP para nutrir OLAP
# Crea cientos de pedidos/reservas distribuidos en 6-12 meses
python scripts/generate_oltp_data.py
```

> **Nota:** `scripts/generate_oltp_data.py` debe crearse como parte de los cambios
> pre-ejecución (ver `TODO.md` sección "Cambios Pendientes pre-Ejecución").

---

## Fase 7 — ETL y Análisis Spark

### Opción A: Local
```bash
cd warehouse/etl && python run_etl.py && cd ../..
```

### Opción B: Docker exec directo
```bash
# Exportar datos desde el contenedor donde corra Python
docker exec api python /app/warehouse/etl/export_data.py

# Spark jobs
cd spark && bash submit_jobs.sh && cd ..
```

### Opción C: Airflow
```bash
docker exec airflow-webserver airflow dags trigger etl_pipeline
# Monitorear en http://localhost:8085 (admin / admin)
```

---

## Fase 8 — Verificación

```bash
# 23. Suite completa de validación
bash scripts/verify_all.sh

# 24. Verificaciones individuales
python scripts/verify_warehouse.py       # Conteos en Hive vs OLTP
bash scripts/verify_etl.sh               # Idempotencia y graceful degradation
bash scripts/verify_spark.sh             # Outputs CSV de análisis
bash scripts/verify_neo4j.sh             # Consultas Cypher
bash scripts/verify_airflow.sh           # DAG execution

# 25. Superset dashboards
# Abrir http://localhost:8088 (admin / admin)

# 26. Probar endpoint de rutas de entrega
TOKEN=$(curl -s -X POST http://localhost/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"carlos.vargas@lafondacr.com","password":"..."}' | jq -r '.access_token')

REST_ID=$(curl -s http://localhost/api/restaurants | jq -r '.[0].id')
curl "http://localhost/api/delivery/routes?desde=$REST_ID&hasta=ORDER_ID_AQUI&algoritmo=nearest" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Orden de Arranque (Dependencias)

```
db ──→ keycloak-db ──→ keycloak
  │
  ├──→ elasticsearch ──→ kibana ──→ api ──→ nginx
  │                              └── search
  ├──→ redis
  │
  ├──→ hadoop-namenode ──→ hadoop-datanode ──→ hive-metastore ──→ hive-server
  │                                                                   │
  │                                                   spark-master ──→┘
  │                                                       │
  │                                                   spark-worker
  │
  ├──→ superset-db ──→ superset-init ──→ superset
  │
  ├──→ airflow-db ──→ airflow-init ──→ airflow-webserver
  │                                    ├── airflow-scheduler
  │                                    └── airflow-worker
  │
  └──→ neo4j
```

---

## Errores conocidos corregidos durante ejecución

| # | Síntoma | Causa | Solución aplicada |
|---|---------|-------|-------------------|
| 1 | HiveServer2 no abre puerto 10000 | `hive-site.xml` sin `hive.metastore.uris` apuntando al metastore remoto | Agregar `<property><name>hive.metastore.uris</name><value>thrift://hive-metastore:9083</value></property>` en `hive/hive-site.xml` |
| 2 | HiveServer2 no encuentra hive-site.xml | `hive-server` no montaba `hive/hive-site.xml` | Agregar volumen `- ./hive/hive-site.xml:/opt/hive/conf/hive-site.xml:ro` en `docker-compose.yml` para `hive-server` |
| 3 | HiveServer2: `Permission denied: user=hive access=WRITE inode="/"` | HDFS raíz no escribe para user `hive` | `hdfs dfs -mkdir -p /user/hive/warehouse /tmp/hive && hdfs dfs -chmod -R 777 /user/hive /tmp /` y cambiar `hive.exec.scratchdir` a `/tmp/hive` y `hive.user.install.directory` a `/user/hive` |
| 4 | HiveServer2 tarda ~5 min en iniciar | Tez + Hive 4.0.0 con modo local | Normal. Esperar ~5-7 min después del primer `docker-compose up -d hive-server` |
| 5 | `generate_geo.py` falla con `UnicodeDecodeError` | Encoding del DSN en Windows | Ejecutar dentro de Docker: `docker run --rm --network bd2-p01-restaurantes_default -v "$PWD/scripts:/scripts" ... python:3.11-slim` |
| 6 | `generate_geo.py` requiere columna `ubicacion` en PostgreSQL | No existía en schema inicial | `ALTER TABLE restaurantes ADD COLUMN ubicacion JSONB DEFAULT '{}'::jsonb;` |
| 7 | `spark-submit` no encontrado | No está en PATH del contenedor | Usar ruta completa: `/opt/spark/bin/spark-submit` |
| 8 | API container no tiene Python | Dockerfile Alpine multi-stage sin Python | Ejecutar scripts ETL desde contenedor Python externo con `--network bd2-p01-restaurantes_default` |
| 9 | `curl` en PowerShell es alias de `Invoke-WebRequest` | Windows PowerShell | Usar `curl.exe` en lugar de `curl` |
| 10 | `analisis_crecimiento.py` no encuentra tablas | Conflicto tablas Hive nativas vs Spark-managed | Ejecutar análisis directamente desde Hive beeline o recrear tablas como externas |
| 11 | `dim_order_status` no existe en Hive | DDL `dim_order_status.hql` no llegó a montarse en `hive-server:/opt/warehouse/ddl/` | Ejecutar `init_hive.sh` corrigiendo montura o crear tabla manualmente con beeline |
| 12 | `dim_order_status` datos corruptos (status_key 1 perdido) | `TBLPROPERTIES ("skip.header.line.count"="1")` en tabla poblada con `INSERT INTO VALUES` (no desde CSV con header) | Recrear tabla sin `skip.header.line.count`. Insertar con VALUES múltiples |
| 13 | `dim_reservation_status` tabla vacía | DDL sin INSERT statements y con `skip.header.line.count="1"` | Agregar INSERT con los 3 status y quitar `skip.header.line.count` |
| 14 | Superset: Error `400 CSRF token missing` en API REST | Flask-WTF CSRF protección activa en Superset 4.1.1 | Agregar `WTF_CSRF_ENABLED = False` en `superset/superset_config.py` |
| 15 | Superset: Error `FAILED: SemanticException [Error 10025]: Line 1:40 Expression not in GROUP BY key` | Superset infiere nombres de columna incorrectos | Usar alias explícitos `AS` en SQL: `d.year AS year` en vez de `d.year` |
| 16 | Superset: `granularity_sqla` causa `DATE_TRUNC` no soportado por Hive | Params de chart virtual contienen campo de granularidad | Remover `granularity_sqla`, `time_range`, `granularity` de los params del chart |
| 17 | Superset: Error `Unknown field` en `columns` al crear dataset via API | API Superset 4.1.1 no acepta `columns` en payload POST `/api/v1/dataset/` | Remover `columns` del payload de creación de dataset |
| 18 | Superset: Hive connection error `FAILED: ParseException line 1:31 cannot recognize input near 'SHOW' 'CATALOGS'` | Hive 4.0.0 no soporta `SHOW CATALOGS`; HiveEngineSpec lo invoca en `get_catalog_names()` | Parchear HiveEngineSpec: sobreescribir `get_catalog_names()`, `get_default_catalog()`, `get_schema_names()` y `get_default_schema()` para evitar el comando |

## Troubleshooting

| Síntoma | Causa probable | Solución |
|---------|---------------|----------|
| Hive no arranca | HDFS no listo / safemode | `docker exec hadoop-namenode hdfs dfsadmin -safemode leave` |
| Spark no escribe en Hive | `hive-site.xml` mal configurado | Verificar `thrift://hive-metastore:9083` en `spark/hive-site.xml` |
| Superset no conecta a Hive | Driver PyHive faltante | `docker exec superset pip install pyhive thrift thrift-sasl` |
| Airflow DAG falla | Conexiones no configuradas | Revisar `airflow/config/connections.yaml`, re-ejecutar `airflow-init` |
| Neo4j no carga CSVs | Permisos en staging | Verificar montura `./neo4j/staging:/var/lib/neo4j/import:ro` |
| ETL export falla con PostgreSQL | Falta columna `ciudad` | `ALTER TABLE restaurantes ADD COLUMN ciudad VARCHAR(100) DEFAULT '';` |
| Puerto 80 ocupado | Otro servicio web | Cambiar en `docker-compose.yml` y `nginx/nginx.conf` |
| spark-submit no encontrado | PATH incorrecto | Usar `/opt/spark/bin/spark-submit` en docker exec |
| API no tiene Python | Alpine image | Usar contenedor python:3.11-slim con `--network` para ETL |
