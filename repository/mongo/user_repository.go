package mongo

import (
	"context"
	"time"

	"restaurante/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type UserRepository struct {
	col *mongo.Collection
}

func NewUserRepository(db *mongo.Database) *UserRepository {
	return &UserRepository{col: db.Collection("usuarios")}
}

func (r *UserRepository) GetByID(id string) (*models.User, error) {
	var result bson.M
	err := r.col.FindOne(context.Background(), bson.M{"_id": id}).Decode(&result)
	if err != nil {
		return nil, err
	}
	user := &models.User{
		ID:            id,
		Nombre:        toString(result["nombre"]),
		Email:         toString(result["email"]),
		IdRol:         toInt(result["id_rol"]),
		FechaCreacion: time.Time{},
	}
	if fecha, ok := result["fecha_creacion"].(primitive.DateTime); ok {
		user.FechaCreacion = fecha.Time()
	} else {
		user.FechaCreacion = time.Time{}
	}
	return user, nil
}

func (r *UserRepository) GetByEmail(email string) (*models.User, error) {
	var result bson.M
	err := r.col.FindOne(context.Background(), bson.M{"email": email}).Decode(&result)
	if err != nil {
		return nil, err
	}
	id := objectIDToString(result["_id"])
	return &models.User{ID: id, Email: email}, nil
}

func (r *UserRepository) Create(nombre, email, keycloakID string) (string, error) {
	doc := bson.M{
		"_id":            primitive.NewObjectID().Hex(),
		"nombre":         nombre,
		"email":          email,
		"keycloak_id":    keycloakID,
		"id_rol":         2,
		"fecha_creacion": time.Now(),
	}
	res, err := r.col.InsertOne(context.Background(), doc)
	if err != nil {
		return "", err
	}
	return objectIDToString(res.InsertedID), nil
}

func (r *UserRepository) Update(id, nombre, email string) error {
	update := bson.M{}
	if nombre != "" {
		update["nombre"] = nombre
	}
	if email != "" {
		update["email"] = email
	}
	_, err := r.col.UpdateOne(context.Background(), bson.M{"_id": id}, bson.M{"$set": update})
	return err
}

func (r *UserRepository) Delete(id string) error {
	_, err := r.col.DeleteOne(context.Background(), bson.M{"_id": id})
	return err
}

func (r *UserRepository) UpsertFromToken(id, nombre, email string, isAdmin bool) error {
	idRol := 2
	if isAdmin {
		idRol = 1
	}
	_, err := r.col.UpdateOne(
		context.Background(),
		bson.M{"_id": id},
		bson.M{
			"$set": bson.M{
				"nombre": nombre,
				"email":  email,
				"id_rol": idRol,
			},
			"$setOnInsert": bson.M{
				"_id":            id,
				"fecha_creacion": time.Now(),
			},
		},
		options.Update().SetUpsert(true),
	)
	return err
}
