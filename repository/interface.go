package repository

import (
	"time"

	"restaurante/models"
)

// UserRepository define las operaciones de persistencia para usuarios.
type UserRepository interface {
	GetByID(id string) (*models.User, error)
	GetByEmail(email string) (*models.User, error)
	Create(nombre, email, passwordHash string) (string, error)
	Update(id, nombre, email string) error
	Delete(id string) error
	UpsertFromToken(id, nombre, email string, isAdmin bool) error //sincroniza un usuario que viene del JWT de Keycloak
}

// RestaurantRepository define las operaciones de persistencia para restaurantes.
type RestaurantRepository interface {
	List() ([]models.Restaurant, error)
	GetByID(id string) (*models.Restaurant, error)
	Create(nombre, direccion, telefono, adminID string, capacidad int) (string, error)
}

// MenuRepository define las operaciones de persistencia para ítems del menú.
type MenuRepository interface {
	List() ([]models.MenuItem, error)
	GetByRestaurant(restaurantID string) ([]models.MenuItem, error)
	Create(restaurantID, nombre, descripcion string, precio float64, categoria string) (string, error)
	Update(id, nombre, descripcion string, precio float64, disponible bool, categoria string) error
	Delete(id string) error
	GetAdminID(platoID string) (string, error) //devuelve el id_administrador del restaurante al que pertenece el plato
}

// ReservationRepository define las operaciones de persistencia para reservas.
type ReservationRepository interface {
	Create(userID, restaurantID string, fecha time.Time, personas int) (*models.Reservation, error)
	GetOwnerID(reservationID string) (string, error)
	Cancel(reservationID string) error
	GetRestaurantCapacity(restaurantID string) (int, error)
}

// OrderRepository define las operaciones de persistencia para pedidos.
type OrderRepository interface {
	Create(userID, restaurantID string, esRecogida bool, detalles []models.OrderDetailRequest) (*models.Order, error)
	GetByID(orderID string) (*models.Order, error)
}
