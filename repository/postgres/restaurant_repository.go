package postgres

import (
	"bytes"
	"database/sql"
	"encoding/json"

	"restaurante/models"

	"github.com/elastic/go-elasticsearch/v8"
)

type RestaurantRepository struct {
	db            *sql.DB
	elasticClient *elasticsearch.Client
}

func NewRestaurantRepository(db *sql.DB, elasticClient *elasticsearch.Client) *RestaurantRepository {
	return &RestaurantRepository{db: db, elasticClient: elasticClient}
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

	if err != nil {
		return "", err
	}
	if r.elasticClient != nil {
		nuevoRestaurante := map[string]interface{}{
			"id":               id,
			"nombre":           nombre,
			"direccion":        direccion,
			"telefono":         telefono,
			"id_administrador": adminID,
			"capacidad_total":  capacidad,
		}
		go func() {
			data, _ := json.Marshal(nuevoRestaurante)
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
