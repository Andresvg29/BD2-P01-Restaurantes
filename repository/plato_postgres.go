package repository

import (
	"context"
	"database/sql"
)

type postgresRepo struct {
	db *sql.DB
}

// NewPostgresRepo inicializa el repositorio físico de PostgreSQL
func NewPostgresRepo(db *sql.DB) PlatoRepository {
	return &postgresRepo{db: db}
}

func (r *postgresRepo) ObtenerTodos(ctx context.Context) ([]Plato, error) {
	query := `SELECT id, nombre, descripcion, precio FROM platos`
	
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var platos []Plato
	for rows.Next() {
		var p Plato
		if err := rows.Scan(&p.ID, &p.Nombre, &p.Descripcion, &p.Precio); err != nil {
			return nil, err
		}
		platos = append(platos, p)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return platos, nil
}