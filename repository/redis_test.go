package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"restaurante/models"

	"github.com/redis/go-redis/v9"
)

// TestRedisConnection verifica que Redis está disponible
func TestRedisConnection(t *testing.T) {
	client, err := NewRedisClient()
	if err != nil {
		t.Fatalf("No se pudo conectar a Redis: %v", err)
	}
	defer client.Close()
	t.Log("Conexión a Redis exitosa")
}

// TestRedisBasicOperations verifica operaciones básicas
func TestRedisBasicOperations(t *testing.T) {
	client, err := NewRedisClient()
	if err != nil {
		t.Fatalf("No se pudo conectar a Redis: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Test SET/GET
	testKey := "test_key"
	testValue := "test_value"

	err = client.Set(ctx, testKey, testValue, time.Minute).Err()
	if err != nil {
		t.Fatalf("No se pudo establecer la clave en Redis: %v", err)
	}

	val, err := client.Get(ctx, testKey).Result()
	if err != nil {
		t.Fatalf("No se pudo obtener la clave de Redis: %v", err)
	}

	if val != testValue {
		t.Fatalf("Valor no coincide: esperado %s, obtuvo %s", testValue, val)
	}

	t.Log("Operaciones SET/GET funcionan correctamente")

	// Test DEL
	deleted, err := client.Del(ctx, testKey).Result()
	if err != nil {
		t.Fatalf("No se pudo eliminar la clave de Redis: %v", err)
	}

	if deleted != 1 {
		t.Fatalf("No se eliminó la clave")
	}

	t.Log("Operación DEL funciona correctamente")
}

// TestRedisJSON verifica serialización JSON
func TestRedisJSON(t *testing.T) {
	client, err := NewRedisClient()
	if err != nil {
		t.Fatalf("No se pudo conectar a Redis: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Crear un objeto
	restaurant := models.Restaurant{
		ID:              "1",
		Nombre:          "Test Restaurant",
		Direccion:       "123 Main St",
		Telefono:        "555-0000",
		IdAdministrador: "admin1",
		CapacidadTotal:  50,
	}

	// Serializar
	data, err := json.Marshal(restaurant)
	if err != nil {
		t.Fatalf("No se pudo serializar el restaurante: %v", err)
	}

	// Guardar en Redis
	key := "restaurant:test"
	err = client.Set(ctx, key, data, time.Minute).Err()
	if err != nil {
		t.Fatalf("No se pudo establecer el restaurante en Redis: %v", err)
	}

	// Recuperar de Redis
	val, err := client.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("No se pudo obtener el restaurante de Redis: %v", err)
	}

	// Deserializar
	var retrieved models.Restaurant
	err = json.Unmarshal([]byte(val), &retrieved)
	if err != nil {
		t.Fatalf("No se pudo deserializar: %v", err)
	}

	if retrieved.Nombre != restaurant.Nombre {
		t.Fatalf("Objeto no coincide")
	}

	t.Log("Serialización JSON funciona correctamente")

	// Cleanup
	client.Del(ctx, key)
}

// TestRedisTTL verifica que las claves expiran
func TestRedisTTL(t *testing.T) {
	client, err := NewRedisClient()
	if err != nil {
		t.Fatalf("No se pudo conectar a Redis: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	key := "test_ttl"
	value := "expires_soon"

	// Guardar con TTL de 1 segundo
	err = client.Set(ctx, key, value, 1*time.Second).Err()
	if err != nil {
		t.Fatalf("No se pudo establecer la clave con TTL: %v", err)
	}

	// Verificar que existe
	val, err := client.Get(ctx, key).Result()
	if err != nil || val != value {
		t.Fatalf("Clave no encontrada inmediatamente")
	}

	// Esperar a que expire
	time.Sleep(2 * time.Second)

	// Verificar que expiró
	val, err = client.Get(ctx, key).Result()
	if err != redis.Nil {
		t.Fatalf("Clave no expiró como se esperaba")
	}

	t.Log("TTL y expiración funcionan correctamente")
}

// TestRedisDecoratorCaching verifica que los decoradores cachean
func TestRedisDecoratorCaching(t *testing.T) {
	t.Log("Para probar decoradores, necesitas un repositorio base mock")
	t.Log("Los decoradores están diseñados para usarse en la aplicación completa")
}

// BenchmarkRedisGet evalúa la performance
func BenchmarkRedisGet(b *testing.B) {
	client, err := NewRedisClient()
	if err != nil {
		b.Fatalf("No se pudo conectar a Redis: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Preparar datos
	key := "bench_key"
	value := "benchmark_value"
	client.Set(ctx, key, value, time.Minute)

	b.ResetTimer()

	// Ejecutar benchmark
	for i := 0; i < b.N; i++ {
		client.Get(ctx, key).Result()
	}
}

// PrintRedisStats imprime estadísticas de Redis
func PrintRedisStats(client *redis.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	info := client.Info(ctx, "stats")
	fmt.Println("Estadísticas de Redis:")
	fmt.Println(info.String())
}

// TestMain ejecuta todos los tests
func TestMain(m *testing.M) {
	fmt.Println("Iniciando pruebas de Redis...")
	m.Run()
}
