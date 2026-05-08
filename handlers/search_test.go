package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"restaurante/elasticsearch"
	"restaurante/handlers"
	"restaurante/mocks"
	"restaurante/models"

	esapi "github.com/elastic/go-elasticsearch/v8"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// RoundTripFunc permite mockear el transporte del cliente de ES
type RoundTripFunc func(req *http.Request) *http.Response

func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

// Helper para crear un cliente de ES mockeado
func createMockESClient(body string, status int) *esapi.Client {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))

	cfg := esapi.Config{
		Addresses: []string{ts.URL},
	}

	client, _ := esapi.NewClient(cfg)
	return client
}

func TestSearchProducts(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Debe fallar si no hay query 'q'", func(t *testing.T) {
		r := gin.New()
		r.GET("/search", handlers.SearchProducts)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/search", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Debe proporcionar un término de búsqueda")
	})

	t.Run("Búsqueda exitosa con resultados", func(t *testing.T) {
		// Simulación de respuesta de Elasticsearch con un "Gallo Pinto"
		esResponse := `{
			"hits": {
				"total": { "value": 1 },
				"hits": [
					{
						"_source": {
							"id": "p1",
							"nombre": "Gallo Pinto Premium",
							"descripcion": "El mejor de Cartago",
							"precio": 3500,
							"categoria": "Desayunos"
						}
					}
				]
			}
		}`

		// Sustituimos temporalmente el cliente global
		oldClient := elasticsearch.ESClient
		elasticsearch.ESClient = createMockESClient(esResponse, http.StatusOK)
		defer func() { elasticsearch.ESClient = oldClient }()

		r := gin.New()
		r.GET("/search", handlers.SearchProducts)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/search?q=pinto", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var results []models.MenuItem
		json.Unmarshal(w.Body.Bytes(), &results)

		assert.Len(t, results, 1)
		assert.Equal(t, "Gallo Pinto Premium", results[0].Nombre)
	})
}

func TestSearchByCategory(t *testing.T) {
	t.Run("Búsqueda por categoría exitosa", func(t *testing.T) {
		esResponse := `{
			"hits": {
				"hits": [
					{ "_source": { "nombre": "Ceviche", "categoria": "Mariscos" } }
				]
			}
		}`

		oldClient := elasticsearch.ESClient
		elasticsearch.ESClient = createMockESClient(esResponse, http.StatusOK)
		defer func() { elasticsearch.ESClient = oldClient }()

		r := gin.New()
		r.GET("/search/category/:categoria", handlers.SearchByCategory)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/search/category/Mariscos", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Ceviche")
	})
}

func TestReindexSearch(t *testing.T) {
	mockMenuRepo := new(mocks.MockMenuRepository)

	t.Run("Reindexación completa exitosa", func(t *testing.T) {
		// 1. Setup Datos del Repo
		platos := []models.MenuItem{
			{ID: "1", Nombre: "Olla de Carne"},
		}
		mockMenuRepo.On("List").Return(platos, nil).Once()

		// 2. Mock de ES para la creación del índice (Bulk o Index)
		// En ReindexSearch se llama a ESClient por cada plato
		esResponse := `{"result": "created"}`
		oldClient := elasticsearch.ESClient
		elasticsearch.ESClient = createMockESClient(esResponse, http.StatusOK)
		defer func() { elasticsearch.ESClient = oldClient }()

		r := gin.New()
		// Pasamos el mock al handler
		r.POST("/search/reindex", handlers.ReindexSearch(mockMenuRepo))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/search/reindex", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Proceso de reindexación completado")
		mockMenuRepo.AssertExpectations(t)
	})
}
