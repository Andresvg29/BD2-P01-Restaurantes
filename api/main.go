package main

import (
	"context"
	"log"
	"os"

	"restaurante/elasticsearch"
	"restaurante/handlers"
	"restaurante/keycloak"
	"restaurante/middleware"
	"restaurante/repository"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	// Postgres o MongoDB segun el engine
	repos, cleanup, err := repository.NewRepos()
	if err != nil {
		log.Fatalf("Error iniciando repositorios: %v", err)
	}
	defer cleanup()
	log.Printf("Motor de BD: %s", os.Getenv("DB_ENGINE"))

	// ElasticSearch
	esUrl := os.Getenv("ELASTICSEARCH_URL")
	elasticsearch.InitClient(esUrl)

	// Keycloak
	kc := keycloak.New()
	if err := kc.Setup(context.Background()); err != nil {
		log.Fatalf("Error configurando Keycloak: %v", err)
	}

	// router
	router := gin.Default()

	// Request logger middleware (request ID, method, path, status, duración)
	router.Use(middleware.RequestLogger())

	// Redis cache middleware (cache GET públicos + invalida en mutaciones)
	cache := middleware.NewRedisCacheMiddleware(nil,
		middleware.WithSkipPaths("/auth/", "/users/", "/orders/", "/reservations/"),
	)
	if repos.RedisClient != nil {
		cache = middleware.NewRedisCacheMiddleware(repos.RedisClient.Client,
			middleware.WithSkipPaths("/auth/", "/users/", "/orders/", "/reservations/"),
		)
	}
	router.Use(cache.Middleware())

	// Rutas publicas
	router.GET("/restaurants", handlers.ListRestaurants(repos.Restaurant))
	router.GET("/menus/:id", handlers.GetMenu(repos.Menu))
	router.POST("/auth/login", handlers.Login(kc))
	router.POST("/auth/register", handlers.RegisterUser(kc, repos.User))

	// admin
	protected := router.Group("")
	protected.Use(middleware.Auth(kc, repos.User))
	adminOnly := protected.Group("")
	adminOnly.Use(middleware.RequireAdmin())
	{
		adminOnly.POST("/restaurants", handlers.CreateRestaurant(repos.Restaurant))
		adminOnly.POST("/menus", handlers.CreateMenu(repos.Menu, repos.Restaurant))
		adminOnly.PUT("/menus/:id", handlers.UpdateMenu(repos.Menu))
		adminOnly.DELETE("/menus/:id", handlers.DeleteMenu(repos.Menu))
	}

	// Rutas protegidas para autenticados
	{
		protected.GET("/users/me", handlers.GetCurrentUser(repos.User))
		protected.PUT("/users/:id", handlers.UpdateUser(repos.User))
		protected.DELETE("/users/:id", handlers.DeleteUser(repos.User))

		protected.POST("/reservations", handlers.CreateReservation(repos.Reservation))
		protected.DELETE("/reservations/:id", handlers.CancelReservation(repos.Reservation))

		protected.POST("/orders", handlers.CreateOrder(repos.Order))
		protected.GET("/orders/:id", handlers.GetOrder(repos.Order))
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}
	log.Printf("Servidor en http://0.0.0.0:%s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
