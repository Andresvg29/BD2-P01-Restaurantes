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
		// Si es nil o no es el tipo correcto, asignamos una fecha por defecto
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
				"restaurantes",        // Índice
				bytes.NewReader(data), // Body
				r.elasticClient.Index.WithDocumentID(id),
			)
			if err == nil {
				res.Body.Close()
			}
		}()
	}

	return id, nil
}

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

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	case primitive.ObjectID:
		return s.Hex()
	default:
		return ""
	}
}

func toBool(v interface{}) bool {
	if v == nil {
		return false
	}
	switch b := v.(type) {
	case bool:
		return b
	default:
		return false
	}
}

func toFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch i := v.(type) {
	case float64:
		return i
	case int32:
		return float64(i)
	case int:
		return float64(i)
	case int64:
		return float64(i)
	default:
		return 0
	}
}

func objectIDToString(v interface{}) string {
	switch id := v.(type) {
	case string:
		return id
	case primitive.ObjectID:
		return id.Hex()
	default:
		return ""
	}
}

func toInt(v interface{}) int {
	if v == nil {
		return 0
	}
	switch i := v.(type) {
	case int:
		return i
	case int32:
		return int(i)
	case int64:
		return int(i)
	case float64:
		return int(i)
	default:
		return 0
	}
}
