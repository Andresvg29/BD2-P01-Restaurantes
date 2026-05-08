package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"restaurante/models"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// skipIfNoRedis salta el test si Redis no está disponible
func skipIfNoRedis(t *testing.T) *RedisClient {
	t.Helper()
	client, err := NewRedisClient()
	if err != nil {
		t.Skip("Redis no disponible, saltando test de integración")
	}
	return client
}

func TestCacheKeyConstants(t *testing.T) {
	assert.Equal(t, CacheKey("restaurants:list"), KeyRestaurantList)
	assert.NotEmpty(t, string(KeyRestaurantByID))
	assert.NotEmpty(t, string(KeyMenuByRestaurant))
	assert.NotEmpty(t, string(KeyUserByID))
	assert.NotEmpty(t, string(KeyOrderByID))
	assert.Equal(t, 15*time.Minute, CacheDuration)
}

func TestRedisConnection(t *testing.T) {
	client := skipIfNoRedis(t)
	defer client.Close()
	assert.NotNil(t, client)
}
func TestRedisBasicOperations(t *testing.T) {
	client := skipIfNoRedis(t)
	defer client.Close()

	ctx := context.Background()
	key := "test:basic"

	err := client.Set(ctx, key, "valor_prueba", time.Minute).Err()
	assert.NoError(t, err)

	val, err := client.Get(ctx, key).Result()
	assert.NoError(t, err)
	assert.Equal(t, "valor_prueba", val)

	deleted, err := client.Del(ctx, key).Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(1), deleted)
}

func TestRedisJSON(t *testing.T) {
	client := skipIfNoRedis(t)
	defer client.Close()

	ctx := context.Background()
	key := "test:json:restaurant"
	defer client.Del(ctx, key)

	restaurant := models.Restaurant{
		ID:             "r-1",
		Nombre:         "Test Restaurant",
		Direccion:      "San José",
		CapacidadTotal: 50,
	}

	data, err := json.Marshal(restaurant)
	assert.NoError(t, err)

	err = client.Set(ctx, key, data, time.Minute).Err()
	assert.NoError(t, err)

	val, err := client.Get(ctx, key).Result()
	assert.NoError(t, err)

	var retrieved models.Restaurant
	err = json.Unmarshal([]byte(val), &retrieved)
	assert.NoError(t, err)
	assert.Equal(t, restaurant.Nombre, retrieved.Nombre)
	assert.Equal(t, restaurant.CapacidadTotal, retrieved.CapacidadTotal)
}

func TestRedisTTL(t *testing.T) {
	client := skipIfNoRedis(t)
	defer client.Close()

	ctx := context.Background()
	key := "test:ttl"

	err := client.Set(ctx, key, "expira_pronto", 1*time.Second).Err()
	assert.NoError(t, err)

	val, err := client.Get(ctx, key).Result()
	assert.NoError(t, err)
	assert.Equal(t, "expira_pronto", val)

	time.Sleep(2 * time.Second)

	_, err = client.Get(ctx, key).Result()
	assert.Equal(t, redis.Nil, err)
}
