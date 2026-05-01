package main

import (
	"context"
	"log"
	"os"

	"restaurante/handlers"
	"restaurante/keycloak"
	"restaurante/middleware"
	"restaurante/repository"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	// ── Repositorios (Postgres o MongoDB según DB_ENGINE) ──────────────────
	repos, cleanup, err := repository.NewRepos()
	if err != nil {
		log.Fatalf("Error iniciando repositorios: %v", err)
	}
	defer cleanup()
	log.Printf("Motor de BD: %s", os.Getenv("DB_ENGINE"))

	// ── Keycloak: configurar realm, cliente y roles al arrancar ───────────
	kc := keycloak.New()
	if err := kc.Setup(context.Background()); err != nil {
		log.Fatalf("Error configurando Keycloak: %v", err)
	}

	// ── Router ────────────────────────────────────────────────────────────
	router := gin.Default()

	// Rutas públicas
	router.GET("/restaurants", handlers.ListRestaurants(repos.Restaurant))
	router.GET("/menus/:id", handlers.GetMenu(repos.Menu))
	router.POST("/auth/login", handlers.Login(kc))
	router.POST("/auth/register", handlers.RegisterUser(kc, repos.User))

	// Rutas protegidas: el middleware valida el JWT contra Keycloak
	protected := router.Group("")
	protected.Use(middleware.Auth(kc, repos.User))
	{
		protected.GET("/users/me", handlers.GetCurrentUser(repos.User))
		protected.PUT("/users/:id", handlers.UpdateUser(repos.User))
		protected.DELETE("/users/:id", handlers.DeleteUser(repos.User))

		protected.POST("/restaurants", handlers.CreateRestaurant(repos.Restaurant))

		protected.POST("/menus", handlers.CreateMenu(repos.Menu, repos.Restaurant))
		protected.PUT("/menus/:id", handlers.UpdateMenu(repos.Menu))
		protected.DELETE("/menus/:id", handlers.DeleteMenu(repos.Menu))

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
