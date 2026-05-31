# Configuración de Redis (Middleware HTTP)

## Arquitectura

Redis funciona como **middleware HTTP** que intercepta requests GET en el router de Gin, cachea responses completas e invalida la caché en operaciones de mutación (POST/PUT/DELETE).

### Flujo

```
Request → Gin Route → Redis Middleware → Handler → Base Repo (Postgres/Mongo) → DB
                       ↕ Redis
```

### Graceful Degradation

Si Redis falla o no está disponible, la API **sigue funcionando sin interrupción**:
- Toda operación Redis usa contexto con timeout de 500ms
- Redis error → log + pass-through al handler
- cliente `nil` → pass-through directo sin intentar Redis
- Nunca se retorna error al cliente por falla de Redis

## Formato de Cache Keys

Las claves usan el patrón `METHOD:/path`:

```
GET:/restaurants              → "GET:/restaurants"
GET:/restaurants/123          → "GET:/restaurants/123"
GET:/menus                    → "GET:/menus"
GET:/menus/5                  → "GET:/menus/5"
```

Se generan con: `method + ":" + c.Request.URL.Path`

## Formato del Valor Cacheado

```go
type cachedResponse struct {
    Status      int    `json:"s"`          // status code (200, 201...)
    ContentType string `json:"c"`          // ej: "application/json; charset=utf-8"
    Body        []byte `json:"b,omitempty"` // response body
}
```

Se guarda como JSON en Redis. En cache hit se deserializa y se sirve con `c.Data(status, contentType, body)`.

## Rutas Cacheadas

Se cachean únicamente responses GET de rutas públicas:
- `GET /restaurants` — listado completo
- `GET /menus/:id` — menú por restaurante

### Rutas Excluidas (SkipPaths)

No se cachean rutas con datos de usuario para evitar servir datos de un usuario a otro:
- `/auth/` — login/register
- `/users/` — datos de perfil
- `/orders/` — pedidos
- `/reservations/` — reservas

## Duración del Caché

- **Por defecto**: 15 minutos (`defaultCacheDuration = 15 * time.Minute`)
- Configurable con opción `WithDuration()` en `middleware/redis_cache.go`

## Invalidación de Caché

En operaciones de mutación (POST/PUT/DELETE), si el handler responde con status 2xx, se invalidan las claves relacionadas:

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

- Keys exactas: se resuelven reemplazando `:param` por `c.Param("param")`
- Keys por SCAN: se usa `SCAN {cursor} MATCH {prefix}*` + `DEL` en lotes de 100
- No se invalida si el handler responde con status >= 300

## Configuración

### Variables de Entorno

```env
# Redis
REDIS_HOST=redis
REDIS_PORT=6379
REDIS_PASSWORD=
ENABLE_REDIS=true
```

### Deshabilitar Redis

```bash
# En .env
ENABLE_REDIS=false
```

## Monitoreo

```bash
# Conectarse a Redis CLI
docker exec -it redis redis-cli

# Comandos útiles:
PING                    # Verifica conexión
KEYS *                  # Ver todas las claves (evitar en producción)
SCAN 0 MATCH GET:/*     # Listar claves de caché (recomendado)
GET "GET:/restaurants"  # Obtener valor cacheado
FLUSHALL                # Limpiar todo Redis
INFO stats              # Ver estadísticas
```

## Rendimiento

Con Redis habilitado:
- **Consultas cacheadas**: ~0.5-1ms (vs 50-100ms sin caché)
- **Reducción de carga BD**: ~70-80% menos consultas para datos frecuentes
- **Memoria adicional**: ~5-20MB dependiendo del volumen de datos

## Troubleshooting

### Conexión rechazada
```
error conectando a redis: connection refused
```

**Solución**: Verifica que el contenedor Redis está corriendo:
```bash
docker ps | grep redis
docker-compose logs redis
```

### Redis lleno
```
OOM command not allowed when used memory > 'maxmemory'.
```

**Solución**: Limpiar caché manual:
```bash
docker exec -it redis redis-cli FLUSHALL
```

O establecer política de evicción:
```yaml
command: redis-server --maxmemory 256mb --maxmemory-policy allkeys-lru
```

## Detalles Técnicos

### Custom ResponseWriter

Wrapper que incrusta `gin.ResponseWriter` para capturar body y status:

```go
type responseWriter struct {
    gin.ResponseWriter
    body   bytes.Buffer
    status int
}
```

### Código Fuente

- `middleware/redis_cache.go` — Implementación del middleware
- `middleware/redis_cache_test.go` — Tests con miniredis (15 tests)
- `repository/redis.go` — Conexión Redis y constantes de claves

### Migración desde Decorators

Anteriormente Redis se implementaba como decorators sobre los repositorios (`repository/restaurant_redis.go`, `user_redis.go`, `menu_redis.go`). Esos archivos fueron eliminados. La lógica de caché ahora reside completamente en el middleware HTTP, simplificando los repositorios y centralizando la gestión de caché.
