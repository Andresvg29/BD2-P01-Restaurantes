package mongo

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"restaurante/models"

	"github.com/elastic/go-elasticsearch/v8"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type RestaurantRepository struct {
	col           *mongo.Collection
	elasticClient *elasticsearch.Client
}

func NewRestaurantRepository(db *mongo.Database, ec *elasticsearch.Client) *RestaurantRepository {
	return &RestaurantRepository{
		col:           db.Collection("restaurantes"),
		elasticClient: ec,
	}
}

func (r *RestaurantRepository) List() ([]models.Restaurant, error) {
	cursor, err := r.col.Find(context.Background(), bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	var list []models.Restaurant
	for cursor.Next(context.Background()) {
		var doc bson.M
		cursor.Decode(&doc)
		list = append(list, models.Restaurant{
			ID:              objectIDToString(doc["_id"]),
			Nombre:          toString(doc["nombre"]),
			Direccion:       toString(doc["direccion"]),
			Telefono:        toString(doc["telefono"]),
			IdAdministrador: toString(doc["id_administrador"]),
			CapacidadTotal:  toInt(doc["capacidad_total"]),
		})
	}
	return list, nil
}

func (r *RestaurantRepository) GetByID(id string) (*models.Restaurant, error) {
	var doc bson.M
	err := r.col.FindOne(context.Background(), bson.M{"_id": id}).Decode(&doc)
	if err != nil {
		return nil, err
	}
	return &models.Restaurant{
		ID:              id,
		Nombre:          toString(doc["nombre"]),
		Direccion:       toString(doc["direccion"]),
		Telefono:        toString(doc["telefono"]),
		IdAdministrador: toString(doc["id_administrador"]),
		CapacidadTotal:  toInt(doc["capacidad_total"]),
	}, nil
}

func (r *RestaurantRepository) Create(nombre, direccion, telefono, adminID string, capacidad int) (string, error) {
	id := primitive.NewObjectID().Hex()
	nuevoRestaurante := bson.M{
		"_id":              id,
		"nombre":           nombre,
		"direccion":        direccion,
		"telefono":         telefono,
		"id_administrador": adminID,
		"capacidad_total":  capacidad,
		"fecha_creacion":   time.Now(),
	}
	_, err := r.col.InsertOne(context.Background(), nuevoRestaurante)
	if err != nil {
		return "", err
	}
	if r.elasticClient != nil {
		go func() {
			esDoc := map[string]interface{}{
				"id":               id,
				"nombre":           nombre,
				"direccion":        direccion,
				"capacidad_total":  capacidad,
				"id_administrador": adminID,
			}
			data, _ := json.Marshal(esDoc)
			res, err := r.elasticClient.Index(
				"restaurantes",
				bytes.NewReader(data),
				r.elasticClient.Index.WithDocumentID(id),
			)
			if err == nil {
				res.Body.Close()
			}
		}()
	}
	return id, nil
}
