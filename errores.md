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
