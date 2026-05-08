package elasticsearch

import (
	"testing"

	"restaurante/models"

	"github.com/stretchr/testify/assert"
)

func TestIndexPlato(t *testing.T) {
	cleanup := SetupMockClient(t)
	defer cleanup()

	plato := models.MenuItem{
		ID:        "test-plato-1",
		Nombre:    "Plato Test",
		Precio:    1000,
		Categoria: "Test",
	}

	err := IndexPlato(plato)
	assert.NoError(t, err)
}

func TestDeletePlato(t *testing.T) {
	cleanup := SetupMockClient(t)
	defer cleanup()

	err := DeletePlato("test-plato-1")
	assert.NoError(t, err)
}

func TestReindexAll(t *testing.T) {
	cleanup := SetupMockClient(t)
	defer cleanup()

	platos := []models.MenuItem{
		{ID: "p1", Nombre: "Plato 1", Precio: 1000},
		{ID: "p2", Nombre: "Plato 2", Precio: 2000},
	}

	err := ReindexAll(platos)
	assert.NoError(t, err)
}
