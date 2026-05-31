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
		// DecodeAccessToken verifica la firma con las claves publicas de Keycloak
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
		log.Printf("Usuario %s detectado como Admin: %v", userID, isAdmin) // Agrega esto para debugear
		// Sincronizar usuario con la BD local (crea si no existe aún)
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

		// Inyectar en el contexto para los handlers
		c.Set("user_id", userID)
		c.Set("user_email", userEmail)
		c.Set("user_role_id", user.IdRol) // roles del usuario
		c.Next()
	}
}

// RequireRole verifica que el JWT contenga un rol específico en realm_access.roles
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

// RequireAdmin es middleware que verifica que el rol del usuario sea admin (id_rol == 1).
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		roleID, exists := c.Get("user_role_id")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "Identidad de rol no encontrada"})
			c.Abort()
			return
		}

		// Verificamos si el ID de rol es 1 (Admin)
		if roleID.(int) != 1 {
			c.JSON(http.StatusForbidden, gin.H{"error": "Acceso restringido"})
			c.Abort()
			return
		}

		c.Next()
	}
}
