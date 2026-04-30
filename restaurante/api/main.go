package main

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"strings"

	"github.com/andre/restaurante/handlers"
	"github.com/andre/restaurante/repository"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	repos, cleanup, err := repository.NewRepos()
	if err != nil {
		log.Fatalf("Error iniciando repositorios: %v", err)
	}
	defer cleanup()

	log.Printf("Conectado usando motor: %s", os.Getenv("DB_ENGINE"))

//router
	router := gin.Default()

	// Rutas publicas
	router.GET("/restaurants", handlers.ListRestaurants(repos.Restaurant))
	router.GET("/menus/:id", handlers.GetMenu(repos.Menu))
	router.POST("/auth/login-info", handlers.LoginInfo())
	router.POST("/auth/register", handlers.RegisterUser(repos.User))

	// Rutas protegidas con JWT de Keycloak
	auth := router.Group("")
	auth.Use(keycloakMiddleware(repos.User))
	{
		auth.GET("/users/me", handlers.GetCurrentUser(repos.User))
		auth.PUT("/users/:id", handlers.UpdateUser(repos.User))
		auth.DELETE("/users/:id", handlers.DeleteUser(repos.User))

		auth.POST("/restaurants", handlers.CreateRestaurant(repos.Restaurant))

		auth.POST("/menus", handlers.CreateMenu(repos.Menu, repos.Restaurant))
		auth.PUT("/menus/:id", handlers.UpdateMenu(repos.Menu))
		auth.DELETE("/menus/:id", handlers.DeleteMenu(repos.Menu))

		auth.POST("/reservations", handlers.CreateReservation(repos.Reservation))
		auth.DELETE("/reservations/:id", handlers.CancelReservation(repos.Reservation))

		auth.POST("/orders", handlers.CreateOrder(repos.Order))
		auth.GET("/orders/:id", handlers.GetOrder(repos.Order))
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}
	log.Printf("Servidor iniciando en puerto %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

// keycloakMiddleware valida el JWT y sincroniza el usuario en la BD.
func keycloakMiddleware(users repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header requerido"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			url := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/certs",
				os.Getenv("KEYCLOAK_URL"), os.Getenv("KEYCLOAK_REALM"))
			return getPublicKeyFromKeycloak(url, token)
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token inválido o expirado"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "No se pudieron leer los claims"})
			c.Abort()
			return
		}

		userID := claims["sub"].(string)
		userName := claims["preferred_username"].(string)
		userEmail := claims["email"].(string)
		// Sincronizar usuario con la BD
		_ = users.UpsertFromToken(userID, userName, userEmail)
		c.Set("user_id", userID)
		c.Next()
	}
}

func getPublicKeyFromKeycloak(certsURL string, token *jwt.Token) (interface{}, error) {
	resp, err := http.Get(certsURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var jwks struct {
		Keys []struct {
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	json.NewDecoder(resp.Body).Decode(&jwks)

	kid := token.Header["kid"].(string)
	for _, key := range jwks.Keys {
		if key.Kid == kid {
			nBytes, _ := base64.RawURLEncoding.DecodeString(key.N)
			eBytes, _ := base64.RawURLEncoding.DecodeString(key.E)
			var e int
			for _, b := range eBytes {
				e = e<<8 + int(b)
			}
			return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}, nil
		}
	}
	return nil, fmt.Errorf("llave no encontrada")
}