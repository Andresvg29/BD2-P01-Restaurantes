package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"restaurante/models"

	"github.com/redis/go-redis/v9"
)

type restaurantRedisDecorator struct {
	base   RestaurantRepository
	client *redis.Client
}

func NewRestaurantRedisDecorator(base RestaurantRepository, client *redis.Client) RestaurantRepository {
	return &restaurantRedisDecorator{
		base:   base,
		client: client,
	}
}

func (r *restaurantRedisDecorator) List() ([]models.Restaurant, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// intentar obtener del caché
	cacheKey := string(KeyRestaurantList)
	val, err := r.client.Get(ctx, cacheKey).Result()
	if err == nil {
		var restaurants []models.Restaurant
		if err := json.Unmarshal([]byte(val), &restaurants); err == nil {
			return restaurants, nil
		}
	}

	// obtener de la base de datos
	restaurants, err := r.base.List()
	if err != nil {
		return nil, err
	}

	// guardar en caché
	restaurantsJSON, err := json.Marshal(restaurants)
	if err == nil {
		r.client.Set(ctx, cacheKey, restaurantsJSON, CacheDuration)
	}

	return restaurants, nil
}

func (r *restaurantRedisDecorator) GetByID(id string) (*models.Restaurant, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cacheKey := fmt.Sprintf(string(KeyRestaurantByID), id)

	// intentar obtener del caché
	val, err := r.client.Get(ctx, cacheKey).Result()
	if err == nil {
		var restaurant models.Restaurant
		if err := json.Unmarshal([]byte(val), &restaurant); err == nil {
			return &restaurant, nil
		}
	}

	// obtener de la base de datos
	restaurant, err := r.base.GetByID(id)
	if err != nil {
		return nil, err
	}

	// guardar en caché
	restaurantJSON, err := json.Marshal(restaurant)
	if err == nil {
		r.client.Set(ctx, cacheKey, restaurantJSON, CacheDuration)
	}

	return restaurant, nil
}

func (r *restaurantRedisDecorator) Create(nombre, direccion, telefono, adminID string, capacidad int) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	id, err := r.base.Create(nombre, direccion, telefono, adminID, capacidad)
	if err != nil {
		return "", err
	}

	// invalidar caché de lista
	r.client.Del(ctx, string(KeyRestaurantList))

	return id, nil
}
