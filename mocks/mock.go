package mocks

import (
	"context"
	"time"

	"restaurante/models"

	"github.com/Nerzal/gocloak/v13"
	"github.com/stretchr/testify/mock"
)

// ── MockUserRepository ────────────────────────────────────────────────────────

type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) GetByID(id string) (*models.User, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) GetByEmail(email string) (*models.User, error) {
	args := m.Called(email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) Create(nombre, email, passwordHash string) (string, error) {
	args := m.Called(nombre, email, passwordHash)
	return args.String(0), args.Error(1)
}

func (m *MockUserRepository) Update(id, nombre, email string) error {
	args := m.Called(id, nombre, email)
	return args.Error(0)
}

func (m *MockUserRepository) Delete(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockUserRepository) UpsertFromToken(id, nombre, email string, isAdmin bool) error {
	args := m.Called(id, nombre, email, isAdmin)
	return args.Error(0)
}

// ── MockRestaurantRepository ──────────────────────────────────────────────────

type MockRestaurantRepository struct {
	mock.Mock
}

func (m *MockRestaurantRepository) List() ([]models.Restaurant, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Restaurant), args.Error(1)
}

func (m *MockRestaurantRepository) GetByID(id string) (*models.Restaurant, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Restaurant), args.Error(1)
}

func (m *MockRestaurantRepository) Create(nombre, direccion, telefono, adminID string, capacidad int) (string, error) {
	args := m.Called(nombre, direccion, telefono, adminID, capacidad)
	return args.String(0), args.Error(1)
}

// ── MockMenuRepository ────────────────────────────────────────────────────────

type MockMenuRepository struct {
	mock.Mock
}

func (m *MockMenuRepository) List() ([]models.MenuItem, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.MenuItem), args.Error(1)
}

func (m *MockMenuRepository) GetByRestaurant(restaurantID string) ([]models.MenuItem, error) {
	args := m.Called(restaurantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.MenuItem), args.Error(1)
}

func (m *MockMenuRepository) Create(restaurantID, nombre, descripcion string, precio float64, categoria string) (string, error) {
	args := m.Called(restaurantID, nombre, descripcion, precio, categoria)
	return args.String(0), args.Error(1)
}

func (m *MockMenuRepository) Update(id, nombre, descripcion string, precio float64, disponible bool, categoria string) error {
	args := m.Called(id, nombre, descripcion, precio, disponible, categoria)
	return args.Error(0)
}

func (m *MockMenuRepository) Delete(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockMenuRepository) GetAdminID(platoID string) (string, error) {
	args := m.Called(platoID)
	return args.String(0), args.Error(1)
}

// ── MockReservationRepository ─────────────────────────────────────────────────

type MockReservationRepository struct {
	mock.Mock
}

func (m *MockReservationRepository) Create(userID, restaurantID string, fecha time.Time, personas int) (*models.Reservation, error) {
	args := m.Called(userID, restaurantID, fecha, personas)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Reservation), args.Error(1)
}

func (m *MockReservationRepository) GetOwnerID(reservationID string) (string, error) {
	args := m.Called(reservationID)
	return args.String(0), args.Error(1)
}

func (m *MockReservationRepository) Cancel(reservationID string) error {
	args := m.Called(reservationID)
	return args.Error(0)
}

func (m *MockReservationRepository) GetRestaurantCapacity(restaurantID string) (int, error) {
	args := m.Called(restaurantID)
	return args.Int(0), args.Error(1)
}

// ── MockOrderRepository ───────────────────────────────────────────────────────

type MockOrderRepository struct {
	mock.Mock
}

func (m *MockOrderRepository) Create(userID, restaurantID string, esRecogida bool, detalles []models.OrderDetailRequest) (*models.Order, error) {
	args := m.Called(userID, restaurantID, esRecogida, detalles)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Order), args.Error(1)
}

func (m *MockOrderRepository) GetByID(orderID string) (*models.Order, error) {
	args := m.Called(orderID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Order), args.Error(1)
}

// ── MockKeycloakClient ────────────────────────────────────────────────────────

type MockKeycloakClient struct {
	mock.Mock
}

func (m *MockKeycloakClient) LoginUser(ctx context.Context, email, password string) (*gocloak.JWT, error) {
	args := m.Called(email, password)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gocloak.JWT), args.Error(1)
}

func (m *MockKeycloakClient) RegisterUser(ctx context.Context, nombre, apellido, email, password string) (string, error) {
	args := m.Called(nombre, apellido, email, password)
	return args.String(0), args.Error(1)
}
