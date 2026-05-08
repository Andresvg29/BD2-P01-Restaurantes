package elasticsearch

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	esapi "github.com/elastic/go-elasticsearch/v8"
)

// SetupMockClient crea un servidor mock de Elasticsearch y reemplaza el cliente global
// Retorna una función de cleanup que debe llamarse con defer
// Esta función está disponible para ser usada desde otros paquetes en pruebas
func SetupMockClient(t *testing.T) func() {
	t.Helper()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Elastic-Product", "Elasticsearch")

		var response map[string]interface{}

		// Simular respuestas según el endpoint
		switch {
		case r.Method == "HEAD" && r.URL.Path == "/platos":
			// Verificar existencia del índice
			w.WriteHeader(http.StatusOK)
			return
		case r.Method == "PUT" && r.URL.Path == "/platos":
			// Crear índice
			response = map[string]interface{}{
				"acknowledged":        true,
				"shards_acknowledged": true,
				"index":               "platos",
			}
		case r.Method == "POST" && r.URL.Path == "/platos/_doc" ||
			(r.Method == "PUT" && len(r.URL.Path) >= 13 && r.URL.Path[:13] == "/platos/_doc/"):
			// Indexar documento
			response = map[string]interface{}{
				"result": "created",
				"_id":    "mock-id",
				"_index": "platos",
			}
		case r.Method == "DELETE" && len(r.URL.Path) >= 13 && r.URL.Path[:13] == "/platos/_doc/":
			// Eliminar documento
			response = map[string]interface{}{
				"result": "deleted",
			}
		case r.Method == "POST" && r.URL.Path == "/platos/_search":
			// Búsqueda
			response = map[string]interface{}{
				"hits": map[string]interface{}{
					"total": map[string]interface{}{"value": 0},
					"hits":  []interface{}{},
				},
			}
		default:
			w.WriteHeader(http.StatusOK)
			response = map[string]interface{}{"status": "ok"}
		}

		json.NewEncoder(w).Encode(response)
	}))

	oldClient := ESClient
	cfg := esapi.Config{
		Addresses: []string{ts.URL},
	}
	client, _ := esapi.NewClient(cfg)
	ESClient = client

	return func() {
		ESClient = oldClient
		ts.Close()
	}
}
