package search

import (
	"log"
	"os"

	"restaurante/elasticsearch"
	"restaurante/handlers"
	"restaurante/repository"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	// ── ElasticSearch ──────────────────────────────────────────────────────
	if err := elasticsearch.InitClient(); err != nil {
		log.Fatalf("Error iniciando cliente ElasticSearch: %v", err)
	}
	log.Println("ElasticSearch conectado")

	// ── Repositorios (solo necesario para POST /search/reindex) ───────────
	repos, cleanup, err := repository.NewRepos()
	if err != nil {
		log.Fatalf("Error iniciando repositorios: %v", err)
	}
	defer cleanup()
	log.Printf("✓ BD conectada (motor: %s)", os.Getenv("DB_ENGINE"))

	// ── Router ────────────────────────────────────────────────────────────
	router := gin.Default()

	// Todas las rutas de búsqueda son públicas (no requieren JWT)
	search := router.Group("/search")
	{
		search.GET("/products", handlers.SearchProducts)
		search.GET("/products/category/:categoria", handlers.SearchByCategory)
		search.POST("/reindex", handlers.ReindexSearch(repos.Menu))
	}

	// ── Arrancar servidor ─────────────────────────────────────────────────
	port := os.Getenv("SEARCH_PORT")
	if port == "" {
		port = "8001"
	}
	log.Printf("Microservicio de busqueda en http://0.0.0.0:%s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
