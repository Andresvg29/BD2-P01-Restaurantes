package postgres

import (
	"database/sql"

	"restaurante/models"
)

type MenuRepository struct {
	db *sql.DB
}

func NewMenuRepository(db *sql.DB) *MenuRepository {
	return &MenuRepository{db: db}
}

func (r *MenuRepository) List() ([]models.MenuItem, error) {
	rows, err := r.db.Query(
		`SELECT id, id_restaurante, nombre, descripcion, precio, esta_disponible, categoria FROM platos`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.MenuItem
	for rows.Next() {
		var item models.MenuItem
		if err := rows.Scan(&item.ID, &item.IdRestaurante, &item.Nombre,
			&item.Descripcion, &item.Precio, &item.EstaDisponible, &item.Categoria); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *MenuRepository) GetByRestaurant(restaurantID string) ([]models.MenuItem, error) {
	rows, err := r.db.Query(
		`SELECT id, id_restaurante, nombre, descripcion, precio, esta_disponible, categoria
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
			&item.Descripcion, &item.Precio, &item.EstaDisponible, &item.Categoria); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *MenuRepository) Create(restaurantID, nombre, descripcion string, precio float64, categoria string) (string, error) {
	if descripcion == "" {
		descripcion = "Producto sin descripcion"
	}
	var id string
	err := r.db.QueryRow(
		`INSERT INTO platos (id_restaurante, nombre, descripcion, precio, categoria) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		restaurantID, nombre, descripcion, precio, categoria,
	).Scan(&id)
	return id, err
}

func (r *MenuRepository) Update(id, nombre, descripcion string, precio float64, disponible bool, categoria string) error {
	_, err := r.db.Exec(
		`UPDATE platos SET
			nombre      = COALESCE(NULLIF($1, ''), nombre),
			descripcion = COALESCE(NULLIF($2, ''), descripcion),
			precio      = CASE WHEN $3 > 0 THEN $3 ELSE precio END,
			esta_disponible = $4,
			categoria = COALESCE(NULLIF($5, ''), categoria)
		 WHERE id = $6`,
		nombre, descripcion, precio, disponible, categoria, id,
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
