package mongo

import (
	"context"

	"restaurante/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type MenuRepository struct {
	platosCol       *mongo.Collection
	restaurantesCol *mongo.Collection
}

func NewMenuRepository(db *mongo.Database) *MenuRepository {
	return &MenuRepository{
		platosCol:       db.Collection("platos"),
		restaurantesCol: db.Collection("restaurantes"),
	}
}

func (r *MenuRepository) List() ([]models.MenuItem, error) {
	cursor, err := r.platosCol.Find(context.Background(), bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	var items []models.MenuItem
	for cursor.Next(context.Background()) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		items = append(items, models.MenuItem{
			ID:             objectIDToString(doc["_id"]),
			IdRestaurante:  toString(doc["id_restaurante"]),
			Nombre:         toString(doc["nombre"]),
			Descripcion:    toString(doc["descripcion"]),
			Precio:         toFloat64(doc["precio"]),
			EstaDisponible: toBool(doc["esta_disponible"]),
			Categoria:      toString(doc["categoria"]),
		})
	}
	return items, nil
}

func (r *MenuRepository) GetByRestaurant(restaurantID string) ([]models.MenuItem, error) {
	cursor, err := r.platosCol.Find(context.Background(), bson.M{"id_restaurante": restaurantID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	var items []models.MenuItem
	for cursor.Next(context.Background()) {
		var doc bson.M
		cursor.Decode(&doc)
		items = append(items, models.MenuItem{
			ID:             objectIDToString(doc["_id"]),
			IdRestaurante:  restaurantID,
			Nombre:         toString(doc["nombre"]),
			Descripcion:    toString(doc["descripcion"]),
			Precio:         toFloat64(doc["precio"]),
			EstaDisponible: toBool(doc["esta_disponible"]),
			Categoria:      toString(doc["categoria"]),
		})
	}
	return items, nil
}

func (r *MenuRepository) Create(restaurantID, nombre, descripcion string, precio float64, categoria string) (string, error) {
	id := primitive.NewObjectID().Hex()
	if descripcion == "" {
		descripcion = "Producto sin descripcion"
	}
	_, err := r.platosCol.InsertOne(context.Background(), bson.M{
		"_id":             id,
		"id_restaurante":  restaurantID,
		"nombre":          nombre,
		"descripcion":     descripcion,
		"precio":          precio,
		"esta_disponible": true,
		"categoria":       categoria,
	})
	return id, err
}

func (r *MenuRepository) Update(id, nombre, descripcion string, precio float64, disponible bool, categoria string) error {
	update := bson.M{"esta_disponible": disponible}
	if nombre != "" {
		update["nombre"] = nombre
	}
	if descripcion != "" {
		update["descripcion"] = descripcion
	}
	if precio > 0 {
		update["precio"] = precio
	}
	if categoria != "" {
		update["categoria"] = categoria
	}
	_, err := r.platosCol.UpdateOne(context.Background(), bson.M{"_id": id}, bson.M{"$set": update})
	return err
}

func (r *MenuRepository) Delete(id string) error {
	_, err := r.platosCol.DeleteOne(context.Background(), bson.M{"_id": id})
	return err
}

func (r *MenuRepository) GetAdminID(platoID string) (string, error) {
	var plato bson.M
	if err := r.platosCol.FindOne(context.Background(), bson.M{"_id": platoID}).Decode(&plato); err != nil {
		return "", err
	}
	restaurantID := toString(plato["id_restaurante"])

	var restaurant bson.M
	if err := r.restaurantesCol.FindOne(context.Background(), bson.M{"_id": restaurantID}).Decode(&restaurant); err != nil {
		return "", err
	}
	return toString(restaurant["id_administrador"]), nil
}
