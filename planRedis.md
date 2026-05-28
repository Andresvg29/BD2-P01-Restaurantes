# Plan: Redis como Middleware HTTP (Gin)

## Objetivo

Migrar del patrón **Decorator en repositorios** a un **Middleware Gin** que intercepte requests HTTP y cachee responses GET completas en Redis.

---

## Arquitectura

### Actual
```
Request → Gin Route → Handler → Repo Decorator (Redis) → Base Repo (Postgres/Mongo) → DB
                                 ↕ Redis
```

### Propuesta
```
Request → Gin Route → Redis Middleware → Handler → Base Repo (Postgres/Mongo) → DB
                       ↕ Redis
```

---

## Diseño del Middleware

### 1. Estructura (`middleware/redis_cache.go`)

```go
type RedisCacheMiddleware struct {
    client            *redis.Client
    duration          time.Duration            // TTL para las entradas en caché
    timeout           time.Duration            // timeout para operaciones Redis (500ms)
    skipPaths         []string                 // paths que NO se cachean (ej: /auth/*)
    invalidationRules map[string]invalidationRule  // reglas de invalidación por ruta
}

type invalidationRule struct {
    exact []string // keys exactas con placeholders :param opcionales
    scans []string // prefijos para SCAN (ej: "GET:/menus")
}
```

### 2. Formato de Cache Keys

```
GET:/restaurants          → "GET:/restaurants"
GET:/restaurants/123      → "GET:/restaurants/123"
GET:/menus                → "GET:/menus"
GET:/menus/5              → "GET:/menus/5"
```

Se genera con: `method + ":" + c.Request.URL.Path`

### 3. Formato del Valor Cacheado

```go
type cachedResponse struct {
    Status      int    `json:"s"`          // status code (200, 201...)
    ContentType string `json:"c"`          // ej: "application/json; charset=utf-8"
    Body        []byte `json:"b,omitempty"` // response body
}
```

Se guarda como JSON en Redis. En cache hit se deserializa y se sirve con `c.Data(status, contentType, body)`.

### 4. Custom ResponseWriter

Wrapper incrusta `gin.ResponseWriter` para capturar body y status:

```go
type responseWriter struct {
    gin.ResponseWriter
    body   bytes.Buffer
    status int
}
```

- `Write(data)` → copia a `body` + escribe al writer original
- `WriteString(s)` → igual
- `WriteHeader(code)` → captura status + pasa al original

### 5. Flujo del Middleware

```
Request GET /restaurants (cacheable)
  ├─> ¿Path en skipPaths? → next()
  ├─> ¿client == nil? → next()
  ├─> Check Redis: GET "GET:/restaurants"
  │    ├─> Hit → c.Data(status, contentType, body) + c.Abort()
  │    └─> Miss/Error → next() (handler ejecuta via responseWriter)
  ├─> Handler escribe en responseWriter (body capturado)
  └─> ¿Status 2xx y body > 0? → SET "GET:/restaurants" → JSON{cachedResponse} [TTL 15min]

Request POST /menus (mutación)
  └─> next() (handler ejecuta)
  └─> ¿Status 2xx? → lookup invalidationRule → DEL exactas + SCAN prefijos

Request GET /users/me (skipPaths coincide)
  └─> next() → handler ejecuta normalmente, no se cachea
```

### 6. Graceful Degradation

- Toda operación Redis usa **contexto con timeout de 500ms**
- Redis error → log.Printf + pass-through (next())
- cliente `nil` → pass-through directo sin intentar Redis
- **Nunca** se retorna error al cliente por falla de Redis

### 7. Invalidación de Caché

Mapeo estático de rutas mutantes a keys a invalidar (`defaultInvalidationRules`):

| Ruta Mutante | Keys Exactas | Keys por SCAN |
|---|---|---|
| `POST /restaurants` | — | `GET:/restaurants` |
| `POST /menus` | — | `GET:/menus` |
| `PUT /menus/:id` | `GET:/menus/:id` | `GET:/menus` |
| `DELETE /menus/:id` | `GET:/menus/:id` | `GET:/menus` |
| `PUT /users/:id` | `GET:/users/:id`, `GET:/users/me` | — |
| `DELETE /users/:id` | `GET:/users/:id` | `GET:/users` |
| `POST /reservations` | — | `GET:/restaurants` |
| `DELETE /reservations/:id` | — | `GET:/restaurants` |
| `POST /orders` | — | `GET:/restaurants` |

- Keys exactas: se resuelven con `resolveKey()` reemplazando `:param` por `c.Param("param")`
- Keys por SCAN: se usa `SCAN {cursor} MATCH {prefix}*` + `DEL` en lotes de 100
- Solo se invalida si el handler responde con status 2xx---

## Sesiones de Trabajo

### ✅ Sesión 1 — Crear `middleware/redis_cache.go`

**Qué se hizo:**
- `RedisCacheMiddleware` struct con constructor funcional
- Opciones: `WithDuration`, `WithTimeout`, `WithSkipPaths`
- `responseWriter` que captura body + status (implementa `gin.ResponseWriter`)
- `cacheKey()` genera clave `METHOD:/path`
- Lógica de cache hit/miss para GETs: `cachedResponse` struct serializado como JSON
- Timeout de 500ms en operaciones Redis
- Graceful degradation: errores Redis loguean y hacen pass-through

**Archivos:** `middleware/redis_cache.go` (creado)

---

### ✅ Sesión 2 — Actualizar `factory.go` + `api/main.go`

**`repository/factory.go`:**
- Eliminadas llamadas a `NewUserRedisDecorator`, `NewRestaurantRedisDecorator`, `NewMenuRedisDecorator`
- Añadido campo `RedisClient *RedisClient` al struct `Repos`
- `RedisClient` se setea en `newPostgresRepos()` y `newMongoRepos()` (sigue creándose si `ENABLE_REDIS=true`)

**`api/main.go`:**
- Middleware registrado con `router.Use(cache.Middleware())` (router raíz)
- SkipPaths: `/auth/`, `/users/`, `/orders/`, `/reservations/` (no cachear datos autenticados)
- Redis client se obtiene de `repos.RedisClient.Client` (o `nil` si no hay Redis)

**Archivos:** `repository/factory.go` (modificado), `api/main.go` (modificado)

---

### ✅ Sesión 3 — Invalidación de Caché

**Qué se agregó:**
- Tipo `invalidationRule` con campos `exact []string` y `scans []string`
- `defaultInvalidationRules`: mapa con 9 reglas para rutas mutantes
- `invalidate(c *gin.Context)`: busca regla por `METHOD /fullPath`, ejecuta DEL/SCAN
- `deleteByPrefix(ctx, prefix)`: `SCAN` cursor-based + `DEL` en lotes de 100
- `resolveKey(key, c)`: reemplaza placeholders `:id` por `c.Param("id")`
- Opción `WithInvalidationRules(rules)` para sobrescribir defaults
- Estructura de `Middleware()` separada en `handleGET()` y rama de invalidación para non-GET

**Archivos:** `middleware/redis_cache.go` (modificado)

---

### ✅ Sesión 4 — Tests Unitarios

**`middleware/redis_cache_test.go`** — 15 tests con `miniredis`:

| Test | Descripción |
|---|---|
| `TestCacheHit` | Cache poblado → handler NO se ejecuta, se sirve body cacheado |
| `TestCacheMiss` | Cache vacío → handler se ejecuta, respuesta se guarda en Redis |
| `TestCacheHit_RespectsStatusAndContentType` | Cache hit respeta status y Content-Type |
| `TestSkipPaths` | Path en skipPaths → handler ejecutado sin cache |
| `TestNonGET_NoCache` | POST no intenta leer caché |
| `TestRedisDown` | Redis caído → handler se ejecuta normalmente |
| `TestCacheHit_BadCacheEntry` | Entrada corrupta en Redis → fallback a handler |
| `TestInvalidation_POST_Restaurants` | POST /restaurants → invalida `GET:/restaurants` |
| `TestInvalidation_POST_Menus` | POST /menus → invalida `GET:/menus*` (SCAN) |
| `TestInvalidation_PUT_MenuByID` | PUT /menus/5 → invalida key exacta + SCAN |
| `TestInvalidation_DELETE_MenuByID` | DELETE /menus/3 → invalida key exacta + SCAN |
| `TestInvalidation_SkipsOnNon2xx` | Error 400 → NO invalida |
| `TestNilClient_DoesNothing` | cliente nil → pass-through |
| `TestInvalidation_UserRoutes` | PUT /users/:id → invalida user + users/me |
| `TestInvalidation_NoRuleForRoute` | PATCH sin regla → caché intacto |

**Archivos:** `middleware/redis_cache_test.go` (creado), dependencia `github.com/alicebob/miniredis/v2` (agregada)

---

### ✅ Sesión 5 — Eliminar Decorators Legacy

**Archivos eliminados:**
- `repository/restaurant_redis.go`
- `repository/user_redis.go`
- `repository/menu_redis.go`
- `repository/restaurant_redis_test.go`
- `repository/user_redis_test.go`

---

### ✅ Sesión 6 — Prueba Final

Todos los paquetes compilan y pasan `go vet`:
- `go build ./...` → OK
- `go vet ./...` → OK
- `go test ./...` → 5 paquetes, todos OK

```
ok  restaurante/elasticsearch   0.17s
ok  restaurante/handlers        0.17s
ok  restaurante/middleware      2.32s  (15 tests)
ok  restaurante/repository      4.53s
ok  restaurante/repository/postgres 0.67s
```

---

## Resumen de Archivos

| Archivo | Acción |
|---|---|
| `middleware/redis_cache.go` | ✅ Creado |
| `middleware/redis_cache_test.go` | ✅ Creado (15 tests) |
| `repository/factory.go` | ✅ Modificado (quitar decorators, exponer RedisClient) |
| `api/main.go` | ✅ Modificado (registrar middleware con skipPaths) |
| `repository/restaurant_redis.go` | ❌ Eliminado |
| `repository/user_redis.go` | ❌ Eliminado |
| `repository/menu_redis.go` | ❌ Eliminado |
| `repository/restaurant_redis_test.go` | ❌ Eliminado |
| `repository/user_redis_test.go` | ❌ Eliminado |

---

## Riesgos y Consideraciones

1. **Precisión de invalidación**: Al usar rutas como keys, perdemos granularidad. Ej: `PUT /users/:id` no puede invalidar `user:email:{old_email}`. Aceptable por TTL de 15min.

2. **Broad invalidation con SCAN**: Para rutas como `POST /reservations`, usamos SCAN O(N). Con pocas keys es aceptable.

3. **Body grande**: Responses muy grandes pueden saturar Redis. Considerar compresión o límite de tamaño.

4. **Cache de datos autenticados**: `/users/me` y rutas similares se excluyen vía `skipPaths` para evitar cachear datos de un usuario y servirlos a otro.

## Decisiones Técnicas

1. **miniredis para tests**: Se usó `github.com/alicebob/miniredis/v2` en lugar de mockear interfaces de go-redis. Proporciona un servidor Redis real en memoria para tests deterministas.

2. **Invalidación con SCAN**: Para borrado por prefijo se usa `SCAN` (cursor-based, no bloqueante) en lugar de `KEYS` (peligroso en producción con muchas keys).

3. **skipPaths vs grupo de rutas**: Se optó por middleware en el router raíz con skipPaths en lugar de un grupo separado. Esto permite que la invalidación corra en todas las rutas mutantes (incluso en grupos auth/protected) mientras el cacheo GET se salta rutas con datos de usuario.

4. **Estructura de caché**: Se cachea el status code, Content-Type y body como un JSON compacto. Esto permite cachear cualquier response, no solo JSON.
