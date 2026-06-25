package mongo

import (
	"context"

	"restaurante/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type OrderRepository struct {
	pedidosCol *mongo.Collection
	platosCol  *mongo.Collection
}

func NewOrderRepository(db *mongo.Database) *OrderRepository {
	return &OrderRepository{
		pedidosCol: db.Collection("pedidos"),
		platosCol:  db.Collection("platos"),
	}
}

func (r *OrderRepository) Create(userID, restaurantID string, esRecogida bool, detalles []models.OrderDetailRequest) (*models.Order, error) {
	orderID := primitive.NewObjectID().Hex()
	var total float64
	var details []models.OrderDetail

	for _, d := range detalles {
		var plato bson.M
		r.platosCol.FindOne(context.Background(), bson.M{"_id": d.IdPlato}).Decode(&plato)
		precio := toFloat64(plato["precio"])
		detailID := primitive.NewObjectID().Hex()
		total += precio * float64(d.Cantidad)
		details = append(details, models.OrderDetail{
			ID: detailID, IdPlato: d.IdPlato, Cantidad: d.Cantidad, Precio: precio,
		})
	}

	_, err := r.pedidosCol.InsertOne(context.Background(), bson.M{
		"_id":              orderID,
		"id_usuario":       userID,
		"id_restaurante":   restaurantID,
		"es_recogida":      esRecogida,
		"id_estado_pedido": 1,
		"total":            total,
		"detalles":         details,
	})
	if err != nil {
		return nil, err
	}
	return &models.Order{
		ID: orderID, IdUsuario: userID, IdRestaurante: restaurantID,
		EsRecogida: esRecogida, IdEstadoPedido: 1, Total: total, Detalles: details,
	}, nil
}

func (r *OrderRepository) GetByID(orderID string) (*models.Order, error) {
	var doc bson.M
	if err := r.pedidosCol.FindOne(context.Background(), bson.M{"_id": orderID}).Decode(&doc); err != nil {
		return nil, err
	}
	order := &models.Order{
		ID:             orderID,
		IdUsuario:      toString(doc["id_usuario"]),
		IdRestaurante:  toString(doc["id_restaurante"]),
		EsRecogida:     toBool(doc["es_recogida"]),
		IdEstadoPedido: toInt(doc["id_estado_pedido"]),
		Total:          toFloat64(doc["total"]),
	}
	return order, nil
}
