package elasticsearch

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	esapi "github.com/elastic/go-elasticsearch/v8"
)

// SetupMockClient creates a mock Elasticsearch server and replaces the global client
// Returns a cleanup function that must be called with defer
// This function is available for use from other packages in tests
func SetupMockClient(t *testing.T) func() {
	t.Helper()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Elastic-Product", "Elasticsearch")

		var response map[string]interface{}

		// Simulate responses based on endpoint
		switch {
		case r.Method == "HEAD" && r.URL.Path == "/platos":
			// Check index existence
			w.WriteHeader(http.StatusOK)
			return
		case r.Method == "PUT" && r.URL.Path == "/platos":
			// Create index
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
			// Search
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
