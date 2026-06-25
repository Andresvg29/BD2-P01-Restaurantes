package postgres

import (
	"database/sql"

	"restaurante/models"
)

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
