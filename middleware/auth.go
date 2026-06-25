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

// Auth validates the JWT against Keycloak and injects user_id and user_email into the context.
func Auth(kc *keycloak.Client, users repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header requerido"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		// DecodeAccessToken verifies the signature with Keycloak's public keys
		userID, userName, userEmail, roles, err := kc.DecodeToken(context.Background(), tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token inválido o expirado"})
			c.Abort()
			return
		}

		isAdmin := false
		for _, r := range roles {
			if r == "admin" {
				isAdmin = true
				break
			}
		}
			// Sync user with the local DB (creates if it does not exist yet)
		err = users.UpsertFromToken(userID, userName, userEmail, isAdmin)
		if err != nil {
			log.Printf("Error de sincronizacion: %v", err)
		}

		user, err := users.GetByID(userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al recuperar perfil"})
			c.Abort()
			return
		}

		// Inject into the context for the handlers
		c.Set("user_id", userID)
		c.Set("user_email", userEmail)
		c.Set("user_role_id", user.IdRol) // user roles
		c.Next()
	}
}

// RequireRole checks that the JWT contains a specific role in realm_access.roles
func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		roles, exists := c.Get("user_roles")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "Sin roles asignados"})
			c.Abort()
			return
		}
		rolesList, ok := roles.([]string)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "Formato de roles inválido"})
			c.Abort()
			return
		}
		for _, r := range rolesList {
			if r == role {
				c.Next()
				return
			}
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "Rol requerido: " + role})
		c.Abort()
	}
}

// RequireAdmin is middleware that checks the user's role is admin (id_rol == 1).
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		roleID, exists := c.Get("user_role_id")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "Identidad de rol no encontrada"})
			c.Abort()
			return
		}

		// Check if role ID is 1 (Admin)
		roleIDInt, ok := roleID.(int)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "Formato de rol inválido"})
			c.Abort()
			return
		}
		if roleIDInt != 1 {
			c.JSON(http.StatusForbidden, gin.H{"error": "Acceso restringido"})
			c.Abort()
			return
		}

		c.Next()
	}
}
