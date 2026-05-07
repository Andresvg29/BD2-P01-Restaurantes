package repository

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisClient encapsula la conexión a Redis
type RedisClient struct {
	*redis.Client
}

// NewRedisClient crea una nueva conexión a Redis
func NewRedisClient() (*RedisClient, error) {
	host := getEnv("REDIS_HOST", "localhost")
	port := getEnv("REDIS_PORT", "6379")
	password := os.Getenv("REDIS_PASSWORD") // opcional

	addr := fmt.Sprintf("%s:%s", host, port)

	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           0,
		MaxRetries:   3,
		PoolSize:     10,
		MinIdleConns: 5,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	// verificar conexión
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("error conectando a redis en %s: %w", addr, err)
	}

	return &RedisClient{client}, nil
}

// Close cierra la conexión a Redis
func (r *RedisClient) Close() error {
	return r.Client.Close()
}

// CacheKey genera una clave consistente para caché
type CacheKey string

const (
	KeyRestaurantList   CacheKey      = "restaurants:list"
	KeyRestaurantByID   CacheKey      = "restaurant:%s"
	KeyMenuList         CacheKey      = "menus:list"
	KeyMenuByRestaurant CacheKey      = "menus:restaurant:%s"
	KeyMenuByID         CacheKey      = "menu:%s"
	KeyUserByID         CacheKey      = "user:%s"
	KeyUserByEmail      CacheKey      = "user:email:%s"
	KeyReservationByID  CacheKey      = "reservation:%s"
	KeyOrderByID        CacheKey      = "order:%s"
	CacheDuration       time.Duration = 15 * time.Minute
)
