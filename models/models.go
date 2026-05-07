package models

import "time"

type User struct {
	ID            string    `json:"id" bson:"_id,omitempty"`
	Nombre        string    `json:"nombre" bson:"nombre"`
	Email         string    `json:"email" bson:"email"`
	KeycloakID    string    `json:"keycloak_id" bson:"keycloak_id"`
	IdRol         int       `json:"id_rol" bson:"id_rol"`
	FechaCreacion time.Time `json:"fecha_creacion" bson:"fecha_creacion"`
}

type Restaurant struct {
	ID              string `json:"id" bson:"_id,omitempty"`
	Nombre          string `json:"nombre" bson:"nombre"`
	Direccion       string `json:"direccion" bson:"direccion"`
	Telefono        string `json:"telefono" bson:"telefono"`
	IdAdministrador string `json:"id_administrador" bson:"id_administrador"`
	CapacidadTotal  int    `json:"capacidad_total" bson:"capacidad_total"`
}

type MenuItem struct {
	ID             string  `json:"id" bson:"_id,omitempty"`
	IdRestaurante  string  `json:"id_restaurante" bson:"id_restaurante"`
	Nombre         string  `json:"nombre" bson:"nombre"`
	Descripcion    string  `json:"descripcion" bson:"descripcion"`
	Precio         float64 `json:"precio" bson:"precio"`
	EstaDisponible bool    `json:"esta_disponible" bson:"esta_disponible"`
	Categoria      string  `json:"categoria" bson:"categoria"`
}

type Reservation struct {
	ID               string    `json:"id" bson:"_id,omitempty"`
	IdUsuario        string    `json:"id_usuario" bson:"id_usuario"`
	IdRestaurante    string    `json:"id_restaurante" bson:"id_restaurante"`
	FechaReserva     time.Time `json:"fecha_reserva" bson:"fecha_reserva"`
	CantidadPersonas int       `json:"cantidad_personas" bson:"cantidad_personas"`
	IdEstado         int       `json:"id_estado" bson:"id_estado"`
	NombreEstado     string    `json:"nombre_estado" bson:"nombre_estado"`
}

type Order struct {
	ID             string        `json:"id" bson:"_id,omitempty"`
	IdUsuario      string        `json:"id_usuario" bson:"id_usuario"`
	IdRestaurante  string        `json:"id_restaurante" bson:"id_restaurante"`
	EsRecogida     bool          `json:"es_recogida" bson:"es_recogida"`
	IdEstadoPedido int           `json:"id_estado_pedido" bson:"id_estado_pedido"`
	Total          float64       `json:"total" bson:"total"`
	Detalles       []OrderDetail `json:"detalles" bson:"detalles"`
}

type OrderDetail struct {
	ID       string  `json:"id" bson:"_id,omitempty"`
	IdPlato  string  `json:"id_plato" bson:"id_plato"`
	Cantidad int     `json:"cantidad" bson:"cantidad"`
	Precio   float64 `json:"precio" bson:"precio"`
}

// se usa al crear un pedido de parte del body request
type OrderDetailRequest struct {
	IdPlato  string `json:"id_plato"  binding:"required"`
	Cantidad int    `json:"cantidad"  binding:"required,gt=0"`
}
