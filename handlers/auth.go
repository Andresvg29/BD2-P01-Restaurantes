package handlers

import (
	"context"
	"database/sql"
	"net/http"

	"restaurante/keycloak"
	"restaurante/repository"

	"github.com/gin-gonic/gin"
)

// Login autentica al usuario contra Keycloak y devuelve el JWT
func Login(kc *keycloak.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Email    string `json:"email"    binding:"required"`
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		token, err := kc.LoginUser(context.Background(), req.Email, req.Password)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Credenciales invalidas"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"access_token":  token.AccessToken,
			"refresh_token": token.RefreshToken,
			"expires_in":    token.ExpiresIn,
			"token_type":    token.TokenType,
		})
	}
}

// crea el usuario en Keycloak y guarda referencia en la BD
func RegisterUser(kc *keycloak.Client, users repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Nombre   string `json:"nombre"   binding:"required"`
			Apellido string `json:"apellido" binding:"required"`
			Email    string `json:"email"    binding:"required"`
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		// crear en Keycloak
		keycloakID, err := kc.RegisterUser(context.Background(), req.Nombre, req.Apellido, req.Email, req.Password)
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}

		// sincronizar en la BD local
		if err := users.UpsertFromToken(keycloakID, req.Nombre, req.Email, false); err != nil {
			// el usuario existe en Keycloak aunque falle la BD local
			c.JSON(http.StatusCreated, gin.H{
				"id":      keycloakID,
				"message": "Usuario creado (sincronizacion BD pendiente)",
			})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"id":      keycloakID,
			"nombre":  req.Nombre,
			"email":   req.Email,
			"message": "Usuario registrado exitosamente",
		})
	}
}

//devuelve el perfil del usuario autenticado.

func GetCurrentUser(users repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		user, err := users.GetByID(userID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Usuario no encontrado"})
			return
		}
		c.JSON(http.StatusOK, user)
	}
}

// actualiza nombre y/o email del usuario.
func UpdateUser(users repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID != c.Param("id") {
			c.JSON(http.StatusForbidden, gin.H{"error": "No autorizado"})
			return
		}
		var req struct {
			Nombre string `json:"nombre"`
			Email  string `json:"email"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := users.Update(userID, req.Nombre, req.Email); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error actualizando usuario"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Usuario actualizado"})
	}
}

// elimina al usuario autenticado
func DeleteUser(users repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID != c.Param("id") {
			c.JSON(http.StatusForbidden, gin.H{"error": "No autorizado"})
			return
		}
		if err := users.Delete(userID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error eliminando usuario"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Usuario eliminado"})
	}
}

// helper para distinguir "no encontrado" de error real
func isSQLNoRows(err error) bool {
	return err == sql.ErrNoRows
}

var _ = http.StatusOK
