package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"restaurante/models"

	"github.com/redis/go-redis/v9"
)

type menuRedisDecorator struct {
	base   MenuRepository
	client *redis.Client
}

func NewMenuRedisDecorator(base MenuRepository, client *redis.Client) MenuRepository {
	return &menuRedisDecorator{
		base:   base,
		client: client,
	}
}

func (m *menuRedisDecorator) List() ([]models.MenuItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cacheKey := string(KeyMenuList)
	val, err := m.client.Get(ctx, cacheKey).Result()
	if err == nil {
		var items []models.MenuItem
		if err := json.Unmarshal([]byte(val), &items); err == nil {
			return items, nil
		}
	}

	items, err := m.base.List()
	if err != nil {
		return nil, err
	}

	itemsJSON, err := json.Marshal(items)
	if err == nil {
		m.client.Set(ctx, cacheKey, itemsJSON, CacheDuration)
	}

	return items, nil
}

func (m *menuRedisDecorator) GetByRestaurant(restaurantID string) ([]models.MenuItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cacheKey := fmt.Sprintf(string(KeyMenuByRestaurant), restaurantID)
	val, err := m.client.Get(ctx, cacheKey).Result()
	if err == nil {
		var items []models.MenuItem
		if err := json.Unmarshal([]byte(val), &items); err == nil {
			return items, nil
		}
	}

	items, err := m.base.GetByRestaurant(restaurantID)
	if err != nil {
		return nil, err
	}

	itemsJSON, err := json.Marshal(items)
	if err == nil {
		m.client.Set(ctx, cacheKey, itemsJSON, CacheDuration)
	}

	return items, nil
}

func (m *menuRedisDecorator) Create(restaurantID, nombre, descripcion string, precio float64, categoria string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	id, err := m.base.Create(restaurantID, nombre, descripcion, precio, categoria)
	if err != nil {
		return "", err
	}

	// invalidar cachés relacionados
	m.client.Del(ctx,
		string(KeyMenuList),
		fmt.Sprintf(string(KeyMenuByRestaurant), restaurantID),
	)

	return id, nil
}

func (m *menuRedisDecorator) Update(id, nombre, descripcion string, precio float64, disponible bool, categoria string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.base.Update(id, nombre, descripcion, precio, disponible, categoria)
	if err != nil {
		return err
	}

	// invalidar cachés cuando se actualiza un menú
	m.client.Del(ctx,
		string(KeyMenuList),
		fmt.Sprintf(string(KeyMenuByID), id),
	)

	return nil
}

func (m *menuRedisDecorator) Delete(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.base.Delete(id)
	if err != nil {
		return err
	}

	// invalidar cachés cuando se elimina un menú
	m.client.Del(ctx, string(KeyMenuList), fmt.Sprintf(string(KeyMenuByID), id))

	return nil
}

func (m *menuRedisDecorator) GetAdminID(platoID string) (string, error) {
	// No cachear este método ya que se usa principalmente en validaciones
	return m.base.GetAdminID(platoID)
}
