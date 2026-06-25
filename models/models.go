package models

import "time"

// User represents an authenticated user via Keycloak with a local DB reference.
type User struct {
	ID            string    `json:"id" bson:"_id,omitempty"`
	Nombre        string    `json:"nombre" bson:"nombre"`
	Email         string    `json:"email" bson:"email"`
	KeycloakID    string    `json:"keycloak_id" bson:"keycloak_id"`
	IdRol         int       `json:"id_rol" bson:"id_rol"`
	FechaCreacion time.Time `json:"fecha_creacion" bson:"fecha_creacion"`
}

// Restaurant represents a restaurant with its contact details and administrator.
type Restaurant struct {
	ID              string `json:"id" bson:"_id,omitempty"`
	Nombre          string `json:"nombre" bson:"nombre"`
	Direccion       string `json:"direccion" bson:"direccion"`
	Telefono        string `json:"telefono" bson:"telefono"`
	IdAdministrador string `json:"id_administrador" bson:"id_administrador"`
	CapacidadTotal  int    `json:"capacidad_total" bson:"capacidad_total"`
}

// MenuItem is an individual dish within a restaurant's menu.
type MenuItem struct {
	ID             string  `json:"id" bson:"_id,omitempty"`
	IdRestaurante  string  `json:"id_restaurante" bson:"id_restaurante"`
	Nombre         string  `json:"nombre" bson:"nombre"`
	Descripcion    string  `json:"descripcion" bson:"descripcion"`
	Precio         float64 `json:"precio" bson:"precio"`
	EstaDisponible bool    `json:"esta_disponible" bson:"esta_disponible"`
	Categoria      string  `json:"categoria" bson:"categoria"`
}

// Reservation represents a reservation made by a user at a restaurant.
type Reservation struct {
	ID               string    `json:"id" bson:"_id,omitempty"`
	IdUsuario        string    `json:"id_usuario" bson:"id_usuario"`
	IdRestaurante    string    `json:"id_restaurante" bson:"id_restaurante"`
	FechaReserva     time.Time `json:"fecha_reserva" bson:"fecha_reserva"`
	CantidadPersonas int       `json:"cantidad_personas" bson:"cantidad_personas"`
	IdEstado         int       `json:"id_estado" bson:"id_estado"`
	NombreEstado     string    `json:"nombre_estado" bson:"nombre_estado"`
}

// Order represents an order placed by a user at a restaurant.
type Order struct {
	ID             string        `json:"id" bson:"_id,omitempty"`
	IdUsuario      string        `json:"id_usuario" bson:"id_usuario"`
	IdRestaurante  string        `json:"id_restaurante" bson:"id_restaurante"`
	EsRecogida     bool          `json:"es_recogida" bson:"es_recogida"`
	IdEstadoPedido int           `json:"id_estado_pedido" bson:"id_estado_pedido"`
	Total          float64       `json:"total" bson:"total"`
	Detalles       []OrderDetail `json:"detalles" bson:"detalles"`
}

// OrderDetail contains information about a dish within an order.
type OrderDetail struct {
	ID       string  `json:"id" bson:"_id,omitempty"`
	IdPlato  string  `json:"id_plato" bson:"id_plato"`
	Cantidad int     `json:"cantidad" bson:"cantidad"`
	Precio   float64 `json:"precio" bson:"precio"`
}

// OrderDetailRequest is the structure used in the body when creating an order.
type OrderDetailRequest struct {
	IdPlato  string `json:"id_plato"  binding:"required"`
	Cantidad int    `json:"cantidad"  binding:"required,gt=0"`
}

// Location represents a geographic location (node in Neo4j).
type Location struct {
	ID      string  `json:"id"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	Address string  `json:"address"`
}

// RouteStep es un punto en la ruta de entrega.
type RouteStep struct {
	Direccion string  `json:"direccion"`
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	Orden     int     `json:"orden"`
}

// RouteResponse is the response from the delivery route endpoint.
type RouteResponse struct {
	Ruta           []RouteStep `json:"ruta"`
	DistanciaTotal float64     `json:"distancia_total_km"`
	Algoritmo      string      `json:"algoritmo_usado"`
}
