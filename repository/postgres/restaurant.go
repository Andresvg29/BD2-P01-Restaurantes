package postgres

import (
	"database/sql"
	"time"

	"restaurante/models"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) GetByID(id string) (*models.User, error) {
	u := &models.User{}
	err := r.db.QueryRow(
		`SELECT id, nombre, email, id_rol, fecha_creacion FROM usuarios WHERE id = $1`, id,
	).Scan(&u.ID, &u.Nombre, &u.Email, &u.IdRol, &u.FechaCreacion)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *UserRepository) GetByEmail(email string) (*models.User, error) {
	u := &models.User{}
	err := r.db.QueryRow(
		`SELECT id FROM usuarios WHERE email = $1`, email,
	).Scan(&u.ID)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *UserRepository) Create(nombre, email, keycloakID string) (string, error) {
	var id string
	err := r.db.QueryRow(
		`INSERT INTO usuarios (nombre, email, keycloak_id, id_rol)
		 VALUES ($1, $2, $3, (SELECT id FROM roles_usuario WHERE nombre_rol = 'cliente'))
		 RETURNING id`,
		nombre, email, keycloakID,
	).Scan(&id)
	return id, err
}

func (r *UserRepository) Update(id, nombre, email string) error {
	_, err := r.db.Exec(
		`UPDATE usuarios SET
			nombre = COALESCE(NULLIF($1, ''), nombre),
			email  = COALESCE(NULLIF($2, ''), email)
		 WHERE id = $3`,
		nombre, email, id,
	)
	return err
}

func (r *UserRepository) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM usuarios WHERE id = $1`, id)
	return err
}

func (r *UserRepository) UpsertFromToken(id, nombre, email string, isAdmin bool) error {
	idRol := 2
	if isAdmin {
		idRol = 1
	}
	query :=
		`INSERT INTO usuarios (id, nombre, email, keycloak_id, id_rol)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (id) DO UPDATE SET
			nombre = EXCLUDED.nombre,
			email  = EXCLUDED.email,
			id_rol = EXCLUDED.id_rol`
	_, err := r.db.Exec(query, id, nombre, email, id, idRol)
	return err
}

type RestaurantRepository struct {
	db *sql.DB
}

func NewRestaurantRepository(db *sql.DB) *RestaurantRepository {
	return &RestaurantRepository{db: db}
}

func (r *RestaurantRepository) List() ([]models.Restaurant, error) {
	rows, err := r.db.Query(
		`SELECT id, nombre, direccion, telefono, id_administrador, capacidad_total FROM restaurantes`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Restaurant
	for rows.Next() {
		var res models.Restaurant
		if err := rows.Scan(&res.ID, &res.Nombre, &res.Direccion, &res.Telefono,
			&res.IdAdministrador, &res.CapacidadTotal); err != nil {
			return nil, err
		}
		list = append(list, res)
	}
	return list, nil
}

func (r *RestaurantRepository) GetByID(id string) (*models.Restaurant, error) {
	res := &models.Restaurant{}
	err := r.db.QueryRow(
		`SELECT id, nombre, direccion, telefono, id_administrador, capacidad_total FROM restaurantes WHERE id = $1`, id,
	).Scan(&res.ID, &res.Nombre, &res.Direccion, &res.Telefono, &res.IdAdministrador, &res.CapacidadTotal)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (r *RestaurantRepository) Create(nombre, direccion, telefono, adminID string, capacidad int) (string, error) {
	var id string
	err := r.db.QueryRow(
		`INSERT INTO restaurantes (nombre, direccion, telefono, id_administrador, capacidad_total)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		nombre, direccion, telefono, adminID, capacidad,
	).Scan(&id)
	return id, err
}

type ReservationRepository struct {
	db *sql.DB
}

func NewReservationRepository(db *sql.DB) *ReservationRepository {
	return &ReservationRepository{db: db}
}

func (r *ReservationRepository) GetRestaurantCapacity(restaurantID string) (int, error) {
	var cap int
	err := r.db.QueryRow(
		`SELECT capacidad_total FROM restaurantes WHERE id = $1`, restaurantID,
	).Scan(&cap)
	return cap, err
}

func (r *ReservationRepository) Create(userID, restaurantID string, fecha time.Time, personas int) (*models.Reservation, error) {
	var stateID int
	r.db.QueryRow(`SELECT id FROM estados_reserva WHERE nombre_estado = 'pendiente'`).Scan(&stateID)

	var id string
	err := r.db.QueryRow(
		`INSERT INTO reservas (id_usuario, id_restaurante, fecha_reserva, cantidad_personas, id_estado)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		userID, restaurantID, fecha, personas, stateID,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return &models.Reservation{
		ID: id, IdUsuario: userID, IdRestaurante: restaurantID,
		FechaReserva: fecha, CantidadPersonas: personas,
		IdEstado: stateID, NombreEstado: "pendiente",
	}, nil
}

func (r *ReservationRepository) GetOwnerID(reservationID string) (string, error) {
	var ownerID string
	err := r.db.QueryRow(
		`SELECT id_usuario FROM reservas WHERE id = $1`, reservationID,
	).Scan(&ownerID)
	return ownerID, err
}

func (r *ReservationRepository) Cancel(reservationID string) error {
	var cancelStateID int
	r.db.QueryRow(`SELECT id FROM estados_reserva WHERE nombre_estado = 'cancelada'`).Scan(&cancelStateID)
	_, err := r.db.Exec(`UPDATE reservas SET id_estado = $1 WHERE id = $2`, cancelStateID, reservationID)
	return err
}

type MenuRepository struct {
	db *sql.DB
}

func NewMenuRepository(db *sql.DB) *MenuRepository {
	return &MenuRepository{db: db}
}

func (r *MenuRepository) GetByRestaurant(restaurantID string) ([]models.MenuItem, error) {
	rows, err := r.db.Query(
		`SELECT id, id_restaurante, nombre, descripcion, precio, esta_disponible
		 FROM platos WHERE id_restaurante = $1 ORDER BY nombre`,
		restaurantID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.MenuItem
	for rows.Next() {
		var item models.MenuItem
		if err := rows.Scan(&item.ID, &item.IdRestaurante, &item.Nombre,
			&item.Descripcion, &item.Precio, &item.EstaDisponible); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *MenuRepository) Create(restaurantID, nombre, descripcion string, precio float64) (string, error) {
	var id string
	err := r.db.QueryRow(
		`INSERT INTO platos (id_restaurante, nombre, descripcion, precio) VALUES ($1, $2, $3, $4) RETURNING id`,
		restaurantID, nombre, descripcion, precio,
	).Scan(&id)
	return id, err
}

func (r *MenuRepository) Update(id, nombre, descripcion string, precio float64, disponible bool) error {
	_, err := r.db.Exec(
		`UPDATE platos SET
			nombre      = COALESCE(NULLIF($1, ''), nombre),
			descripcion = COALESCE(NULLIF($2, ''), descripcion),
			precio      = CASE WHEN $3 > 0 THEN $3 ELSE precio END,
			esta_disponible = $4
		 WHERE id = $5`,
		nombre, descripcion, precio, disponible, id,
	)
	return err
}

func (r *MenuRepository) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM platos WHERE id = $1`, id)
	return err
}

func (r *MenuRepository) GetAdminID(platoID string) (string, error) {
	var adminID string
	err := r.db.QueryRow(
		`SELECT r.id_administrador FROM restaurantes r
		 JOIN platos p ON r.id = p.id_restaurante
		 WHERE p.id = $1`,
		platoID,
	).Scan(&adminID)
	return adminID, err
}

type OrderRepository struct {
	db *sql.DB
}

func NewOrderRepository(db *sql.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

func (r *OrderRepository) Create(userID, restaurantID string, esRecogida bool, detalles []models.OrderDetailRequest) (*models.Order, error) {
	var stateID int
	r.db.QueryRow(`SELECT id FROM estados_pedido WHERE nombre_estado = 'pendiente'`).Scan(&stateID)

	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var orderID string
	tx.QueryRow(
		`INSERT INTO pedidos (id_usuario, id_restaurante, es_recogida, id_estado_pedido, total)
		 VALUES ($1, $2, $3, $4, 0) RETURNING id`,
		userID, restaurantID, esRecogida, stateID,
	).Scan(&orderID)

	var total float64
	var details []models.OrderDetail

	for _, d := range detalles {
		var precio float64
		tx.QueryRow(
			`SELECT precio FROM platos WHERE id = $1 AND id_restaurante = $2`,
			d.IdPlato, restaurantID,
		).Scan(&precio)

		var detailID string
		tx.QueryRow(
			`INSERT INTO detalles_pedido (id_pedido, id_plato, cantidad, precio)
			 VALUES ($1, $2, $3, $4) RETURNING id`,
			orderID, d.IdPlato, d.Cantidad, precio,
		).Scan(&detailID)

		total += precio * float64(d.Cantidad)
		details = append(details, models.OrderDetail{
			ID: detailID, IdPlato: d.IdPlato, Cantidad: d.Cantidad, Precio: precio,
		})
	}

	tx.Exec(`UPDATE pedidos SET total = $1 WHERE id = $2`, total, orderID)
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &models.Order{
		ID: orderID, IdUsuario: userID, IdRestaurante: restaurantID,
		EsRecogida: esRecogida, IdEstadoPedido: stateID,
		Total: total, Detalles: details,
	}, nil
}

func (r *OrderRepository) GetByID(orderID string) (*models.Order, error) {
	order := &models.Order{}
	err := r.db.QueryRow(
		`SELECT id, id_usuario, id_restaurante, es_recogida, id_estado_pedido, total
		 FROM pedidos WHERE id = $1`,
		orderID,
	).Scan(&order.ID, &order.IdUsuario, &order.IdRestaurante,
		&order.EsRecogida, &order.IdEstadoPedido, &order.Total)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Query(
		`SELECT id, id_plato, cantidad, precio FROM detalles_pedido WHERE id_pedido = $1`, orderID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var d models.OrderDetail
		rows.Scan(&d.ID, &d.IdPlato, &d.Cantidad, &d.Precio)
		order.Detalles = append(order.Detalles, d)
	}
	return order, nil
}
