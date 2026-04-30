package models

import "time"

type User struct {
	ID           string    `json:"id"`
	Nombre       string    `json:"nombre"`
	Email        string    `json:"email"`
	IdRol        int       `json:"id_rol"`
	FechaCreacion time.Time `json:"fecha_creacion"`
}

type Restaurant struct {
	ID              string `json:"id"`
	Nombre          string `json:"nombre"`
	Direccion       string `json:"direccion"`
	Telefono        string `json:"telefono"`
	IdAdministrador string `json:"id_administrador"`
	CapacidadTotal  int    `json:"capacidad_total"`
}

type MenuItem struct {
	ID             string  `json:"id"`
	IdRestaurante  string  `json:"id_restaurante"`
	Nombre         string  `json:"nombre"`
	Descripcion    string  `json:"descripcion"`
	Precio         float64 `json:"precio"`
	EstaDisponible bool    `json:"esta_disponible"`
}

type Reservation struct {
	ID               string    `json:"id"`
	IdUsuario        string    `json:"id_usuario"`
	IdRestaurante    string    `json:"id_restaurante"`
	FechaReserva     time.Time `json:"fecha_reserva"`
	CantidadPersonas int       `json:"cantidad_personas"`
	IdEstado         int       `json:"id_estado"`
	NombreEstado     string    `json:"nombre_estado"`
}

type Order struct {
	ID             string        `json:"id"`
	IdUsuario      string        `json:"id_usuario"`
	IdRestaurante  string        `json:"id_restaurante"`
	EsRecogida     bool          `json:"es_recogida"`
	IdEstadoPedido int           `json:"id_estado_pedido"`
	Total          float64       `json:"total"`
	Detalles       []OrderDetail `json:"detalles"`
}

type OrderDetail struct {
	ID       string  `json:"id"`
	IdPlato  string  `json:"id_plato"`
	Cantidad int     `json:"cantidad"`
	Precio   float64 `json:"precio"`
}

// se usa al crear un pedido de parte del body request
type OrderDetailRequest struct {
	IdPlato  string `json:"id_plato"  binding:"required"`
	Cantidad int    `json:"cantidad"  binding:"required,gt=0"`
}