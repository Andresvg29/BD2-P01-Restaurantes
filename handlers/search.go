package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"restaurante/elasticsearch"
	"restaurante/models"
	"restaurante/repository"

	"github.com/gin-gonic/gin"
)

// GET /search/products?q=texto
func SearchProducts(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Debe proporcionar un término de búsqueda"})
		return
	}

	//busqueda difusa
	esQuery := map[string]interface{}{
		"query": map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":     query,
				"fields":    []string{"nombre^3", "descripcion"}, // El nombre tiene triple prioridad
				"fuzziness": "AUTO",
			},
		},
	}

	performSearch(c, esQuery)
}

// GET /search/products/category/:categoria
func SearchByCategory(c *gin.Context) {
	categoria := c.Param("categoria")

	// filtro por categoria usando el 'keyword' definido en el mapping
	esQuery := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"categoria": strings.ToLower(categoria),
			},
		},
	}

	performSearch(c, esQuery)
}

// POST /search/reindex
// recibe un repo y devuelve un handler para reindexar todo el contenido de la BD en ES
func ReindexSearch(menuRepo repository.MenuRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		platos, err := menuRepo.List()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener platos de la BD"})
			return
		}
		if err := elasticsearch.ReindexAll(platos); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error durante la reindexación en ES"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "Proceso de reindexación completado con éxito",
			"count":   len(platos),
		})
	}
}

// func auxiliar para ejecutar la busqueda y parsear resultados
func performSearch(c *gin.Context, query map[string]interface{}) {
	var buf strings.Builder
	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al codificar consulta"})
		return
	}

	res, err := elasticsearch.ESClient.Search(
		elasticsearch.ESClient.Search.WithContext(context.Background()),
		elasticsearch.ESClient.Search.WithIndex("platos"),
		elasticsearch.ESClient.Search.WithBody(strings.NewReader(buf.String())),
		elasticsearch.ESClient.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error en la comunicación con Elasticsearch"})
		return
	}
	defer res.Body.Close()

	// Parsear la respuesta compleja de ES a MenuItem
	var r map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al parsear respuesta"})
		return
	}

	var results []models.MenuItem
	for _, hit := range r["hits"].(map[string]interface{})["hits"].([]interface{}) {
		source := hit.(map[string]interface{})["_source"]
		var item models.MenuItem
		jsonBytes, _ := json.Marshal(source)
		json.Unmarshal(jsonBytes, &item)
		results = append(results, item)
	}

	c.JSON(http.StatusOK, results)
}
