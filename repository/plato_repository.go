package repository

import (
	"context"
)

type Plato struct {
	ID          string  `json:"id"`
	Nombre      string  `json:"nombre"`
	Descripcion string  `json:"descripcion"`
	Precio      float64 `json:"precio"`
}

type PlatoRepository interface {
	ObtenerTodos(ctx context.Context) ([]Plato, error)
}