package repository

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisClient encapsulates the connection to Redis
type RedisClient struct {
	*redis.Client
}

// NewRedisClient creates a new connection to Redis
func NewRedisClient() (*RedisClient, error) {
	host := getEnv("REDIS_HOST", "localhost")
	port := getEnv("REDIS_PORT", "6379")
	password := os.Getenv("REDIS_PASSWORD") // optional

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

	// verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("error conectando a redis en %s: %w", addr, err)
	}

	return &RedisClient{client}, nil
}

// Close closes the connection to Redis
func (r *RedisClient) Close() error {
	return r.Client.Close()
}


