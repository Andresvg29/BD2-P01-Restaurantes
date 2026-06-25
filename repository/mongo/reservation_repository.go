package mongo

import (
	"context"
	"time"

	"restaurante/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type ReservationRepository struct {
	col             *mongo.Collection
	restaurantesCol *mongo.Collection
}

func NewReservationRepository(db *mongo.Database) *ReservationRepository {
	return &ReservationRepository{
		col:             db.Collection("reservas"),
		restaurantesCol: db.Collection("restaurantes"),
	}
}

func (r *ReservationRepository) GetRestaurantCapacity(restaurantID string) (int, error) {
	var doc bson.M
	if err := r.restaurantesCol.FindOne(context.Background(), bson.M{"_id": restaurantID}).Decode(&doc); err != nil {
		return 0, err
	}
	return toInt(doc["capacidad_total"]), nil
}

func (r *ReservationRepository) Create(userID, restaurantID string, fecha time.Time, personas int) (*models.Reservation, error) {
	id := primitive.NewObjectID().Hex()
	_, err := r.col.InsertOne(context.Background(), bson.M{
		"_id":               id,
		"id_usuario":        userID,
		"id_restaurante":    restaurantID,
		"fecha_reserva":     fecha,
		"cantidad_personas": personas,
		"id_estado":         1,
		"nombre_estado":     "pendiente",
	})
	if err != nil {
		return nil, err
	}
	return &models.Reservation{
		ID: id, IdUsuario: userID, IdRestaurante: restaurantID,
		FechaReserva: fecha, CantidadPersonas: personas,
		IdEstado: 1, NombreEstado: "pendiente",
	}, nil
}

func (r *ReservationRepository) GetOwnerID(reservationID string) (string, error) {
	var doc bson.M
	err := r.col.FindOne(context.Background(), bson.M{"_id": reservationID}).Decode(&doc)
	if err != nil {
		return "", err
	}
	return toString(doc["id_usuario"]), nil
}

func (r *ReservationRepository) Cancel(reservationID string) error {
	_, err := r.col.UpdateOne(
		context.Background(),
		bson.M{"_id": reservationID},
		bson.M{"$set": bson.M{"id_estado": 3, "nombre_estado": "cancelada"}},
	)
	return err
}
