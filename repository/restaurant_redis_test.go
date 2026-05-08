package repository

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"restaurante/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRestaurantRepository para decorador
type MockRestaurantRepositoryForDecorator struct {
	mock.Mock
}

func (m *MockRestaurantRepositoryForDecorator) List() ([]models.Restaurant, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Restaurant), args.Error(1)
}

func (m *MockRestaurantRepositoryForDecorator) GetByID(id string) (*models.Restaurant, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Restaurant), args.Error(1)
}

func (m *MockRestaurantRepositoryForDecorator) Create(nombre, direccion, telefono, adminID string, capacidad int) (string, error) {
	args := m.Called(nombre, direccion, telefono, adminID, capacidad)
	return args.String(0), args.Error(1)
}

// TestRestaurantRedisDecorator_ListHit
func TestRestaurantRedisDecorator_ListHit(t *testing.T) {
	redisClient, err := NewRedisClient()
	if err != nil {
		t.Skip("Redis no disponible")
	}
	defer redisClient.Close()

	mockBase := new(MockRestaurantRepositoryForDecorator)
	decorator := NewRestaurantRedisDecorator(mockBase, redisClient.Client)

	ctx := context.Background()

	restaurants := []models.Restaurant{
		{ID: "r-1", Nombre: "La Trattoria", Direccion: "San José"},
	}
	data, _ := json.Marshal(restaurants)
	redisClient.Client.Set(ctx, "restaurants:all", string(data), 24*time.Hour)

	result, err := decorator.List()

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "La Trattoria", result[0].Nombre)
	mockBase.AssertNotCalled(t, "List")
}

// TestRestaurantRedisDecorator_ListMiss
func TestRestaurantRedisDecorator_ListMiss(t *testing.T) {
	redisClient, err := NewRedisClient()
	if err != nil {
		t.Skip("Redis no disponible")
	}
	defer redisClient.Close()

	mockBase := new(MockRestaurantRepositoryForDecorator)
	restaurants := []models.Restaurant{
		{ID: "r-1", Nombre: "El Fogón", Direccion: "Cartago"},
	}
	mockBase.On("List").Return(restaurants, nil)

	decorator := NewRestaurantRedisDecorator(mockBase, redisClient.Client)

	ctx := context.Background()
	redisClient.Client.Del(ctx, "restaurants:all")

	result, err := decorator.List()

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	mockBase.AssertCalled(t, "List")
}

// TestRestaurantRedisDecorator_GetByIDHit
func TestRestaurantRedisDecorator_GetByIDHit(t *testing.T) {
	redisClient, err := NewRedisClient()
	if err != nil {
		t.Skip("Redis no disponible")
	}
	defer redisClient.Close()

	mockBase := new(MockRestaurantRepositoryForDecorator)
	decorator := NewRestaurantRedisDecorator(mockBase, redisClient.Client)

	ctx := context.Background()

	restaurant := &models.Restaurant{ID: "r-1", Nombre: "Pizza Palace", Direccion: "SJ"}
	data, _ := json.Marshal(restaurant)
	redisClient.Client.Set(ctx, "restaurant:r-1", string(data), 24*time.Hour)

	result, err := decorator.GetByID("r-1")

	assert.NoError(t, err)
	assert.Equal(t, "Pizza Palace", result.Nombre)
	mockBase.AssertNotCalled(t, "GetByID")
}

// TestRestaurantRedisDecorator_GetByIDMiss
func TestRestaurantRedisDecorator_GetByIDMiss(t *testing.T) {
	redisClient, err := NewRedisClient()
	if err != nil {
		t.Skip("Redis no disponible")
	}
	defer redisClient.Close()

	mockBase := new(MockRestaurantRepositoryForDecorator)
	restaurant := &models.Restaurant{ID: "r-1", Nombre: "Sushi Bar", Direccion: "SJ"}
	mockBase.On("GetByID", "r-1").Return(restaurant, nil)

	decorator := NewRestaurantRedisDecorator(mockBase, redisClient.Client)

	ctx := context.Background()
	redisClient.Client.Del(ctx, "restaurant:r-1")

	result, err := decorator.GetByID("r-1")

	assert.NoError(t, err)
	assert.Equal(t, "Sushi Bar", result.Nombre)
	mockBase.AssertCalled(t, "GetByID", "r-1")
}

// TestRestaurantRedisDecorator_CreateInvalidatesCache
func TestRestaurantRedisDecorator_CreateInvalidatesCache(t *testing.T) {
	redisClient, err := NewRedisClient()
	if err != nil {
		t.Skip("Redis no disponible")
	}
	defer redisClient.Close()

	mockBase := new(MockRestaurantRepositoryForDecorator)
	mockBase.On("Create", "New Rest", "Address", "1234567", "admin-1", 50).Return("r-new", nil)

	decorator := NewRestaurantRedisDecorator(mockBase, redisClient.Client)

	ctx := context.Background()
	redisClient.Client.Set(ctx, "restaurants:all", "cached_data", 24*time.Hour)

	id, err := decorator.Create("New Rest", "Address", "1234567", "admin-1", 50)

	assert.NoError(t, err)
	assert.Equal(t, "r-new", id)

	// Verificar que cache fue invalidado
	val, _ := redisClient.Client.Get(ctx, "restaurants:all").Result()
	assert.Equal(t, "", val)
	mockBase.AssertCalled(t, "Create", "New Rest", "Address", "1234567", "admin-1", 50)
}

// TestRestaurantRedisDecorator_BaseError
func TestRestaurantRedisDecorator_BaseError(t *testing.T) {
	redisClient, err := NewRedisClient()
	if err != nil {
		t.Skip("Redis no disponible")
	}
	defer redisClient.Close()

	mockBase := new(MockRestaurantRepositoryForDecorator)
	mockBase.On("List").Return(nil, errors.New("db error"))

	decorator := NewRestaurantRedisDecorator(mockBase, redisClient.Client)

	ctx := context.Background()
	redisClient.Client.Del(ctx, "restaurants:all")

	result, err := decorator.List()

	assert.Error(t, err)
	assert.Nil(t, result)
	mockBase.AssertCalled(t, "List")
}
