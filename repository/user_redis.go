package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"restaurante/models"

	"github.com/redis/go-redis/v9"
)

type userRedisDecorator struct {
	base   UserRepository
	client *redis.Client
}

func NewUserRedisDecorator(base UserRepository, client *redis.Client) UserRepository {
	return &userRedisDecorator{
		base:   base,
		client: client,
	}
}

func (u *userRedisDecorator) GetByID(id string) (*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cacheKey := fmt.Sprintf(string(KeyUserByID), id)
	val, err := u.client.Get(ctx, cacheKey).Result()
	if err == nil {
		var user models.User
		if err := json.Unmarshal([]byte(val), &user); err == nil {
			return &user, nil
		}
	}

	user, err := u.base.GetByID(id)
	if err != nil {
		return nil, err
	}

	userJSON, err := json.Marshal(user)
	if err == nil {
		u.client.Set(ctx, cacheKey, userJSON, CacheDuration)
	}

	return user, nil
}

func (u *userRedisDecorator) GetByEmail(email string) (*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cacheKey := fmt.Sprintf(string(KeyUserByEmail), email)
	val, err := u.client.Get(ctx, cacheKey).Result()
	if err == nil {
		var user models.User
		if err := json.Unmarshal([]byte(val), &user); err == nil {
			return &user, nil
		}
	}

	user, err := u.base.GetByEmail(email)
	if err != nil {
		return nil, err
	}

	userJSON, err := json.Marshal(user)
	if err == nil {
		u.client.Set(ctx, cacheKey, userJSON, CacheDuration)
	}

	return user, nil
}

func (u *userRedisDecorator) Create(nombre, email, passwordHash string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	id, err := u.base.Create(nombre, email, passwordHash)
	if err != nil {
		return "", err
	}

	// invalidar caché de búsqueda por email
	u.client.Del(ctx, fmt.Sprintf(string(KeyUserByEmail), email))

	return id, nil
}

func (u *userRedisDecorator) Update(id, nombre, email string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := u.base.Update(id, nombre, email)
	if err != nil {
		return err
	}

	// invalidar cachés del usuario
	u.client.Del(ctx,
		fmt.Sprintf(string(KeyUserByID), id),
		fmt.Sprintf(string(KeyUserByEmail), email),
	)

	return nil
}

func (u *userRedisDecorator) Delete(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := u.base.Delete(id)
	if err != nil {
		return err
	}

	// invalidar caché del usuario
	u.client.Del(ctx, fmt.Sprintf(string(KeyUserByID), id))

	return nil
}

func (u *userRedisDecorator) UpsertFromToken(id, nombre, email string, isAdmin bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := u.base.UpsertFromToken(id, nombre, email, isAdmin)
	if err != nil {
		return err
	}

	// invalidar cachés cuando se actualiza desde token
	u.client.Del(ctx,
		fmt.Sprintf(string(KeyUserByID), id),
		fmt.Sprintf(string(KeyUserByEmail), email),
	)

	return nil
}
