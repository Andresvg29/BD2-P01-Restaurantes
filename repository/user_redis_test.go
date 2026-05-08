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

// MockUserRepository para decorador
type MockUserRepositoryForDecorator struct {
	mock.Mock
}

func (m *MockUserRepositoryForDecorator) GetByID(id string) (*models.User, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepositoryForDecorator) GetByEmail(email string) (*models.User, error) {
	args := m.Called(email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepositoryForDecorator) Create(email, nombre, apellido string) (string, error) {
	args := m.Called(email, nombre, apellido)
	return args.String(0), args.Error(1)
}

func (m *MockUserRepositoryForDecorator) Update(id, nombre, apellido string) error {
	args := m.Called(id, nombre, apellido)
	return args.Error(0)
}

func (m *MockUserRepositoryForDecorator) Delete(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockUserRepositoryForDecorator) UpsertFromToken(id, email, nombre string, isAdmin bool) error {
	args := m.Called(id, email, nombre, isAdmin)
	return args.Error(0)
}

// TestUserRedisDecorator_GetByIDHit
func TestUserRedisDecorator_GetByIDHit(t *testing.T) {
	redisClient, err := NewRedisClient()
	if err != nil {
		t.Skip("Redis no disponible")
	}
	defer redisClient.Close()

	mockBase := new(MockUserRepositoryForDecorator)
	decorator := NewUserRedisDecorator(mockBase, redisClient.Client)

	ctx := context.Background()

	user := &models.User{ID: "user-1", Email: "ana@mail.com", Nombre: "Ana"}
	data, _ := json.Marshal(user)
	redisClient.Client.Set(ctx, "user:user-1", string(data), 24*time.Hour)

	result, err := decorator.GetByID("user-1")

	assert.NoError(t, err)
	assert.Equal(t, "Ana", result.Nombre)
	mockBase.AssertNotCalled(t, "GetByID")
}

// TestUserRedisDecorator_GetByIDMiss
func TestUserRedisDecorator_GetByIDMiss(t *testing.T) {
	redisClient, err := NewRedisClient()
	if err != nil {
		t.Skip("Redis no disponible")
	}
	defer redisClient.Close()

	mockBase := new(MockUserRepositoryForDecorator)
	user := &models.User{ID: "user-1", Email: "juan@mail.com", Nombre: "Juan"}
	mockBase.On("GetByID", "user-1").Return(user, nil)

	decorator := NewUserRedisDecorator(mockBase, redisClient.Client)

	ctx := context.Background()
	redisClient.Client.Del(ctx, "user:user-1")

	result, err := decorator.GetByID("user-1")

	assert.NoError(t, err)
	assert.Equal(t, "Juan", result.Nombre)
	mockBase.AssertCalled(t, "GetByID", "user-1")
}

// TestUserRedisDecorator_GetByEmailHit
func TestUserRedisDecorator_GetByEmailHit(t *testing.T) {
	redisClient, err := NewRedisClient()
	if err != nil {
		t.Skip("Redis no disponible")
	}
	defer redisClient.Close()

	mockBase := new(MockUserRepositoryForDecorator)
	decorator := NewUserRedisDecorator(mockBase, redisClient.Client)

	ctx := context.Background()

	user := &models.User{ID: "user-2", Email: "maria@mail.com", Nombre: "Maria"}
	data, _ := json.Marshal(user)
	redisClient.Client.Set(ctx, "user:email:maria@mail.com", string(data), 24*time.Hour)

	result, err := decorator.GetByEmail("maria@mail.com")

	assert.NoError(t, err)
	assert.Equal(t, "Maria", result.Nombre)
	mockBase.AssertNotCalled(t, "GetByEmail")
}

// TestUserRedisDecorator_GetByEmailMiss
func TestUserRedisDecorator_GetByEmailMiss(t *testing.T) {
	redisClient, err := NewRedisClient()
	if err != nil {
		t.Skip("Redis no disponible")
	}
	defer redisClient.Close()

	mockBase := new(MockUserRepositoryForDecorator)
	user := &models.User{ID: "user-2", Email: "pedro@mail.com", Nombre: "Pedro"}
	mockBase.On("GetByEmail", "pedro@mail.com").Return(user, nil)

	decorator := NewUserRedisDecorator(mockBase, redisClient.Client)

	ctx := context.Background()
	redisClient.Client.Del(ctx, "user:email:pedro@mail.com")

	result, err := decorator.GetByEmail("pedro@mail.com")

	assert.NoError(t, err)
	assert.Equal(t, "Pedro", result.Nombre)
	mockBase.AssertCalled(t, "GetByEmail", "pedro@mail.com")
}

// TestUserRedisDecorator_CreateInvalidatesCache
func TestUserRedisDecorator_CreateInvalidatesCache(t *testing.T) {
	redisClient, err := NewRedisClient()
	if err != nil {
		t.Skip("Redis no disponible")
	}
	defer redisClient.Close()

	mockBase := new(MockUserRepositoryForDecorator)
	mockBase.On("Create", "new@mail.com", "New", "User").Return("user-new", nil)

	decorator := NewUserRedisDecorator(mockBase, redisClient.Client)

	ctx := context.Background()
	redisClient.Client.Set(ctx, "user:email:new@mail.com", "cached", 24*time.Hour)

	id, err := decorator.Create("new@mail.com", "New", "User")

	assert.NoError(t, err)
	assert.Equal(t, "user-new", id)
	mockBase.AssertCalled(t, "Create", "new@mail.com", "New", "User")
}

// TestUserRedisDecorator_Update
func TestUserRedisDecorator_Update(t *testing.T) {
	redisClient, err := NewRedisClient()
	if err != nil {
		t.Skip("Redis no disponible")
	}
	defer redisClient.Close()

	mockBase := new(MockUserRepositoryForDecorator)
	mockBase.On("Update", "user-1", "Updated", "Name").Return(nil)

	decorator := NewUserRedisDecorator(mockBase, redisClient.Client)

	ctx := context.Background()
	redisClient.Client.Set(ctx, "user:user-1", "cached", 24*time.Hour)

	err = decorator.Update("user-1", "Updated", "Name")

	assert.NoError(t, err)

	// Cache debe ser invalidado
	val, _ := redisClient.Client.Get(ctx, "user:user-1").Result()
	assert.Equal(t, "", val)
	mockBase.AssertCalled(t, "Update", "user-1", "Updated", "Name")
}

// TestUserRedisDecorator_Delete
func TestUserRedisDecorator_Delete(t *testing.T) {
	redisClient, err := NewRedisClient()
	if err != nil {
		t.Skip("Redis no disponible")
	}
	defer redisClient.Close()

	mockBase := new(MockUserRepositoryForDecorator)
	mockBase.On("Delete", "user-1").Return(nil)

	decorator := NewUserRedisDecorator(mockBase, redisClient.Client)

	ctx := context.Background()
	redisClient.Client.Set(ctx, "user:user-1", "cached", 24*time.Hour)

	err = decorator.Delete("user-1")

	assert.NoError(t, err)

	// Cache debe ser invalidado
	val, _ := redisClient.Client.Get(ctx, "user:user-1").Result()
	assert.Equal(t, "", val)
	mockBase.AssertCalled(t, "Delete", "user-1")
}

// TestUserRedisDecorator_UpsertFromToken
func TestUserRedisDecorator_UpsertFromToken(t *testing.T) {
	redisClient, err := NewRedisClient()
	if err != nil {
		t.Skip("Redis no disponible")
	}
	defer redisClient.Close()

	mockBase := new(MockUserRepositoryForDecorator)
	mockBase.On("UpsertFromToken", "token-user", "token@mail.com", "Token", true).Return(nil)

	decorator := NewUserRedisDecorator(mockBase, redisClient.Client)

	err = decorator.UpsertFromToken("token-user", "token@mail.com", "Token", true)

	assert.NoError(t, err)
	mockBase.AssertCalled(t, "UpsertFromToken", "token-user", "token@mail.com", "Token", true)
}

// TestUserRedisDecorator_GetByIDError
func TestUserRedisDecorator_GetByIDError(t *testing.T) {
	redisClient, err := NewRedisClient()
	if err != nil {
		t.Skip("Redis no disponible")
	}
	defer redisClient.Close()

	mockBase := new(MockUserRepositoryForDecorator)
	mockBase.On("GetByID", "user-999").Return(nil, errors.New("not found"))

	decorator := NewUserRedisDecorator(mockBase, redisClient.Client)

	ctx := context.Background()
	redisClient.Client.Del(ctx, "user:user-999")

	result, err := decorator.GetByID("user-999")

	assert.Error(t, err)
	assert.Nil(t, result)
	mockBase.AssertCalled(t, "GetByID", "user-999")
}
