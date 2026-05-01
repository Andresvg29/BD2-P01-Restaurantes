package middleware

import (
	"context"
	"log"
	"net/http"
	"strings"

	"restaurante/keycloak"
	"restaurante/repository"

	"github.com/gin-gonic/gin"
)

// Auth valida el JWT contra Keycloak e inyecta user_id y user_email en el contexto.
func Auth(kc *keycloak.Client, users repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header requerido"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// DecodeAccessToken verifica la firma con las claves públicas de Keycloak
		// y devuelve los claims completos del JWT (sub, email, preferred_username, etc.)
		userID, userName, userEmail, err := kc.DecodeToken(context.Background(), tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token inválido o expirado"})
			c.Abort()
			return
		}

		// Sincronizar usuario con la BD local (crea si no existe aún)
		err = users.UpsertFromToken(userID, userName, userEmail)
		if err != nil {
			log.Printf("Error crítico de sincronización: %v", err)
		}

		// Inyectar en el contexto para los handlers
		c.Set("user_id", userID)
		c.Set("user_email", userEmail)
		c.Next()
	}
}

// RequireRole verifica que el JWT contenga un rol específico en realm_access.roles.
// Se usa encadenado después de Auth: protected.Use(Auth(...)), protected.Use(RequireRole("admin"))
func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		roles, exists := c.Get("user_roles")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "Sin roles asignados"})
			c.Abort()
			return
		}
		for _, r := range roles.([]string) {
			if r == role {
				c.Next()
				return
			}
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "Rol requerido: " + role})
		c.Abort()
	}
}
