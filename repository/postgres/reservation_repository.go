package postgres

import (
	"database/sql"
	"time"

	"restaurante/models"
)

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
