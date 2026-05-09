# BD2-P01-Restaurantes

API REST para gestión de restaurantes, menús, reservas y pedidos. Aplicación con Go, soporta múltiples motores de bases de datos (PostgreSQL/MongoDB), caché con Redis y búsqueda avanzada con Elasticsearch.

## Tabla de Contenidos

- [Características](#características)
- [Configuración Inicial](#configuración-inicial)
- [Ejecución del Proyecto](#ejecución-del-proyecto)
- [Cambiar Entre Motores de BD](#cambiar-entre-motores-de-bd)
- [URLs de Acceso](#urls-de-acceso)
- [Estructura del Proyecto](#estructura-del-proyecto)
- [Troubleshooting](#troubleshooting)

## Características

- **API REST** construida en Go con Gin framework
- **Múltiples Motores de BD**: PostgreSQL o MongoDB (intercambiables)
- **MongoDB Sharding**: 3 shards con replicación
- **Redis**: Caché distribuido para optimizar consultas
- **Elasticsearch**: Búsqueda de texto completo e indexación
- **Autenticación**: Keycloak con OAuth2/OpenID Connect
- **Autorización**: RBAC (Admin/Client)
- **Nginx**: Proxy inverso para enrutamiento
- **Docker Compose**: Orquestación completa de servicios




**Archivo `.env` (estructura actual):**
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

# Redis (Caché)
REDIS_HOST=redis
REDIS_PORT=6379
REDIS_PASSWORD=               # Dejar vacío si no hay contraseña
ENABLE_REDIS=true
```

### 3. Crear Archivo `.env` (si no existe)
```bash
cp .env.example .env  # Si existe el archivo ejemplo
# O crear manualmente con los valores anteriores
```

## 🚀 Ejecución del Proyecto

### Opción 1: Ejecutar Todo Completo (Recomendado)

#### Iniciar Todos los Servicios
```bash
docker-compose up --build -d
```

**Esto levantará:**
- ✅ PostgreSQL (puerto 5432)
- ✅ MongoDB Sharded (puerto 27017)
- ✅ Keycloak (puerto 8080)
- ✅ Redis (puerto 6379)
- ✅ Elasticsearch (puerto 9200)
- ✅ Kibana (puerto 5601)
- ✅ API Principal (puerto 8000, via Nginx puerto 80)
- ✅ Microservicio Search (puerto 8001, via Nginx)
- ✅ Adminer - DB Admin (puerto 8081)
- ✅ Nginx Proxy (puerto 80)


#### Solo API y PostgreSQL
```bash
# Parar todos los servicios primero
docker-compose down

# Levantar solo PostgreSQL
docker-compose up -d db keycloak redis elasticsearch

# En otra terminal, compilar y ejecutar la API
go run api/main.go
```

#### Solo API y MongoDB
```bash
docker-compose down

# Levantar MongoDB sharded
docker-compose up -d configsvr mongo1 mongo2 mongo3 mongos

# Esperar a que MongoDB esté listo (~30 segundos), luego:
docker-compose up -d keycloak redis elasticsearch

# Compilar y ejecutar
go run api/main.go
```

#### Solo los servicios de soporte (sin API)
```bash
docker-compose up -d db keycloak redis elasticsearch elasticsearch
```

### Opción 3: Detener y Limpiar

```bash
# Detener todos los contenedores
docker-compose down

# Detener y eliminar volúmenes (borra todos los datos)
docker-compose down -v

# Detener un servicio específico
docker-compose stop api

# Reiniciar un servicio específico
docker-compose restart api
```

## 🔄 Cambiar Entre Motores de BD

### PostgreSQL vs MongoDB

El proyecto soporta dos motores de base de datos intercambiables mediante la variable `DB_ENGINE` en `.env`.

### 📊 Pasos para Cambiar a PostgreSQL

#### 1. Editar `.env`
```env
DB_ENGINE=postgres
```

#### 2. Asegurar que PostgreSQL esté corriendo
```bash
docker-compose up -d db
```

#### 3. Inicializar la Base de Datos
```bash
# El archivo init.sql se ejecuta automáticamente en el contenedor
# Si necesitas reinicializar:
docker exec -it db psql -U admin -d restaurante_db -f /docker-entrypoint-initdb.d/init.sql
```

#### 4. Levantar los servicios restantes
```bash
docker-compose up -d keycloak redis elasticsearch
```

#### 5. Compilar y ejecutar la API
```bash
go run api/main.go
```

#### 6. Acceder a PostgreSQL mediante Adminer
- URL: http://localhost:8081
- Motor: PostgreSQL
- Servidor: db
- Usuario: admin
- Contraseña: password
- Base de datos: restaurante_db

---

### 🍃 Pasos para Cambiar a MongoDB

#### 1. Editar `.env`
```env
DB_ENGINE=mongodb
```

#### 2. Inicializar MongoDB Sharded
```bash
# Levantar toda la infraestructura de MongoDB
docker-compose up -d configsvr mongo1 mongo2 mongo3 mongos

# Esperar a que todos los contenedores estén healthy (~60 segundos)
docker-compose ps

# Ejecutar script de inicialización
docker exec mongos mongosh --port 27017 < initMongo.js

# Verificar que los shards estén configurados
docker exec mongos mongosh --port 27017 --eval "sh.status()"
```

#### 3. Levantar los servicios restantes
```bash
docker-compose up -d keycloak redis elasticsearch
```

#### 4. Compilar y ejecutar la API
```bash
go run api/main.go
```

#### 5. Acceder a MongoDB mediante MongoDB Compass
```bash
# Conexión URI
mongodb://localhost:27017

# Verificar en terminal
docker exec mongos mongosh --port 27017 --eval "show dbs"
```

#### 6. Verificar la configuración de Sharding
```bash
# Ver estado del sharding
docker exec mongos mongosh --port 27017 --eval "sh.status()"

# Ver colecciones
docker exec mongos mongosh --port 27017 restaurante_db --eval "show collections"
```

---

### 📋 Comparativa de Configuración

| Aspecto | PostgreSQL | MongoDB |
|--------|-----------|---------|
| **Motor** | Relacional | Documento |
| **Variable .env** | `DB_ENGINE=postgres` | `DB_ENGINE=mongodb` |
| **Contenedor Principal** | `db` | `mongos` (+ configsvr, mongo1-3) |
| **Puerto** | 5432 | 27017 |
| **Admin UI** | Adminer (8081) | MongoDB Compass | 
| **Inicialización** | `init.sql` | `initMongo.js` |
| **Escalabilidad** | Single instance | Sharded (3 shards) |
| **Caché** | Redis | Redis |
| **Búsqueda** | Elasticsearch | Elasticsearch |

## 🌐 URLs de Acceso

Una vez que todo esté corriendo, accede a los servicios en:

| Servicio | URL | Usuario | Contraseña |
|----------|-----|---------|-----------|
| **API Principal** | http://localhost/api | OAuth2 (Keycloak) | - |
| **Microservicio Search** | http://localhost/search | OAuth2 (Keycloak) | - |
| **Keycloak Admin** | http://localhost:8080 | admin | admin |
| **Elasticsearch** | http://localhost:9200 | - | - |
| **Kibana** | http://localhost:5601 | - | - |
| **Redis CLI** | `docker exec -it redis redis-cli` | - | - |
| **PostgreSQL (Adminer)** | http://localhost:8081 | admin | password |
| **MongoDB (Local)** | mongodb://localhost:27017 | - | - |
| **Nginx** | http://localhost:80 | - | - |

### Verificar que los servicios están listos
```bash
# PostgreSQL
docker exec db pg_isready -U admin

# MongoDB
docker exec mongos mongosh --port 27017 --eval "db.adminCommand('ping')"

# Redis
docker exec redis redis-cli ping

# Elasticsearch
curl -s http://localhost:9200/_cluster/health | jq .
```

## 📁 Estructura del Proyecto

```
.
├── api/
│   └── main.go                 # Servidor API principal (Gin)
├── handlers/
│   ├── auth.go                 # Autenticación (Keycloak)
│   ├── restaurant.go           # Gestión de restaurantes
│   ├── reservations.go         # Gestión de reservas
│   ├── search.go               # Búsqueda avanzada
│   └── *_test.go               # Tests unitarios
├── repository/
│   ├── interface.go            # Interfaces de repositorio
│   ├── factory.go              # Factory pattern para BD
│   ├── postgres/               # Implementación PostgreSQL
│   │   └── restaurant.go
│   ├── mongodb/                # Implementación MongoDB
│   │   └── restaurant.go
│   └── redis/                  # Decoradores con caché Redis
│       ├── user_redis.go
│       ├── restaurant_redis.go
│       └── menu_redis.go
├── models/
│   ├── models.go               # Estructuras de datos
│   └── plato.go
├── middleware/
│   └── auth.go                 # Middleware de autenticación
├── elasticsearch/
│   ├── client.go               # Cliente Elasticsearch
│   ├── indexer.go              # Indexación
│   └── mock.go
├── keycloak/
│   └── setup.go                # Configuración Keycloak
├── search/
│   └── main.go                 # Microservicio de búsqueda
├── nginx/
│   └── nginx.conf              # Configuración Nginx
├── docker-compose.yml          # Orquestación de contenedores
├── Dockerfile                  # Build de API principal
├── Dockerfile.search           # Build del microservicio search
├── init.sql                    # Script de inicialización PostgreSQL
├── initMongo.js                # Script de inicialización MongoDB
├── .env                        # Variables de entorno
└── README.md                   # Este archivo
```

## 🔧 Comandos Útiles

### Compilación y Ejecución Local

```bash
# Compilar la API
go build -o api_bin ./api

# Ejecutar tests
go test ./...

# Ejecutar tests con cobertura
go test -cover ./...

# Ejecutar linter
golangci-lint run

# Ver módulos de Go
go mod tidy
```

### Docker

```bash
# Reconstruir imagen de la API
docker-compose build api

# Ver logs con timestamp
docker-compose logs --timestamps -f

# Ejecutar comando en un contenedor
docker exec -it api bash
docker exec -it db psql -U admin -d restaurante_db

# Copiar archivos hacia/desde contenedor
docker cp ./init.sql db:/tmp/init.sql
```

### MongoDB

```bash
# Conectar a MongoDB CLI
docker exec -it mongos mongosh --port 27017

# Ver bases de datos
show dbs

# Cambiar base de datos
use restaurante_db

# Ver colecciones
show collections

# Contar documentos en una colección
db.restaurants.countDocuments()

# Ver estado de sharding
sh.status()

# Ver configuración de shards
db.adminCommand({ listShards: 1 })
```

### PostgreSQL

```bash
# Conectar a PostgreSQL CLI
docker exec -it db psql -U admin -d restaurante_db

# Listar tablas
\dt

# Ver estructura de una tabla
\d restaurants

# Ejecutar query
SELECT * FROM restaurants LIMIT 10;

# Salir
\q
```

### Redis

```bash
# Conectar a Redis CLI
docker exec -it redis redis-cli

# Ver todas las claves
KEYS *

# Ver tipo de una clave
TYPE key_name

# Ver valor de una clave
GET key_name

# Limpiar caché
FLUSHDB  # Base de datos actual
FLUSHALL # Todas las bases de datos

# Salir
exit
```

### Elasticsearch

```bash
# Ver salud del cluster
curl -s http://localhost:9200/_cluster/health | jq

# Listar índices
curl -s http://localhost:9200/_cat/indices?v

# Ver documentos en un índice
curl -s http://localhost:9200/restaurants/_search | jq

# Reindexar
curl -X POST http://localhost:9200/_reindex -H 'Content-Type: application/json' -d '{...}'
```

## 🐛 Troubleshooting

### Problema: Contenedores no inician

**Solución:**
```bash
# Verificar logs del contenedor
docker-compose logs api

# Reconstruir imagen
docker-compose build --no-cache api

# Limpiar todo y empezar de nuevo
docker-compose down -v
docker-compose up --build -d
```

### Problema: "Connection refused" en PostgreSQL

**Solución:**
```bash
# Verificar que PostgreSQL esté corriendo
docker-compose ps db

# Ver logs de PostgreSQL
docker-compose logs db

# Reconectar
docker-compose restart db
```

### Problema: MongoDB no inicia o falla sharding

**Solución:**
```bash
# Esperar más tiempo a que MongoDB esté listo
sleep 60

# Reintentar inicialización
docker exec mongos mongosh --port 27017 < initMongo.js

# Verificar réplicas
docker exec mongo1 mongosh --port 27018 --eval "rs.status()"
```

### Problema: Redis no responde

**Solución:**
```bash
# Verificar conexión
docker exec redis redis-cli ping

# Limpiar datos
docker exec redis redis-cli FLUSHALL

# Reiniciar servicio
docker-compose restart redis
```

### Problema: Elasticsearch no indexa

**Solución:**
```bash
# Verificar estado de ES
curl -s http://localhost:9200/_cluster/health

# Ver logs
docker-compose logs elasticsearch

# Recrear índices
curl -X DELETE http://localhost:9200/*
docker-compose restart elasticsearch
```

### Problema: API devuelve errores 401/403

**Solución:**
```bash
# Verificar que Keycloak esté corriendo
docker-compose logs keycloak

# Obtener token de autenticación
curl -X POST http://localhost:8080/realms/restaurante-api/protocol/openid-connect/token \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  -d 'client_id=backend-api&client_secret=CozoA7kBJYwEtR4gL2m5bJ6gSCUVC32E&grant_type=client_credentials'
```

### Problema: Volúmenes llenos o corrupto

**Solución:**
```bash
# Eliminar volúmenes específicos
docker volume rm bd2-p01-restaurantes_postgres_data
docker volume rm bd2-p01-restaurantes_mongo1_data
docker volume ls | grep restaurantes

# Limpiar todo
docker-compose down -v
docker volume prune
```

## 📚 Documentación Adicional
- [REDIS_CONFIG.md](REDIS_CONFIG.md) - Configuración avanzada de Redis
- [docker-compose.yml](docker-compose.yml) - Configuración de servicios

