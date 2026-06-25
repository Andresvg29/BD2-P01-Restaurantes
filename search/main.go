package main

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

	// elasticsearch
	esUrl := os.Getenv("ELASTICSEARCH_URL")
	elasticsearch.InitClient(esUrl)
	log.Println("ElasticSearch conectado")

	// repositorios
	repos, cleanup, err := repository.NewRepos()
	if err != nil {
		log.Fatalf("Error iniciando repositorios: %v", err)
	}
	defer cleanup()
	log.Printf("BD conectada (motor: %s)", os.Getenv("DB_ENGINE"))

	// router
	router := gin.Default()

	// search routes
	search := router.Group("/search")
	{
		search.GET("/products", handlers.SearchProducts)
		search.GET("/products/category/:categoria", handlers.SearchByCategory)
		search.POST("/reindex", handlers.ReindexSearch(repos.Menu))
	}

	port := os.Getenv("SEARCH_PORT")
	if port == "" {
		port = "8001"
	}
	log.Printf("Search microservice on http://0.0.0.0:%s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
