package repository

import (
	"time"

	"restaurante/models"
)

// UserRepository defines the persistence operations for users.
type UserRepository interface {
	GetByID(id string) (*models.User, error)
	GetByEmail(email string) (*models.User, error)
	Create(nombre, email, passwordHash string) (string, error)
	Update(id, nombre, email string) error
	Delete(id string) error
	UpsertFromToken(id, nombre, email string, isAdmin bool) error // syncs a user that comes from the Keycloak JWT
}

// RestaurantRepository defines the persistence operations for restaurants.
type RestaurantRepository interface {
	List() ([]models.Restaurant, error)
	GetByID(id string) (*models.Restaurant, error)
	Create(nombre, direccion, telefono, adminID string, capacidad int) (string, error)
}

// MenuRepository defines the persistence operations for menu items.
type MenuRepository interface {
	List() ([]models.MenuItem, error)
	GetByRestaurant(restaurantID string) ([]models.MenuItem, error)
	Create(restaurantID, nombre, descripcion string, precio float64, categoria string) (string, error)
	Update(id, nombre, descripcion string, precio float64, disponible bool, categoria string) error
	Delete(id string) error
	GetAdminID(platoID string) (string, error) // returns the admin id of the restaurant the dish belongs to
}

// ReservationRepository defines the persistence operations for reservations.
type ReservationRepository interface {
	Create(userID, restaurantID string, fecha time.Time, personas int) (*models.Reservation, error)
	GetOwnerID(reservationID string) (string, error)
	Cancel(reservationID string) error
	GetRestaurantCapacity(restaurantID string) (int, error)
}

// OrderRepository defines the persistence operations for orders.
type OrderRepository interface {
	Create(userID, restaurantID string, esRecogida bool, detalles []models.OrderDetailRequest) (*models.Order, error)
	GetByID(orderID string) (*models.Order, error)
}


