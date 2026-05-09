# Configuración de Redis
## Funcionalidades Cacheadas

Redis cachea automáticamente los siguientes datos:

### 1. **Restaurantes** (15 minutos)
- Listado completo de restaurantes
- Restaurantes por ID

```go
// Métodos cacheados:
restaurantRepo.List()           // Lista todas los restaurantes
restaurantRepo.GetByID(id)      // Obtiene restaurante por ID
// Automáticamente se invalida al crear nuevos
restaurantRepo.Create(...)      // Invalida el caché de lista
```

### 2. **Menús** (15 minutos)
- Listado completo de menús
- Menús por restaurante
- Menús por ID

```go
// Métodos cacheados:
menuRepo.List()                         // Lista todos los menús
menuRepo.GetByRestaurant(restaurantID)  // Menús de un restaurante
menuRepo.Create(...)                    // Invalida cachés relacionados
menuRepo.Update(...)                    // Invalida cachés
menuRepo.Delete(...)                    // Invalida cachés
```

### 3. **Usuarios** (15 minutos)
- Usuarios por ID
- Usuarios por Email

```go
// Métodos cacheados:
userRepo.GetByID(id)            // Obtiene usuario por ID
userRepo.GetByEmail(email)      // Obtiene usuario por email
userRepo.Create(...)            // Invalida caché de email
userRepo.Update(...)            // Invalida cachés del usuario
userRepo.Delete(...)            // Invalida caché
userRepo.UpsertFromToken(...)   // Invalida cachés
```

## Estructura de Claves en Redis

Las claves en Redis siguen un patrón consistente:

```
restaurants:list              # Lista de restaurantes
restaurant:{id}               # Restaurante individual
menus:list                    # Lista de menús
menus:restaurant:{id}         # Menús de un restaurante
menu:{id}                     # Menú individual
user:{id}                     # Usuario por ID
user:email:{email}            # Usuario por email
reservation:{id}              # Reservación
order:{id}                    # Orden
```

## Duración del Caché

- **Por defecto**: 15 minutos (`CacheDuration = 15 * time.Minute`)
- Puedes ajustar en [repository/redis.go](repository/redis.go#L49)

## Monitoreo de Redis

### Conectarse a Redis en línea de comandos

```bash
# Dentro del contenedor
docker exec -it redis redis-cli

# Comandos útiles:
PING                    # Verifica conexión
KEYS *                  # Ver todas las claves
GET key_name            # Obtener valor de una clave
DEL key_name            # Eliminar una clave
FLUSHALL                # Limpiar todo Redis
INFO stats              # Ver estadísticas
MONITOR                 # Ver comandos en tiempo real
```

### Monitoreo desde la aplicación

El proyecto registra (logs) conexiones y eventos de Redis:

```go
log.Printf("Redis conectado exitosamente")
log.Printf("Advertencia: No se pudo conectar a Redis: %v. Funcionando sin caché.", err)
```

## Desabilitar Redis

Si necesitas deshabilitar Redis temporalmente:

```bash
# En .env
ENABLE_REDIS=false
```

O en `docker-compose.yml`, comenta el servicio de Redis y remueve la dependencia del contenedor `api`.

## Persistencia

Redis está configurado con `--appendonly yes` para persistencia:

- Los datos se guardan en el volumen `redis_data`
- Si el contenedor se detiene, los datos se mantienen
- En desarrollo, puedes usar `docker volume rm redis_data` para limpiar

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

### Timeout en operaciones Redis
```
i/o timeout
```

**Solución**: Aumenta los timeouts en [repository/redis.go](repository/redis.go#L29-L32)

### Redis lleno
```
OOM command not allowed when used memory > 'maxmemory'.
```

**Solución**: Limpiar caché manual:
```bash
docker exec -it redis redis-cli FLUSHALL
```

O establecer una política de evicción en Docker:
```yaml
command: redis-server --maxmemory 256mb --maxmemory-policy allkeys-lru
```

## Decoradores (Patrón)

Los decoradores de Redis envuelven los repositorios base implementando el mismo interfaz:

- [restaurant_redis.go](repository/restaurant_redis.go)
- [menu_redis.go](repository/menu_redis.go)
- [user_redis.go](repository/user_redis.go)
- [plato_redis.go](repository/plato_redis.go)

Esto permite:
- ✅ Caché transparente sin cambiar los handlers
- ✅ Invalidación automática en operaciones de escritura
- ✅ Fallback a BD si Redis no está disponible
- ✅ Fácil de remover o customizar

## Ejemplo de uso en handlers

No necesitas cambiar nada en los handlers. El caché funciona automáticamente:

```go
// En handlers/restaurant.go
func ListRestaurants(repo RestaurantRepository) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Esta llamada usa caché automáticamente si Redis está disponible
        restaurants, err := repo.List()
        // ...
    }
}
```

## Próximos pasos

1. ✅ Configurar variables de entorno
2. ✅ Ejecutar `docker-compose up --build`
3. ✅ Verificar logs de Redis: `docker-compose logs redis`
4. ✅ Monitorear caché: `docker exec -it redis redis-cli`
5. ✅ Ajustar duración de caché según tus necesidades
