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

- [ ] Renombrar paquete `repository/mongodb` → `repository/mongo` (en `factory.go` se importa como `mongoRepo "restaurante/repository/mongodb"` pero el `package` dentro declara `mongo`)
- [ ] Corregir keys de tests: en `restaurant_redis_test.go` se usa `restaurants:all` pero la constante real es `KeyRestaurantList = "restaurants:list"`

#### 2.3 Separación de Responsabilidades

- [ ] Mover handlers a archivos por dominio:
  - `handlers/restaurant.go` → solo `ListRestaurants`, `CreateRestaurant`
  - `handlers/menu.go` → `GetMenu`, `CreateMenu`, `UpdateMenu`, `DeleteMenu`
  - `handlers/reservation.go` → `CreateReservation`, `CancelReservation`
  - `handlers/order.go` → `CreateOrder`, `GetOrder`
  - `handlers/user.go` → `GetCurrentUser`, `UpdateUser`, `DeleteUser`
  - `handlers/auth.go` → `Login`, `RegisterUser`

#### 2.4 Mejoras Arquitectónicas

- [ ] Estandarizar manejo de errores: crear helpers `respondError(c, status, msg)` y `respondJSON(c, status, data)`
- [ ] Extraer lógica repetitiva de autorización (`adminID != userID`) a helpers
- [ ] Agregar logs estructurados con request ID y contexto

---

### 3. Tests Unitarios

#### 3.1 Tests de Redis Middleware

- [x] Tests en `middleware/redis_cache_test.go` con `miniredis` (15 tests):
  - Cache hit, cache miss, status/content-type preservation
  - Invalidación POST/PUT/DELETE, skip on non-2xx
  - Graceful degradation: Redis caído, cliente nil, entrada corrupta
  - SkipPaths, NonGET, rutas sin regla de invalidación

#### 3.2 Tests de Handlers

- [ ] Crear interfaz `KeycloakClient` para poder mockear `Login` y `RegisterUser` (actualmente reciben `*keycloak.Client` concreto, no se puede mockear)
- [ ] Agregar `TestLogin_Success` y `TestRegisterUser_Success`
- [ ] Agregar tests faltantes para `CreateReservation_BadBody`, `CancelReservation_DBError`, etc.

#### 3.3 Tests de Repositorios MongoDB

- [ ] Crear `repository/mongo/repository_test.go` con tests para todos los repos:
  - `UserRepository`, `RestaurantRepository`, `MenuRepository`, `ReservationRepository`, `OrderRepository`

#### 3.4 Tests de Factory

- [ ] Refactorizar `repository/factory_test.go` — actualmente solo testea `getEnv()`
- [ ] Agregar tests que validen comportamiento según `DB_ENGINE`, `ENABLE_REDIS`, etc.

---

### 4. Documentación

- [ ] **README.md**:
  - Diagrama de arquitectura (componentes y flujo)
  - Documentar nuevo patrón: Redis como middleware HTTP
  - Agregar sección "Decisiones Técnicas"
  - Mejorar ejemplos de API con `curl`
  - Documentar transición OLTP → OLAP

- [ ] **REDIS_CONFIG.md**: Actualizar para reflejar middleware HTTP, nuevo formato de claves, graceful degradation

- [ ] **GoDoc en funciones exportadas**: `models/`, `repository/interface.go`, `handlers/`, `middleware/`

---

## 🚀 Fase 2: Servicios OLAP

### Pendiente definir

- Endpoints analíticos (ventas, platos populares, ingresos por restaurante, tendencias, etc.)
- Evaluar: ¿nuevo microservicio o endpoints en API actual?
- Evaluar motor: ¿agregaciones MongoDB, vistas PostgreSQL, o Elasticsearch?

---

## Progreso

| Área | Estado |
|------|--------|
| Redis como Middleware | ✅ Completado |
| Limpieza código legacy (decorators Redis + Plato) | ✅ Completado |
| Separación handlers por dominio | ⬜ Pendiente |
| Estandarización errores/logs | ⬜ Pendiente |
| Tests unitarios (Redis middleware) | ✅ Completado |
| Documentación | ⬜ Pendiente |
| Servicios OLAP | ⬜ Pendiente (Fase 2) |
