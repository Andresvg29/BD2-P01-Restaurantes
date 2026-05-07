package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisDecorator struct {
	base   PlatoRepository
	client *redis.Client
}

func NewRedisDecorator(base PlatoRepository, client *redis.Client) PlatoRepository {
	return &redisDecorator{
		base:   base,
		client: client,
	}
}

func (r *redisDecorator) ObtenerTodos(ctx context.Context) ([]Plato, error) {
	cacheKey := "platos:todos"

	val, err := r.client.Get(ctx, cacheKey).Result()
	if err == nil {
		var platos []Plato
		if err := json.Unmarshal([]byte(val), &platos); err == nil {
			return platos, nil
		}
	}

	platos, err := r.base.ObtenerTodos(ctx)
	if err != nil {
		return nil, err
	}

	platosJSON, err := json.Marshal(platos)
	if err == nil {
		r.client.Set(ctx, cacheKey, platosJSON, 10*time.Minute)
	}

	return platos, nil
}